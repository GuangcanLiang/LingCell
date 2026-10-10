package app

import (
	"bytes"
	"strings"
	"testing"
)

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
