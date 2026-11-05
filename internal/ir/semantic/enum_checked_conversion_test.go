package semantic

import "testing"

// Checked enum conversions use ordinary numeric failure CFG; a potentially
// overflowing candidate never becomes an unconditional enum cast.
// Rules: rules/declarations/enums.md §12.1; rules/compiler/semantic_ir.md §124.
func TestCheckedEnumConversion(t *testing.T) {
	program, analyzer := sliceBoundaryInput(t, "semantic_ir/enum_checked_conversion/main.sec")
	module, err := Build(program, analyzer, BuildOptions{RequestedModule: "main", MaxPackage: 23})
	if err != nil {
		t.Fatal(err)
	}
	if err := Verify(module); err != nil {
		t.Fatal(err)
	}
	counts := map[OpKind]int{}
	for _, function := range module.Functions {
		calls := 0
		for _, block := range function.Blocks {
			for _, operation := range block.Operations {
				counts[operation.Kind]++
				if operation.Kind == OpDirectCall {
					calls++
				}
				if operation.Kind == OpEnumToInteger && (operation.ConversionProof == nil || operation.ConversionProof.Basis != "source-domain") {
					t.Fatal("carrier extraction lost its exactness proof", operation)
				}
				if operation.Kind == OpNumericConvertChecked && operation.ConversionFailure != "OutOfRange" {
					t.Fatal("wrong failure", operation)
				}
			}
		}
		if function.Name == "FromCall" && calls != 1 {
			t.Fatalf("operand evaluated %d times", calls)
		}
	}
	if counts[OpNumericConvertChecked] != 8 || counts[OpEnumToInteger] != 8 || counts[OpResultErr] < 3 {
		t.Fatalf("lost checked conversions or carriers: %v", counts)
	}
}
