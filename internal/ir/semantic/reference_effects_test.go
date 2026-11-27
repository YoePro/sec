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
		if !reachesOrdinaryFailure(function) {
			function.NoPanic, function.NoPanicSource = true, "written"
		}
	}
	err := Verify(module)
	if err == nil || !strings.Contains(err.Error(), "main::Dynamic()") || !strings.Contains(err.Error(), "reaches panic InvalidReferenceGeneration") {
		t.Fatalf("verify = %v, want the dynamic function rejected for InvalidReferenceGeneration", err)
	}
	module = package15Module()
	for _, function := range module.Functions {
		if function.Name != "Dynamic" && !reachesOrdinaryFailure(function) {
			function.NoPanic, function.NoPanicSource = true, "written"
		}
	}
	if err := Verify(module); err != nil {
		t.Fatalf("proven reference uses under @noPanic: %v", err)
	}
}

// reachesOrdinaryFailure reports a function with a checked-arithmetic or
// bounds failure endpoint, which @noPanic rejects on its own.
func reachesOrdinaryFailure(function *Function) bool {
	for _, block := range function.Blocks {
		for _, op := range block.Operations {
			if op.Kind == OpArithmeticFailure || op.Kind == OpBoundsFailure {
				return true
			}
		}
	}
	return false
}

// A @noPanic function that still reaches an ordinary checked-arithmetic or
// bounds failure endpoint contradicts the guarantee Sema verified.
// Rules: rules/errors/panic.md — § 21 "@noPanic";
// rules/compiler/semantic_ir.md — § 55(3).
func TestNoPanicRejectsReachableArithmeticAndBoundsFailure(t *testing.T) {
	module := package15Module()
	marked := 0
	for _, function := range module.Functions {
		if reachesOrdinaryFailure(function) {
			function.NoPanic, function.NoPanicSource = true, "written"
			marked++
		}
	}
	if marked == 0 {
		t.Fatal("fixture has no arithmetic or bounds failure endpoint")
	}
	err := Verify(module)
	if err == nil || !strings.Contains(err.Error(), "@noPanic function reaches fail.") {
		t.Fatalf("verify = %v, want an arithmetic or bounds failure rejected under @noPanic", err)
	}
}

// A Sema-accepted @noPanic function with a proven constant index lowers
// without a failure endpoint, so the contradiction check accepts it.
// Rules: rules/errors/panic.md — § 21 "@noPanic".
func TestNoPanicAcceptsProvenIndex(t *testing.T) {
	module, err := hostedModule(t, "module main\n\n@noPanic\nfn Safe(v: int[3]) int {\n    return v[1]\n}\n\nfn main() int {\n    return 0\n}\n", 18)
	if err != nil {
		t.Fatal(err)
	}
	if err := Verify(module); err != nil {
		t.Fatalf("proven index under @noPanic: %v", err)
	}
}
