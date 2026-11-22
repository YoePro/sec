package semantic

import (
	"os"
	"testing"
)

// TestNegativeLiteralProofReachesIR materializes safe negative constants while
// keeping real dynamic negation checked; all resulting modules verify.
// Rules: rules/errors/runtime_checks.md — §§5.1,7;
// rules/compiler/semantic_ir.md — §58.
func TestNegativeLiteralProofReachesIR(t *testing.T) {
	for _, width := range []uint16{32, 64} {
		for _, file := range []string{"try_negative_literal_valid.sec", "negative_literal_boundaries.sec"} {
			data, err := os.ReadFile("../../../testdata/sema/" + file)
			if err != nil {
				t.Fatal(err)
			}
			_, module, err := buildWithPlan(t, string(data), width)
			if err != nil {
				t.Fatal(width, file, err)
			}
			if err := Verify(module); err != nil {
				t.Fatal(width, file, err)
			}
			constants := map[string]bool{}
			negations := 0
			for _, function := range module.Functions {
				for _, block := range function.Blocks {
					for _, op := range block.Operations {
						if op.Kind == OpConstInt {
							constants[op.Integer.String()] = true
						}
						if op.Kind == OpIntNegChecked {
							negations++
						}
					}
				}
			}
			if file == "try_negative_literal_valid.sec" && (!constants["-1"] || negations != 0) {
				t.Fatal(constants, negations, Format(module))
			}
			if file == "negative_literal_boundaries.sec" && (!constants["-2147483648"] || !constants["-9223372036854775808"] || !constants["0"] || negations != 1) {
				t.Fatal(constants, negations, Format(module))
			}
		}
	}
}
