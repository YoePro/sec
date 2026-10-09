package sema

import (
	"testing"

	"sec/internal/lexer"
	"sec/internal/parser"
)

// The LSP analyzes recovered programs that still contain parse errors. A
// declaration whose name was not written yet (here `test {` while typing)
// keeps a nil name node, and preparing call-graph syntax origins must not
// dereference it.
//
// Rules:
//   - rules/analysis/call_graph.md — "Callable node identity"
//   - rules/compiler/parser_recovery.md — recovered syntax remains analyzable
func TestCallGraphOriginsTolerateRecoveredNamelessDeclarations(t *testing.T) {
	p := parser.New(lexer.NewWithFile("module main\n\ntest {\n}\n\nfn Ok() void {\n}\n", "recovered.sec"))
	program := p.ParseProgram()
	if len(p.Errors()) == 0 {
		t.Fatal("expected the nameless test declaration to be a parse error")
	}
	defer func() {
		if recovered := recover(); recovered != nil {
			t.Fatalf("analysis of a recovered program panicked: %v", recovered)
		}
	}()
	NewAnalyzer().Analyze(program)
}
