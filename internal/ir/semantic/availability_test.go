package semantic

import "testing"

// TestStaticAvailabilityTestLowersWithoutOwnershipState verifies the mandatory
// static fold and ensures no hidden runtime availability representation is
// introduced when Sema already knows the answer.
//
// Rules:
//   - rules/memory/ownership.md — §21.2 "Static resolution is mandatory"
//   - rules/compiler/semantic_ir.md — ownership availability facts
func TestStaticAvailabilityTestLowersWithoutOwnershipState(t *testing.T) {
	module, err := analyzedModule(t, `module main
fn Check(value: int) int {
  if value is available { return value }
  return 0
}
`, 14)
	if err != nil {
		t.Fatal(err)
	}
	if err := Verify(module); err != nil {
		t.Fatal(err)
	}
	counts := map[OpKind]int{}
	for _, block := range module.Functions[0].Blocks {
		for _, operation := range block.Operations {
			counts[operation.Kind]++
		}
	}
	if counts[OpConstBool] != 1 || counts[OpCondBranch] != 1 {
		t.Fatalf("static availability was not folded canonically: %#v\n%s", counts, Format(module))
	}
}
