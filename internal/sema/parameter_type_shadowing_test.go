package sema

import (
	"strings"
	"testing"

	"sec/internal/diagnostics"
)

// Rules: rules/foundations/names_scopes_visibility.md — §3 module surface,
// §8 shadowing of visible declarations by parameters.
func TestFunctionParameterCannotShadowSameModuleType(t *testing.T) {
	source := `module main

type Packet struct {}

fn Decode(Packet: int) void {}

fn Forward(Later: int) void {}

type Later struct {}

impl Packet {
    fn Check(Packet: int) void {}
}

fn Use(packet: Packet) void {}
`
	_, errors := analyzeSourceWithAnalyzer(t, source)
	if len(errors) != 3 {
		t.Fatalf("errors = %v, want three parameter shadowing errors", errors)
	}
	for _, diagnostic := range errors {
		if !strings.Contains(diagnostic.Message, "parameter") || !strings.Contains(diagnostic.Message, "shadows visible type") {
			t.Errorf("unexpected error: %+v", diagnostic)
		}
		if diagnostic.ID != diagnostics.ParameterShadowsType || diagnostic.Line == 0 || diagnostic.PreviousLine == 0 || diagnostic.PreviousColumn == 0 {
			t.Errorf("missing declaration locations: %+v", diagnostic)
		}
	}
}
