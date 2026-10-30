package main

import (
	"path/filepath"
	"strings"
	"testing"
)

// TestRegisterShadowKnownLSP derives IsShadowKnown completion and hover from
// the compiler-known registry and Sema's selector resolution, and navigates
// from a selector step to the declared field.
// Rules: rules/tooling/lsp.md — "Register shadow knowledge (MD-113)";
// rules/declarations/registers.md — § 11.7.
func TestRegisterShadowKnownLSP(t *testing.T) {
	source := `module main

type InterruptBits register[4] {
    Pending: bit[4] write-one-clear shadow,
}

type Control register[8] {
    Mode: bit[3] shadow,
    IRQ: InterruptBits,
    _: bit,
}

impl Control {
    fn HasPending() bool {
        return self.IsShadowKnown(Control.IRQ.Pending)
    }
}

@address(0x40000000)
let mut control: Control

fn Use() bool {
    return control.HasPending()
}
`
	uri := uriFromPath(filepath.Join(t.TempDir(), "main.sec"))
	for _, diagnostic := range analyze(uri, source) {
		if diagnostic.Severity == 1 {
			t.Fatalf("unexpected error: %+v", diagnostic)
		}
	}
	hover, ok := hoverForSource(uri, source, offsetPosition(source, strings.Index(source, "IsShadowKnown")))
	if !ok || !strings.Contains(hover.Contents.Value, "fn IsShadowKnown(Control.IRQ.Pending) bool") || !strings.Contains(hover.Contents.Value, "Performs no hardware access") ||
		!strings.Contains(hover.Contents.Value, "Selected field: `Pending` (write-one-clear, shadow)") {
		t.Fatal(hover, ok)
	}
	locations := definitionsForSource(uri, source, offsetPosition(source, strings.Index(source, "Pending)")))
	if len(locations) != 1 || locations[0].Range.Start.Line != 3 {
		t.Fatal(locations)
	}
	completion := strings.Replace(source, "self.IsShadowKnown(Control.IRQ.Pending)", "self.IsSh", 1)
	items := completeSource(uri, completion, strings.Index(completion, "self.IsSh")+len("self.IsSh"))
	found := false
	for _, item := range items {
		found = found || item.Label == "IsShadowKnown" && strings.Contains(item.Detail, "fn IsShadowKnown(Control.Field) bool")
	}
	if !found {
		t.Fatalf("completion = %+v", items)
	}
}
