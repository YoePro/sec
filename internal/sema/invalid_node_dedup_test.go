package sema

import (
	"strings"
	"testing"

	"sec/internal/lexer"
	"sec/internal/parser"
)

// A recovered invalid node whose diagnostic the parser already emitted is not
// reported again by Sema, so the LSP, which publishes both, shows each error
// once; invalid nodes that only Sema diagnoses (impl member fallbacks) are
// still reported.
//
// Rules:
//   - rules/compiler/parser_recovery.md — "Diagnostic deduplication", "Invalid nodes"
//   - rules/tooling/diagnostics.md — one diagnostic per root cause
func TestRecoveredInvalidNodesAreReportedOnce(t *testing.T) {
	source := `module main

fn Moves() void {
    let source := 1
    let typed: int :<- source
}

@inline
fn Unknown() void {
}

fn Separator() void {
    let value := 1;
}

type Box struct {
    value: int,
}

impl Box {
    addedField: int
}
`
	result := parser.New(lexer.New(source)).Parse()
	messages := []string{}
	for _, diagnostic := range result.Diagnostics {
		messages = append(messages, diagnostic.Message)
	}
	for _, err := range NewAnalyzer().Analyze(result.Program) {
		messages = append(messages, err.Message)
	}
	joined := strings.Join(messages, "\n")
	for _, cause := range []string{"typed move initializer must use '<-'", "unknown attribute @inline", "semicolon is not used as a statement terminator"} {
		if count := strings.Count(joined, cause); count != 1 {
			t.Errorf("%q reported %d times in %q, want once", cause, count, messages)
		}
	}
	if strings.Count(joined, "stored fields are not allowed inside impl") != 1 {
		t.Errorf("Sema-owned impl member diagnostic is missing: %q", messages)
	}
}
