package sema

import (
	"testing"

	"sec/internal/layout"
	"sec/internal/lexer"
	"sec/internal/parser"
)

// Every integer consumer that shapes an untyped literal validates it against
// the target-selected int and uint bounds: 3000000000 is rejected on a 32-bit
// plan and accepted on a 64-bit plan.
//
// Rules:
//   - rules/types/types.md — "int and uint", "Unsuffixed literal inference", "Context shaping"
//   - rules/types/contracts.md — "Range", "Integer contracts", "Explicit defaults"
//   - rules/types/default_values.md — "Explicit type defaults"
func TestTargetSizedBoundsReachEveryIntegerConsumer(t *testing.T) {
	cases := map[string]string{
		"typed-let":        "fn F() void {\n let a: int := 3000000000\n discard a\n}",
		"inferred-let":     "fn F() void {\n let a := 3000000000\n discard a\n}",
		"constant-sum":     "fn F() void {\n let a: int := 2000000000 + 1000000000\n discard a\n}",
		"shift":            "fn F() void {\n let a: int := 1 << 40\n discard a\n}",
		"uint":             "fn F() void {\n let a: uint := 5000000000u\n discard a\n}",
		"enum-underlying":  "enum E int {\n A = 3000000000,\n}",
		"range-contract":   "type Big int range 0..3000000000",
		"explicit-default": "type Big int default 3000000000",
		"membership":       "type Big int in [1, 3000000000]",
		"multipleOf":       "type Big int multipleOf 3000000000",
		"call-argument":    "fn G(v: int) void {}\nfn F() void {\n G(3000000000)\n}",
		"return":           "fn F() int {\n return 3000000000\n}",
		"struct-field":     "type S struct {\n v: int,\n}\nfn F() void {\n let s := S { v: 3000000000 }\n discard s\n}",
		"generic-field":    "type Box[T] struct {\n v: T,\n}\nfn F() void {\n let b := Box[int] { v: 3000000000 }\n discard b\n}",
		"array-element":    "fn F() void {\n let a: int[2] := [1, 3000000000]\n discard a\n}",
		"assignment":       "fn F() void {\n let mut a: int := 0\n a = 3000000000\n discard a\n}",
		"compound":         "fn F() void {\n let mut a: int := 2000000000\n a += 2000000000\n discard a\n}",
		"comparison":       "fn F(a: int) bool {\n return a == 3000000000\n}",
		"static":           "let Limit: int := 3000000000",
	}
	for name, body := range cases {
		for _, width := range []uint16{32, 64} {
			p := parser.New(lexer.New("module main\n" + body + "\n"))
			program := p.ParseProgram()
			if len(p.Errors()) > 0 {
				t.Fatalf("%s: parse errors %v", name, p.Errors())
			}
			analyzer := NewAnalyzerWithScalarPlan(layout.ResolvedScalarPlan{PointerWidthBits: width})
			errors := analyzer.Analyze(program)
			if width == 32 && len(errors) != 1 {
				t.Errorf("%s on 32-bit: errors = %v, want exactly one representability error", name, errors)
			}
			if width == 64 && len(errors) != 0 {
				t.Errorf("%s on 64-bit: errors = %v, want none", name, errors)
			}
		}
	}
}

// Untyped literals shaped by fixed-width destinations at call, return, and
// array-element boundaries must be representable; contracted array elements
// are proven against the element type's contracts.
//
// Rules:
//   - rules/types/types.md — "Context shaping"
//   - rules/types/contracts.md — "Initialization and assignment"
func TestUntypedLiteralsAreShapedAtCallReturnAndElementBoundaries(t *testing.T) {
	errors := analyzeSourceRaw(t, `module main

type P int range 0..10

fn G(v: int8) void {}

fn R() int8 {
    return 300
}

fn F() void {
    G(300)
    let a: int8[2] := [1, 300]
    let b: P[2] := [1, 20]
    discard a
    discard b
}
`)
	assertSemaErrors(t, errors, []string{
		"value 300 overflows int8 at 8:12",
		"value 300 overflows int8 at 12:7",
		"value 300 overflows int8 at 13:27",
		"value 20 violates range contract P 0..10 at 14:24",
	})
}
