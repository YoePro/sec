package parser

import (
	"testing"

	"sec/internal/ast"
	compilerdiagnostics "sec/internal/diagnostics"
	"sec/internal/lexer"
)

// Rules:
//   - rules/compiler/parser_recovery.md — "Struct declaration recovery"
//   - rules/compiler/parser_recovery.md — "Missing comma"
func TestMissingStructFieldCommaRetainsLaterLineField(t *testing.T) {
	p := New(lexer.New(`
type Record struct {
    First: int
    Second: int,
}
`))
	result := p.Parse()
	if len(result.Diagnostics) != 1 || result.Diagnostics[0].ID != compilerdiagnostics.ParserMissingToken {
		t.Fatalf("missing comma did not produce one structured missing-token diagnostic: %+v", result.Diagnostics)
	}

	declaration := result.Program.Statements[0].(*ast.TypeDeclStatement)
	if len(declaration.StructType.Fields) != 2 || declaration.StructType.Fields[1].Name.Value != "Second" {
		t.Fatalf("missing-comma recovery lost later field: %+v", declaration.StructType.Fields)
	}
	found := false
	for _, event := range result.Recovery {
		if event.Kind == RecoveryInsertMissingToken && len(event.Expected) == 1 && event.Expected[0] == lexer.COMMA {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("missing comma did not record a virtual token repair: %+v", result.Recovery)
	}
}

func TestMissingStructFieldCommaDoesNotRepairSameLineAdjacency(t *testing.T) {
	p := New(lexer.New("type Record struct { First: int Second: int }"))
	result := p.Parse()
	declaration := result.Program.Statements[0].(*ast.TypeDeclStatement)
	if len(declaration.StructType.Fields) != 1 {
		t.Fatalf("ambiguous same-line fields were reinterpreted: %+v", declaration.StructType.Fields)
	}
	for _, event := range result.Recovery {
		if event.Kind == RecoveryInsertMissingToken && len(event.Expected) == 1 && event.Expected[0] == lexer.COMMA {
			t.Fatalf("same-line adjacency received an unsupported comma repair: %+v", result.Recovery)
		}
	}
}
