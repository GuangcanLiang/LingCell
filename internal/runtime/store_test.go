package runtime

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func testStore(t *testing.T) *Store {
	t.Helper()
	s, err := Open(filepath.Join(t.TempDir(), "state", "runtime.db"), "credential-sentinel-exact")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return s
}
func TestMigrationPermissionsAndFutureSchema(t *testing.T) {
	s := testStore(t)
	var version int
	if err := s.db.QueryRow("PRAGMA user_version").Scan(&version); err != nil || version != 1 {
		t.Fatalf("migration %d %v", version, err)
	}
	for _, p := range []string{s.path, s.path + "-wal", s.path + "-shm"} {
		info, err := os.Stat(p)
		if err != nil {
			t.Fatal(err)
		}
		if info.Mode().Perm() != 0600 {
			t.Fatalf("permissions %s %o", p, info.Mode().Perm())
		}
	}
	info, err := os.Stat(filepath.Dir(s.path))
	if err != nil || info.Mode().Perm() != 0700 {
		t.Fatalf("directory %v %v", info, err)
	}
	if _, err = s.db.Exec("PRAGMA user_version=99"); err != nil {
		t.Fatal(err)
	}
	other, err := Open(s.path)
	if err == nil {
		other.Close()
		t.Fatal("future schema accepted")
	}
	if _, err = s.db.Exec("PRAGMA user_version=1"); err != nil {
		t.Fatal(err)
	}
	if err = s.initialize(); err != nil {
		t.Fatal("idempotent migration", err)
	}
}
func TestStorageLinksCorruptionAndWriteFault(t *testing.T) {
	path := filepath.Join(t.TempDir(), "runtime.db")
	if err := os.WriteFile(path, []byte("not sqlite"), 0600); err != nil {
		t.Fatal(err)
	}
	if s, err := Open(path); err == nil {
		s.Close()
		t.Fatal("corrupt database accepted")
	}
	link := filepath.Join(t.TempDir(), "runtime.db")
	if err := os.Symlink(path, link); err != nil {
		t.Fatal(err)
	}
	if s, err := Open(link); err == nil {
		s.Close()
		t.Fatal("link accepted")
	}
	s := testStore(t)
	id, err := newID()
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.db.Exec("INSERT INTO sessions VALUES(?,?,?)", id, "/project", timestamp()); err != nil {
		t.Fatal(err)
	}
	if _, err = s.db.Exec("PRAGMA query_only=ON"); err != nil {
		t.Fatal(err)
	}
	if err = s.saveTurn(id, Turn{Number: 1, Status: "running"}); err == nil {
		t.Fatal("readonly write succeeded")
	}
}
func TestSessionLockAndArchiveBound(t *testing.T) {
	s := testStore(t)
	id, err := newID()
	if err != nil {
		t.Fatal(err)
	}
	f, err := s.lock(id)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if other, err := s.lock(id); err == nil {
		other.Close()
		t.Fatal("duplicate session lock acquired")
	}
	if _, err = s.db.Exec("INSERT INTO sessions VALUES(?,?,?)", id, "/project", timestamp()); err != nil {
		t.Fatal(err)
	}
	t1 := Turn{Number: 1, Status: "completed", Steps: []Step{{ID: "call", Arguments: `{"command":"echo credential-sentinel-exact"}`, Result: "api_key=test-sensitive"}}}
	if err = s.saveTurn(id, t1); err != nil {
		t.Fatal(err)
	}
	turns, err := s.turns(id)
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := json.Marshal(turns)
	if strings.Contains(string(raw), "credential-sentinel-exact") || strings.Contains(string(raw), "test-sensitive") {
		t.Fatal("credentials retained")
	}
	if !strings.Contains(t1.Steps[0].Arguments, "credential-sentinel-exact") {
		t.Fatal("redaction mutated executable arguments")
	}
	t1.Steps[0].Result = strings.Repeat("x", maxTurnBytes)
	if err = s.saveTurn(id, t1); err == nil {
		t.Fatal("unbounded archive")
	}
	if err = s.DeleteSession(id, "/project"); err == nil {
		t.Fatal("deleted locked session")
	}
}

func TestRedactionPreservesOrdinaryConstraints(t *testing.T) {
	text := "Our task-specific code is ORCHID42; please keep it"
	if got := Redact(text); got != text {
		t.Fatalf("ordinary constraint redacted: %q", got)
	}
	if got := Redact("credential sk-abcdefgh12345"); strings.Contains(got, "sk-abcdefgh") {
		t.Fatal("recognizable credential retained")
	}
}

func TestDiskFullRejectsArchiveWrite(t *testing.T) {
	s := testStore(t)
	id, err := newID()
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.db.Exec("INSERT INTO sessions VALUES(?,?,?)", id, "/project", timestamp()); err != nil {
		t.Fatal(err)
	}
	if err = s.saveTurn(id, Turn{Number: 1, Status: "running"}); err != nil {
		t.Fatal(err)
	}
	var pages int
	if err = s.db.QueryRow("PRAGMA page_count").Scan(&pages); err != nil {
		t.Fatal(err)
	}
	if _, err = s.db.Exec(fmt.Sprintf("PRAGMA max_page_count=%d", pages)); err != nil {
		t.Fatal(err)
	}
	large := Turn{Number: 1, Status: "completed", Steps: []Step{{ID: "large", Result: strings.Repeat("x", 128*1024)}}}
	if err = s.saveTurn(id, large); err == nil {
		t.Fatal("disk-full write accepted")
	}
	records, err := s.turns(id)
	if err != nil || len(records) != 1 || records[0].Status != "running" || len(records[0].Steps) != 0 {
		t.Fatal("failed transaction changed original record", err)
	}
}
