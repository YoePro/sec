package secmlir

import (
	"strconv"
	"strings"
	"testing"
)

// Folded unit metadata reads lower to ordinary typed Sec MLIR constants: the
// nominal core enum, a decimal Scale projection, and bool/uint results of
// folded UnitDimension equality and properties. The schema-11 verifiers
// accept them when SEC_MLIR_BIN names the built tools.
//
// Rules:
//   - rules/types/units.md — "Unit metadata value types", "Semantic IR and lowering"
//   - rules/corrections/md062-unit-metadata-value-types-correction-20261027.md — §§ 7(12), 12(7)–12(8), 13(8)
func TestUnitMetadataReadsEmitSchema11(t *testing.T) {
	source := `module main

unit m physical

impl <m> {
    Dimension: [length^1]
    Kind: UnitKind.Length
}

unit third physical

impl <third> {
    Dimension: [length^1]
    Kind: UnitKind.Length
    Scale: 1 / 3
}

fn Scale() decimal {
    return <third>.Scale
}

fn main() int {
    if <m>.Kind == UnitKind.Length && <m>.Dimension == <third>.Dimension && <third>.Dimension.AxisCount == 1 {
        return 0
    }
    return 1
}
`
	for _, width := range []uint16{32, 64} {
		t.Run(strconv.Itoa(int(width)), func(t *testing.T) {
			module := buildPackage15EmitModule(t, "unit_metadata", source)
			output, err := Emit(module, testPlan(width))
			if err != nil {
				t.Fatalf("emit: %v", err)
			}
			text := string(output)
			for _, want := range []string{`identity = "core::UnitKind"`, `"sec.enum.constant"() <{caseOrdinal = 2 : i32}>`, `coefficient = "3333333333333333333", lexeme = "0.3333333333333333333", scale = 19 : i32`} {
				if !strings.Contains(text, want) {
					t.Errorf("missing %q in:\n%s", want, text)
				}
			}
			verifyPackage15MLIR(t, "unit_metadata", output)
		})
	}
}
