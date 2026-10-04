package target

import "testing"

// linux-riscv64 and freebsd-armv7 are registered hosted targets (decision
// 2026-10-04) with their own C ABI data models.
//
// Rules:
//   - rules/platform/platform_model.md — target registry
//   - rules/platform/abi.md — § 19 "C scalar representation"
func TestRegistryResolvesLinuxRISCV64AndFreeBSDARMv7(t *testing.T) {
	tests := []struct {
		target  string
		pointer uint16
		cabi    string
		triple  string
	}{
		{"linux-riscv64", 64, "riscv-lp64d", "riscv64-unknown-linux-gnu"},
		{"freebsd-armv7", 32, "aapcs-freebsd-gnueabihf", "armv7-unknown-freebsd-gnueabihf"},
	}
	for _, test := range tests {
		t.Run(test.target, func(t *testing.T) {
			parsed, ok := Parse(test.target)
			if !ok {
				t.Fatalf("Parse(%q) failed", test.target)
			}
			definition, ok := Find(parsed)
			if !ok {
				t.Fatalf("%s is not registered", test.target)
			}
			plan, err := definition.ScalarPlan()
			if err != nil {
				t.Fatal(err)
			}
			if plan.PointerWidthBits != test.pointer || plan.CABI.Name != test.cabi || plan.LLVMTriple != test.triple || plan.Profile != "hosted" {
				t.Fatalf("plan = %+v", plan)
			}
			if !definition.CanParse || !definition.CanCheck || definition.Status != Planned {
				t.Fatalf("definition = %+v", definition)
			}
		})
	}
}
