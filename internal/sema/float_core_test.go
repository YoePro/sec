package sema

import (
	"strings"
	"testing"
)

// core-library.md §§ 7.1, 8.1, 9.1, 9.3: float core methods type as T, bool
// or Result[T, RangeError]; numeric associated properties are PascalCase;
// a static property value may receive a method call.
func TestFloatCoreMembersResolve(t *testing.T) {
	analyzer, errors := analyzeSourceWithAnalyzerRaw(t, `
module main

fn Core(x: float64, y: float32, z: float) float64 {
    let a := x.Abs()
    let b := x.Min(2.0)
    let c := x.Max(a)
    let n: bool := x.IsNaN()
    let f: bool := y.IsSubnormal()
    let r: float32 := y.Round()
    let w: float := z.Truncate()
    let m: float64 := float64.Max
    let e: float32 := float32.Epsilon
    let i: int8 := int8.Min
    let k: uint := uint16.Bits
    let g := float64.NegativeInfinity.Abs()
    discard n
    discard f
    discard r
    discard w
    discard e
    discard i
    discard k
    discard g
    return a + b + c + m
}

fn Bounded(x: float64) Result[float64, RangeError] {
    let v := try x.Clamp(0.0, 1.0)
    return Ok(v)
}
`)
	assertSemaErrors(t, errors, nil)
	ids := map[string]bool{}
	for _, member := range analyzer.compilerKnownMemberFacts {
		ids[member.ID] = true
	}
	for _, id := range []string{"CKM-FLOAT-ABS", "CKM-FLOAT-MIN", "CKM-FLOAT-ISNAN", "CKM-FLOAT-ROUND", "CKM-FLOAT-CLAMP", "CKM-FLOAT-EPSILON", "CKM-FLOAT-NEGATIVEINFINITY", "CKM-NUMERIC-MIN", "CKM-NUMERIC-BITS"} {
		if !ids[id] {
			t.Fatalf("missing resolved member %s in %v", id, ids)
		}
	}
}

// Arguments must have the receiver's float type and arity is exact; the
// decided PascalCase spelling admits no lowercase alias; SquareRoot waits for
// MathError (P20 § 37).
func TestFloatCoreMemberRejections(t *testing.T) {
	errors := analyzeSourceRaw(t, `
module main

fn Bad(x: float64, y: float32) bool {
    let a := x.Min(y)
    let b := x.Abs(1.0)
    let c := x.Clamp(0.0)
    let d := float64.max
    let e := x.SquareRoot()
    discard a
    discard b
    discard c
    discard d
    discard e
    return true
}
`)
	joined := ""
	for _, err := range errors {
		joined += err.Error() + "\n"
	}
	for _, expected := range []string{
		"float64.Min argument must be float64, got float32",
		"float64.Abs expects 0 arguments, got 1",
		"float64.Clamp expects 2 arguments, got 1",
		"float64 has no associated property max; did you mean Max?",
		"x.SquareRoot",
	} {
		if !strings.Contains(joined, expected) {
			t.Fatalf("missing %q in:\n%s", expected, joined)
		}
	}
	if len(errors) < 5 {
		t.Fatalf("want five rejections, got:\n%s", joined)
	}
}
