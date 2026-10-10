package project

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func fixture(t *testing.T) *Reader {
	t.Helper()
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, ".git"), 0700); err != nil {
		t.Fatal(err)
	}
	r, err := New(root)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { r.Close() })
	return r
}
func put(t *testing.T, r *Reader, path, content string) {
	t.Helper()
	name := filepath.Join(r.Root, path)
	if err := os.MkdirAll(filepath.Dir(name), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(name, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}
}
func TestScopeAndExclusions(t *testing.T) {
	r := fixture(t)
	put(t, r, "src/main.go", "package main\n")
	put(t, r, ".env.local", "secret")
	put(t, r, ".ssh/config", "secret")
	put(t, r, "node_modules/a.js", "dependency")
	put(t, r, "id_rsa", "secret")
	put(t, r, "cert.pem", "secret")
	put(t, r, "binary", string([]byte{0, 1, 2}))
	put(t, r, "key.txt", "-----BEGIN PRIVATE KEY-----")
	outside := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(outside, []byte("outside"), 0600); err != nil {
		t.Fatal(err)
	}
	for name, target := range map[string]string{"outside": outside, "env-alias": filepath.Join(r.Root, ".env.local"), "safe-link": filepath.Join(r.Root, "src/main.go")} {
		if err := os.Symlink(target, filepath.Join(r.Root, name)); err != nil {
			t.Fatal(err)
		}
	}
	task := r.NewTask()
	for _, path := range []string{"../file", outside, "src/../../file", ".git/config", ".env.local", ".ssh/config", "node_modules/a.js", "id_rsa", "cert.pem", "outside", "env-alias", "binary", "key.txt"} {
		if _, err := task.ReadFile(path); err == nil {
			t.Errorf("allowed %q", path)
		}
	}
	for _, path := range []string{"src/main.go", "safe-link"} {
		data, err := task.ReadFile(path)
		if err != nil || data.Content != "package main\n" {
			t.Errorf("safe read %s: %+v %v", path, data, err)
		}
	}
	data, err := task.ListFiles(".")
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{".env.local", ".ssh", ".git", "node_modules", "outside", "env-alias"} {
		if strings.Contains(data.Content, name) {
			t.Errorf("listing leaked %q: %s", name, data.Content)
		}
	}
	nested, err := New(filepath.Join(r.Root, "src"))
	if err != nil {
		t.Fatal(err)
	}
	defer nested.Close()
	if nested.Root != r.Root {
		t.Fatal("nearest Git root not found")
	}
}
func TestGitignoreAndSymlinkTarget(t *testing.T) {
	r := fixture(t)
	put(t, r, ".gitignore", "*.log\nignored/\n!keep.log\n")
	put(t, r, "error.log", "hidden")
	put(t, r, "keep.log", "visible")
	put(t, r, "ignored/child.txt", "hidden")
	put(t, r, "src/.gitignore", "hidden.txt\n!allowed.log\n")
	put(t, r, "src/hidden.txt", "hidden")
	put(t, r, "src/allowed.log", "visible")
	put(t, r, "src/other.log", "hidden")
	if err := os.Symlink(filepath.Join(r.Root, "error.log"), filepath.Join(r.Root, "alias.txt")); err != nil {
		t.Fatal(err)
	}
	task := r.NewTask()
	for _, path := range []string{"error.log", "ignored/child.txt", "src/hidden.txt", "src/other.log", "alias.txt"} {
		if _, err := task.ReadFile(path); err == nil {
			t.Errorf("ignored path allowed: %s", path)
		}
	}
	for _, path := range []string{"keep.log", "src/allowed.log"} {
		if _, err := task.ReadFile(path); err != nil {
			t.Errorf("negated path denied: %s: %v", path, err)
		}
	}
	// Loading a linked .gitignore must fail closed instead of reading outside.
	if err := os.Remove(filepath.Join(r.Root, "src/.gitignore")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(t.TempDir(), "outside"), filepath.Join(r.Root, "src/.gitignore")); err != nil {
		t.Fatal(err)
	}
	if _, err := task.ReadFile("src/allowed.log"); err == nil {
		t.Fatal("unsafe .gitignore allowed")
	}
}
func TestBudgetsAndSpecialFiles(t *testing.T) {
	r := fixture(t)
	put(t, r, "large.txt", strings.Repeat("a", FileLimit+20))
	task := r.NewTask()
	for i := 0; i < 8; i++ {
		data, err := task.ReadFile("large.txt")
		if err != nil {
			t.Fatal(err)
		}
		if len(data.Content) != FileLimit || !data.Truncated {
			t.Fatal("file limit missing")
		}
	}
	if _, err := task.ReadFile("large.txt"); err == nil {
		t.Fatal("task budget not enforced")
	}
	if _, err := r.NewTask().ReadFile("large.txt"); err != nil {
		t.Fatal("task budget did not reset")
	}
	put(t, r, "utf8.txt", strings.Repeat("中", FileLimit/3+5))
	if data, err := r.NewTask().ReadFile("utf8.txt"); err != nil || !data.Truncated {
		t.Fatalf("UTF-8 boundary: %+v %v", data, err)
	}
}
