package sema

import "testing"

// decimal.scale is a uint8 property of a decimal value (core-library.md
// § 10.1), spelled canonically Scale with the core spelling scale kept as a
// legacy name like Ptr/ptr and Len/len. It is not a type-qualified static
// property, and decimal128 has no member until MD-068 (P21-D3) is decided.
//
// Rules:
//   - rules/library/core-library.md — § 10.1
//   - rules/compiler/compiler_known_members.md — "Decimal types"
func TestDecimalScaleIsAUint8ValueProperty(t *testing.T) {
	for name, test := range map[string]struct {
		body  string
		valid bool
	}{
		"canonical":   {"fn Use(x: decimal) uint8 {\n    return x.Scale\n}\n", true},
		"legacy":      {"fn Use(x: decimal) uint8 {\n    return x.scale\n}\n", true},
		"type static": {"fn Use() uint {\n    return decimal.Scale\n}\n", false},
		"decimal128":  {"fn Use(x: decimal128) uint8 {\n    return x.Scale\n}\n", false},
		"not int":     {"fn Use(x: decimal) int {\n    let s: int := x.Scale\n    return s\n}\n", false},
		"read-only":   {"fn Use(x: decimal) void {\n    let mut y := x\n    y.Scale = 2\n}\n", false},
	} {
		t.Run(name, func(t *testing.T) {
			errors := analyzeSourceRaw(t, "module main\n"+test.body)
			if test.valid != (len(errors) == 0) {
				t.Fatalf("valid=%t, errors=%v", test.valid, errors)
			}
		})
	}
}

// MD-082: the lowercase migration spelling is the same registry member as
// the canonical Scale, not a second property.
func TestDecimalScaleLegacySpellingSharesMemberID(t *testing.T) {
	canonical, ok := compilerKnownMember(builtinType("decimal"), "Scale", false)
	legacy, legacyOK := compilerKnownMember(builtinType("decimal"), "scale", false)
	if !ok || !legacyOK || canonical.ID != "CKM-DECIMAL-SCALE" || legacy.ID != canonical.ID || canonical.Result.Name != "uint8" {
		t.Fatalf("canonical %+v (%t), legacy %+v (%t)", canonical, ok, legacy, legacyOK)
	}
}
