package project

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"golang.org/x/sys/unix"
)

func TestSearchAndRange(t *testing.T) {
	r := fixture(t)
	put(t, r, "z.txt", "Needle\nneedle twice needle\n")
	put(t, r, "src/nested/main.go", "package main\n\nfunc needle() {}\nlast")
	task := r.NewTask()
	data, err := task.SearchText(context.Background(), ".", "needle")
	if err != nil {
		t.Fatal(err)
	}
	want := []Match{{Path: "src/nested/main.go", Line: 3, Text: "func needle() {}"}, {Path: "z.txt", Line: 2, Text: "needle twice needle"}}
	if !reflect.DeepEqual(data.Matches, want) || data.Truncated || data.ScannedBytes == 0 {
		t.Fatalf("literal ordered search: %+v", data)
	}
	for _, c := range []struct {
		start, count, first, last int
		text                      string
	}{
		{3, 1, 3, 3, "func needle() {}\n"},
		{3, 0, 3, 4, "func needle() {}\nlast"},
		{99, 100, 0, 0, ""},
	} {
		got, err := task.ReadFileRange(context.Background(), "src/nested/main.go", c.start, c.count)
		if err != nil || got.StartLine != c.first || got.EndLine != c.last || got.Content != c.text || got.Truncated {
			t.Errorf("range %+v: %+v / %v", c, got, err)
		}
	}
	for _, c := range [][2]int{{0, 1}, {-1, 1}, {1, -1}, {1, 201}} {
		if _, err := task.ReadFileRange(context.Background(), "z.txt", c[0], c[1]); err == nil {
			t.Errorf("accepted invalid range %v", c)
		}
	}
	if _, err := task.SearchText(context.Background(), ".", ""); err == nil {
		t.Fatal("accepted empty query")
	}
	if _, err := task.SearchText(context.Background(), "z.txt", "needle"); err == nil {
		t.Fatal("searched non-directory")
	}
}

// TestSearchBoundaries checks both aliases and targets, nested ignores,
// non-text/key content and special files without blocking on FIFO opens.
func TestSearchBoundaries(t *testing.T) {
	r := fixture(t)
	put(t, r, ".gitignore", "ignored/\n*.log\n!keep.log\n")
	for _, path := range []string{".env.local", "ignored/a", "other.log", "src/hidden", ".ssh/key", "node_modules/x"} {
		put(t, r, path, "needle secret")
	}
	put(t, r, "src/.gitignore", "hidden\n")
	put(t, r, "keep.log", "needle allowed")
	put(t, r, "key.txt", "needle\n-----BEGIN PRIVATE KEY-----\n")
	put(t, r, "binary", "needle\x00")
	put(t, r, "invalid", "needle\xff")
	put(t, r, "nested/visible", "needle visible\n")
	outside := filepath.Join(t.TempDir(), "external")
	if err := os.WriteFile(outside, []byte("needle outside"), 0600); err != nil {
		t.Fatal(err)
	}
	for name, target := range map[string]string{"escape": outside, "ignored-link": "other.log", "dir-link": "nested", "safe-link": "nested/visible"} {
		if err := os.Symlink(target, filepath.Join(r.Root, name)); err != nil {
			t.Fatal(err)
		}
	}
	if err := unix.Mkfifo(filepath.Join(r.Root, "fifo"), 0600); err != nil {
		t.Fatal(err)
	}
	data, err := r.NewTask().SearchText(context.Background(), ".", "needle")
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"keep.log", "nested/visible", "safe-link"}
	var paths []string
	for _, m := range data.Matches {
		paths = append(paths, m.Path)
	}
	if !reflect.DeepEqual(paths, want) || data.SkippedEntries == 0 {
		t.Fatalf("boundary failure: %+v", data)
	}
	for _, path := range []string{"../outside", outside, ".env.local", "escape", "ignored-link", "key.txt", "binary", "invalid", "fifo", "src/hidden"} {
		if _, err := r.NewTask().ReadFileRange(context.Background(), path, 1, 100); err == nil {
			t.Errorf("range allowed %q", path)
		}
	}
	if _, err := r.NewTask().SearchText(context.Background(), "dir-link", "needle"); err == nil {
		t.Fatal("followed directory link as search root")
	}
	// A private-key header before the selected slice still excludes the file.
	if _, err := r.NewTask().ReadFileRange(context.Background(), "key.txt", 3, 1); err == nil {
		t.Fatal("private key escaped via range selection")
	}
}

func TestSearchLimitsAndSharedBudgets(t *testing.T) {
	r := fixture(t)
	put(t, r, "matches", strings.Repeat("needle\n", 120))
	put(t, r, "oversized", strings.Repeat("x", SearchFileLimit+1))
	task := r.NewTask()
	got, err := task.SearchText(context.Background(), ".", "needle")
	if err != nil || len(got.Matches) != 100 || !got.Truncated || !hasLimit(got.Limits, "match_limit") {
		t.Fatalf("match cap: %+v / %v", got, err)
	}
	got, err = r.NewTask().SearchText(context.Background(), ".", "absent")
	if err != nil || got.SkippedLarge != 1 || !hasLimit(got.Limits, "large_files_skipped") {
		t.Fatalf("large file skip: %+v / %v", got, err)
	}
	put(t, r, "long", "needle"+strings.Repeat("中", FileLimit))
	got, err = r.NewTask().SearchText(context.Background(), ".", "needle"+strings.Repeat("中", 10))
	if err != nil || !hasLimit(got.Limits, "return_limit") {
		t.Fatalf("search byte cap: %+v / %v", got, err)
	}
	rangeData, err := r.NewTask().ReadFileRange(context.Background(), "long", 1, 1)
	encoded, _ := json.Marshal(rangeData)
	if err != nil || !rangeData.Truncated || len(encoded) > FileLimit || rangeData.StartLine != 1 || rangeData.EndLine != 1 {
		t.Fatalf("range byte cap: %d %+v / %v", len(encoded), rangeData, err)
	}
	put(t, r, "legacy", strings.Repeat("x", FileLimit))
	task = r.NewTask()
	for i := 0; i < 8; i++ {
		if _, err := task.ReadFile("legacy"); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := task.SearchText(context.Background(), ".", "missing"); err == nil {
		t.Fatal("search ignored legacy return budget")
	}
	if _, err := task.ReadFileRange(context.Background(), "legacy", 1, 1); err == nil {
		t.Fatal("range ignored legacy return budget")
	}
}

func hasLimit(limits []string, name string) bool {
	for _, limit := range limits {
		if limit == name {
			return true
		}
	}
	return false
}

// TestScanAndEntryCaps uses actual files to exercise per-call and per-task
// scan accounting; rejected files also consume the bytes that were inspected.
func TestScanAndEntryCaps(t *testing.T) {
	r := fixture(t)
	for i := 0; i < 10; i++ {
		put(t, r, fmt.Sprintf("scan/%02d.txt", i), strings.Repeat("x", SearchFileLimit-1))
	}
	task := r.NewTask()
	for i := 0; i < 2; i++ {
		got, err := task.SearchText(context.Background(), "scan", "absent")
		if err != nil || got.ScannedBytes != ScanCallLimit || !got.Truncated || !hasLimit(got.Limits, "call_scan_limit") {
			t.Fatalf("scan cap %d: %+v / %v", i, got, err)
		}
	}
	if task.scanned != ScanTaskLimit {
		t.Fatalf("shared scan accounting: %d", task.scanned)
	}
	if _, err := task.ReadFileRange(context.Background(), "scan/00.txt", 1, 1); err == nil {
		t.Fatal("range did not share exhausted scan budget")
	}
	if _, err := r.NewTask().ReadFileRange(context.Background(), "scan/00.txt", 1, 1); err != nil {
		t.Fatal("scan budget did not reset")
	}
	put(t, r, "huge", strings.Repeat("x", ScanCallLimit+1))
	ranged, err := r.NewTask().ReadFileRange(context.Background(), "huge", 999, 1)
	if err != nil || !ranged.Truncated || ranged.Content != "" || ranged.ScannedBytes != ScanCallLimit {
		t.Fatalf("range scan cap: %+v / %v", ranged, err)
	}
	for i := 0; i < 4100; i++ {
		put(t, r, fmt.Sprintf("entries/%04d", i), "")
	}
	got, err := r.NewTask().SearchText(context.Background(), "entries", "absent")
	if err != nil || got.CheckedEntries != 4096 || !hasLimit(got.Limits, "directory_entry_limit") {
		t.Fatalf("entry cap: %+v / %v", got, err)
	}
}

func TestSearchAndRangeCancellation(t *testing.T) {
	r := fixture(t)
	put(t, r, "file", "needle\n")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	task := r.NewTask()
	if _, err := task.SearchText(ctx, ".", "needle"); !errors.Is(err, context.Canceled) {
		t.Fatalf("search cancellation: %v", err)
	}
	if _, err := task.ReadFileRange(ctx, "file", 1, 1); !errors.Is(err, context.Canceled) {
		t.Fatalf("range cancellation: %v", err)
	}
	if task.scanned != 0 || task.used != 0 {
		t.Fatal("canceled reads consumed budgets")
	}
}

// cancelAfterChecks cancels deterministically during chunked IO, avoiding a
// timing-dependent race between a fast read and a sleeping goroutine.
type cancelAfterChecks struct {
	context.Context
	cancel context.CancelFunc
	checks int
}

func (c *cancelAfterChecks) Err() error {
	c.checks++
	if c.checks == 8 {
		c.cancel()
	}
	return c.Context.Err()
}

func TestCancellationDuringScan(t *testing.T) {
	r := fixture(t)
	put(t, r, "file", strings.Repeat("x", SearchFileLimit-1))
	for _, search := range []bool{true, false} {
		base, cancel := context.WithCancel(context.Background())
		ctx := &cancelAfterChecks{Context: base, cancel: cancel}
		task := r.NewTask()
		var err error
		if search {
			_, err = task.SearchText(ctx, ".", "x")
		} else {
			_, err = task.ReadFileRange(ctx, "file", 1, 1)
		}
		cancel()
		if !errors.Is(err, context.Canceled) || task.scanned == 0 || task.scanned >= SearchFileLimit || task.used != 0 {
			t.Fatalf("mid-scan cancellation search=%v: %d/%d %v", search, task.scanned, task.used, err)
		}
	}
}
