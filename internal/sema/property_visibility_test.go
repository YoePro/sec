package sema

import (
	"os"
	"sec/internal/ast"
	"sec/internal/diagnostics"
	"sec/internal/lexer"
	"sec/internal/parser"
	"testing"
)

// TestPropertyVisibility checks both accessor directions, static/instance
// access, different owners, same-module extensions and exact module boundaries.
// Rules: rules/foundations/names_scopes_visibility.md — §§12.1–12.3;
// rules/declarations/properties.md — §§2,4–5.
func TestPropertyVisibility(t *testing.T) {
	for _, tc := range []struct {
		files  []string
		errors int
	}{
		{[]string{"valid.sec", "sibling.sec"}, 0},
		{[]string{"owner_invalid.sec"}, 6},
		{[]string{"valid.sec", "module_invalid.sec"}, 4},
		{[]string{"core_owner.sec"}, 0},
		{[]string{"core_owner.sec", "core_consumer_invalid.sec"}, 2},
	} {
		program := &ast.Program{SourceProvenance: map[string]ast.SourceProvenance{}}
		for _, name := range tc.files {
			file := "../../testdata/sema/property_visibility/" + name
			data, err := os.ReadFile(file)
			if err != nil {
				t.Fatal(err)
			}
			p := parser.New(lexer.NewWithFile(string(data), file))
			part := p.ParseProgram()
			if len(p.Errors()) != 0 {
				t.Fatal(name, p.Errors())
			}
			program.Statements = append(program.Statements, part.Statements...)
			if name == "core_owner.sec" {
				program.SourceProvenance[file] = ast.SourceCore
			}
		}
		errors := NewAnalyzer().Analyze(program)
		if len(errors) != tc.errors {
			t.Fatalf("%v: %v", tc.files, errors)
		}
		for _, d := range errors {
			if d.ID != diagnostics.PropertyNotVisible || d.PreviousLine == 0 || d.RelatedLocationLabel() != "property declared here" || d.Help == "" {
				t.Fatalf("missing visibility provenance: %+v", d)
			}
		}
	}
}
