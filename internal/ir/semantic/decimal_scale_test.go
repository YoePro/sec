package semantic

import (
	"strings"
	"testing"
)

const fixedScaleSource = `module main

type Negative decimal range -1.00..<0.000
type Money decimal scale 2
type Price Money range 1..10

type Holder struct {
    negative: Negative,
    money: Money,
    price: Price,
}

fn Make() Holder {
    return Holder {}
}

fn Shaped() Holder {
    return Holder { money: 1.5, price: 2 }
}
`

func decimalConstants(module *Module) []Operation {
	var constants []Operation
	for _, fn := range module.Functions {
		for _, block := range fn.Blocks {
			for _, op := range block.Operations {
				if op.Kind == OpConstDecimal {
					constants = append(constants, op)
				}
			}
		}
	}
	return constants
}

// MD-049 semantic_ir.md § 128(2): a fixed-scale named type carries its frozen
// scale and origin; defaults come from Sema's (coefficient, scale) pair and
// literals are placed exactly on the type's grid.
func TestFixedScaleDecimalFactsInSemanticIR(t *testing.T) {
	module, err := analyzedModule(t, fixedScaleSource, 15)
	if err != nil {
		t.Fatal(err)
	}
	if err := Verify(module); err != nil {
		t.Fatal(err)
	}
	text := Format(module)
	for _, want := range []string{
		`named "main::Negative" base !2 fixed-scale=3 origin=inferred`,
		`named "main::Money" base !2 fixed-scale=2 origin=explicit`,
		`fixed-scale=2 origin=inherited`,
		`const.decimal "-0.001" coefficient=-1 scale=3`,
		`const.decimal "0.00" coefficient=0 scale=2`,
		`const.decimal "1.00" coefficient=100 scale=2`,
		`coefficient=150 scale=2`,
		`coefficient=200 scale=2`,
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("missing %q in\n%s", want, text)
		}
	}
	data, err := Serialize(module)
	if err != nil {
		t.Fatal(err)
	}
	restored, err := Deserialize(data)
	if err != nil {
		t.Fatal(err)
	}
	if Format(restored) != text {
		t.Fatal("fixed-scale facts changed through serialization")
	}
}

// semantic_ir.md § 128(3): a const.decimal of a fixed-scale type with another
// scale is rejected.
func TestFixedScaleDecimalVerifierRejectsWrongScale(t *testing.T) {
	module, err := analyzedModule(t, fixedScaleSource, 15)
	if err != nil {
		t.Fatal(err)
	}
	for _, fn := range module.Functions {
		for b := range fn.Blocks {
			for o := range fn.Blocks[b].Operations {
				op := &fn.Blocks[b].Operations[o]
				if op.Kind == OpConstDecimal {
					decimal := *op.Decimal
					decimal.Scale++
					op.Decimal = &decimal
				}
			}
		}
	}
	if err := Verify(module); err == nil || !strings.Contains(err.Error(), "fixed-scale") {
		t.Fatalf("verify = %v", err)
	}
}

// A literal that needs rounding to reach the fixed scale fails closed.
func TestFixedScaleDecimalLiteralNeedsExactPlacement(t *testing.T) {
	_, err := analyzedModule(t, "module main\n\ntype Money decimal scale 2\n\ntype Holder struct {\n    money: Money,\n}\n\nfn Bad() Holder {\n    return Holder { money: 1.555 }\n}\n", 15)
	if err == nil || !strings.Contains(err.Error(), "fixed scale 2") {
		t.Fatalf("err = %v", err)
	}
}

// A proven constant conversion lowers to const.decimal on the target grid.
func TestFixedScaleConstantConversionLowersToConstant(t *testing.T) {
	module, err := analyzedModule(t, "module main\n\ntype Amount decimal scale 2 range 0..1000\n\ntype Holder struct {\n    amount: Amount,\n}\n\nfn F() Holder {\n    return Holder { amount: Amount(12.340m) }\n}\n", 15)
	if err != nil {
		t.Fatal(err)
	}
	if text := Format(module); !strings.Contains(text, "coefficient=1234 scale=2") {
		t.Fatalf("conversion constant missing:\n%s", text)
	}
}
