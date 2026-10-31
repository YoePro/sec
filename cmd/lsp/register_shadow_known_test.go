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

@address(ControlEndpoint)
let mut control: Control

fn Use() bool {
    return control.HasPending()
}

hardware region RegisterSpace {
    domain: "cpu-physical",
    kind: HardwareRegionKind.MMIO,
    base: 0x40000000,
    size: 0x10000,
    readable: true,
    writable: true,
}
hardware resource ControlState { width: 8, }
hardware endpoint ControlEndpoint {
    kind: HardwareEndpointKind.MemoryMapped,
    region: RegisterSpace,
    resource: ControlState,
    offset: 0x0,
    size: 1,
    alignment: 1,
    read_widths: [8],
    write_widths: [8],
    read_footprint: HardwareFootprintKind.Whole,
    write_footprint: HardwareFootprintKind.Whole,
    read_effect: HardwareReadEffect.NonDestructive,
    write_operation: HardwareWriteOperation.Replace,
    ordering: HardwareOrdering.Volatile,
    completion: HardwareCompletion.Issued,
    atomicity: HardwareAtomicity.Unspecified,
    context: HardwareAccessContext.Current,
    fault: HardwareFaultContract.Platform,
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
