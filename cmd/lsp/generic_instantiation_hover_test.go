package main

import (
	"os"
	"strings"
	"testing"
)

// TestGenericInstantiationHover verifies declaration-ordered concrete bindings
// at uses while declaration constraints and symbolic uses remain unspecialized.
// Rules: rules/tooling/lsp.md — §§12.1(3),18(2);
// rules/declarations/generics.md — §§18–20,25–26,30.
func TestGenericInstantiationHover(t *testing.T) {
	enableAnalysisCache()
	t.Cleanup(disableAnalysisCache)
	data, err := os.ReadFile("../../testdata/lsp/generic_instantiation_hover/main.sec")
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	const uri = "file:///tmp/generic-instantiation-hover/main.sec"
	for _, test := range []struct{ use, token, want string }{
		{"Pick(1, true)", "Pick", "Z = int, A = bool"},
		{"Pick[bool, int]", "Pick", "Z = bool, A = int"},
		{"Pick[bool, int]", "bool", "Z = bool, A = int"},
		{"Pick[bool]", "Pick", "Z = bool, A = int"},
		{"Identity(4)", "Identity", "T = int"},
		{"box.Keep()", "Keep", "Z = int"},
		{"box.Select[bool]", "bool", "Z = int, A = bool"},
		{"ref Box[int]", "Box", "Z = int"},
		{"box: ref Box[int]", "box", "Z = int"},
		{"ref Box[int]", "int", "Z = int"},
		{"Choice[bool]", "Choice", "C = bool"},
		{"Flag[int]", "Flag", "F = int"},
		{"Source[int]", "Source", "S = int"},
		{"Alias[int]", "Alias", "A = int"},
		{"Box[Box[int]]", "Box", "Z = Box[int]"},
		{"Box[int]]", "Box", "Z = int"},
	} {
		t.Run(test.use+"/"+test.token, func(t *testing.T) {
			offset := strings.Index(text, test.use)
			if offset < 0 {
				t.Fatal("missing test use", test.use)
			}
			offset += strings.Index(test.use, test.token)
			hover, ok := hoverForSource(uri, text, offsetPosition(text, offset))
			if !ok || !strings.Contains(hover.Contents.Value, "Type arguments: `"+test.want+"`") {
				t.Fatalf("hover = %+v (%v), want %q", hover, ok, test.want)
			}
			if hover.Range != offsetsRange(text, offset, offset+len(test.token)) {
				t.Fatalf("range = %+v", hover.Range)
			}
		})
	}
	for _, use := range []string{"fn Pick[", "Identity(<-value)", "Plain(5)"} {
		offset := strings.Index(text, use)
		if strings.HasPrefix(use, "fn ") {
			offset += len("fn ")
		}
		hover, _ := hoverForSource(uri, text, offsetPosition(text, offset))
		if strings.Contains(hover.Contents.Value, "Type arguments:") {
			t.Fatalf("invented specialization for %s: %+v", use, hover)
		}
	}
	invalid := strings.Replace(text, "Pick(1, true)", "Pick[bool, int, string](1, true)", 1)
	offset := strings.Index(invalid, "Pick[bool, int, string]")
	hover, _ := hoverForSource(uri, invalid, offsetPosition(invalid, offset))
	if strings.Contains(hover.Contents.Value, "Type arguments:") {
		t.Fatalf("invented binding for rejected instantiation: %+v", hover)
	}
}

// TestGenericInstantiationHoverImported retains defining-module bindings and
// nested argument identity while using the same import resolver as navigation.
// Rules: rules/tooling/lsp.md — §18(2);
// rules/declarations/generics.md — §§4,25–26,30.
func TestGenericInstantiationHoverImported(t *testing.T) {
	enableAnalysisCache()
	t.Cleanup(disableAnalysisCache)
	uri, source, _, _ := genericNavigationFixture(t, "use")
	for _, test := range []struct{ use, token, parameter string }{
		{"catalog.Wrap(value)", "Wrap", "T"},
		{"catalog.Wrap[catalog.Item]", "Item", "T"},
		{"box: catalog.Box[catalog.Item]", "Box", "T"},
	} {
		offset := strings.Index(source, test.use) + strings.Index(test.use, test.token)
		hover, ok := hoverForSource(uri, source, offsetPosition(source, offset))
		if !ok || !strings.Contains(hover.Contents.Value, "Type arguments: `"+test.parameter+" = ") || !strings.Contains(hover.Contents.Value, "Item") {
			t.Fatalf("%s: hover = %+v (%v)", test.use, hover, ok)
		}
	}
}
