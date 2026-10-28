package sema

import (
	"strings"
	"testing"
)

// Scale is the int32 property of a decimal or decimal128 value (MD-066,
// superseding uint8), spelled canonically Scale with the core spelling scale
// kept as a legacy name like Ptr/ptr and Len/len. It is not a type-qualified
// static property.
//
// Rules:
//   - rules/library/core-library.md — §§ 10.1, 10.4
//   - rules/compiler/compiler_known_members.md — "Decimal types"
//   - rules/corrections/md066-md067-md068-decimal-correction-20261028.md — §§ 2(2), 2(5), 14 DEC-01
func TestDecimalScaleIsAnInt32ValueProperty(t *testing.T) {
	for name, test := range map[string]struct {
		body  string
		valid bool
	}{
		"canonical":   {"fn Use(x: decimal) int32 {\n    return x.Scale\n}\n", true},
		"legacy":      {"fn Use(x: decimal) int32 {\n    return x.scale\n}\n", true},
		"decimal128":  {"fn Use(x: decimal128) int32 {\n    return x.Scale\n}\n", true},
		"type static": {"fn Use() int32 {\n    return decimal.Scale\n}\n", false},
		"not uint8":   {"fn Use(x: decimal) uint8 {\n    return x.Scale\n}\n", false},
	} {
		t.Run(name, func(t *testing.T) {
			errors := analyzeSourceRaw(t, "module main\n"+test.body)
			if test.valid != (len(errors) == 0) {
				t.Fatalf("valid=%t, errors=%v", test.valid, errors)
			}
		})
	}
}

// The Scale setter is fallible and rescales a mutable receiver in place:
// it requires try, reports PrecisionError, and rejects read-only receivers
// and non-place receivers.
//
// Rules:
//   - rules/library/core-library.md — § 10.1 (MD-066)
//   - rules/errors/errorhandling.md — § 24
//   - rules/corrections/md066-md067-md068-decimal-correction-20261028.md — §§ 2(3)–2(4), 11, 14 DEC-13, 14(2)
func TestDecimalScaleSetterIsFallible(t *testing.T) {
	for name, test := range map[string]struct {
		body string
		want string
	}{
		"propagates":  {"fn Use(x: decimal) Result[decimal, PrecisionError] {\n    let mut y := x\n    try y.Scale = 2\n    return Ok(y)\n}\n", ""},
		"handled":     {"fn Use(x: decimal) decimal {\n    let mut y := x\n    try y.Scale = 2 {\n        Err(PrecisionError.PrecisionLoss) => {\n            y = x\n        }\n        Err(_) => {\n            y = x\n        }\n    }\n    return y\n}\n", ""},
		"decimal128":  {"fn Use(x: decimal128) Result[decimal128, PrecisionError] {\n    let mut y := x\n    try y.Scale = -3\n    return Ok(y)\n}\n", ""},
		"needs try":   {"fn Use(x: decimal) void {\n    let mut y := x\n    y.Scale = 2\n}\n", "assigning fallible property Scale requires try"},
		"read-only":   {"fn Use(x: decimal) Result[void, PrecisionError] {\n    let y := x\n    try y.Scale = 2\n    return Ok()\n}\n", "cannot assign to immutable place"},
		"wrong error": {"fn Use(x: decimal) Result[void, RangeError] {\n    let mut y := x\n    try y.Scale = 2\n    return Ok()\n}\n", "propagates PrecisionError"},
		"int32 only":  {"fn Use(x: decimal, s: int64) Result[void, PrecisionError] {\n    let mut y := x\n    try y.Scale = s\n    return Ok()\n}\n", "cannot assign int64 to int32"},
		"not a place": {"fn Make() decimal {\n    return 1.5\n}\nfn Use() Result[void, PrecisionError] {\n    try Make().Scale = 2\n    return Ok()\n}\n", "can only be set on an assignable decimal"},
	} {
		t.Run(name, func(t *testing.T) {
			errors := analyzeSourceRaw(t, "module main\n"+test.body)
			if test.want == "" {
				if len(errors) != 0 {
					t.Fatalf("want valid, got %v", errors)
				}
				return
			}
			if len(errors) == 0 || !strings.Contains(errors[0].Message, test.want) {
				t.Fatalf("want %q, got %v", test.want, errors)
			}
		})
	}
}

// MD-082: the lowercase migration spelling is the same registry member as
// the canonical Scale, not a second property; decimal128 has its own single
// identity with the same int32 type.
func TestDecimalScaleLegacySpellingSharesMemberID(t *testing.T) {
	canonical, ok := compilerKnownMember(builtinType("decimal"), "Scale", false)
	legacy, legacyOK := compilerKnownMember(builtinType("decimal"), "scale", false)
	if !ok || !legacyOK || canonical.ID != "CKM-DECIMAL-SCALE" || legacy.ID != canonical.ID || canonical.Result.Name != "int32" {
		t.Fatalf("canonical %s/%s (%t), legacy %s (%t)", canonical.ID, canonical.Result.Name, ok, legacy.ID, legacyOK)
	}
	wide, ok := compilerKnownMember(builtinType("decimal128"), "Scale", false)
	if !ok || wide.ID != "CKM-DECIMAL128-SCALE" || wide.Result.Name != "int32" {
		t.Fatalf("decimal128 Scale %s/%s (%t)", wide.ID, wide.Result.Name, ok)
	}
}
