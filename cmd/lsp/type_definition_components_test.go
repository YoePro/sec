package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Deliberately selected type components retain their own declaration in every
// wrapper. The module alias, bracket/extent and an ambiguous container value
// must never stand in for the element's nominal source identity.
// Rules: rules/tooling/lsp.md — "Navigation and references", A.15;
// rules/types/types.md — "Generic and parameterized types", "Safe references".
func TestTypeDefinitionCompoundComponents(t *testing.T) {
	catalogBytes, err := os.ReadFile("../../testdata/lsp/type_components/catalog.sec")
	if err != nil {
		t.Fatal(err)
	}
	useBytes, err := os.ReadFile("../../testdata/lsp/type_components/use.sec")
	if err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	for _, dir := range []string{".sec", "catalog"} {
		if err := os.Mkdir(filepath.Join(root, dir), 0755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(root, ".sec", "sec.toml"), []byte("[project]\nname = \"type-components\"\n"), 0644); err != nil {
		t.Fatal(err)
	}
	catalogFile := filepath.Join(root, "catalog", "catalog.sec")
	if err := os.WriteFile(catalogFile, catalogBytes, 0644); err != nil {
		t.Fatal(err)
	}
	catalog := string(catalogBytes)
	source := string(useBytes)
	uri := uriFromPath(filepath.Join(root, "main.sec"))
	if parsed := parseProgramForLSP(uri, source); parsed == nil {
		t.Fatal("component fixture must parse")
	}
	if diagnostics := analyze(uri, source); len(diagnostics) != 0 {
		t.Fatalf("fixture must type-check: %+v", diagnostics)
	}
	for _, tc := range []struct{ line, selected, declaration string }{
		{"generic: catalog.Box[catalog.Item]", "Box", "type Box"}, {"generic: catalog.Box[catalog.Item]", "Item", "type Item"},
		{"result: Result[catalog.Item, catalog.Failure]", "Item", "type Item"}, {"result: Result[catalog.Item, catalog.Failure]", "Failure", "enum Failure"},
		{"option: Option[catalog.Item]", "Item", "type Item"}, {"reference: ref catalog.Item", "Item", "type Item"},
		{"mutable: ref mut catalog.Item", "Item", "type Item"}, {"array: catalog.Item[2]", "Item", "type Item"},
		{"dynamic: catalog.Item[]", "Item", "type Item"}, {"slice: ref catalog.Item[]", "Item", "type Item"},
		{"listValue: list[catalog.Item]", "Item", "type Item"}, {"mapValue: map[catalog.Number, catalog.Item]", "Number", "type Number"},
		{"mapValue: map[catalog.Number, catalog.Item]", "Item", "type Item"}, {"setValue: set[catalog.Number]", "Number", "type Number"},
		{"vectorValue: vector[catalog.Number, 3]", "Number", "type Number"}, {"matrixValue: matrix[catalog.Number, 2, 3]", "Number", "type Number"},
		{"tensorValue: tensor[catalog.Number, 2]", "Number", "type Number"}, {"view: tensor_view[catalog.Number, 2]", "Number", "type Number"},
		{"nested: Option[list[catalog.Box[catalog.Item]]]", "Box", "type Box"}, {"nested: Option[list[catalog.Box[catalog.Item]]]", "Item", "type Item"},
	} {
		t.Run(tc.line+"/"+tc.selected, func(t *testing.T) {
			start := strings.Index(source, tc.line)
			offset := start + strings.Index(tc.line, tc.selected)
			got := typeDefinitionsForSource(uri, source, offsetPosition(source, offset))
			want := offsetPosition(catalog, strings.Index(catalog, tc.declaration)+strings.Index(tc.declaration, " ")+1)
			if len(got) != 1 || got[0].URI != uriFromPath(catalogFile) || got[0].Range.Start != want {
				t.Fatalf("definition=%+v, want %s at %+v", got, uriFromPath(catalogFile), want)
			}
		})
	}
	for _, marker := range []string{"generic: catalog.Box", "array: catalog.Item[2]", "matrixValue: matrix[catalog.Number, 2, 3]"} {
		start := strings.Index(source, marker)
		var offset int
		if strings.HasPrefix(marker, "generic:") {
			offset = start + len("generic: ")
		} else if strings.HasPrefix(marker, "array:") {
			offset = start + strings.Index(marker, "2]")
		} else {
			offset = start + strings.Index(marker, "3]")
		}
		if got := typeDefinitionsForSource(uri, source, offsetPosition(source, offset)); len(got) != 0 {
			t.Fatalf("qualifier/extent guessed component declaration: %+v", got)
		}
	}
	for _, binding := range []string{"result", "nested"} {
		cursor := strings.Index(source, binding+":")
		for _, target := range typeDefinitionsForSource(uri, source, offsetPosition(source, cursor)) {
			if target.URI == uriFromPath(catalogFile) {
				t.Fatalf("compound binding selected a catalog element: %+v", target)
			}
		}
	}

}

// Unresolved explicit components cannot fall back to a same-spelled local
// type. This request does not guess one element for a compound value.
// Rules: rules/tooling/lsp.md — "Navigation and references", A.15;
// rules/foundations/names_scopes_visibility.md — resolved declaration identity.
func TestTypeDefinitionUnresolvedComponentHasNoGuessedTarget(t *testing.T) {
	input, err := os.ReadFile("../../testdata/lsp/type_components/unresolved_invalid.sec")
	if err != nil {
		t.Fatal(err)
	}
	source := string(input)
	uri := uriFromPath(filepath.Join(t.TempDir(), "main.sec"))
	if diagnostics := analyze(uri, source); len(diagnostics) == 0 {
		t.Fatal("unresolved type fixture must fail analysis")
	}
	cursor := strings.Index(source, "missing.Item") + len("missing.")
	if got := typeDefinitionsForSource(uri, source, offsetPosition(source, cursor)); len(got) != 0 {
		t.Fatalf("unresolved component guessed declaration: %+v", got)
	}
}
