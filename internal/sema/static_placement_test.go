package sema

import (
	"os"
	"sec/internal/diagnostics"
	"sec/internal/lexer"
	"sec/internal/parser"
	"strings"
	"testing"
)

// Static lifetime and type association grant no physical address, memory
// space, MMIO authority or instance layout. Ordinary register snapshots remain
// ordinary storage; an explicit hardware endpoint is a different mechanism.
// Rules: rules/declarations/static.md — §§6–7,22;
// rules/foundations/attributes.md — "Purpose" (physical placement boundary).
func TestStaticDoesNotSelectPhysicalPlacement(t *testing.T) {
	data, err := os.ReadFile("../../testdata/sema/static_placement/valid.sec")
	if err != nil {
		t.Fatal(err)
	}
	a, errors := analyzeSourceWithAnalyzerRaw(t, string(data))
	if len(errors) != 0 {
		t.Fatal(errors)
	}
	for _, name := range []string{"Global", "Explicit", "Mutable", "OrdinarySnapshot", "Counter.Limit", "Counter.Total"} {
		symbol, ok := a.symbols[name]
		if !ok || symbol.Storage != StorageOriginStatic || symbol.RegisterAccess != "" {
			t.Fatalf("static symbol: %s %+v", name, symbol)
		}
		place, ok := a.rootPlace(name)
		if !ok || place.AddressSpace != AddressSpaceOrdinary {
			t.Fatalf("static acquired physical space: %s %+v", name, place)
		}
	}
	if len(a.ResolvedHardwareBindings()) != 0 {
		t.Fatal("static created a hardware binding")
	}
	counter := a.Types()["Counter"]
	if len(counter.Fields) != 1 || counter.Fields[0].Name != "field" {
		t.Fatalf("static changed instance layout: %+v", counter.Fields)
	}
	for _, static := range a.ResolvedStaticDeclarations() {
		if static.Declaration.Address != nil {
			t.Fatalf("static acquired a fixed address: %+v", static)
		}
	}
}

// static does not extend the closed compiler-known attribute set, including
// at callable-local declaration sites.
// Rules: rules/declarations/static.md — §22; rules/foundations/attributes.md — "Initial compiler-known attribute set".
func TestStaticDoesNotAuthorizePlacementAttributes(t *testing.T) {
	data, err := os.ReadFile("../../testdata/sema/static_placement/placement_invalid.sec")
	if err != nil {
		t.Fatal(err)
	}
	result := parser.New(lexer.New(string(data))).Parse()
	if len(result.Diagnostics) != 2 {
		t.Fatal(result.Diagnostics)
	}
	for _, d := range result.Diagnostics {
		if d.ID != diagnostics.UnknownAttribute {
			t.Fatalf("unexpected placement diagnostic: %+v", d)
		}
	}
	// In a callable, unknown @name(...) syntax is a runtime-call expression,
	// not an attribute; it must still reject this unregistered operation.
	errors := NewAnalyzer().Analyze(result.Program)
	if len(errors) != 1 || !strings.Contains(errors[0].Message, "unknown runtime function @section") {
		t.Fatal(errors)
	}

}
