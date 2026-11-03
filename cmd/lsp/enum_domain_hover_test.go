package main

import (
	"os"
	"strings"
	"testing"
)

// Enum tooling projects closed/open domain and conversion facts from Sema;
// aliases and the named subset of a hardware domain never manufacture proof.
// Rules: rules/declarations/enums.md — §§7,9–10,12,14,18;
// rules/tooling/lsp.md — "Hover", "Diagnostics", "Unknown facts".
func TestEnumDomainHover(t *testing.T) {
	data, err := os.ReadFile("../../testdata/tooling/enum_domains/valid.sec")
	if err != nil {
		t.Fatal(err)
	}
	source := string(data)
	for _, d := range analyze("", source) {
		if d.Severity == 1 {
			t.Fatal(d)
		}
	}
	tests := []struct {
		marker, selected string
		wants            []string
	}{
		{"enum Closed uint16", "Closed", []string{"Underlying type: `uint16`", "Domain: closed", "Aliases share a value class", "EnumValueError.UndeclaredValue"}},
		{"enum Open bit[8]", "Open", []string{"Underlying type: `bit[8]`", "Domain: open", "Undeclared bit patterns are valid", "unknown-value fallback", "EnumValueError.OutOfRange"}},
		{"Closed.Alias =>", "Alias", []string{"Closed.Alias", "Domain: closed"}},
		{"Open.One =>", "One", []string{"Open.One", "Domain: open"}},
		{"try Closed(value)", "Closed", []string{"int → Closed", "result: Result[Closed, EnumValueError]", "Checked conversion"}},
		{"try Open(value)", "Open", []string{"int → Open", "result: Result[Open, EnumValueError]", "Checked conversion", "in-range unnamed value does not fail"}},
		{"try Text(value)", "Text", []string{"string → Text", "result: Result[Text, EnumValueError]", "Checked conversion"}},
		{"ClosedExact(value: Closed) uint8 { return uint8(value)", "return uint8", []string{"Closed → uint8", "result: uint8", "Infallible conversion", "declared value classes of the closed enum", "source bounds `0` through `255` (inclusive)"}},
		{"ClosedChecked(value: Closed) int8 { return try int8(value)", "try int8", []string{"Closed → int8", "result: Result[int8, ConversionError]", "Checked conversion"}},
		{"OpenExact(value: Open) uint8 { return uint8(value)", "return uint8", []string{"Open → uint8", "Infallible conversion", "complete bit domain, including unnamed patterns", "source bounds `0` through `255` (inclusive)"}},
		{"OpenChecked(value: Open) int8 { return try int8(value)", "try int8", []string{"Open → int8", "result: Result[int8, ConversionError]", "Checked conversion"}},
		{"return Open(value)", "Open", []string{"uint8 → Open", "result: Open", "Infallible conversion"}},
		{"return Open(255)", "Open", []string{"result: Open", "Infallible conversion"}},
		{"return int8(Closed.Zero)", "int8", []string{"result: int8", "Infallible conversion", "Exact conversion proved from the known constant value", "source bounds `0` through `0` (inclusive)"}},
		{"return Text(\"one\")", "Text", []string{"result: Text", "Infallible conversion"}},
		{"return string(value)", "string", []string{"Text → string", "result: string", "Infallible conversion"}},
	}
	for _, test := range tests {
		t.Run(test.marker, func(t *testing.T) {
			offset := strings.Index(source, test.marker)
			if offset < 0 {
				t.Fatal("missing marker")
			}
			delta := strings.Index(test.marker, test.selected) + len(test.selected) - 1
			h, ok := hoverForSource("", source, offsetPosition(source, offset+delta))
			for _, want := range test.wants {
				if !ok || !strings.Contains(h.Contents.Value, want) {
					t.Errorf("missing %q in %+v", want, h)
				}
			}
			if strings.Contains(h.Contents.Value, "Domain: open") && strings.Contains(h.Contents.Value, "EnumValueError.UndeclaredValue") {
				t.Fatal("open domain incorrectly rejects unnamed values", h)
			}
		})
	}
	offset := strings.Index(source, "return Selected(0)") + len("return S")
	h, _ := hoverForSource("", source, offsetPosition(source, offset))
	if strings.Contains(h.Contents.Value, "Infallible conversion") || strings.Contains(h.Contents.Value, "Checked conversion") {
		t.Fatal("user function mistaken for conversion", h)
	}
}

// Match diagnostics preserve Sema's domain distinction and repair guidance;
// valid alias coverage and complete bit domains are tested by the valid fixture.
// Rules: rules/declarations/enums.md — §§14,18.
func TestEnumDomainMatchDiagnostics(t *testing.T) {
	data, err := os.ReadFile("../../testdata/tooling/enum_domains/match_invalid.sec")
	if err != nil {
		t.Fatal(err)
	}
	source := string(data)
	var messages []string
	for _, d := range analyze("", source) {
		if d.Severity == 1 {
			messages = append(messages, d.Message)
			line := strings.Split(source, "\n")[d.Range.Start.Line]
			if !strings.Contains(line, "return match") {
				t.Errorf("wrong diagnostic location: %+v", d)
			}
		}
	}
	if len(messages) != 3 {
		t.Fatalf("diagnostics = %v", messages)
	}
	for _, want := range []string{"closed enum Closed; cover every declared underlying value class", "open bit-backed enum Open; undeclared hardware encodings remain possible; add _ or cover the complete bit[8] domain", "closed enum Text; cover every declared underlying value class"} {
		if !strings.Contains(strings.Join(messages, "\n"), want) {
			t.Errorf("missing %q in %v", want, messages)
		}
	}
}

// Known-invalid constant conversions retain language errors and must not be
// described as accepted infallible conversions in a recovered hover result.
// Rules: rules/declarations/enums.md — §§12.2,12.4,12.6,18.
func TestEnumDomainInvalidConversionHover(t *testing.T) {
	data, err := os.ReadFile("../../testdata/tooling/enum_domains/conversion_invalid.sec")
	if err != nil {
		t.Fatal(err)
	}
	source := string(data)
	errors := 0
	for _, d := range analyze("", source) {
		if d.Severity == 1 {
			errors++
		}
	}
	if errors != 3 {
		t.Fatalf("errors = %d", errors)
	}
	for _, marker := range []string{"return Closed(2)", "return Open(256)", "return Text(\"unknown\")"} {
		offset := strings.Index(source, marker) + len("return ") + 1
		h, _ := hoverForSource("", source, offsetPosition(source, offset))
		if strings.Contains(h.Contents.Value, "Infallible conversion:") {
			t.Fatal("invalid conversion presented as accepted", h)
		}
	}
}
