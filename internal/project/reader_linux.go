// Package project provides bounded, read-only access to one fixed project root.
package project

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"unicode/utf8"

	gitignore "github.com/denormal/go-gitignore"
	"golang.org/x/sys/unix"
)

// FileLimit bounds one returned file or directory listing, in bytes.
const FileLimit = 32 * 1024

// TaskLimit bounds the total content returned for one Agent task, in bytes.
const TaskLimit = 256 * 1024

// Reader owns a directory descriptor for a fixed, canonical project root.
// Call Close after the last task; closing concurrently with reads is unsupported.
type Reader struct {
	Root      string
	fd        int
	closeOnce sync.Once
	closeErr  error
}

// Task tracks one task's read budget. A Task must not be shared by goroutines.
type Task struct {
	reader *Reader
	used   int
}

// Data is bounded text returned to the model, with an explicit truncation flag.
type Data struct {
	Path      string `json:"path"`
	Content   string `json:"content"`
	Truncated bool   `json:"truncated"`
}

// New fixes the reading scope to the nearest Git root, or start without Git.
func New(start string) (*Reader, error) {
	root, err := filepath.Abs(start)
	if err != nil {
		return nil, err
	}
	root, err = filepath.EvalSymlinks(root)
	if err != nil {
		return nil, err
	}
	for dir := root; ; dir = filepath.Dir(dir) {
		if _, err := os.Stat(filepath.Join(dir, ".git")); err == nil {
			root = dir
			break
		}
		if filepath.Dir(dir) == dir {
			break
		}
	}
	fd, err := unix.Open(root, unix.O_RDONLY|unix.O_DIRECTORY|unix.O_CLOEXEC, 0)
	if err != nil {
		return nil, err
	}
	return &Reader{Root: root, fd: fd}, nil
}

// Close releases the root descriptor. Repeated calls return the first result.
func (r *Reader) Close() error {
	r.closeOnce.Do(func() { r.closeErr = unix.Close(r.fd) })
	return r.closeErr
}

// NewTask creates an independent reading budget for an Agent task.
func (r *Reader) NewTask() *Task { return &Task{reader: r} }

func excluded(rel string) bool {
	for _, part := range strings.Split(filepath.ToSlash(rel), "/") {
		lower := strings.ToLower(part)
		switch lower {
		case ".git", "node_modules", "vendor", "dist", "build", "target", ".next", ".venv", "venv", "__pycache__", ".cache", ".ssh", ".aws", ".gnupg", ".kube", ".docker", ".azure", ".codex", ".agents", "gcloud", "credentials", "secrets":
			return true
		case "id_rsa", "id_dsa", "id_ecdsa", "id_ed25519", "credentials.json", "credentials.yml", "credentials.yaml", ".netrc", ".npmrc", ".pypirc", "auth.json":
			return true
		}
		if strings.HasPrefix(lower, ".env") {
			return true
		}
		for _, ext := range []string{".pem", ".key", ".p12", ".pfx", ".jks"} {
			if strings.HasSuffix(lower, ext) {
				return true
			}
		}
	}
	return false
}

// Open the canonical path without permitting symlinks to be substituted between
// validation and open. openat2 also enforces the root boundary in the kernel.
func (r *Reader) open(rel string) (*os.File, error) {
	fd, err := unix.Openat2(r.fd, rel, &unix.OpenHow{Flags: unix.O_RDONLY | unix.O_CLOEXEC | unix.O_NONBLOCK, Resolve: unix.RESOLVE_BENEATH | unix.RESOLVE_NO_SYMLINKS})
	if err != nil {
		return nil, fmt.Errorf("cannot safely open project path: %w", err)
	}
	return os.NewFile(uintptr(fd), rel), nil
}

func (r *Reader) checked(path string) (string, error) {
	if filepath.IsAbs(path) {
		return "", errors.New("use a project-relative path")
	}
	for _, part := range strings.Split(filepath.ToSlash(path), "/") {
		if part == ".." {
			return "", errors.New("directory traversal is not allowed")
		}
	}
	rel := filepath.Clean(path)
	if excluded(rel) {
		return "", errors.New("path is excluded from automatic reading")
	}
	absolute, err := filepath.EvalSymlinks(filepath.Join(r.Root, rel))
	if err != nil {
		return "", err
	}
	real, err := filepath.Rel(r.Root, absolute)
	if err != nil || real == ".." || strings.HasPrefix(real, "../") {
		return "", errors.New("symlink leaves the project root")
	}
	if excluded(real) {
		return "", errors.New("symlink targets an excluded path")
	}
	for _, candidate := range []string{rel, real} {
		if err := r.checkIgnore(candidate); err != nil {
			return "", err
		}
	}
	return real, nil
}

func (r *Reader) checkIgnore(rel string) error {
	if rel == "." {
		return nil
	}
	parts := strings.Split(rel, string(filepath.Separator))
	var ignores []gitignore.GitIgnore
	for i := range parts {
		parent := filepath.Join(parts[:i]...)
		if parent == "" {
			parent = "."
		}
		ignorePath := filepath.Join(parent, ".gitignore")
		f, err := r.open(ignorePath)
		if err == nil {
			info, e := f.Stat()
			if e != nil || !info.Mode().IsRegular() {
				f.Close()
				return errors.New("unsafe .gitignore")
			}
			data, e := io.ReadAll(io.LimitReader(f, FileLimit+1))
			f.Close()
			if e != nil || len(data) > FileLimit {
				return errors.New(".gitignore exceeds safe reading limit")
			}
			var parseErr bool
			matcher := gitignore.New(bytes.NewReader(data), filepath.Join(r.Root, parent), func(gitignore.Error) bool { parseErr = true; return false })
			if parseErr {
				return errors.New("invalid .gitignore")
			}
			ignores = append(ignores, matcher)
		} else if !errors.Is(err, unix.ENOENT) {
			return err
		}
		candidate := filepath.Join(r.Root, filepath.Join(parts[:i+1]...))
		isDir := i < len(parts)-1
		if !isDir {
			st, e := os.Stat(candidate)
			if e != nil {
				return e
			}
			isDir = st.IsDir()
		}
		ignored := false
		for _, matcher := range ignores {
			if m := matcher.Absolute(candidate, isDir); m != nil {
				ignored = m.Ignore()
			}
		}
		if ignored {
			return errors.New("path is excluded by .gitignore")
		}
	}
	return nil
}

func (t *Task) finish(path, content string, truncated bool) (Data, error) {
	if len(content) > FileLimit {
		content = content[:FileLimit]
		truncated = true
	}
	if len(content) > TaskLimit-t.used {
		return Data{}, errors.New("project reading budget (256 KiB) exhausted")
	}
	t.used += len(content)
	// A byte limit may bisect a UTF-8 character. Trim only that partial suffix.
	for !utf8.ValidString(content) && len(content) > 0 {
		content = content[:len(content)-1]
	}
	return Data{Path: path, Content: content, Truncated: truncated}, nil
}

// ReadFile reads an allowed UTF-8 regular file relative to the fixed root.
func (t *Task) ReadFile(path string) (Data, error) {
	rel, err := t.reader.checked(path)
	if err != nil {
		return Data{}, err
	}
	f, err := t.reader.open(rel)
	if err != nil {
		return Data{}, err
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return Data{}, err
	}
	if !info.Mode().IsRegular() {
		return Data{}, errors.New("only regular text files can be read")
	}
	b, err := io.ReadAll(io.LimitReader(f, FileLimit+1))
	if err != nil {
		return Data{}, err
	}
	truncated := len(b) > FileLimit
	if truncated {
		b = b[:FileLimit]
	}
	// Partial final UTF-8 rune is allowed at the truncation boundary only.
	if truncated {
		for len(b) > 0 && !utf8.Valid(b) && len(b) > FileLimit-4 {
			b = b[:len(b)-1]
		}
	}
	if bytes.ContainsRune(b, 0) || !utf8.Valid(b) {
		return Data{}, errors.New("file is not UTF-8 text")
	}
	if bytes.Contains(b, []byte("PRIVATE KEY-----")) {
		return Data{}, errors.New("private key content is excluded")
	}
	return t.finish(path, string(b), truncated)
}

// ListFiles lists at most 256 allowed entries in a project-relative directory.
func (t *Task) ListFiles(path string) (Data, error) {
	rel, err := t.reader.checked(path)
	if err != nil {
		return Data{}, err
	}
	f, err := t.reader.open(rel)
	if err != nil {
		return Data{}, err
	}
	defer f.Close()
	entries, err := f.ReadDir(4096)
	if err != nil && err != io.EOF {
		return Data{}, err
	}
	var result strings.Builder
	count := 0
	truncated := len(entries) == 4096
	for _, entry := range entries {
		candidate := filepath.Join(rel, entry.Name())
		if _, err := t.reader.checked(candidate); err != nil {
			continue
		}
		if count >= 256 || result.Len()+len(entry.Name())+2 > FileLimit {
			truncated = true
			break
		}
		result.WriteString(entry.Name())
		if entry.IsDir() {
			result.WriteByte('/')
		}
		result.WriteByte('\n')
		count++
	}
	return t.finish(path, result.String(), truncated)
}
