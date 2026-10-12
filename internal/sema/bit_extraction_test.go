package sema

import (
	"strings"
	"testing"

	"sec/internal/ast"
	"sec/internal/layout"
	"sec/internal/lexer"
	"sec/internal/parser"
)

func analyzeBitExtraction(t *testing.T, source string, width uint16) (*Analyzer, *ast.Program, []Error) {
	t.Helper()
	p := parser.New(lexer.New(source))
	program := p.ParseProgram()
	if len(p.Errors()) != 0 {
		t.Fatalf("parse: %v", p.Errors())
	}
	analyzer := NewAnalyzerWithScalarPlan(layout.ResolvedScalarPlan{PointerWidthBits: width})
	return analyzer, program, analyzer.Analyze(program)
}

// `bit[N](value)` extracts the N low bits of a primitive integer, byte or
// char as the smallest existing unsigned carrier, with the proven interval
// 0..2^N-1, and a plain register field accepts an extraction no wider than
// the field (MD-046/MD-056).
//
// Rules:
//   - rules/types/types.md — "Explicit low-bit extraction"
//   - rules/declarations/registers.md — § 12
//   - rules/corrections/applied/md046-md056-bit-extraction-correction-20261012.md — §§ 2-6, 10
func TestBitExtractionTypesAndRegisterWrites(t *testing.T) {
	analyzer, program, errors := analyzeBitExtraction(t, `
module main

type Sensor register[16] {
    Reading: bit[12],
    _: bit[4],
}

fn Valid(sensor: Sensor, w8: uint8, w16: uint16, w32: int32, w64: int64, w128: uint128, w256: int256, b: byte, c: char, n: int) void {
    let a1 := bit[1](w8)
    let a8 := bit[8](w8)
    let a9 := bit[9](w16)
    let a16 := bit[16](w16)
    let a17 := bit[17](w32)
    let a32 := bit[32](w32)
    let a33 := bit[33](w64)
    let a64 := bit[64](w64)
    let a65 := bit[65](w128)
    let a128 := bit[128](w128)
    let a129 := bit[129](w256)
    let a256 := bit[256](w256)
    let fromByte := bit[8](b)
    let fromChar := bit[8](c)
    let fromInt := bit[64](n)
    let narrow := uint8(bit[8](w16))
    sensor.Reading = bit[12](w16)
}
`, 64)
	assertSemaErrors(t, errors, nil)
	want := map[string]string{
		"a1": "uint8", "a8": "uint8", "a9": "uint16", "a16": "uint16", "a17": "uint32", "a32": "uint32",
		"a33": "uint64", "a64": "uint64", "a65": "uint128", "a128": "uint128", "a129": "uint256", "a256": "uint256",
		"fromByte": "uint8", "fromChar": "uint8", "fromInt": "uint64",
	}
	for _, statement := range program.Statements {
		function, ok := statement.(*ast.FunctionDeclaration)
		if !ok {
			continue
		}
		for _, body := range function.Body.Statements {
			let, ok := body.(*ast.LetStatement)
			if !ok {
				continue
			}
			extraction, ok := let.Value.(*ast.BitExtractionExpression)
			if !ok {
				continue
			}
			fact, resolved := analyzer.ResolvedBitExtractionOf(extraction)
			if !resolved || fact.ResultType.Name != want[let.Name.Value] {
				t.Errorf("%s: fact = %+v, %t; want result %s", let.Name.Value, fact, resolved, want[let.Name.Value])
			}
			minimum, maximum, known := analyzer.integerInterval(extraction)
			if !known || minimum.Sign() != 0 || maximum.BitLen() != int(fact.Width) {
				t.Errorf("%s: interval %v..%v, want 0..2^%d-1", let.Name.Value, minimum, maximum, fact.Width)
			}
		}
	}
}

func TestBitExtractionConstantsUseNonNegativeModulo(t *testing.T) {
	analyzer, program, errors := analyzeBitExtraction(t, `
module main

fn Constants() void {
    let negative := bit[8](-1)
    let wrapped := bit[4](300)
}
`, 64)
	assertSemaErrors(t, errors, nil)
	function := program.Statements[len(program.Statements)-1].(*ast.FunctionDeclaration)
	for index, want := range []int64{255, 12} {
		value, ok := analyzer.integerConstantValue(function.Body.Statements[index].(*ast.LetStatement).Value)
		if !ok || value.Int64() != want {
			t.Errorf("constant %d = %v, %t; want %d", index, value, ok, want)
		}
	}
}

func TestBitExtractionRejections(t *testing.T) {
	_, _, errors := analyzeBitExtraction(t, `
module main

type Sensor register[16] {
    Reading: bit[12],
    _: bit[4],
}

type Percent int range 0..100

fn Invalid(sensor: Sensor, source: uint8, width: int, p: Percent, flag: bool, r: rune, f: float64, wide: uint16) void {
    let zero := bit[0](source)
    let tooWide := bit[9](source)
    let huge := bit[257](source)
    let dynamic := bit[width](source)
    let named := bit[4](p)
    let boolean := bit[1](flag)
    let scalar := bit[8](r)
    let floating := bit[8](f)
    sensor.Reading = bit[13](wide)
    sensor.Reading = wide
}
`, 64)
	wants := []string{
		"bit[0] extraction width must be in 1..256",
		"bit[9] exceeds the 8-bit width of uint8",
		"bit[257] extraction width must be in 1..256",
		"bit[N] extraction width must be a compile-time integer constant",
		"cannot extract bits from Percent",
		"cannot extract bits from bool",
		"cannot extract bits from rune",
		"cannot extract bits from float64",
		"cannot assign uint16 to bit[12]",
		"cannot assign uint16 to bit[12]",
	}
	if len(errors) != len(wants) {
		t.Fatalf("errors = %v", errors)
	}
	for index, want := range wants {
		if !strings.Contains(errors[index].Message, want) {
			t.Errorf("error %d = %q, want %q", index, errors[index].Message, want)
		}
		if errors[index].Line == 0 {
			t.Errorf("error %d has no source position", index)
		}
	}

	_, _, errors = analyzeBitExtraction(t, "module main\n\nfn F(value: int) uint64 {\n    return bit[64](value)\n}\n", 32)
	if len(errors) != 1 || !strings.Contains(errors[0].Message, "bit[64] exceeds the 32-bit width of int") {
		t.Fatalf("32-bit int source = %v, want the target width from the CompilationPlan", errors)
	}
}
