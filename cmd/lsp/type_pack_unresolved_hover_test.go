package main

import (
	"os"
	"strings"
	"testing"
)

// Unresolved calls show visible declared pack templates, with receiving modes
// intact and unknown inferred types explicitly unavailable. Fixed/mixed calls
// keep ordinary hover, and resolved pack calls retain concrete facts.
// Rules: rules/declarations/functions.md — §38A(10)(3);
// rules/tooling/lsp.md — §12(5)-(7).
func TestTypePackUnresolvedCallHover(t *testing.T) {
	data, err := os.ReadFile("../../testdata/lsp/type_pack_hover/unresolved_invalid.sec")
	if err != nil {
		t.Fatal(err)
	}
	source := string(data)
	for _, test := range []struct{ marker, label string }{
		{`Shared("a", missing)`, "fn Shared[...Types](prefix: string, values: ...ref Types) void"},
		{`Context[int](1, missing)`, "fn Context[T, ...Types](head: T, values: ...ref Types) T"},
		{`Mutable(missing)`, "fn Mutable[...Types](values: ...ref mut Types) void"},
		{`Owned(<-missing)`, "fn Owned[...Types](values: ...Types) void"},
		{`Count(missing)`, "fn Box.Count[...Types](values: ...ref Types) uint"},
		{`Trace("a", missing)`, "fn Box.Trace[...Types](prefix: string, values: ...Types) void"},
	} {
		t.Run(test.marker, func(t *testing.T) {
			at := strings.Index(source, test.marker)
			if at < 0 {
				t.Fatal(test.marker)
			}
			hover, ok := hoverForSource("file:///tmp/pack-hover.sec", source, offsetPosition(source, at+1))
			if !ok || !strings.Contains(hover.Contents.Value, test.label) || !strings.Contains(hover.Contents.Value, "not resolved") {
				t.Fatalf("found=%v hover=%+v", ok, hover)
			}
			for _, forbidden := range []string{"Inferred type pack:", "Pack receiving types:", "Types = ("} {
				if strings.Contains(hover.Contents.Value, forbidden) {
					t.Fatalf("invented inference: %s", hover.Contents.Value)
				}
			}
			if hover.Range.Start != offsetPosition(source, at) {
				t.Fatalf("wrong callee range: %+v", hover.Range)
			}
		})
	}
	for _, marker := range []string{"Ordinary(missing)", "Mixed(missing)"} {
		at := strings.Index(source, marker)
		hover, ok := hoverForSource("file:///tmp/pack-hover.sec", source, offsetPosition(source, at+1))
		if ok && (strings.Contains(hover.Contents.Value, "not resolved") || strings.Contains(hover.Contents.Value, "Inferred type pack:")) {
			t.Fatalf("ordinary/mixed call acquired pack fallback: %s", hover.Contents.Value)
		}
	}
	at := strings.Index(source, `Shared("a", 1, true)`)
	hover, ok := hoverForSource("file:///tmp/pack-hover.sec", source, offsetPosition(source, at+1))
	if !ok || !strings.Contains(hover.Contents.Value, "Types = (int, bool)") || strings.Contains(hover.Contents.Value, "not resolved") {
		t.Fatalf("resolved facts replaced: found=%v hover=%+v", ok, hover)
	}
}

// Incomplete calls retain declared hover, and ambiguous pack overloads remain
// a set of declarations rather than a fabricated selected specialization.
// Rules: rules/declarations/functions.md — §38A(4),(10)(3);
// rules/tooling/lsp.md — §12(5)-(7).
func TestTypePackIncompleteAndAmbiguousHover(t *testing.T) {
	for _, test := range []struct {
		file, marker string
		labels       []string
	}{
		{"incomplete_invalid.sec", `Shared("a", missing,`, []string{"fn Shared[...Types](prefix: string, values: ...ref Types) void"}},
		{"unresolved_invalid.sec", `Select(missing)`, []string{"fn Select[...Types](prefix: int, values: ...ref Types) void", "fn Select[...Types](prefix: string, values: ...ref Types) void"}},
	} {
		data, err := os.ReadFile("../../testdata/lsp/type_pack_hover/" + test.file)
		if err != nil {
			t.Fatal(err)
		}
		source := string(data)
		at := strings.Index(source, test.marker)
		if at < 0 {
			t.Fatal(test.marker)
		}
		hover, ok := hoverForSource("file:///tmp/pack-hover.sec", source, offsetPosition(source, at+1))
		if !ok || !strings.Contains(hover.Contents.Value, "not resolved") || strings.Contains(hover.Contents.Value, "Inferred type pack:") {
			t.Fatalf("%s: found=%v hover=%+v", test.file, ok, hover)
		}
		for _, label := range test.labels {
			if !strings.Contains(hover.Contents.Value, label) {
				t.Fatalf("missing candidate %s: %s", label, hover.Contents.Value)
			}
		}
	}
}
