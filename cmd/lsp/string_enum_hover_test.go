package main

import (
	"os"
	"strings"
	"testing"
)

// TestStringEnumHoverUsesResolvedDomain preserves constant-evaluated values,
// aliases, defaults, documentation and the canonical checked conversion surface.
// Rules: rules/declarations/enums.md — §§5,7–8,12.1,12.6,18;
// rules/tooling/lsp.md — "Hover".
func TestStringEnumHoverUsesResolvedDomain(t *testing.T) {
	data, err := os.ReadFile("../../testdata/tooling/string_enum_hover.sec")
	if err != nil {
		t.Fatal(err)
	}
	source := string(data)
	for _, diagnostic := range analyze("", source) {
		if diagnostic.Severity == 1 {
			t.Fatal(diagnostic)
		}
	}
	for _, test := range []struct {
		marker, selected string
		want             []string
	}{
		{"enum Program string", "Program", []string{"enum Program string", `Primary = "Alpha Beta"`, `Alias = "Alpha Beta"`, `Escaped = "line\n\"quoted\""`, "Underlying type: `string`", "Domain: closed", "Default: `Program.Other`", "Alias: `Program.Alias` = `Program.Primary`", "Result[Program, EnumValueError]", "EnumValueError.UndeclaredValue", "Conversion to `string` is infallible.", "Modes accepted by the service."}},
		{"Program.Alias", "Alias", []string{`Program.Alias = "Alpha Beta"`, "Alias: `Program.Alias` = `Program.Primary`", "Compatibility name for the primary mode."}},
		{"enum Implicit string", "Implicit", []string{"Default: `Implicit.First`", `First = "one"`, `Next = "two"`}},
	} {
		offset := strings.Index(source, test.marker) + strings.Index(test.marker, test.selected)
		hover, ok := hoverForSource("", source, offsetPosition(source, offset+1))
		for _, want := range test.want {
			if !ok || !strings.Contains(hover.Contents.Value, want) {
				t.Errorf("%s: missing %q in %+v", test.marker, want, hover)
			}
		}
		if strings.Contains(hover.Contents.Value, "EnumValueError.OutOfRange") {
			t.Fatal("string conversion invented a width failure", hover)
		}
	}
}

// TestStringEnumConversionDiagnosticLSP publishes Sema's valid-value repair;
// an unknown compile-time string is not repaired by merely adding try.
// Rules: rules/declarations/enums.md — §12.6; rules/tooling/lsp.md — "Diagnostics".
func TestStringEnumConversionDiagnosticLSP(t *testing.T) {
	data, err := os.ReadFile("../../testdata/tooling/string_enum_conversion_invalid.sec")
	if err != nil {
		t.Fatal(err)
	}
	items := analyze("", string(data))
	if len(items) != 2 {
		t.Fatal(items)
	}
	found := false
	for _, item := range items {
		if !strings.Contains(item.Message, `string value "unknown"`) {
			continue
		}
		found = true
		for _, want := range []string{"Program.Primary", "Program.Other", `"Alpha Beta"`, `"Gamma"`} {
			if !strings.Contains(item.Message, want) {
				t.Fatalf("missing repair %q in %+v", want, item)
			}
		}
		if strings.Contains(item.Message, "Program.Alias") {
			t.Fatal("alias duplicated the valid-value repair", item)
		}
	}
	if !found {
		t.Fatal("missing undeclared constant diagnostic", items)
	}
}
