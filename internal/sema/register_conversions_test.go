package sema

import (
	"strings"
	"testing"

	"sec/internal/ast"
)

const md110Registers = `
module main

type Packet8 register[8] { Data: bit[8] }
type Packet12 register[12] { Data: bit[12] }
type Packet16 register[16] { Data: bit[16] }
`

// MD-110 §§ 3–5, 11(1)–(4): an integer builds a register from its
// bit pattern — a signed negative value is a valid two's-complement pattern —
// with a constant, source-domain, bit-extraction or guarded (checked) proof,
// and a register converts to an integer of at least its width.
func TestMD110RegisterBitPatternConversions(t *testing.T) {
	analyzer, errors := analyzeSourceWithAnalyzerRaw(t, md110Registers+`
fn Build(small: int8, byteValue: uint8, wide: int16, any16: uint16, big: int32) Result[Packet8, ArithmeticError] {
    let a := Packet8(-1)
    let b := Packet8(255)
    let c := Packet8(small)
    let d := Packet8(byteValue)
    let e := Packet16(wide)
    let f := Packet16(byteValue)
    let g := Packet8(bit[8](any16))
    let h := try Packet8(any16)
    let i := try Packet8(big)
    discard a
    discard b
    discard c
    discard d
    discard e
    discard f
    discard g
    discard h
    return Ok(i)
}

fn Read(p: Packet8, q: Packet12) int32 {
    let u8 := uint8(p)
    let i8 := int8(p)
    let u16 := uint16(p)
    let i16 := int16(q)
    discard u8
    discard i8
    discard u16
    return int32(i16)
}
`)
	assertSemaErrors(t, errors, nil)
	bases := map[string]RegisterFitBasis{}
	kinds := map[RegisterConversionKind]int{}
	for operand, conversion := range analyzer.resolvedRegisterConversions {
		kinds[conversion.Kind]++
		if conversion.Kind == RegisterFromBits {
			bases[operand.String()] = conversion.Basis
		}
	}
	for operand, want := range map[string]RegisterFitBasis{
		"(-1)": RegisterFitConstant, "255": RegisterFitConstant, "small": RegisterFitSourceDomain, "byteValue": RegisterFitSourceDomain,
		"wide": RegisterFitSourceDomain, "any16": RegisterFitGuarded, "big": RegisterFitGuarded,
	} {
		if bases[operand] != want {
			t.Fatalf("operand %s basis %q, want %q (all %v)", operand, bases[operand], want, bases)
		}
	}
	extraction := false
	for operand, conversion := range analyzer.resolvedRegisterConversions {
		if _, ok := operand.(*ast.BitExtractionExpression); ok && conversion.Basis == RegisterFitBitExtraction {
			extraction = true
		}
	}
	if !extraction || kinds[RegisterToBits] != 4 {
		t.Fatalf("bit-extraction proof %v, to-bits %d", extraction, kinds[RegisterToBits])
	}
}

// MD-110 §§ 3(4), 3(6), 5(3), 10, 11(2): a constant whose pattern needs more
// bits is rejected (try does not rescue it); named numeric types need their
// own conversion first; a register converts only to an integer at least as
// wide.
func TestMD110RegisterConversionRejections(t *testing.T) {
	errors := analyzeSourceRaw(t, md110Registers+`
type Count int32

fn Bad(count: Count, p: Packet12) Result[Packet8, ArithmeticError] {
    let a := Packet8(0x01FF)
    let b := Packet8(256)
    let c := Packet8(-129)
    let d := try Packet8(0x01FF)
    let e := Packet8(count)
    let f := uint8(p)
    return Ok(a)
}
`)
	want := []string{
		"integer value 511 does not fit register Packet8",
		"integer value 256 does not fit register Packet8",
		"integer value -129 does not fit register Packet8",
		"integer value 511 does not fit register Packet8",
		"register Packet8 is built from a primitive integer; Count keeps its own identity",
		"uint8 is 8 bits wide, narrower than register Packet12 (12 bits)",
	}
	joined := ""
	for _, err := range errors {
		joined += err.Error() + "\n"
	}
	for _, expected := range want {
		if !strings.Contains(joined, expected) {
			t.Fatalf("missing %q in:\n%s", expected, joined)
		}
	}
}
