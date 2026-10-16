package target

import (
	"testing"

	"sec/internal/layout"
)

// The WebAssembly targets are registered as planned freestanding modules:
// they parse and check, resolve 32/64-bit scalar plans and the WebAssembly
// BasicCABI data models, and do not claim emission, linking or running.
//
// Rules:
//   - rules/platform/platform_model.md — target registry and target selection
//   - rules/platform/abi.md — § 19 C data models
func TestWebAssemblyTargetsArePlannedFreestanding(t *testing.T) {
	for _, test := range []struct {
		name    string
		triple  string
		pointer uint16
		model   layout.CABIModel
	}{
		{"unknown-wasm32", "wasm32-unknown-unknown", 32, layout.CModelWasm32},
		{"unknown-wasm64", "wasm64-unknown-unknown", 64, layout.CModelWasm64},
	} {
		target, ok := Parse(test.name)
		if !ok {
			t.Fatalf("%s does not parse", test.name)
		}
		definition, ok := Find(target)
		if !ok || definition.LLVMTriple != test.triple || definition.PointerWidthBits != test.pointer || definition.Profile != "freestanding" {
			t.Fatalf("%s: %+v", test.name, definition)
		}
		if definition.Status != Planned || !definition.CanParse || !definition.CanCheck || definition.CanEmitLLVM || definition.CanLink || definition.CanRun {
			t.Fatalf("%s claims more than parse/check: %+v", test.name, definition)
		}
		if definition.CABI.Name != test.model.Name || !definition.CABI.CharSigned || definition.CABI.LongDouble != layout.CFloatBinary128 {
			t.Fatalf("%s C ABI: %+v", test.name, definition.CABI)
		}
		plan, err := definition.ScalarPlan()
		if err != nil {
			t.Fatalf("%s scalar plan: %v", test.name, err)
		}
		if err := plan.Validate(); err != nil {
			t.Fatalf("%s scalar plan invalid: %v", test.name, err)
		}
	}
}
