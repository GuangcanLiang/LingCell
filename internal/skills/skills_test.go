package skills

import (
	"context"
	"encoding/json"
	"golang.org/x/sys/unix"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"lingcell/internal/project"
)

func write(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}
}

func setup(t *testing.T) (string, *project.Task) {
	t.Helper()
	root := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	reader, err := project.New(root)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { reader.Close() })
	return root, reader.NewTask()
}

// TestDiscoveryAndReferences checks independent library identities, explicit
// external roots, exact scope despite Git ancestors, and lazy document reads.
func TestDiscoveryAndReferences(t *testing.T) {
	root, budget := setup(t)
	extra := filepath.Join(t.TempDir(), "library")
	write(t, filepath.Join(filepath.Dir(extra), ".git", "marker"), "")
	for _, dir := range []string{filepath.Join(root, ".lingcell", "skills"), filepath.Join(os.Getenv("XDG_CONFIG_HOME"), "lingcell", "skills"), extra} {
		write(t, filepath.Join(dir, "review", "SKILL.md"), "---\nname: review\ndescription: Review code\n---\nUse references/check.md\n")
		write(t, filepath.Join(dir, "review", "references", "check.md"), "check evidence")
	}
	write(t, filepath.Join(extra, "incomplete", "notes.md"), "not a skill")
	task := NewTask(root, []string{extra}, budget)
	defer task.Close()
	catalog, err := task.List(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(catalog.Skills) != 3 || catalog.Truncated || len(catalog.Issues) != 0 {
		t.Fatalf("catalog: %+v", catalog)
	}
	raw, _ := json.Marshal(catalog)
	if strings.Contains(string(raw), "Use references") {
		t.Fatal("discovery uploaded body")
	}
	for _, source := range []string{"project", "user", "extra-1"} {
		data, err := task.Read(context.Background(), source, "review", "")
		if err != nil || !strings.Contains(data.Content, "description: Review code") {
			t.Fatalf("read: %+v %v", data, err)
		}
		data, err = task.Read(context.Background(), source, "review", "references/check.md")
		if err != nil || data.Content != "check evidence" {
			t.Fatalf("reference: %+v %v", data, err)
		}
	}
}

// TestBoundaries rejects supporting-file escapes, unsafe content, ignored paths,
// FIFO files and symlinks without treating configured roots as arbitrary file access.
func TestBoundaries(t *testing.T) {
	root, budget := setup(t)
	base := filepath.Join(root, ".lingcell", "skills", "review")
	write(t, filepath.Join(base, "SKILL.md"), "review")
	write(t, filepath.Join(base, ".env.local"), "secret")
	write(t, filepath.Join(base, "key.txt"), "-----BEGIN PRIVATE KEY-----")
	write(t, filepath.Join(base, "binary.txt"), "abc\x00def")
	write(t, filepath.Join(base, "ignored.txt"), "ignored")
	write(t, filepath.Join(root, ".gitignore"), ".lingcell/skills/review/ignored.txt\n")
	if err := os.Symlink("SKILL.md", filepath.Join(base, "link.md")); err != nil {
		t.Fatal(err)
	}
	if err := unix.Mkfifo(filepath.Join(base, "fifo"), 0600); err != nil {
		t.Fatal(err)
	}
	task := NewTask(root, nil, budget)
	defer task.Close()
	for _, path := range []string{"../review/SKILL.md", "/etc/passwd", ".env.local", "key.txt", "binary.txt", "ignored.txt", "link.md", "fifo", "."} {
		if _, err := task.Read(context.Background(), "project", "review", path); err == nil {
			t.Errorf("accepted %q", path)
		}
	}
	if _, err := task.Read(context.Background(), "project", "../review", ""); err == nil {
		t.Fatal("accepted name traversal")
	}
	if _, err := task.Read(context.Background(), "unknown", "review", ""); err == nil {
		t.Fatal("accepted source")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := task.List(ctx); err != context.Canceled {
		t.Fatalf("cancel list: %v", err)
	}
	if _, err := task.Read(ctx, "project", "review", ""); err != context.Canceled {
		t.Fatalf("cancel read: %v", err)
	}
}

// TestEncodedAndSharedBudget proves JSON escape overhead is bounded and skill
// reads compete with ordinary project reads for the same task's return budget.
func TestEncodedAndSharedBudget(t *testing.T) {
	root, budget := setup(t)
	write(t, filepath.Join(root, ".lingcell", "skills", "large", "SKILL.md"), strings.Repeat("\t", project.FileLimit+1))
	write(t, filepath.Join(root, "large.txt"), strings.Repeat("x", project.FileLimit))
	task := NewTask(root, nil, budget)
	defer task.Close()
	data, err := task.Read(context.Background(), "project", "large", "")
	if err != nil {
		t.Fatal(err)
	}
	encoded, _ := json.Marshal(data)
	if !data.Truncated || len(encoded) > project.FileLimit || len(data.Content) == 0 {
		t.Fatalf("encoded %d, data %+v", len(encoded), data)
	}
	for i := 0; i < 7; i++ {
		if _, err := budget.ReadFile("large.txt"); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := task.Read(context.Background(), "project", "large", ""); err == nil {
		t.Fatal("skill read bypassed shared budget")
	}
}

func TestMissingAndUnsafeLibraries(t *testing.T) {
	root, budget := setup(t)
	target := t.TempDir()
	write(t, filepath.Join(target, "review", "SKILL.md"), "external")
	if err := os.MkdirAll(filepath.Join(root, ".lingcell"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, filepath.Join(root, ".lingcell", "skills")); err != nil {
		t.Fatal(err)
	}
	task := NewTask(root, nil, budget)
	defer task.Close()
	result, err := task.List(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Skills) != 0 || len(result.Issues) == 0 {
		t.Fatalf("unsafe root: %+v", result)
	}
}
