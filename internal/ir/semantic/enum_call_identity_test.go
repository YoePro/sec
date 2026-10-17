package semantic

import (
	"strings"
	"testing"
)

// Ordinary calls keep their selected body even when their signature resembles
// an enum conversion in either direction, including fallible results.
// Rules: rules/declarations/functions.md — §§ 19–20, 24;
// rules/declarations/enums.md — § 12; rules/compiler/semantic_ir.md — resolved calls.
func TestEnumConversionFactsPreserveOrdinaryCalls(t *testing.T) {
	module, err := analyzedModule(t, `module main
enum State { Idle = 17, Ready = 29 }
enum Failure error { Bad }
fn Read(value: State) int { return 42 }
fn Make(value: int) State { return State.Ready }
fn Checked(value: State) Result[int, Failure] { return Ok(42) }
fn Use() int {
    let state := Make(1)
    let checked := Checked(state)
    let explicit := int(state)
    let constructed := State(17)
    return Read(constructed)
}
`, 11)
	if err != nil {
		t.Fatal(err)
	}
	var calls []string
	counts := map[OpKind]int{}
	for _, fn := range module.Functions {
		if fn.Name != "Use" {
			continue
		}
		for _, block := range fn.Blocks {
			for _, op := range block.Operations {
				counts[op.Kind]++
				if op.Kind == OpDirectCall {
					calls = append(calls, string(op.Callee))
				}
			}
		}
	}
	for _, want := range []string{"main::Make(int)", "main::Checked(main::State)", "main::Read(main::State)"} {
		if !strings.Contains(strings.Join(calls, "\n"), want) {
			t.Errorf("missing selected call %s: %v", want, calls)
		}
	}
	if counts[OpEnumToInteger] != 1 || counts[OpEnumFromInteger] != 1 || len(calls) != 3 {
		t.Fatalf("calls=%v counts=%v\n%s", calls, counts, Format(module))
	}
}
