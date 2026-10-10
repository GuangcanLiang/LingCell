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
	Shell      Kind = "shell"
	Agent      Kind = "agent"
	Ambiguous  Kind = "ambiguous"
	Incomplete Kind = "incomplete"
)

// Decision is a route with a human-readable explanation.
type Decision struct {
	Kind   Kind
	Reason string
}

// Router applies local rules using the current Bash command-name resolver.
type Router struct{ Knows func(string) bool }

func parse(input string) (*syntax.File, error) {
	return syntax.NewParser(syntax.Variant(syntax.LangBash)).Parse(strings.NewReader(input), "")
}

// Route classifies input without expanding or executing shell expressions.
func (r Router) Route(input string) Decision {
	input = strings.TrimSpace(input)
	if input == "" {
		return Decision{Shell, "empty input"}
	}
	first := strings.Fields(input)[0]
	known := r.Knows != nil && r.Knows(first)
	file, err := parse(input)
	explicit := known || shellStructure(file) || strings.ContainsAny(input, "|;&<>$`\n") || strings.ContainsAny(first, "/=")
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
		lower := strings.ToLower(input)
		for _, p := range []string{"please ", "help me ", "how do ", "how can ", "why ", "can you ", "could you "} {
			if strings.HasPrefix(lower, p) {
				return Decision{Agent, "natural language"}
			}
		}
	}
	return Decision{Ambiguous, "requires model classification"}
}

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

// NeedsContinuation applies after the input is known (or forced) to be shell.
func NeedsContinuation(input string) bool { _, err := parse(input); return syntax.IsIncomplete(err) }

// Diagnose only exempts single, simple commands with a normal negative result.
func Diagnose(input string, code int) bool {
	if code == 0 || code == 130 || code == 148 {
		return false
	}
	if code != 1 {
		return true
	}
	f, err := parse(input)
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
