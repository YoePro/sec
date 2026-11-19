package sema

import (
	"strings"
	"testing"

	"sec/internal/layout"
)

// P24-16..20: Sema types map to layout requirements with canonical identity,
// scalar kind and width, carrier, fields, elements and indirection, and the
// canonical resolver consumes them without any guessed size or alignment.
// Rules: rules/memory/layout.md — §§ 4, 11–13, 18, 19, 25, 39.
func TestLayoutRequirementOf(t *testing.T) {
	analyzer, errors := analyzeArraySourceWithWidth(t, `module main

type Percent int32 range 0..100
type Celsius float64

type Pair struct {
    b: byte,
    n: int64,
    flag: bool,
    values: int32[4],
    name: string,
}
`, 64)
	if len(errors) != 0 {
		t.Fatalf("errors: %s", joinedSemaErrors(errors))
	}

	for name, want := range map[string]struct {
		scalar      layout.ScalarKind
		width       uint16
		targetSized bool
	}{
		"bool": {layout.ScalarBool, 0, false}, "byte": {layout.ScalarByte, 8, false}, "char": {layout.ScalarChar, 8, false},
		"rune": {layout.ScalarRune, 0, false}, "int8": {layout.ScalarSigned, 8, false}, "int128": {layout.ScalarSigned, 128, false},
		"uint256": {layout.ScalarUnsigned, 256, false}, "int": {layout.ScalarSigned, 0, true}, "uint": {layout.ScalarUnsigned, 0, true},
		"float": {layout.ScalarFloat, 0, true}, "float32": {layout.ScalarFloat, 32, false}, "float64": {layout.ScalarFloat, 64, false},
		"decimal": {layout.ScalarDecimal, 64, false}, "decimal128": {layout.ScalarDecimal, 128, false},
	} {
		requirement := LayoutRequirementOf(builtinType(name))
		if requirement.Kind != layout.LayoutScalar || requirement.Scalar != want.scalar || requirement.WidthBits != want.width || requirement.TargetSized != want.targetSized {
			t.Errorf("%s = %+v", name, requirement)
		}
	}

	percent := LayoutRequirementOf(analyzer.types["Percent"])
	if percent.Kind != layout.LayoutNamed || percent.Carrier == nil || percent.Carrier.TypeIdentity != canonicalTypeIdentity(builtinType("int32")) ||
		percent.TypeIdentity == percent.Carrier.TypeIdentity || !strings.Contains(percent.Validity, "RangeContract") {
		t.Fatalf("Percent = %+v", percent)
	}
	celsius := LayoutRequirementOf(analyzer.types["Celsius"])
	if celsius.Kind != layout.LayoutNamed || celsius.Carrier == nil || celsius.Carrier.WidthBits != 64 || celsius.Validity != "" {
		t.Fatalf("Celsius = %+v", celsius)
	}

	pair := LayoutRequirementOf(analyzer.types["Pair"])
	if pair.Kind != layout.LayoutStruct || len(pair.Fields) != 5 {
		t.Fatalf("Pair = %+v", pair)
	}
	for index, name := range []string{"b", "n", "flag", "values", "name"} {
		if pair.Fields[index].Name != name {
			t.Errorf("field %d = %s, want %s (declaration order)", index, pair.Fields[index].Name, name)
		}
	}
	values := pair.Fields[3].Requirement
	if values.Kind != layout.LayoutArray || values.Count == nil || values.Count.Int64() != 4 || values.Element == nil || values.Element.WidthBits != 32 {
		t.Errorf("values = %+v", values)
	}
	if pair.Fields[4].Requirement.Kind != layout.LayoutDescriptor {
		t.Errorf("string field = %+v, want a descriptor", pair.Fields[4].Requirement)
	}

	alignments := layout.ScalarAlignments{1: 1, 2: 2, 4: 4, 8: 8, 16: 16, 32: 32}
	resolver, err := layout.NewResolver(layout.ResolvedScalarPlan{TargetOS: "linux", TargetArch: "test", PointerWidthBits: 64, Endianness: layout.LittleEndian}, alignments)
	if err != nil {
		t.Fatal(err)
	}
	if resolved := resolver.Resolve(percent); !resolved.Complete() || resolved.SizeBytes.Int64() != 4 {
		t.Errorf("resolved Percent = %+v", resolved)
	}
	if resolved := resolver.Resolve(pair); resolved.Completeness != layout.LayoutUnsupported || resolved.SizeBytes != nil {
		t.Errorf("resolved Pair = %s %v, want unsupported through its string descriptor", resolved.Completeness, resolved.Missing)
	}
}
