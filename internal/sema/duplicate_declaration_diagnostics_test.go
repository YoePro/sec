package sema

import (
	"strings"
	"testing"

	"sec/internal/diagnostics"
	"sec/internal/lexer"
	"sec/internal/parser"
)

// Duplicate and conflicting declarations report a stable registered
// diagnostic that identifies both the new declaration and the earlier one it
// conflicts with: duplicate struct fields, enum values and interface
// requirements and a property named like a field are S1221
// declarations.duplicate-declaration; a duplicate parameter of a function or
// lambda is S1004 names.duplicate-local-variable.
//
// Rules:
//   - rules/foundations/names_scopes_visibility.md — duplicate declarations
//   - rules/tooling/diagnostics.md — stable identities and related locations
func TestDuplicateDeclarationsNameBothDeclarations(t *testing.T) {
	for _, check := range []struct {
		name, source, id, message string
		line, previousLine        int
	}{
		{"field", "type U struct {\n    x: int,\n    x: int,\n}", diagnostics.DuplicateDeclaration, `duplicate field "x" in struct U`, 5, 4},
		{"enum value", "enum E {\n    a,\n    a,\n}", diagnostics.DuplicateDeclaration, `duplicate enum value "a" in enum E`, 5, 4},
		{"interface requirement", "interface I {\n    fn M() void\n    fn M() void\n}", diagnostics.DuplicateDeclaration, `duplicate interface method requirement "M" in I`, 5, 4},
		{"property and field", "type X struct {\n    x: int,\n}\n\nimpl X {\n    property x: int {\n        get {\n            return 1\n        }\n    }\n}", diagnostics.DuplicateDeclaration, "property x conflicts with field x in X", 8, 4},
		{"parameter", "fn D(\n    x: int,\n    x: int,\n) void {\n}", diagnostics.DuplicateLocalVariable, `duplicate parameter "x"`, 5, 4},
		{"lambda parameter", "fn K() void {\n    let f := fn(x: int, x: int) int {\n        return x\n    }\n    discard f\n}", diagnostics.DuplicateLocalVariable, `duplicate parameter "x"`, 4, 4},
	} {
		t.Run(check.name, func(t *testing.T) {
			parsed := parser.New(lexer.NewWithFile("module main\n\n"+check.source+"\n", "dup.sec")).Parse()
			if parsed.HasErrors {
				t.Fatal(parsed.Diagnostics)
			}
			var found *Error
			errors := NewAnalyzer().Analyze(parsed.Program)
			for index := range errors {
				if strings.Contains(errors[index].Message, check.message) {
					found = &errors[index]
				}
			}
			if found == nil {
				t.Fatalf("no %q diagnostic in %v", check.message, errors)
			}
			if found.ID != check.id || found.Line != check.line || found.PreviousLine != check.previousLine || found.PreviousFile != "dup.sec" {
				t.Fatalf("diagnostic = %s at line %d, previous %s:%d; want %s at line %d, previous line %d", found.ID, found.Line, found.PreviousFile, found.PreviousLine, check.id, check.line, check.previousLine)
			}
		})
	}
}
