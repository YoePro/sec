package semantic

import (
	"errors"
	"testing"
)

func TestPackage10BuildsWideIntegerHandlers(t *testing.T) {
	module, err := analyzedModule(t, `module main
fn Wide(left: int128, right: int128) int128 {
  return try left + right { Err(error) => 0 }
}
fn Huge(left: uint256, right: uint256) uint256 {
  return try left - right { Err(error) => 0 }
}
`, 10)
	if err != nil {
		t.Fatal(err)
	}
	if err := Verify(module); err != nil {
		t.Fatal(err)
	}
	for _, function := range module.Functions {
		if countOperations(function, OpIntBinaryChecked) != 1 || countOperations(function, OpArithmeticErrorFromReason) != 1 {
			t.Fatalf("wide handler path missing in %s: %s", function.ID, Format(module))
		}
	}
}

func TestPackage10RejectsUnsupportedUserErrorLoweringExplicitly(t *testing.T) {
	_, err := analyzedModule(t, `module main
enum Failure { failed, }
fn Source(value: int) Result[int, Failure] { return Ok(value) }
fn Handle(value: int) int {
  return try Source(value) { Err(Failure.failed) => 0 }
}
`, 10)
	var unsupported *UnsupportedFeatureError
	if !errors.As(err, &unsupported) || unsupported.Feature != "type Failure" {
		t.Fatalf("unsupported user error lowering = %v", err)
	}
}

func TestPackage10BuildsOkDiscardAndExhaustiveVariantOnlyHandlers(t *testing.T) {
	module, err := analyzedModule(t, `module main
fn Divide(left: int, right: int) int {
  return try left / right {
    Ok(_) => 7
    Err(ArithmeticError.Overflow) => 1
    Err(ArithmeticError.DivisionByZero) => 2
    Err(ArithmeticError.InvalidShift) => 3
  }
}
`, 10)
	if err != nil {
		t.Fatal(err)
	}
	if err := Verify(module); err != nil {
		t.Fatal(err)
	}
	function := module.Functions[0]
	if countOperations(function, OpCoreErrorIsVariant) != 2 {
		t.Fatalf("exhaustive final variant should be the unmatched route: %s", Format(module))
	}
	foundDiscardHandler := false
	for _, block := range function.Blocks {
		for _, operation := range block.Operations {
			if operation.Kind == OpBranch && operation.TryHandlerKind == TryHandlerOK && operation.TryHandlerIndex == 0 {
				foundDiscardHandler = true
			}
		}
	}
	if !foundDiscardHandler {
		t.Fatalf("explicit Ok(_) handler provenance is missing: %s", Format(module))
	}
}
