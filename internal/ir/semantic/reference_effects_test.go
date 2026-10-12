package semantic

import (
	"strings"
	"testing"
)

// P15-48: a @noPanic function may contain proven reference uses, but not a
// reachable fail.reference-generation of a dynamic-epoch validity guard.
//
// Rules:
//   - rules/mlir/packages/sec-mlir-dialect_package15.md — P15-48
//   - rules/errors/panic.md — § 21 "@noPanic"
func TestNoPanicRejectsReachableInvalidReferenceGeneration(t *testing.T) {
	module := package15Module()
	for _, function := range module.Functions {
		function.NoPanic, function.NoPanicSource = true, "written"
	}
	err := Verify(module)
	if err == nil || !strings.Contains(err.Error(), "main::Dynamic()") || !strings.Contains(err.Error(), "reaches panic InvalidReferenceGeneration") {
		t.Fatalf("verify = %v, want the dynamic function rejected for InvalidReferenceGeneration", err)
	}
	module = package15Module()
	for _, function := range module.Functions {
		if function.Name != "Dynamic" {
			function.NoPanic, function.NoPanicSource = true, "written"
		}
	}
	if err := Verify(module); err != nil {
		t.Fatalf("proven reference uses under @noPanic: %v", err)
	}
}
