package sema

import (
	"os"
	"strings"
	"testing"

	"sec/internal/ast"
	"sec/internal/diagnostics"
	"sec/internal/lexer"
)

func TestNominalTypeCapitalization(t *testing.T) {
	for _, test := range []struct {
		file    string
		invalid []string
	}{
		{"nominal_capitalization_valid.sec", nil},
		{"nominal_capitalization_invalid.sec", []string{
			"percent", "person", "response", "color", "reader", "_state", "__private", "ångström", "box", "nested", "nestedColor",
		}},
	} {
		t.Run(test.file, func(t *testing.T) {
			source, err := os.ReadFile("../../testdata/names/" + test.file)
			if err != nil {
				t.Fatal(err)
			}
			_, errors := analyzeSourceWithAnalyzer(t, string(source))
			if len(errors) != len(test.invalid) {
				t.Fatalf("errors = %v, want %d naming errors", errors, len(test.invalid))
			}
			for _, name := range test.invalid {
				count := 0
				for _, diagnostic := range errors {
					if strings.Contains(diagnostic.Message, "nominal type "+name+" must begin with an uppercase letter") {
						count++
						if diagnostic.ID != diagnostics.InvalidNominalTypeName || diagnostic.Line == 0 || diagnostic.Column == 0 || diagnostic.Help == "" {
							t.Errorf("missing diagnostic metadata: %+v", diagnostic)
						}
					}
				}
				if count != 1 {
					t.Errorf("%s: got %d diagnostics, want exactly one", name, count)
				}
			}
		})
	}
}

// Qualified internal names retain the source declaration's capitalization
// rule without treating a lowercase module name as part of the type name.
// Rules: rules/foundations/names_scopes_visibility.md — §6, §13.
func TestQualifiedNominalTypeCapitalization(t *testing.T) {
	for _, test := range []struct {
		name    string
		invalid bool
	}{
		{name: "io.IOError"},
		{name: "linux._StandardStream"},
		{name: "io.Owner.Nested"},
		{name: "io.lowercase", invalid: true},
		{name: "io.Owner._private", invalid: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			a := NewAnalyzer()
			a.validateNominalTypeName(&ast.Identifier{
				Value: test.name,
				Token: lexer.Token{Lexeme: test.name, Line: 1, Column: 6},
			})
			if !test.invalid && len(a.errors) != 0 {
				t.Fatalf("unexpected naming errors: %v", a.errors)
			}
			if test.invalid && (len(a.errors) != 1 || a.errors[0].ID != diagnostics.InvalidNominalTypeName) {
				t.Fatalf("naming errors = %v, want one S1038 diagnostic", a.errors)
			}
		})
	}
}
