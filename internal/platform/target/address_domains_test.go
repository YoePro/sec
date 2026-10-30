package target

import (
	"testing"

	"sec/internal/layout"
)

// MD-111 §§ 1(6)-1(7): CPU targets supply the linear byte-addressed
// cpu-physical domain with the CPU address width through the frozen plan;
// WebAssembly has no physical address domain.
// Rules: rules/platform/platform_model.md — § 10 (Amendment MD-111).
func TestTargetsSupplyAddressDomains(t *testing.T) {
	for _, test := range []struct {
		target Target
		bits   uint16
		has    bool
	}{
		{Target{OS: "linux", Arch: "amd64"}, 64, true},
		{Target{OS: "baremetal", Arch: "cortex-m4"}, 32, true},
		{Target{OS: "freertos", Arch: "cortex-m7"}, 32, true},
		{Target{OS: "unknown", Arch: "wasm32"}, 0, false},
	} {
		definition, ok := Find(test.target)
		if !ok {
			t.Fatalf("missing target %v", test.target)
		}
		plan, err := definition.ScalarPlan()
		if err != nil {
			t.Fatal(err)
		}
		domain, found := plan.AddressDomain("cpu-physical")
		if found != test.has || found && (domain.AddressBits != test.bits || domain.UnitBits != 8 || !domain.Linear) {
			t.Fatalf("%v cpu-physical = %+v (%v)", test.target, domain, found)
		}
		if found && domain.Limit().BitLen() != int(test.bits)+1 {
			t.Fatalf("%v limit = %s", test.target, domain.Limit())
		}
	}
}

// A plan with a duplicate or malformed domain is rejected.
func TestScalarPlanRejectsInvalidAddressDomains(t *testing.T) {
	definition, _ := Find(Target{OS: "linux", Arch: "amd64"})
	plan, err := definition.ScalarPlan()
	if err != nil {
		t.Fatal(err)
	}
	for _, domains := range [][]layout.AddressDomain{
		{{Identity: "cpu-physical", AddressBits: 64, UnitBits: 8}, {Identity: "cpu-physical", AddressBits: 32, UnitBits: 8}},
		{{Identity: "", AddressBits: 64, UnitBits: 8}},
		{{Identity: "io", AddressBits: 0, UnitBits: 8}},
		{{Identity: "io", AddressBits: 16, UnitBits: 0}},
	} {
		plan.AddressDomains = domains
		if err := plan.Validate(); err == nil {
			t.Fatalf("invalid domains accepted: %+v", domains)
		}
	}
}
