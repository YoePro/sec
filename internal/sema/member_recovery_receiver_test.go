package sema

import (
	"testing"

	"sec/internal/lexer"
	"sec/internal/parser"
)

// A member access after a receiver that parser recovery could not form (an
// empty bracket list `a[]`, or a reserved keyword such as `after` used as a
// binding name) reports diagnostics and never builds a member expression on a
// missing receiver. The language server analyzes such half-edited test bodies
// on every keystroke, so analysis must not panic.
// Rules: rules/compiler/parser_recovery.md — "Postfix expression", "Member-access recovery".
func TestMemberAccessOnRecoveredReceiverDoesNotPanic(t *testing.T) {
	for _, body := range []string{
		"a[].c",
		"let after := 1\n    let x := after.Value",
		"after.Value",
		"a[].b.c()",
	} {
		source := "module main\n\ntest \"t\" {\n    " + body + "\n}\n"
		t.Run(body, func(t *testing.T) {
			defer func() {
				if r := recover(); r != nil {
					t.Fatalf("analysis panicked: %v", r)
				}
			}()
			parsed := parser.New(lexer.NewWithFile(source, "recovery.sec")).Parse()
			if !parsed.HasErrors {
				t.Fatal("recovery source was accepted")
			}
			NewAnalyzer().Analyze(parsed.Program)
		})
	}
}
