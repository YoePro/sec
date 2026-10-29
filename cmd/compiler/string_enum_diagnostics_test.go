package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestStringEnumConversionRepairsCLI preserves the two distinct repairs:
// a known undeclared constant needs a declared value, while a runtime string
// conversion requires ordinary try handling of EnumValueError.
// Rules: rules/declarations/enums.md — §12.6;
// rules/tooling/diagnostics.md — mentor help and occurrence transport.
func TestStringEnumConversionRepairsCLI(t *testing.T) {
	data, err := os.ReadFile("../../testdata/tooling/string_enum_conversion_invalid.sec")
	if err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(t.TempDir(), "main.sec")
	if err := os.WriteFile(file, data, 0600); err != nil {
		t.Fatal(err)
	}
	_, stderr, code := runCLIForDiagnostics(t, "check", file, "--diagnostic-format=json")
	doc := decodeOccurrenceDocument(t, stderr)
	if code != 3 || len(doc.Occurrences) != 2 {
		t.Fatal(code, stderr)
	}
	constant, runtime := false, false
	for _, occurrence := range doc.Occurrences {
		var help strings.Builder
		for _, entry := range occurrence.Help {
			help.WriteString(entry.Text)
		}
		text := help.String()
		if strings.Contains(occurrence.Message.Text, `string value "unknown"`) {
			constant = true
			for _, want := range []string{"Program.Primary", "Program.Other", `"Alpha Beta"`, `"Gamma"`} {
				if !strings.Contains(text, want) {
					t.Fatalf("missing %q in repair %q", want, text)
				}
			}
			if strings.Contains(text, "Program.Alias") || strings.Contains(text, "try") {
				t.Fatal("invalid constant repair", text)
			}
		} else if strings.Contains(text, "try Program(value)") && strings.Contains(text, "EnumValueError") {
			runtime = true
		}
	}
	if !constant || !runtime {
		t.Fatal("lost a conversion repair", doc)
	}
	_, human, humanCode := runCLIForDiagnostics(t, "check", file)
	if humanCode != 3 || !strings.Contains(human, "Declared string values:") || !strings.Contains(human, "try Program(value)") {
		t.Fatal(humanCode, human)
	}
}
