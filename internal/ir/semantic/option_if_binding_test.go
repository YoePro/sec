package semantic

import "testing"

// TestOptionIfBindingEvaluatesAndProjectsOnce verifies the executable
// copy-trivial frontend slice without reconstructing binding semantics in the
// builder.
//
// Rules:
//   - rules/control-flow/flowcontrol_if.md — §12 "State tests" and §17 "Branch scopes"
//   - rules/compiler/semantic_ir.md — "Unsupported lowerings"
func TestOptionIfBindingEvaluatesAndProjectsOnce(t *testing.T) {
	module, err := analyzedModule(t, `module main
fn Source(value: int) Option[int] { return Some(value) }
fn Read(value: int) int {
  if Source(value) is Some(found) {
    return found
  }
  return 0
}
`, 12)
	if err != nil {
		t.Fatal(err)
	}
	if err := Verify(module); err != nil {
		t.Fatal(err)
	}

	read := module.Functions[1]
	counts := map[OpKind]int{}
	for _, block := range read.Blocks {
		for _, operation := range block.Operations {
			counts[operation.Kind]++
		}
	}
	if counts[OpDirectCall] != 1 || counts[OpUnionIsVariant] != 1 || counts[OpUnionUnwrapPayload] != 1 || counts[OpCondBranch] != 1 {
		t.Fatalf("Option presence binding is not single-evaluation canonical: %#v\n%s", counts, Format(module))
	}
}
