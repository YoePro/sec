package main

import (
	"reflect"
	"strings"
	"testing"
)

const unitMetadataLSPUnits = `module main
enum Domain {
    Flow,
}
unit flow other
impl <flow> {
    Kind: Domain.Flow
}
unit m physical
impl <m> {
    LongName: "Metre"
    Dimension: [length^1]
    Kind: UnitKind.Length
}
unit third physical
impl <third> {
    Dimension: [length^1]
    Scale: 1 / 3
}
`

// Hover on a metadata read shows the Sema-resolved exact type, the concrete
// custom enum, defaults, absence markers, the canonical dimension, and the
// exact rational behind a projected Scale.
//
// Rules:
//   - rules/tooling/lsp.md — "Hover"
//   - rules/types/units.md — "Unit metadata value types" (1), (7), (10)
//   - rules/corrections/md062-unit-metadata-value-types-correction-20261027.md — §§ 12(9), 13(8)
func TestUnitMetadataHover(t *testing.T) {
	source := unitMetadataLSPUnits + `fn Read() void {
    let custom := <flow>.Kind
    let unknown := <third>.Kind
    let dimension := <m>.Dimension
    let scale := <third>.Scale
    let name := <m>.LongName
}
`
	for _, tc := range []struct{ marker, want string }{
		{"Kind\n    let unknown", "unit metadata <flow>.Kind: Domain\n```\n\nValue: `Domain.Flow`"},
		{"Kind\n    let dimension", "Value: `UnitKind.Unknown` (absence marker, not evidence of compatibility) — omitted; default"},
		{"Dimension\n", "unit metadata <m>.Dimension: UnitDimension\n```\n\nValue: `[length^1]` (AxisCount `1`, IsDimensionless `false`)"},
		{"Scale\n", "Value: `0.3333333333333333333` (nearest decimal of the exact `1/3`, ties to even)"},
		{"LongName\n", "unit metadata <m>.LongName: string\n```\n\nValue: `\"Metre\"`"},
	} {
		result, ok := hoverForSource("", source, offsetPosition(source, strings.Index(source, tc.marker)))
		if !ok || !strings.Contains(result.Contents.Value, tc.want) {
			t.Errorf("hover at %q: want %q, got ok=%v %q", tc.marker, tc.want, ok, result.Contents.Value)
		}
	}
}

// `<U>.` completes exactly the metadata members readable on that unit, with
// their Sema types; members whose read is an error are not offered.
// Rules: rules/tooling/lsp.md — completions;
// rules/corrections/md062-unit-metadata-value-types-correction-20261027.md — § 12(9).
func TestUnitMetadataCompletion(t *testing.T) {
	got := completionLabelsAt(t, unitMetadataLSPUnits+"fn Read() void {\n    let value := <m>.|\n}\n", "|")
	want := []string{"BaseUnit", "Category", "Dimension", "Kind", "LongName", "Name", "Offset", "Origin", "Scale", "Status", "System", "Transform"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("<m>. completions = %v, want %v", got, want)
	}
	if got := completionLabelsAt(t, unitMetadataLSPUnits+"fn Read() void {\n    let value := <flow>.Lo|\n}\n", "|"); len(got) != 0 {
		t.Fatalf("<flow>.Lo completions = %v, want none (LongName is undeclared)", got)
	}
	got = completionLabelsAt(t, unitMetadataLSPUnits+"fn Read() void {\n    let dimension := <m>.Dimension\n    let count := dimension.|\n}\n", "|")
	for _, member := range []string{"AxisCount", "IsDimensionless"} {
		if !strings.Contains(strings.Join(got, " "), member) {
			t.Fatalf("UnitDimension completions = %v, missing %s", got, member)
		}
	}
}
