package semantic

import "testing"

// TestReturnTryResultForwardingBuildsBothTerminalCarriers verifies that
// Semantic IR consumes Sema's dedicated return-forwarding fact and reconstructs
// both Result outcomes after evaluating the protected expression once.
//
// Rules:
//   - rules/errors/errorhandling.md — §26 "return try expression"
//   - rules/compiler/semantic_ir.md — "Unsupported lowerings"
func TestReturnTryResultForwardingBuildsBothTerminalCarriers(t *testing.T) {
	module, err := analyzedModule(t, `module main
fn Source(value: int) Result[int, ArithmeticError] { return Ok(value) }
fn Forward(value: int) Result[int, ArithmeticError] { return try Source(value) }
`, 10)
	if err != nil {
		t.Fatal(err)
	}
	if err := Verify(module); err != nil {
		t.Fatal(err)
	}

	forward := module.Functions[1]
	counts := map[OpKind]int{}
	for _, block := range forward.Blocks {
		for _, operation := range block.Operations {
			counts[operation.Kind]++
		}
	}
	if counts[OpDirectCall] != 1 || counts[OpResultIsErr] != 1 || counts[OpResultUnwrapErr] != 1 || counts[OpResultUnwrapOk] != 1 || counts[OpResultErr] != 1 || counts[OpResultOk] != 1 || counts[OpReturn] != 2 {
		t.Fatalf("return try forwarding is not canonical: %#v\n%s", counts, Format(module))
	}
}
