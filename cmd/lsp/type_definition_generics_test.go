package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// genericNavigationFixture builds a real imported module so constraints and
// specialization arguments keep canonical declaration URIs and source ranges.
// Rules: rules/tooling/lsp.md — "Navigation and references", A.15;
// rules/declarations/generics.md — §§12–13,30.
func genericNavigationFixture(t *testing.T, name string) (uri, source, catalogURI, catalog string) {
	t.Helper()
	root := t.TempDir()
	for _, dir := range []string{".sec", "catalog"} {
		if err := os.Mkdir(filepath.Join(root, dir), 0755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(root, ".sec", "sec.toml"), []byte("[project]\nname = \"generic-navigation\"\n"), 0600); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile("../../testdata/lsp/generic_type_navigation/catalog.sec")
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, "catalog", "catalog.sec")
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
	catalogURI, catalog = uriFromPath(path), string(data)
	data, err = os.ReadFile("../../testdata/lsp/generic_type_navigation/" + name + ".sec")
	if err != nil {
		t.Fatal(err)
	}
	return uriFromPath(filepath.Join(root, "main.sec")), string(data), catalogURI, catalog
}

// TestGenericTypeDefinitionNavigation selects each explicit constraint and
// specialized type component independently. Inferred/call/member values keep
// their resolved nominal type, rather than arbitrarily selecting an argument.
// Rules: rules/tooling/lsp.md — "Navigation and references", A.15;
// rules/declarations/generics.md — §§12–13,30.
func TestGenericTypeDefinitionNavigation(t *testing.T) {
	uri, source, catalogURI, catalog := genericNavigationFixture(t, "use")
	for _, diagnostic := range analyze(uri, source) {
		if diagnostic.Severity == 1 {
			t.Fatalf("invalid source fixture: %+v", diagnostic)
		}
	}
	for _, test := range []struct{ marker, selected, declaration string }{
		{"T: catalog.Named", "Named", "interface Named"},
		{"& catalog.Ranked", "Ranked", "interface Ranked"},
		{"& catalog.Source[catalog.Item]", "Source", "interface Source"},
		{"& catalog.Source[catalog.Item]", "Item", "type Item"},
		{"box: catalog.Box[catalog.Item]", "Box", "type Box"},
		{"box: catalog.Box[catalog.Item]", "Item", "type Item"},
		{"pair: catalog.Pair[catalog.Item, catalog.Box[catalog.Item]]", "Pair", "type Pair"},
		{"pair: catalog.Pair[catalog.Item, catalog.Box[catalog.Item]]", "Box", "type Box"},
		{"constrained: catalog.Constrained[catalog.Item]", "Constrained", "type Constrained"},
		{"constrained: catalog.Constrained[catalog.Item]", "Item", "type Item"},
		{"catalog.Wrap[catalog.Item](value)", "Item", "type Item"},
		{"catalog.Wrap(value)", "Wrap", "type Box"},
		{"catalog.Wrap[catalog.Item](value)", "Wrap", "type Box"},
		{"box.Value", "Value", "type Item"},
		{"discard inferred", "inferred", "type Box"},
		{"discard explicit", "explicit", "type Box"},
		{"discard payload", "payload", "type Item"},
	} {
		t.Run(test.marker+"/"+test.selected, func(t *testing.T) {
			offset := strings.Index(source, test.marker) + strings.Index(test.marker, test.selected)
			locations := typeDefinitionsForSource(uri, source, offsetPosition(source, offset+1))
			want := offsetPosition(catalog, strings.Index(catalog, test.declaration)+strings.IndexByte(test.declaration, ' ')+1)
			if len(locations) != 1 || locations[0].URI != catalogURI || locations[0].Range.Start != want {
				t.Fatalf("locations=%+v; want %s at %+v", locations, catalogURI, want)
			}
		})
	}
	for _, marker := range []string{"generic: T", "discard generic", "discard compound"} {
		selected := "generic"
		if marker == "generic: T" {
			selected = "T"
		}
		if marker == "discard compound" {
			selected = "compound"
		}
		offset := strings.Index(source, marker) + strings.Index(marker, selected)
		for _, target := range typeDefinitionsForSource(uri, source, offsetPosition(source, offset+1)) {
			if target.URI == catalogURI {
				t.Fatalf("generic/compound value guessed a constraint or argument: %s %+v", marker, target)
			}
		}
	}
	offset := strings.Index(source, "generic: T") + len("generic: ")
	declarations := definitionsForSource(uri, source, offsetPosition(source, offset))
	wantParameter := offsetPosition(source, strings.Index(source, "Inspect[T:")+len("Inspect["))
	if len(declarations) != 1 || declarations[0].URI != uri || declarations[0].Range.Start != wantParameter {
		t.Fatalf("ordinary definition must retain the generic parameter declaration: %+v", declarations)
	}
	for _, name := range []string{"Named", "Ranked"} {
		constraint := "T: Named & Ranked"
		offset := strings.Index(catalog, constraint) + strings.Index(constraint, name)
		targets := typeDefinitionsForSource(catalogURI, catalog, offsetPosition(catalog, offset+1))
		want := offsetPosition(catalog, strings.Index(catalog, "interface "+name)+len("interface "))
		if len(targets) != 1 || targets[0].URI != catalogURI || targets[0].Range.Start != want {
			t.Fatalf("generic type declaration constraint %s lost its target: %+v", name, targets)
		}
	}
}

// TestGenericConstraintNavigationDoesNotGuessSameSpelledType withholds a
// missing qualified constraint even if an ordinary local type shares its name.
// Rules: rules/tooling/lsp.md — "Navigation and references";
// rules/foundations/names_scopes_visibility.md — resolved declaration identity.
func TestGenericConstraintNavigationDoesNotGuessSameSpelledType(t *testing.T) {
	uri, source, _, _ := genericNavigationFixture(t, "unresolved_invalid")
	hasError := false
	for _, diagnostic := range analyze(uri, source) {
		hasError = hasError || diagnostic.Severity == 1
	}
	if !hasError {
		t.Fatal("unresolved constraint unexpectedly accepted")
	}
	offset := strings.Index(source, "catalog.Absent") + len("catalog.")
	if targets := typeDefinitionsForSource(uri, source, offsetPosition(source, offset+1)); len(targets) != 0 {
		t.Fatal("guessed local Absent declaration", targets)
	}
	offset = strings.Index(source, "discard value") + len("discard ")
	if targets := typeDefinitionsForSource(uri, source, offsetPosition(source, offset+1)); len(targets) != 0 {
		t.Fatal("generic parameter guessed the shadowed concrete T", targets)
	}
}
