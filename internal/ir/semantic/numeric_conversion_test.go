package semantic

import (
	"strings"
	"testing"
)

const numericConversionSource = `module main

// The test harness loads no core library; this mirrors sec/core/error.sec.
type ConversionError union error {
    OutOfRange,
    InvalidScalar,
    PrecisionLoss,
    ScaleOverflow,
    NonFinite,
}

fn Widen(x: float32) float64 {
    return float64(x)
}

fn Truth(x: float64) bool {
    return bool(x)
}

fn ToInt(x: float64) Result[int32, ConversionError] {
    let y := try int32(x)
    return Ok(y)
}
`

// P20 conversions in Semantic IR: an exact widening carries the plan's proof,
// float-to-bool is float.to-bool, and a checked float-to-integer conversion
// lists NonFinite, OutOfRange and PrecisionLoss with one reason flag each and
// routes each reason to its own ConversionError variant.
// Rules: rules/mlir/packages/sec-mlir-dialect_package20.md — §§ 41, 51–56, 63, 70–72.
func TestNumericConversionOperations(t *testing.T) {
	module, err := buildPackage15Source(t, numericConversionSource, "")
	if err != nil {
		t.Fatal(err)
	}
	if err := Verify(module); err != nil {
		t.Fatalf("verify: %v\n%s", err, Format(module))
	}
	text := Format(module)
	for _, want := range []string{
		"numeric.convert-exact %0 proof=source-domain",
		"float.to-bool %0",
		"numeric.convert-checked %0 failures=NonFinite,OutOfRange,PrecisionLoss",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("Semantic IR lacks %q:\n%s", want, text)
		}
	}
	checked := findOperation(t, module, OpNumericConvertChecked)
	if len(checked.Results) != 5 {
		t.Fatalf("checked conversion results = %d, want candidate, failed and three reasons", len(checked.Results))
	}
	if strings.Count(text, "union.construct") < 3 {
		t.Errorf("each reason needs its own ConversionError variant:\n%s", text)
	}
}

// The verifier rejects a float checked conversion whose failures are out of
// priority order, repeated, unknown or without their reason flags, and a
// float.to-bool over a non-float.
// Rules: rules/mlir/packages/sec-mlir-dialect_package20.md — §§ 41, 55–56, 81.
func TestNumericConversionVerifierRejects(t *testing.T) {
	for name, mutate := range map[string]func(*Operation){
		"order":        func(op *Operation) { op.ConversionFailures = []string{"OutOfRange", "NonFinite", "PrecisionLoss"} },
		"repeated":     func(op *Operation) { op.ConversionFailures = []string{"NonFinite", "NonFinite", "PrecisionLoss"} },
		"unknown":      func(op *Operation) { op.ConversionFailures = []string{"NonFinite", "OutOfRange", "Contract"} },
		"flags":        func(op *Operation) { op.Results = op.Results[:4] },
		"both":         func(op *Operation) { op.ConversionFailure = "OutOfRange" },
		"success-only": nil,
		"to-bool":      nil,
	} {
		module, err := buildPackage15Source(t, numericConversionSource, "")
		if err != nil {
			t.Fatal(err)
		}
		if name == "success-only" {
			// Swapping the branch on the failed flag makes the candidate's
			// use sit on the failure edge.
			for _, function := range module.Functions {
				failed := map[ValueID]bool{}
				for _, block := range function.Blocks {
					for index := range block.Operations {
						op := &block.Operations[index]
						if op.Kind == OpNumericConvertChecked {
							failed[op.Results[1].ID] = true
						}
						if op.Kind == OpCondBranch && failed[op.Operands[0]] {
							op.Successors[0], op.Successors[1] = op.Successors[1], op.Successors[0]
						}
					}
				}
			}
		} else if mutate == nil {
			findOperation(t, module, OpFloatToBool)
			for _, function := range module.Functions {
				for _, block := range function.Blocks {
					for index := range block.Operations {
						if block.Operations[index].Kind == OpFloatToBool {
							block.Operations[index].Results[0].Type = function.Parameters[0].Value.Type
						}
					}
				}
			}
		} else {
			for _, function := range module.Functions {
				for _, block := range function.Blocks {
					for index := range block.Operations {
						if block.Operations[index].Kind == OpNumericConvertChecked {
							mutate(&block.Operations[index])
						}
					}
				}
			}
		}
		if err := Verify(module); err == nil {
			t.Errorf("%s: verifier accepted the mutation", name)
		}
	}
}

func findOperation(t *testing.T, module *Module, kind OpKind) *Operation {
	t.Helper()
	for _, function := range module.Functions {
		for _, block := range function.Blocks {
			for index := range block.Operations {
				if block.Operations[index].Kind == kind {
					return &block.Operations[index]
				}
			}
		}
	}
	t.Fatalf("no %s operation", kind)
	return nil
}
