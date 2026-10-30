package main

import (
	"path/filepath"
	"strings"
	"testing"
)

// TestDecimalMembersLSP loads the trusted core: decimal and decimal128 share
// the compiler-known conversions and the core-declared ToString overloads, and
// hover shows the selected conversion signature.
// Rules: rules/library/core-library.md §§ 10.2, 10.4, 10.6, 10.7.
func TestDecimalMembersLSP(t *testing.T) {
	uri := uriFromPath(filepath.Join(t.TempDir(), "decimal_members.sec"))
	source := `module main

fn Use(value: decimal, wide: decimal128) Result[void, error] {
    let whole: int64 := try value.ToInt64(RoundingMode.Floor)
    let ratio: float64 := try wide.ToFloat64()
    let text: string := try wide.ToString("N2", ",", " ")
    let plain: string := try value.ToString("F2")
    discard whole
    discard ratio
    discard text
    discard plain
    return Ok()
}
`
	for _, diagnostic := range analyze(uri, source) {
		if diagnostic.Severity == 1 {
			t.Fatalf("unexpected error: %+v", diagnostic)
		}
	}
	position := offsetPosition(source, strings.Index(source, "ToFloat64"))
	hover, ok := hoverForSource(uri, source, position)
	if !ok || !strings.Contains(hover.Contents.Value, "Result[float64, ConversionError]") {
		t.Fatal(hover, ok)
	}
}
