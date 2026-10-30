package main

import (
	"os"
	"strings"
	"testing"

	"sec/internal/diagnostics"
	"sec/internal/sema"
)

// Only the known pack implementation boundary has an informational editor
// presentation; actual ownership errors retain mandatory Error severity.
// Rules: rules/tooling/diagnostics.md §24(4h).
func TestTypePackCapabilityPresentationOnly(t *testing.T) {
	for _, test := range []struct {
		id       string
		severity int
	}{
		{diagnostics.GenericTypePackExpansionUnavailable, 3},
		{diagnostics.MoveIntoBorrowedParameter, 1},
	} {
		d := semaDiagnostic(sema.Error{ID: test.id, Line: 1, Column: 1}, 3, "x")
		if d.Code != test.id || d.Severity != test.severity {
			t.Fatalf("%s: %+v", test.id, d)
		}
	}
}

// TestTypePackDeclarationTooling projects the compiler's declaration facts,
// preserving owned/shared/mutable signature modes without claiming inference.
// Rules: rules/declarations/generics.md — §16A; MD-117 — §10(3).
func TestTypePackDeclarationTooling(t *testing.T) {
	data, err := os.ReadFile("../../testdata/generics/type_pack_declarations/declarations.sec")
	if err != nil {
		t.Fatal(err)
	}
	source := string(data)
	uri := ""
	for _, test := range []struct{ marker, want string }{
		{"Owned[", "fn Owned[...Types](values: ...Types) void"},
		{"Shared[", "fn Shared[...Types](values: ...ref Types) void"},
		{"Mutable[", "fn Mutable[...Types](values: ...ref mut Types) void"},
		{"Mixed[", "fn Mixed[T, ...Rest](head: T, values: ...Rest) void"},
		{"Method[", "fn Container.Method[...Items](values: ...Items) void"},
	} {
		at := strings.Index(source, test.marker)
		hover, ok := hoverForSource(uri, source, offsetPosition(source, at+1))
		if !ok || !strings.Contains(hover.Contents.Value, test.want) {
			t.Fatalf("%s hover=%+v found=%v", test.marker, hover, ok)
		}
	}
	at := strings.Index(source, "...Types") + 3
	hover, ok := hoverForSource(uri, source, offsetPosition(source, at))
	if !ok || !strings.Contains(hover.Contents.Value, "generic ...Types") || !strings.Contains(hover.Contents.Value, "zero or more") {
		t.Fatal(hover)
	}
	items := analyze(uri, source)
	if len(items) != 4 {
		t.Fatal(items)
	}
	for _, d := range items {
		if d.Code != diagnostics.GenericTypePackExpansionUnavailable || d.Severity != 3 || !strings.Contains(d.Message, "compiler capability unavailable") || !strings.Contains(d.Message, "help:") {
			t.Fatal(d)
		}
	}
}

// Call hover consumes ordered Sema facts, including empty packs and reborrows,
// without deriving a parallel type-pack model in the LSP.
// Rules: rules/declarations/functions.md §38A(10)(3); rules/tooling/lsp.md — "Hover".
func TestTypePackCallHover(t *testing.T) {
	data, err := os.ReadFile("../../testdata/generics/type_pack_declarations/shared_unused.sec")
	if err != nil {
		t.Fatal(err)
	}
	source := string(data)
	for _, test := range []struct{ marker, types, modes string }{
		{`Printf("hello")`, "Types = ()", "Pack receiving types: `()`"},
		{`Printf("values"`, "Types = (int, bool, string)", "(ref int, ref bool, ref string)"},
		{`Printf("reordered"`, "Types = (bool, int)", "(ref bool, ref int)"},
		{`Printf("references"`, "Types = (int)", "(ref int)"},
		{`Printf("reborrow"`, "Types = (int)", "(ref int)"},
	} {
		at := strings.Index(source, test.marker)
		if at < 0 {
			t.Fatal(test.marker)
		}
		hover, ok := hoverForSource("", source, offsetPosition(source, at+1))
		if !ok || !strings.Contains(hover.Contents.Value, test.types) || !strings.Contains(hover.Contents.Value, test.modes) {
			t.Fatalf("%s: found=%v hover=%+v", test.marker, ok, hover)
		}
	}
}
