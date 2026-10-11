package router

import "testing"

// TestRouting checks local command precedence over natural-language hints,
// unknown-command ambiguity and continuation for incomplete Bash syntax.
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

// TestRequestPunctuation protects natural-language requests from accidental
// execution while preserving commands, scripts and compound Bash syntax.
func TestRequestPunctuation(t *testing.T) {
	r := Router{Knows: func(s string) bool { return s == "echo" || s == "ls" || s == "help" || s == "myalias" }}
	cases := map[string]Kind{
		"帮我解释 `$PATH`": Agent, "请解释 a; b": Agent,
		"帮我解释PATH=/bin; echo ROUTE_MARKER": Agent,
		"请解释/tmp/file":                     Agent, "解释LANG=zh_CN.UTF-8": Agent,
		"帮我解释$PATH=/bin":        Agent,
		"help me inspect $PATH": Shell, "please explain echo $(": Agent,
		"can you explain a | b\nand c": Agent, "为什么 echo '": Agent,
		"echo $PATH": Shell, "echo 中文; ls": Shell, "myalias $PATH": Shell,
		"help me": Shell, "could you inspect $PATH": Agent, "FOO=中文": Shell, "FOO=$PATH missing": Shell,
		"./script.sh $PATH": Shell, "/bin/echo 中文": Shell,
		"./帮我查看 $PATH": Shell, "/tmp/请解释": Shell,
		"if true; then echo 中文; fi": Shell, "for i in a; do": Incomplete,
		"f() { echo 中文; }": Shell, "cat <<EOF\n中文": Incomplete,
		"unknown $PATH": Ambiguous, "inspect `pwd`": Ambiguous,
		"unknown 'unfinished": Ambiguous, "lss -la": Ambiguous,
		"unknown > result": Shell, "unknown | cat": Shell,
	}
	for input, want := range cases {
		if got := r.Route(input); got.Kind != want {
			t.Errorf("%q => %s (%s), want %s", input, got.Kind, got.Reason, want)
		}
	}
}

// TestFailurePolicy confines normal-negative exemptions to simple commands
// and keeps interruption and stopped jobs out of automatic diagnosis.
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
