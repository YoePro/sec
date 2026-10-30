package sema

import (
	"strings"
	"testing"
)

// MD-066 § 8 / MD-068 § 9(2) decimal core methods: total roundings and
// Normalize return the receiver type; Rescale and RoundTo report
// PrecisionError, Abs OverflowError and Clamp RangeError; arities and
// argument types are checked.
// Rules: rules/library/core-library.md §§ 10.2, 10.4.
func TestDecimalCoreMethods(t *testing.T) {
	errs := analyzeSourceRaw(t, `
module main

enum RoundingMode {
    NearestEven
    TowardZero
}

enum OverflowError error {
    Overflow
}

fn Use(value: decimal, other: decimal, wide: decimal128) Result[decimal, PrecisionError] {
    let normal := value.Normalize()
    let whole := value.Floor() + value.Ceiling() + value.Truncate() + value.Round()
    let ordered := value.Min(other).Max(normal)
    let positive := try value.Abs() {
        Err(OverflowError.Overflow) => 0m
    }
    let bounded := try value.Clamp(0m, 10m) {
        Err(_) => 0m
    }
    let wideRounded := try wide.RoundTo(2, RoundingMode.TowardZero) {
        Err(_) => 0m
    }
    discard whole
    discard ordered
    discard positive
    discard bounded
    discard wideRounded
    let rescaled := try value.Rescale(4)
    discard rescaled
    return value.RoundTo(2)
}
`)
	assertSemaErrors(t, errs, nil)
	members := map[string]CompilerKnownMember{}
	for _, member := range CompilerKnownMembersForType(builtinType("decimal128"), false) {
		members[member.Name] = member
	}
	if members["RoundTo"].ID != "CKM-DECIMAL128-ROUNDTO" || members["Normalize"].Result.Name != "decimal128" {
		t.Fatalf("decimal128 members = %+v", members)
	}

	invalid := analyzeSourceRaw(t, `
module main

fn Bad(value: decimal) void {
    discard value.Rescale()
    discard value.RoundTo(2, 3)
    discard value.Clamp(1m)
}
`)
	for _, want := range []string{
		"decimal.Rescale expects 1 arguments, got 0",
		"decimal.RoundTo needs the core RoundingMode type",
		"decimal.Clamp expects 2 arguments, got 1",
	} {
		found := false
		for _, err := range invalid {
			found = found || strings.Contains(err.Message, want)
		}
		if !found {
			t.Errorf("missing %q in %v", want, invalid)
		}
	}
}

// MD-068 § 10 decimal conversions: ToInt*/ToFloat* return
// Result[target, ConversionError] with an optional RoundingMode for both
// decimal widths, and reject wrong arities and argument types.
// Rules: rules/library/core-library.md §§ 10.2, 10.4, 10.6.
func TestDecimalConversionMethods(t *testing.T) {
	errs := analyzeSourceRaw(t, `
module main

enum RoundingMode {
    NearestEven
    Floor
}

enum ConversionError error {
    OutOfRange
    PrecisionLoss
}

fn Use(value: decimal, wide: decimal128) Result[int128, ConversionError] {
    let whole: int := try value.ToInt()
    let narrow: int32 := try value.ToInt32(RoundingMode.Floor)
    let long: int64 := try wide.ToInt64()
    let single: float32 := try value.ToFloat32()
    let double: float64 := try wide.ToFloat64(RoundingMode.NearestEven)
    discard whole
    discard narrow
    discard long
    discard single
    discard double
    return wide.ToInt128(RoundingMode.Floor)
}
`)
	assertSemaErrors(t, errs, nil)
	for _, typ := range []string{"decimal", "decimal128"} {
		members := map[string]CompilerKnownMember{}
		for _, member := range CompilerKnownMembersForType(builtinType(typ), false) {
			members[member.Name] = member
		}
		for name, result := range map[string]string{"ToInt": "int", "ToInt32": "int32", "ToInt64": "int64", "ToInt128": "int128", "ToFloat": "float", "ToFloat32": "float32", "ToFloat64": "float64"} {
			member := members[name]
			if member.ID != "CKM-"+strings.ToUpper(typ)+"-"+strings.ToUpper(name) || member.Result.Kind != ResultType || member.Result.TypeArgs[0].Name != result {
				t.Errorf("%s.%s = %+v", typ, name, member)
			}
		}
	}

	invalid := analyzeSourceRaw(t, `
module main

enum RoundingMode {
    NearestEven
}

fn Bad(value: decimal) void {
    discard value.ToInt(3)
    discard value.ToFloat(RoundingMode.NearestEven, 1)
    let wrong: int := value.ToInt64()
}
`)
	for _, want := range []string{
		"decimal.ToInt argument 1 must be RoundingMode, got int",
		"decimal.ToFloat expects 0 or 1 arguments, got 2",
		"cannot initialize int with Result[int64, ConversionError]",
	} {
		found := false
		for _, err := range invalid {
			found = found || strings.Contains(err.Message, want)
		}
		if !found {
			t.Errorf("missing %q in %v", want, invalid)
		}
	}
}
