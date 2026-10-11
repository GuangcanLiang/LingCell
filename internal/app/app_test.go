package app

import (
	"bytes"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestModelTextCannotSendTerminalControlSequences checks readable escaping of
// terminal and bidi controls, including the writer's input-byte count contract.
func TestModelTextCannotSendTerminalControlSequences(t *testing.T) {
	var output bytes.Buffer
	data := []byte("explanation\n\x1b[2J\u202Ehidden\ttext")
	n, err := (modelOutput{&output}).Write(data)
	if err != nil || n != len(data) {
		t.Fatalf("write: %d %v", n, err)
	}
	if strings.ContainsRune(output.String(), '\x1b') || strings.ContainsRune(output.String(), '\u202E') {
		t.Fatal("model text emitted terminal control sequences")
	}
	if !strings.Contains(output.String(), "explanation\n") || !strings.Contains(output.String(), "\\u001b[2J") {
		t.Fatal("model text should remain readable")
	}
}

// brokenWriter fails at any proposal boundary, including a silent short write.
type brokenWriter struct {
	remaining int
	short     bool
}

func (w *brokenWriter) Write(p []byte) (int, error) {
	n := min(len(p), w.remaining)
	w.remaining -= n
	if n != len(p) && !w.short {
		return n, io.ErrClosedPipe
	}
	return n, nil
}

// TestConfirmationDisplayFailure never initializes a prompt: failure must
// return before reading input, even if part of the proposal has been shown.
func TestConfirmationDisplayFailure(t *testing.T) {
	for _, short := range []bool{false, true} {
		for _, remaining := range []int{0, 10, 35, 65} {
			a := App{Output: &brokenWriter{remaining: remaining, short: short}}
			yes, err := a.confirm("touch marker-with-a-long-name", "/actual/work/directory", "purpose description")
			if yes || err == nil {
				t.Fatalf("continued after display failure: %t %v", yes, err)
			}
		}
	}
}

// TestDoctorRedactsAndDoesNotWrite checks malformed configuration and a
// valid local service using private fixture values, without file output.
func TestDoctorRedactsAndDoesNotWrite(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "config.json")
	t.Setenv("DOCTOR_TEST_KEY", "PRIVATE_CREDENTIAL")
	for _, data := range []string{`{"PRIVATE_UNKNOWN_FIELD":"PRIVATE_CONFIG"}`, `{"base_url":"http://127.0.0.1:1/PRIVATE_ENDPOINT","model":"PRIVATE_MODEL","api_key_env":"DOCTOR_TEST_KEY","history_file":"PRIVATE_HISTORY"}`} {
		if err := os.WriteFile(path, []byte(data), 0600); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(root, "file.txt"), []byte("PRIVATE_CONTENT"), 0600); err != nil {
			t.Fatal(err)
		}
		var out bytes.Buffer
		err := Doctor(&out, path, root)
		if strings.Contains(data, "PRIVATE_UNKNOWN") && err == nil {
			t.Fatal("invalid config passed")
		}
		if !strings.Contains(data, "PRIVATE_UNKNOWN") && err != nil {
			t.Fatal(err)
		}
		if strings.Contains(out.String(), "PRIVATE_") {
			t.Fatalf("private data leaked: %s", &out)
		}
		entries, err := os.ReadDir(root)
		if err != nil || len(entries) != 2 {
			t.Fatal("doctor wrote files", err)
		}
	}
	if err := Doctor(&brokenWriter{}, path, root); !errors.Is(err, io.ErrClosedPipe) {
		t.Fatalf("output failure: %v", err)
	}
	var out bytes.Buffer
	if err := Doctor(&out, filepath.Join(root, "missing.json"), root); err == nil || !strings.Contains(out.String(), "FAIL model") {
		t.Fatal("missing model not diagnosed")
	}
}
