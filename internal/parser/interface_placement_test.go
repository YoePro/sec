package parser

import (
	"os"
	"sec/internal/ast"
	"sec/internal/diagnostics"
	"sec/internal/lexer"
	"testing"
)

// Invalid conformance sites retain a dedicated identity, exact keyword range,
// canonical repair and following declarations. Rules: rules/declarations/interfaces.md §§4,12.
func TestInterfacePlacementDiagnostics(t *testing.T) {
	for _, name := range []string{"type", "extends", "separate"} {
		t.Run(name, func(t *testing.T) {
			path := "../../testdata/sema/interface_diagnostics/" + name + "_placement_invalid.sec"
			source, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			p := New(lexer.NewWithFile(string(source), path))
			result := p.Parse()
			ds := p.Diagnostics()
			if len(ds) != 1 || ds[0].ID != diagnostics.ParserInterfaceConformancePlacement || ds[0].Help == "" {
				t.Fatalf("diagnostics: %+v", ds)
			}
			want := "implements"
			if name == "separate" {
				want = "for"
			}
			if ds[0].Primary.Lexeme != want || ds[0].Primary.File != path || ds[0].Primary.Line == 0 {
				t.Fatalf("anchor: %+v", ds[0])
			}
			found := false
			for _, stmt := range result.Program.Statements {
				if fn, ok := stmt.(*ast.FunctionDeclaration); ok && fn.Name.Value == "Following" {
					found = true
				}
			}
			if !found {
				t.Fatal("following declaration lost")
			}
		})
	}
}
