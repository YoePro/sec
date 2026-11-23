package main

import (
	"os"
	"strings"
	"testing"
)

// TestByteRadixFormattingLSP presents the real core enum overload rather than
// substituting a string-format parameter or the parameterless fallback.
// Rules: rules/library/core-library.md — §8.4;
// rules/tooling/lsp.md — "Hover".
func TestByteRadixFormattingLSP(t *testing.T) {
	enableAnalysisCache()
	t.Cleanup(disableAnalysisCache)
	data, err := os.ReadFile("../../testdata/tooling/byte_formatting/members.sec")
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	const uri = "file:///tmp/byte-radix-formatting/members.sec"
	for _, item := range analyze(uri, text) {
		if item.Severity == 1 {
			t.Fatal(item)
		}
	}
	for _, format := range []string{"Decimal", "Hexadecimal", "Octal", "Binary"} {
		use := "value.ToString(byte.ByteStringFormat." + format + ")"
		offset := strings.Index(text, use)
		if offset < 0 {
			t.Fatal(format)
		}
		hover, ok := hoverForSource(uri, text, offsetPosition(text, offset+len("value.")))
		if !ok || !strings.Contains(hover.Contents.Value, "byte.ByteStringFormat") || !strings.Contains(hover.Contents.Value, "Result[string, StringError]") {
			t.Fatalf("%s hover = %+v (%v)", format, hover, ok)
		}
	}
}
