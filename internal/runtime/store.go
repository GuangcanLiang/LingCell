// Package runtime owns bounded conversation records, context and local memory.
// A Runtime is used serially; its session lock lasts until Close or Switch.
package runtime

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"golang.org/x/sys/unix"
	_ "modernc.org/sqlite"

	"lingcell/internal/model"
)

const maxTurnBytes = 2 * 1024 * 1024
const maxSessionBytes = 64 * 1024 * 1024

// Session identifies a conversation bound to one canonical project root.
type Session struct {
	ID      string `json:"id"`
	Root    string `json:"root"`
	Created int64  `json:"created"`
}

// Step records approval and execution independently of a tool message.
type Step struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
	Status    string `json:"status"`
	Approved  bool   `json:"approved"`
	Result    string `json:"result,omitempty"`
}

// Turn is a bounded archive of one request, including failed and interrupted work.
type Turn struct {
	Number   int             `json:"number"`
	Status   string          `json:"status"`
	Messages []model.Message `json:"messages"`
	Steps    []Step          `json:"steps"`
	Created  int64           `json:"created"`
}

// Store owns the SQLite connection. Session locks are managed by Runtime.
type Store struct {
	db      *sql.DB
	path    string
	secrets []string
}

// DefaultPath respects XDG_STATE_HOME, falling back to the user's local state.
func DefaultPath() string {
	d := os.Getenv("XDG_STATE_HOME")
	if d == "" {
		h, _ := os.UserHomeDir()
		d = filepath.Join(h, ".local", "state")
	}
	return filepath.Join(d, "lingcell", "runtime.db")
}

// Open creates private storage and performs a transactional versioned migration.
// It rejects links and unsupported schema versions rather than repairing data.
func Open(path string, secrets ...string) (*Store, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return nil, fmt.Errorf("create runtime directory: %w", err)
	}
	if err := os.Chmod(filepath.Dir(path), 0700); err != nil {
		return nil, err
	}
	for _, p := range []string{path, path + "-wal", path + "-shm"} {
		info, err := os.Lstat(p)
		if err == nil && (!info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0) {
			return nil, errors.New("runtime storage must be regular files")
		}
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			return nil, err
		}
		if err == nil {
			if err := os.Chmod(p, 0600); err != nil {
				return nil, err
			}
		}
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR|unix.O_NOFOLLOW, 0600)
	if err != nil {
		return nil, err
	}
	if err = f.Close(); err != nil {
		return nil, err
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	s := &Store{db: db, path: path, secrets: secrets}
	if err = s.initialize(); err != nil {
		db.Close()
		return nil, fmt.Errorf("initialize runtime storage: %w", err)
	}
	return s, nil
}
func (s *Store) initialize() error {
	for _, q := range []string{"PRAGMA busy_timeout=3000", "PRAGMA foreign_keys=ON", "PRAGMA journal_mode=WAL", "PRAGMA synchronous=FULL"} {
		if _, err := s.db.Exec(q); err != nil {
			return err
		}
	}
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var version int
	if err = tx.QueryRow("PRAGMA user_version").Scan(&version); err != nil {
		return err
	}
	if version > 1 {
		return errors.New("unsupported runtime schema version")
	}
	if version == 0 {
		_, err = tx.Exec(`
CREATE TABLE sessions(id TEXT PRIMARY KEY,root TEXT NOT NULL,created INTEGER NOT NULL);
CREATE TABLE turns(session TEXT NOT NULL REFERENCES sessions(id) ON DELETE CASCADE,number INTEGER NOT NULL,status TEXT NOT NULL,record TEXT NOT NULL,PRIMARY KEY(session,number));
CREATE TABLE summaries(session TEXT PRIMARY KEY REFERENCES sessions(id) ON DELETE CASCADE,through INTEGER NOT NULL,content TEXT NOT NULL);
CREATE TABLE memories(id TEXT PRIMARY KEY,scope TEXT NOT NULL,owner TEXT NOT NULL,kind TEXT NOT NULL,topic TEXT NOT NULL,content TEXT NOT NULL,source TEXT NOT NULL,updated INTEGER NOT NULL,expires INTEGER NOT NULL,version INTEGER NOT NULL,status TEXT NOT NULL);
CREATE TABLE memory_versions(id TEXT NOT NULL,version INTEGER NOT NULL,record TEXT NOT NULL,PRIMARY KEY(id,version));
CREATE VIRTUAL TABLE memory_fts USING fts5(id UNINDEXED,content,topic,tokenize='trigram');
CREATE TRIGGER memory_insert AFTER INSERT ON memories BEGIN INSERT INTO memory_fts(id,content,topic) VALUES(new.id,new.content,new.topic); END;
CREATE TRIGGER memory_update AFTER UPDATE ON memories BEGIN DELETE FROM memory_fts WHERE id=old.id; INSERT INTO memory_fts(id,content,topic) VALUES(new.id,new.content,new.topic); END;
CREATE TRIGGER memory_delete AFTER DELETE ON memories BEGIN DELETE FROM memory_fts WHERE id=old.id; END;
PRAGMA user_version=1;`)
		if err != nil {
			return err
		}
	}
	if err = tx.Commit(); err != nil {
		return err
	}
	for _, p := range []string{s.path, s.path + "-wal", s.path + "-shm"} {
		if err = os.Chmod(p, 0600); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
	}
	return nil
}

// Close releases database resources. Call after closing active runtimes.
func (s *Store) Close() error { return s.db.Close() }
func newID() (string, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(b[:]), nil
}

var sensitive = regexp.MustCompile(`(?i)(-----BEGIN [A-Z ]*PRIVATE KEY-----[\s\S]*|(?:\bsk-[A-Za-z0-9_-]{8,}|\bAKIA[A-Z0-9]{16})|(?:authorization\s*[:=]\s*bearer\s+\S+)|(?:api[_-]?key|password|secret|token)\s*[:=]\s*["']?[^\s"',;]+)`)

// Redact filters known credential values and recognizable sensitive patterns.
// It cannot identify arbitrary secrets. Execution arguments must stay unmodified.
func Redact(text string, secrets ...string) string {
	return redactText(text, secrets, 0)
}

var credentialField = regexp.MustCompile(`(?i)^(?:api[_-]?key|password|secret|token|authorization)$`)

func redactText(text string, secrets []string, depth int) string {
	for _, secret := range secrets {
		if secret != "" {
			text = strings.ReplaceAll(text, secret, "[REDACTED]")
		}
	}
	if depth < 16 {
		var value any
		decoder := json.NewDecoder(strings.NewReader(text))
		decoder.UseNumber()
		if decoder.Decode(&value) == nil {
			var extra any
			if decoder.Decode(&extra) == io.EOF {
				cleaned, changed := redactValue(value, secrets, depth+1)
				if !changed {
					return text
				}
				raw, err := json.Marshal(cleaned)
				if err == nil {
					return string(raw)
				}
			}
		}
	}
	return sensitive.ReplaceAllString(text, "[REDACTED]")
}
func redactValue(value any, secrets []string, depth int) (any, bool) {
	changed := false
	switch v := value.(type) {
	case map[string]any:
		for key, field := range v {
			if text, ok := field.(string); ok && text != "" && credentialField.MatchString(key) {
				v[key] = "[REDACTED]"
				changed = true
				continue
			}
			cleaned, different := redactValue(field, secrets, depth)
			if different {
				v[key] = cleaned
				changed = true
			}
		}
	case []any:
		for i, field := range v {
			cleaned, different := redactValue(field, secrets, depth)
			if different {
				v[i] = cleaned
				changed = true
			}
		}
	case string:
		text := redactText(v, secrets, depth)
		return text, text != v
	}
	return value, changed
}

func (s *Store) clean(text string) string { return Redact(text, s.secrets...) }
func (s *Store) saveTurn(id string, t Turn) error {
	// Copy before redaction: execution still uses the original validated arguments.
	raw, err := json.Marshal(t)
	if err != nil {
		return err
	}
	var cloned Turn
	if err = json.Unmarshal(raw, &cloned); err != nil {
		return err
	}
	t = cloned
	// Redact fields before encoding, so JSON escapes cannot conceal a known key.
	for i := range t.Messages {
		t.Messages[i].Content = s.clean(t.Messages[i].Content)
		for j := range t.Messages[i].ToolCalls {
			t.Messages[i].ToolCalls[j].Function.Arguments = s.clean(t.Messages[i].ToolCalls[j].Function.Arguments)
		}
	}
	for i := range t.Steps {
		t.Steps[i].Arguments = s.clean(t.Steps[i].Arguments)
		t.Steps[i].Result = s.clean(t.Steps[i].Result)
	}
	b, err := json.Marshal(t)
	if err != nil {
		return err
	}
	if len(b) > maxTurnBytes {
		return errors.New("turn archive exceeds 2 MiB")
	}
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var size int
	if err = tx.QueryRow("SELECT coalesce(sum(length(CAST(record AS BLOB))),0) FROM turns WHERE session=? AND number<>?", id, t.Number).Scan(&size); err != nil {
		return err
	}
	if size+len(b) > maxSessionBytes {
		return errors.New("session archive exceeds 64 MiB; start a new session")
	}
	if _, err = tx.Exec("INSERT INTO turns VALUES(?,?,?,?) ON CONFLICT(session,number) DO UPDATE SET status=excluded.status,record=excluded.record", id, t.Number, t.Status, string(b)); err != nil {
		return err
	}
	return tx.Commit()
}
func (s *Store) turns(id string) ([]Turn, error) {
	rows, err := s.db.Query("SELECT record FROM turns WHERE session=? ORDER BY number", id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []Turn
	for rows.Next() {
		var raw string
		var t Turn
		if err = rows.Scan(&raw); err != nil {
			return nil, err
		}
		if err = json.Unmarshal([]byte(raw), &t); err != nil {
			return nil, errors.New("corrupt turn record")
		}
		result = append(result, t)
	}
	return result, rows.Err()
}

// Sessions lists sessions for this fixed project root only.
func (s *Store) Sessions(root string) ([]Session, error) {
	rows, err := s.db.Query("SELECT id,root,created FROM sessions WHERE root=? ORDER BY created DESC LIMIT 100", root)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Session
	for rows.Next() {
		var v Session
		if err = rows.Scan(&v.ID, &v.Root, &v.Created); err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, rows.Err()
}
func (s *Store) lock(id string) (*os.File, error) {
	if len(id) != 32 {
		return nil, errors.New("invalid session ID")
	}
	if _, err := hex.DecodeString(id); err != nil {
		return nil, errors.New("invalid session ID")
	}
	f, err := os.OpenFile(s.path+"."+id+".lock", os.O_CREATE|os.O_RDWR|unix.O_NOFOLLOW, 0600)
	if err != nil {
		return nil, err
	}
	if err = f.Chmod(0600); err != nil {
		f.Close()
		return nil, err
	}
	if err = unix.Flock(int(f.Fd()), unix.LOCK_EX|unix.LOCK_NB); err != nil {
		f.Close()
		return nil, errors.New("session is already in use")
	}
	return f, nil
}

// DeleteSession requires the caller's explicit confirmation and an idle session.
// Lock files remain to avoid inode replacement races between processes.
func (s *Store) DeleteSession(id, root string) error {
	f, err := s.lock(id)
	if err != nil {
		return err
	}
	defer f.Close()
	res, err := s.db.Exec("DELETE FROM sessions WHERE id=? AND root=?", id, root)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err == nil && n == 0 {
		return errors.New("session not found in current project")
	}
	return err
}
func timestamp() int64 { return time.Now().Unix() }
