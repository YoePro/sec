package parser

import (
	"os"
	"strings"
	"testing"

	"sec/internal/ast"
	"sec/internal/lexer"
)

// TestNewArgumentDelimiterRecovery distinguishes complete empty construction
// from incomplete syntax without allowing the editor to invent a closing token.
// Rules: rules/compiler/parser_recovery.md — "Invalid AST nodes", "Interaction with LSP";
// rules/tooling/lsp.md — §13.
func TestNewArgumentDelimiterRecovery(t *testing.T) {
	data, err := os.ReadFile("../../testdata/lsp/initializer_signatures.sec")
	if err != nil {
		t.Fatal(err)
	}
	source := string(data)
	prefix := source[:strings.Index(source, "new Widget(1)")]
	for _, tail := range []string{"new Widget()\n}", "new Widget(1)\n}", "new Widget(", "new Widget(1, "} {
		p := New(lexer.New(prefix + tail))
		program := p.ParseProgram()
		function, ok := program.Statements[len(program.Statements)-1].(*ast.FunctionDeclaration)
		if !ok || function.Body == nil || len(function.Body.Statements) == 0 {
			t.Fatalf("lost recovered function in %q", tail)
		}
		statement, ok := function.Body.Statements[0].(*ast.ReturnStatement)
		if !ok {
			t.Fatalf("lost return in %q", tail)
		}
		construction, ok := statement.Value.(*ast.NewExpression)
		if !ok || construction.ArgumentsOpen.Type != lexer.LPAREN {
			t.Fatalf("lost construction in %q", tail)
		}
		complete := strings.Contains(tail, ")")
		if got := construction.ArgumentsClose.Type == lexer.RPAREN; got != complete {
			t.Fatalf("closing delimiter in %q: got %v; complete %v", tail, got, complete)
		}
		if complete && len(p.Errors()) > 0 || !complete && len(p.Errors()) == 0 {
			t.Fatalf("recovery diagnostics in %q: %v", tail, p.Errors())
		}
	}
}
