package semantic

import (
	"strings"
	"testing"
)

// package20.md §§ 48–54, 71–72 and errorhandling.md § 11: a checked integer
// conversion under try is numeric.convert-checked; its failure edge routes
// the payload-less ConversionError.OutOfRange to propagation or to the local
// handlers, and the candidate is consumed only on the success edge.
func TestCheckedIntegerConversionUnderTry(t *testing.T) {
	module, err := buildPackage15Source(t, `
module main

// The test harness loads no core library; this mirrors sec/core/error.sec.
type ConversionError union error {
    OutOfRange,
    InvalidScalar,
    PrecisionLoss,
}

fn Narrow(x: int32) Result[int8, ConversionError] {
    let v := try int8(x)
    return Ok(v)
}

fn Recover(x: int64) uint16 {
    let v := try uint16(x) {
        Err(e) => 9
    }
    return v
}

fn main() int {
    return 0
}
`, "")
	if err != nil {
		t.Fatal(err)
	}
	if err := Verify(module); err != nil {
		t.Fatal(err)
	}
	text := Format(module)
	if strings.Count(text, "numeric.convert-checked") != 2 || !strings.Contains(text, "failure=OutOfRange") ||
		!strings.Contains(text, "union.construct") || !strings.Contains(text, "result.err") {
		t.Fatalf("checked conversions must route ConversionError:\n%s", text)
	}

	// The candidate must not be consumed on the failure edge: swapping the
	// branch on the failed flag makes the success block the failure edge.
	for _, fn := range module.Functions {
		if fn.Name != "Narrow" {
			continue
		}
		failed := map[ValueID]bool{}
		for _, block := range fn.Blocks {
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
	if err := Verify(module); err == nil || !strings.Contains(err.Error(), "candidate") {
		t.Fatalf("a candidate used where the conversion may have failed must be rejected, got %v", err)
	}
}

// Without try the checked conversion stays a frontend error; Semantic IR
// never sees an unprotected checked conversion.
func TestCheckedIntegerConversionNeedsTry(t *testing.T) {
	_, err := buildPackage15Source(t, `
module main

fn Narrow(x: int32) int8 {
    return int8(x)
}

fn main() int {
    return 0
}
`, "")
	if err == nil {
		t.Fatal("an unprotected checked conversion must not build")
	}
}
