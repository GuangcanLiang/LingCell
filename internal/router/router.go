// Package router classifies input without evaluating shell expressions.
package router

import (
	"strings"
	"unicode"

	"mvdan.cc/sh/v3/syntax"
)

// Kind selects the shell, Agent, classifier or continuation input path.
type Kind string

const (
	// Shell selects direct Bash execution, including empty input.
	Shell Kind = "shell"
	// Agent selects a natural-language task.
	Agent Kind = "agent"
	// Ambiguous requests model classification before choosing an action.
	Ambiguous Kind = "ambiguous"
	// Incomplete requests another line for recognized, unfinished shell input.
	Incomplete Kind = "incomplete"
)

// Decision is a route with a human-readable explanation.
type Decision struct {
	Kind   Kind
	Reason string
}

// Router applies local rules using the current Bash command-name resolver.
// A nil Knows disables name-based recognition; syntax rules still apply.
type Router struct{ Knows func(string) bool }

func parse(input string) (*syntax.File, error) {
	return syntax.NewParser(syntax.Variant(syntax.LangBash)).Parse(strings.NewReader(input), "")
}

// Route classifies input without expanding or executing shell expressions.
// Known Bash names take precedence over language heuristics. Clear requests
// with unknown first words precede punctuation-based shell rules. Unknown input
// may need model classification, even after a parse
// error; recognized shell syntax errors are left for Bash to report.
func (r Router) Route(input string) Decision {
	input = strings.TrimSpace(input)
	if input == "" {
		return Decision{Shell, "empty input"}
	}
	first := strings.Fields(input)[0]
	known := r.Knows != nil && r.Knows(first)
	// Requests may embed paths or assignment examples even in their first word.
	// Only recognized command names take precedence over a clear request prefix;
	// punctuation in an unknown request must not grant execution authority.
	if !known && requestPrefix(input) {
		return Decision{Agent, "natural language request"}
	}
	file, err := parse(input)
	// The parser may return a partial tree. Use available structure to retain
	// shell precedence, including commands whose syntax Bash will reject.
	explicit := known || shellStructure(file) || strings.ContainsAny(input, "|;&<>\n") || strings.ContainsAny(first, "/=")
	if syntax.IsIncomplete(err) && explicit {
		return Decision{Incomplete, "unfinished shell syntax"}
	}
	if explicit {
		return Decision{Shell, "Bash command or shell structure"}
	}
	// Chinese text and common imperative requests are clearly natural language.
	if !known {
		for _, c := range input {
			if unicode.Is(unicode.Han, c) {
				return Decision{Agent, "natural language"}
			}
		}
	}
	return Decision{Ambiguous, "requires model classification"}
}

func requestPrefix(input string) bool {
	lower := strings.ToLower(input)
	for _, p := range []string{"please ", "help me ", "how do ", "how can ", "why ", "can you ", "could you ", "帮我", "请", "解释", "为什么", "如何", "能否", "可以帮"} {
		if strings.HasPrefix(lower, p) {
			return true
		}
	}
	return false
}

// shellStructure identifies compound syntax, redirections and assignments.
// A lone ordinary call is resolved by command name or the classifier instead.
func shellStructure(f *syntax.File) bool {
	if f == nil {
		return false
	}
	if len(f.Stmts) != 1 {
		return len(f.Stmts) > 0
	}
	stmt := f.Stmts[0]
	if stmt.Background || stmt.Negated || len(stmt.Redirs) > 0 {
		return true
	}
	call, ok := stmt.Cmd.(*syntax.CallExpr)
	return !ok || len(call.Assigns) > 0
}

// NeedsContinuation reports whether recognized or forced shell input needs
// another line. Other parse errors are complete input for Bash to diagnose.
func NeedsContinuation(input string) bool { _, err := parse(input); return syntax.IsIncomplete(err) }

// Diagnose reports whether a user command's status needs automatic diagnosis.
// Success, Ctrl-C (130), Ctrl-Z (148), and status 1 from a lone, simple
// grep/diff/test/[ are exempt. Compound commands, assignments and redirections
// do not receive the normal-negative exemption.
func Diagnose(input string, code int) bool {
	if code == 0 || code == 130 || code == 148 {
		return false
	}
	if code != 1 {
		return true
	}
	f, err := parse(input)
	// Be conservative when the failed input cannot be proved to be one simple
	// command; a compound expression may fail for a different reason.
	if err != nil || f == nil || len(f.Stmts) != 1 {
		return true
	}
	s := f.Stmts[0]
	if s.Negated || s.Background || len(s.Redirs) > 0 {
		return true
	}
	c, ok := s.Cmd.(*syntax.CallExpr)
	if !ok || len(c.Assigns) > 0 || len(c.Args) == 0 {
		return true
	}
	name := c.Args[0].Lit()
	return name != "grep" && name != "diff" && name != "test" && name != "["
}
