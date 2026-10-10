package router

import "testing"

func TestRouting(t *testing.T) {
	r := Router{Knows: func(s string) bool { return s == "ls" || s == "echo" || s == "find" || s == "myalias" }}
	cases := map[string]Kind{"": Shell, "ls -la": Shell, "myalias": Shell, "echo 中文": Shell, "FOO=中文": Shell, "请找出大文件": Agent, "please inspect this project": Agent, "lss -la": Ambiguous, "locate large files": Ambiguous, "find large files": Shell, "echo 'unfinished": Incomplete, "echo $(": Incomplete, "printf )": Ambiguous, "ls | head": Shell, "for i in a; do": Incomplete, "./script": Shell}
	for input, want := range cases {
		if got := r.Route(input); got.Kind != want {
			t.Errorf("%q => %s (%s), want %s", input, got.Kind, got.Reason, want)
		}
	}
	if !NeedsContinuation("cat <<EOF\na") || NeedsContinuation("echo done") {
		t.Fatal("continuation failed")
	}
}
func TestFailurePolicy(t *testing.T) {
	cases := []struct {
		command string
		code    int
		want    bool
	}{
		{"false", 1, true}, {"grep nope file", 1, false}, {"grep nope file", 2, true}, {"diff a b", 1, false}, {"test -f missing", 1, false}, {"[ -f missing ]", 1, false},
		{"grep nope file | cat", 1, true}, {"grep nope file; false", 1, true}, {"grep nope file > log", 1, true}, {"false", 130, false}, {"sleep 10", 148, false}, {"true", 0, false}, {"missing", 127, true}, {"exit 200", 200, true},
	}
	for _, c := range cases {
		if got := Diagnose(c.command, c.code); got != c.want {
			t.Errorf("%s/%d => %v", c.command, c.code, got)
		}
	}
}
