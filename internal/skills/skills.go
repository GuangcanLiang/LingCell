// Package skills provides bounded, read-only access to local skill libraries.
package skills

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"unicode/utf8"

	"lingcell/internal/project"
)

// Entry identifies a skill directory containing a regular SKILL.md file.
// Source disambiguates equal names without implicitly overriding a library.
type Entry struct {
	Source string `json:"source"`
	Name   string `json:"name"`
}

// Catalog lists available names without automatically uploading skill bodies.
// Issues and Truncated must be inspected before treating the list as complete.
type Catalog struct {
	Skills    []Entry  `json:"skills"`
	Truncated bool     `json:"truncated"`
	Issues    []string `json:"issues,omitempty"`
}

// Data contains a skill document or a file referenced by that skill.
type Data struct {
	Source    string `json:"source"`
	Name      string `json:"name"`
	Path      string `json:"path"`
	Content   string `json:"content"`
	Truncated bool   `json:"truncated"`
}

type library struct {
	source string
	reader *project.Reader
}

// Task pins libraries for one serial Agent task and shares its return budget
// with project tools. Call Close after use; missing roots are allowed.
type Task struct {
	libraries []library
	issues    []string
	budget    *project.Task
}

// NewTask opens the fixed project's .lingcell/skills, the user's config library,
// and up to 16 explicitly configured absolute directories. It never creates them.
// Project libraries must also pass the project's exclusions and ignore rules.
func NewTask(root string, extra []string, budget *project.Task) *Task {
	t := &Task{budget: budget}
	roots := []struct{ source, path string }{{"project", filepath.Join(root, ".lingcell", "skills")}}
	if dir, err := os.UserConfigDir(); err == nil {
		roots = append(roots, struct{ source, path string }{"user", filepath.Join(dir, "lingcell", "skills")})
	}
	for i, path := range extra {
		if i >= 16 {
			t.issues = append(t.issues, "extra library limit reached")
			break
		}
		roots = append(roots, struct{ source, path string }{fmt.Sprintf("extra-%d", i+1), path})
	}
	for _, location := range roots {
		if location.source == "project" {
			if err := budget.CheckDirectory(".lingcell/skills"); err != nil {
				if !errors.Is(err, os.ErrNotExist) {
					t.issues = append(t.issues, "project: library is unavailable or excluded")
				}
				continue
			}
		}
		r, err := project.NewScope(location.path)
		if err != nil {
			if !errors.Is(err, os.ErrNotExist) {
				t.issues = append(t.issues, location.source+": library is unavailable or unsafe")
			}
			continue
		}
		t.libraries = append(t.libraries, library{source: location.source, reader: r})
	}
	return t
}

// Close releases all library descriptors. It supports repeated calls.
func (t *Task) Close() error {
	var result error
	for _, lib := range t.libraries {
		result = errors.Join(result, lib.reader.Close())
	}
	return result
}

// ValidName accepts portable directory names, without separators or traversal.
func ValidName(name string) bool {
	if len(name) == 0 || len(name) > 64 {
		return false
	}
	for _, c := range name {
		if !(c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c == '-' || c == '_') {
			return false
		}
	}
	return true
}

// List returns at most 256 sorted names. Each library examines at most 4096
// entries through the project reader; no skill body is read for discovery.
func (t *Task) List(ctx context.Context) (Catalog, error) {
	result := Catalog{Skills: []Entry{}, Issues: append([]string(nil), t.issues...)}
	if err := ctx.Err(); err != nil {
		return result, err
	}
	for _, lib := range t.libraries {
		if err := ctx.Err(); err != nil {
			return result, err
		}
		data, err := lib.reader.NewTask().ListFiles(".")
		if err != nil {
			result.Issues = append(result.Issues, lib.source+": cannot list library")
			continue
		}
		result.Truncated = result.Truncated || data.Truncated
		for _, entry := range strings.Split(data.Content, "\n") {
			if err := ctx.Err(); err != nil {
				return result, err
			}
			if !strings.HasSuffix(entry, "/") {
				continue
			}
			name := strings.TrimSuffix(entry, "/")
			if !ValidName(name) || t.check(lib, name, "SKILL.md") != nil {
				continue
			}
			if len(result.Skills) >= 256 {
				result.Truncated = true
				break
			}
			result.Skills = append(result.Skills, Entry{Source: lib.source, Name: name})
		}
	}
	sort.Slice(result.Skills, func(i, j int) bool {
		a, b := result.Skills[i], result.Skills[j]
		if a.Source == b.Source {
			return a.Name < b.Name
		}
		return a.Source < b.Source
	})
	// JSON escaping and metadata count towards the same 32 KiB result limit.
	for {
		encoded, err := json.Marshal(result)
		if err != nil {
			return result, err
		}
		if len(encoded) <= project.FileLimit {
			break
		}
		if len(result.Skills) == 0 {
			return result, errors.New("skill catalog exceeds result budget")
		}
		result.Skills = result.Skills[:len(result.Skills)-1]
		result.Truncated = true
	}
	if err := ctx.Err(); err != nil {
		return result, err
	}
	return result, t.budget.ChargeResult(result)
}

// Read returns SKILL.md by default, or a relative supporting text file. Absolute
// paths, traversal, symlinks, ignored/sensitive paths and special files fail closed.
// It never runs scripts. Complete JSON results share project return limits.
func (t *Task) Read(ctx context.Context, source, name, path string) (Data, error) {
	if err := ctx.Err(); err != nil {
		return Data{}, err
	}
	if !ValidName(name) {
		return Data{}, errors.New("invalid skill name")
	}
	if path == "" {
		path = "SKILL.md"
	}
	if len(path) > 1024 || filepath.IsAbs(path) || strings.ContainsRune(path, 0) {
		return Data{}, errors.New("invalid skill-relative path")
	}
	for _, part := range strings.Split(filepath.ToSlash(path), "/") {
		if part == ".." {
			return Data{}, errors.New("skill path traversal is not allowed")
		}
	}
	for _, lib := range t.libraries {
		if lib.source != source {
			continue
		}
		if err := t.check(lib, name, "SKILL.md"); err != nil {
			return Data{}, errors.New("skill is unavailable")
		}
		if err := t.check(lib, name, path); err != nil {
			return Data{}, err
		}
		raw, err := lib.reader.NewTask().ReadFile(filepath.Join(name, path))
		if err != nil {
			return Data{}, err
		}
		if err := ctx.Err(); err != nil {
			return Data{}, err
		}
		result := Data{Source: source, Name: name, Path: path, Content: raw.Content, Truncated: raw.Truncated}
		for {
			encoded, err := json.Marshal(result)
			if err != nil {
				return Data{}, err
			}
			if len(encoded) <= project.FileLimit {
				break
			}
			cut := min(len(result.Content), max(1, (len(encoded)-project.FileLimit+5)/6))
			if cut == 0 {
				return Data{}, errors.New("skill metadata exceeds result budget")
			}
			result.Content = result.Content[:len(result.Content)-cut]
			for !utf8.ValidString(result.Content) {
				result.Content = result.Content[:len(result.Content)-1]
			}
			result.Truncated = true
		}
		if err := ctx.Err(); err != nil {
			return result, err
		}
		return result, t.budget.ChargeResult(result)
	}
	return Data{}, errors.New("skill source is unavailable; use list_skills to discover sources")
}

func (t *Task) check(lib library, name, path string) error {
	if lib.source == "project" {
		if err := t.budget.CheckRegular(filepath.Join(".lingcell", "skills", name, path)); err != nil {
			return err
		}
	}
	return lib.reader.CheckRegular(filepath.Join(name, path))
}
