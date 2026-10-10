package sema

import (
	"strings"
	"testing"

	"sec/internal/layout"
)

// try converts a fallible explicit integer-family conversion into
// ConversionError (types.md "Explicit conversions"; MD-012 §4). The relation
// is judged at every supported int width, so try stays valid on targets
// where the value happens to fit; a relation that can never fail is not a
// try operand.
func TestTryAcceptsCheckedScalarConversions(t *testing.T) {
	for _, body := range []string{
		"let n := try int32(wide) {\n    Err(_) => int32(0)\n}\ndiscard n",
		"let r := try rune(small) {\n    Err(_) => 'x'\n}\ndiscard r",
		"let c := try char(small) {\n    Err(_) => 'x'\n}\ndiscard c",
		"let p := try int(wide) {\n    Err(_) => 0\n}\ndiscard p",
	} {
		for _, model := range []layout.CABIModel{layout.CModelSysVX8664} {
			if errors := analyzeWithCABIModel(t, model, body); len(errors) != 0 {
				t.Fatalf("%q errors = %v", body, errors)
			}
		}
	}
	errors := analyzeWithCABIModel(t, layout.CModelSysVX8664, "let w := try int64(small) {\n    Err(_) => int64(0)\n}\ndiscard w")
	if len(errors) != 1 || !strings.Contains(errors[0].Message, "contains none") {
		t.Fatalf("widening try errors = %v", errors)
	}
}

// Interval proofs from conditions, immutable bindings, char/rune code values,
// and bitwise operators make a c:: conversion infallible without try.
func TestCheckedConversionProofsFromPathFacts(t *testing.T) {
	for _, body := range []string{
		"if wide in 0..100 {\n    let c := c::int(wide)\n    discard c\n}",
		"if wide < 0 || wide > 1000 {\n    return\n}\nlet c := c::int(wide)\ndiscard c",
		"let masked := c::uchar(wide & 0xFF)\ndiscard masked",
		"let code := int64(small)\nlet narrow := c::int(code)\ndiscard narrow",
	} {
		if errors := analyzeWithCABIModel(t, layout.CModelSysVX8664, body); len(errors) != 0 {
			t.Fatalf("%q errors = %v", body, errors)
		}
	}
}
