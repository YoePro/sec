package sema

import (
	"testing"

	"sec/internal/diagnostics"
)

// Rules: rules/foundations/lexical_structure.md §§7–9 and
// rules/foundations/names_scopes_visibility.md §9.
func TestReservedDeclarationNamesAcrossNamespaces(t *testing.T) {
	tests := []struct {
		name   string
		source string
	}{
		{"module", "module task\n"},
		{"import alias", "module main\nimport map \"dependency\"\n"},
		{"variable", "module main\nlet int := 1\n"},
		{"function", "module main\nfn map() void {}\n"},
		{"parameter", "module main\nfn Use(task: int) void {}\n"},
		{"generic parameter", "module main\ntype Box[even] struct {}\n"},
		{"field", "module main\ntype Holder struct { process: int, }\n"},
		{"enum member", "module main\nenum State { finite, }\n"},
		{"loop binding", "module main\nfn Use(values: int[]) void { for thread in values {} }\n"},
		{"assembly output", "module main\nunsafe fn Use() int { asm { \"nop\" outputs: rax(map) } return map }\n"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			errors := analyzeSourceRaw(t, test.source)
			count := 0
			for _, diagnostic := range errors {
				if diagnostic.ID == diagnostics.ReservedDeclarationName {
					count++
				}
			}
			if count != 1 {
				t.Fatalf("reserved-name diagnostics = %d in %#v, want 1", count, errors)
			}
		})
	}
}

func TestCanonicalIdentifierLikeReservedSpellingsAreRejected(t *testing.T) {
	for _, name := range []string{
		"int", "map", "task", "thread", "process", "even", "finite",
		"multipleOf", "notEmpty", "odd", "unique",
	} {
		t.Run(name, func(t *testing.T) {
			source := "module main\nlet " + name + " := 1\n"
			errors := analyzeSourceRaw(t, source)
			if len(errors) == 0 || errors[0].ID != diagnostics.ReservedDeclarationName {
				t.Fatalf("errors = %#v, want leading %s", errors, diagnostics.ReservedDeclarationName)
			}
		})
	}
}

func TestContextualSetRemainsAvailableAsDeclarationName(t *testing.T) {
	source := `module main

fn set(set: int) int {
    let set := 1
    return set
}
`
	for _, diagnostic := range analyzeSourceRaw(t, source) {
		if diagnostic.ID == diagnostics.ReservedDeclarationName {
			t.Fatalf("contextual set was rejected as reserved: %#v", diagnostic)
		}
	}
}
