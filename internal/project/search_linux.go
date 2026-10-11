package project

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"unicode/utf8"
)

// ScanTaskLimit bounds bytes scanned by search and range reads in one task.
const ScanTaskLimit = 16 * 1024 * 1024

// ScanCallLimit bounds bytes scanned by one search or range read.
const ScanCallLimit = 8 * 1024 * 1024

// SearchFileLimit bounds a searched file; larger files are skipped explicitly.
const SearchFileLimit = 1024 * 1024

// Match is a literal, case-sensitive match on one project-relative text line.
// A line appears once even if the query occurs more than once on it.
type Match struct {
	Path string `json:"path"`
	Line int    `json:"line"`
	Text string `json:"text"`
}

// SearchData reports matches and the limits on the observed search scope.
// Skipped entries include excluded, ignored, unsafe and non-text paths; they
// must not be interpreted as evidence that the entire project was searched.
type SearchData struct {
	Path           string   `json:"path"`
	Query          string   `json:"query"`
	Matches        []Match  `json:"matches"`
	Truncated      bool     `json:"truncated"`
	Limits         []string `json:"limits,omitempty"`
	ScannedBytes   int      `json:"scanned_bytes"`
	CheckedEntries int      `json:"checked_entries"`
	SkippedEntries int      `json:"skipped_entries"`
	SkippedLarge   int      `json:"skipped_large_files"`
}

// RangeData describes the actual returned lines (zero for an empty result).
// Truncated covers incomplete scanning or output, not merely a requested range
// that ends before EOF. Limits explain why the returned evidence is partial.
type RangeData struct {
	Path         string   `json:"path"`
	StartLine    int      `json:"start_line"`
	EndLine      int      `json:"end_line"`
	Content      string   `json:"content"`
	Truncated    bool     `json:"truncated"`
	Limits       []string `json:"limits,omitempty"`
	ScannedBytes int      `json:"scanned_bytes"`
}

func addLimit(limits *[]string, reason string) {
	for _, existing := range *limits {
		if existing == reason {
			return
		}
	}
	*limits = append(*limits, reason)
}

// scan reads in bounded chunks so cancellation is observed even on large files.
// Every byte examined, including rejected text, consumes the shared scan budget.
func (t *Task) scan(ctx context.Context, f *os.File, limit int) ([]byte, bool, error) {
	if err := ctx.Err(); err != nil {
		return nil, false, err
	}
	if limit > ScanTaskLimit-t.scanned {
		limit = ScanTaskLimit - t.scanned
	}
	if limit <= 0 {
		return nil, true, errors.New("project scanning budget (16 MiB) exhausted")
	}
	var data bytes.Buffer
	chunk := make([]byte, FileLimit)
	for data.Len() < limit {
		if err := ctx.Err(); err != nil {
			return nil, false, err
		}
		n, err := f.Read(chunk[:min(len(chunk), limit-data.Len())])
		t.scanned += n
		data.Write(chunk[:n])
		if err == io.EOF {
			return data.Bytes(), false, nil
		}
		if err != nil {
			return nil, false, err
		}
		if n == 0 {
			return nil, false, io.ErrNoProgress
		}
	}
	// Reaching a cap is reported conservatively even if EOF coincides with it.
	return data.Bytes(), true, nil
}

func textError(b []byte) error {
	if bytes.ContainsRune(b, 0) || !utf8.Valid(b) {
		return errors.New("file is not UTF-8 text")
	}
	if bytes.Contains(b, []byte("PRIVATE KEY-----")) {
		return errors.New("private key content is excluded")
	}
	return nil
}

// chargeResult charges the encoded new-tool result, including paths and line
// metadata. Legacy tools retain their existing content-byte accounting.
func (t *Task) chargeResult(result any) error {
	encoded, err := json.Marshal(result)
	if err != nil {
		return err
	}
	if len(encoded) > FileLimit {
		return errors.New("project result exceeds 32 KiB")
	}
	if len(encoded) > TaskLimit-t.used {
		return errors.New("project reading budget (256 KiB) exhausted")
	}
	t.used += len(encoded)
	return nil
}

// SearchText recursively searches allowed UTF-8 files without shell processes
// or indexes. Results are sorted by path and line, capped at 100 matching lines,
// 4096 inspected directory entries, 8 MiB per call and 1 MiB per file. Directory
// symlinks are never followed, including a symlink used as the search root.
func (t *Task) SearchText(ctx context.Context, path, query string) (SearchData, error) {
	result := SearchData{Path: path, Query: query, Matches: []Match{}}
	if query == "" || strings.ContainsRune(query, 0) || !utf8.ValidString(query) {
		return result, errors.New("search_text requires a nonempty UTF-8 literal query without NUL")
	}
	// Bound even an otherwise valid argument before encoding it in a result.
	if len(path)+len(query) > FileLimit/2 {
		return result, errors.New("search_text arguments exceed result budget")
	}
	if err := ctx.Err(); err != nil {
		return result, err
	}
	root, err := t.reader.checked(path)
	if err != nil {
		return result, err
	}
	f, err := t.reader.open(filepath.Clean(path))
	if err != nil {
		return result, err
	}
	info, err := f.Stat()
	f.Close()
	if err != nil || !info.IsDir() {
		return result, errors.New("search_text path must be a directory")
	}
	start := t.scanned
	stopped := false
	var walk func(string) error
	walk = func(dir string) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		if result.CheckedEntries >= 4096 {
			addLimit(&result.Limits, "directory_entry_limit")
			return nil
		}
		d, err := t.reader.open(dir)
		if err != nil {
			result.SkippedEntries++
			addLimit(&result.Limits, "unreadable_paths_skipped")
			return nil
		}
		entries, err := d.ReadDir(4096 - result.CheckedEntries)
		d.Close()
		if err != nil && err != io.EOF {
			return err
		}
		if len(entries) == 4096-result.CheckedEntries {
			addLimit(&result.Limits, "directory_entry_limit")
		}
		result.CheckedEntries += len(entries)
		sort.Slice(entries, func(i, j int) bool { return entries[i].Name() < entries[j].Name() })
		for _, entry := range entries {
			if err := ctx.Err(); err != nil {
				return err
			}
			if stopped || len(result.Matches) >= 100 || t.scanned-start >= ScanCallLimit || t.scanned >= ScanTaskLimit {
				return nil
			}
			candidate := filepath.Join(dir, entry.Name())
			rel, err := t.reader.checked(candidate)
			if err != nil {
				result.SkippedEntries++
				continue
			}
			file, err := t.reader.open(rel)
			if err != nil {
				result.SkippedEntries++
				addLimit(&result.Limits, "unreadable_paths_skipped")
				continue
			}
			st, err := file.Stat()
			if err != nil {
				file.Close()
				return err
			}
			if st.IsDir() {
				file.Close()
				if entry.Type()&os.ModeSymlink != 0 {
					result.SkippedEntries++
					continue
				}
				// Reopen the original directory path with NO_SYMLINKS. Entry.Type
				// alone could be stale if a directory was replaced by a link.
				if err := walk(candidate); err != nil {
					return err
				}
				continue
			}
			if !st.Mode().IsRegular() {
				file.Close()
				result.SkippedEntries++
				continue
			}
			if st.Size() > SearchFileLimit {
				file.Close()
				result.SkippedLarge++
				addLimit(&result.Limits, "large_files_skipped")
				continue
			}
			remaining := min(SearchFileLimit, ScanCallLimit-(t.scanned-start), ScanTaskLimit-t.scanned)
			b, capped, err := t.scan(ctx, file, remaining)
			file.Close()
			if err != nil {
				return err
			}
			// A file at exactly the per-file cap can still be fully inspected;
			// conservatively disclose the cap in case it changed while reading.
			if capped {
				addLimit(&result.Limits, "file_scan_limit")
			}
			if textError(b) != nil {
				result.SkippedEntries++
				continue
			}
			for line := 1; len(b) > 0; line++ {
				if err := ctx.Err(); err != nil {
					return err
				}
				n := bytes.IndexByte(b, '\n')
				if n < 0 {
					n = len(b)
				}
				text := b[:n]
				if bytes.Contains(text, []byte(query)) {
					result.Matches = append(result.Matches, Match{Path: filepath.ToSlash(candidate), Line: line, Text: string(text)})
					encoded, _ := json.Marshal(result)
					if len(encoded) > FileLimit-1024 {
						result.Matches = result.Matches[:len(result.Matches)-1]
						addLimit(&result.Limits, "return_limit")
						stopped = true
						return nil
					}
					if len(result.Matches) == 100 {
						addLimit(&result.Limits, "match_limit")
						stopped = true
						return nil
					}
				}
				b = b[min(n+1, len(b)):]
			}
		}
		return nil
	}
	if err := walk(root); err != nil {
		return SearchData{}, err
	}
	result.ScannedBytes = t.scanned - start
	if result.ScannedBytes >= ScanCallLimit {
		addLimit(&result.Limits, "call_scan_limit")
	}
	if t.scanned >= ScanTaskLimit {
		addLimit(&result.Limits, "task_scan_limit")
	}
	if result.CheckedEntries >= 4096 {
		addLimit(&result.Limits, "directory_entry_limit")
	}
	result.Truncated = len(result.Limits) > 0
	sort.Slice(result.Matches, func(i, j int) bool {
		if result.Matches[i].Path == result.Matches[j].Path {
			return result.Matches[i].Line < result.Matches[j].Line
		}
		return result.Matches[i].Path < result.Matches[j].Path
	})
	if err := t.chargeResult(result); err != nil {
		return SearchData{}, err
	}
	return result, nil
}

// ReadFileRange reads a one-based range from an allowed regular text file.
// Zero count selects 100 lines; start must be positive and count at most 200.
// Scanning is bounded even when the requested start is beyond EOF. UTF-8 and
// private-key checks cover the scanned prefix, including lines before the range.
func (t *Task) ReadFileRange(ctx context.Context, path string, startLine, count int) (RangeData, error) {
	result := RangeData{Path: path}
	// Keep metadata bounded before any IO. Cleaned paths may be tiny while
	// the caller's original spelling contains thousands of redundant './'.
	metadata, _ := json.Marshal(result)
	if len(path) > FileLimit/2 || len(metadata) > FileLimit-1024 {
		return RangeData{}, errors.New("read_file_range path exceeds result budget")
	}
	if count == 0 {
		count = 100
	}
	if startLine < 1 || count < 1 || count > 200 {
		return result, errors.New("read_file_range requires start_line >= 1 and line_count between 1 and 200")
	}
	if err := ctx.Err(); err != nil {
		return result, err
	}
	rel, err := t.reader.checked(path)
	if err != nil {
		return result, err
	}
	f, err := t.reader.open(rel)
	if err != nil {
		return result, err
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return result, err
	}
	if !info.Mode().IsRegular() {
		return result, errors.New("only regular text files can be read")
	}
	before := t.scanned
	b, capped, err := t.scan(ctx, f, ScanCallLimit)
	if err != nil {
		return RangeData{}, err
	}
	result.ScannedBytes = t.scanned - before
	if capped {
		result.Truncated = true
		addLimit(&result.Limits, "call_scan_limit")
		if t.scanned >= ScanTaskLimit {
			addLimit(&result.Limits, "task_scan_limit")
		}
		// A scan cap can split a rune. Only that boundary suffix is removable.
		for len(b) > 0 && !utf8.Valid(b) && len(b) > result.ScannedBytes-4 {
			b = b[:len(b)-1]
		}
	}
	if err := textError(b); err != nil {
		return RangeData{}, err
	}
	for line := 1; len(b) > 0; line++ {
		if err := ctx.Err(); err != nil {
			return RangeData{}, err
		}
		n := bytes.IndexByte(b, '\n')
		if n < 0 {
			n = len(b)
		} else {
			n++
		}
		if line >= startLine {
			if result.StartLine == 0 {
				result.StartLine = line
			}
			result.EndLine = line
			included := min(n, FileLimit-len(result.Content))
			result.Content += string(b[:included])
			if n > included || len(result.Content) >= FileLimit {
				result.Truncated = true
				addLimit(&result.Limits, "return_limit")
				break
			}
			if line-startLine+1 >= count {
				break
			}
		}
		b = b[n:]
	}
	// JSON escaping and metadata also count toward the new tools' return cap.
	for {
		if err := ctx.Err(); err != nil {
			return RangeData{}, err
		}
		encoded, _ := json.Marshal(result)
		if len(encoded) <= FileLimit {
			break
		}
		result.Truncated = true
		addLimit(&result.Limits, "return_limit")
		if len(result.Content) == 0 {
			return RangeData{}, errors.New("read_file_range metadata exceeds result budget")
		}
		remove := max(1, (len(encoded)-FileLimit)/6)
		result.Content = result.Content[:max(0, len(result.Content)-remove)]
	}
	for !utf8.ValidString(result.Content) && len(result.Content) > 0 {
		result.Content = result.Content[:len(result.Content)-1]
	}
	if result.Content == "" {
		result.StartLine, result.EndLine = 0, 0
	} else {
		result.EndLine = result.StartLine + strings.Count(strings.TrimSuffix(result.Content, "\n"), "\n")
	}
	if err := t.chargeResult(result); err != nil {
		return RangeData{}, err
	}
	return result, nil
}
