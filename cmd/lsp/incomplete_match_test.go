package main

import "testing"

// Completion may analyze a parser-recovered try operand while a match subject
// is still missing. Typed-nil partial nodes must remain invalid facts rather
// than escaping into Sema and terminating the language server.
//
// Rules:
//   - rules/compiler/parser_recovery.md — "Recovery principles" and "Invalid node"
//   - rules/tooling/lsp.md — incomplete-document tooling behavior
func TestCompletionSurvivesIncompleteMatchInsideTry(t *testing.T) {
	source := "module main\n\nfn Pending() void {\n    let value := try match\n}"
	defer func() {
		if recovered := recover(); recovered != nil {
			t.Fatalf("completion panicked for incomplete match inside try: %v", recovered)
		}
	}()
	_ = completeSource("file:///tmp/sec-lsp-incomplete-match/main.sec", source, len(source))
}
