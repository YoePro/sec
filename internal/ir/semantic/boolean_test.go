package semantic

import (
	"errors"
	"testing"
)

const booleanOperatorSource = `module main

fn Probe() bool {
    return true
}

fn And(ready: bool) bool {
    return ready && Probe()
}

fn Or(ready: bool) bool {
    return ready || Probe()
}

fn ConstantShortCircuit() bool {
    return true || Probe()
}

fn ConstantRequired(ready: bool) bool {
    return true && ready
}

fn Not(ready: bool) bool {
    return !ready
}

fn Inclusive(value: int) bool {
    return value in 0..100
}

fn Exclusive(value: uint8) bool {
    return value in 1..<10
}

fn Outside(value: int, low: int, high: int) bool {
    return value not in low..high
}

fn Loop(limit: int) int {
    let mut i := 0
    while i < limit && !(i in 40..50) {
        i += 1
    }
    return i
}
`

func blockOf(function *Function, kind OpKind) *Block {
	for _, block := range function.Blocks {
		for _, op := range block.Operations {
			if op.Kind == kind {
				return block
			}
		}
	}
	return nil
}

// `&&` and `||` evaluate the left operand first and the right operand only on
// the non-short-circuit edge; the short-circuit edge passes the left value to
// the merge block parameter. Proven constant left operands select the edge
// without building the unused operand.
//
// Rules:
//   - rules/foundations/operators.md — "Short-circuit evaluation"
//   - rules/compiler/semantic_ir.md — § 67(2)
func TestLogicalOperatorsBuildShortCircuitFlow(t *testing.T) {
	module, err := analyzedModule(t, booleanOperatorSource, 14)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	if err := Verify(module); err != nil {
		t.Fatalf("Verify: %v\n%s", err, Format(module))
	}
	for name, rhsFirst := range map[string]bool{"And": true, "Or": false} {
		function := loopFunction(t, module, name)
		entry := function.Blocks[function.Entry]
		branch := entry.Operations[len(entry.Operations)-1]
		if branch.Kind != OpCondBranch || branch.Operator == "" {
			t.Fatalf("%s entry does not end in the short-circuit branch: %+v", name, branch)
		}
		rhs, short := branch.Successors[0], branch.Successors[1]
		if !rhsFirst {
			rhs, short = short, rhs
		}
		if len(rhs.Arguments) != 0 || len(short.Arguments) != 1 || short.Arguments[0] != function.Parameters[0].Value.ID {
			t.Errorf("%s short-circuit edge does not carry the left value: %+v", name, branch.Successors)
		}
		if call := blockOf(function, OpDirectCall); call == nil || call.ID != rhs.Block {
			t.Errorf("%s right operand is not evaluated only on the non-short-circuit edge", name)
		}
		if merge := function.Blocks[short.Block]; len(merge.Parameters) != 1 {
			t.Errorf("%s merge block has parameters %+v", name, merge.Parameters)
		}
	}
	if function := loopFunction(t, module, "ConstantShortCircuit"); countOperations(function, OpDirectCall) != 0 || countOperations(function, OpCondBranch) != 0 {
		t.Errorf("a proven short circuit built its right operand")
	}
	if function := loopFunction(t, module, "ConstantRequired"); countOperations(function, OpCondBranch) != 0 {
		t.Errorf("a proven non-short-circuit left operand still branches")
	}
	if function := loopFunction(t, module, "Not"); countOperations(function, OpBoolNot) != 1 {
		t.Errorf("! did not build bool.not")
	}
}

// Range membership evaluates the value, then the bounds left to right, once
// each; `..` includes the upper bound, `..<` excludes it, literal bounds take
// the value's type, and `not in` negates after the test.
//
// Rules:
//   - rules/foundations/operators.md — "Membership expression", "Range membership", "Inclusive range", "Exclusive upper range"
func TestRangeMembershipBuildsBoundComparisons(t *testing.T) {
	module, err := analyzedModule(t, booleanOperatorSource, 14)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	predicates := func(name string) []IntegerComparePredicate {
		var got []IntegerComparePredicate
		for _, block := range loopFunction(t, module, name).Blocks {
			for _, op := range block.Operations {
				if op.Kind == OpIntCompare {
					got = append(got, op.IntegerCompare)
				}
			}
		}
		return got
	}
	for name, want := range map[string][]IntegerComparePredicate{
		"Inclusive": {IntegerCompareGE, IntegerCompareLE},
		"Exclusive": {IntegerCompareGE, IntegerCompareLT},
		"Outside":   {IntegerCompareGE, IntegerCompareLE},
	} {
		got := predicates(name)
		if len(got) != len(want) || got[0] != want[0] || got[1] != want[1] {
			t.Errorf("%s predicates = %v, want %v", name, got, want)
		}
	}
	exclusive := loopFunction(t, module, "Exclusive")
	for _, block := range exclusive.Blocks {
		for _, op := range block.Operations {
			if op.Kind == OpConstInt && op.Results[0].Type != exclusive.Parameters[0].Value.Type {
				t.Errorf("literal bound has type !%d, want the value's uint8 !%d", op.Results[0].Type, exclusive.Parameters[0].Value.Type)
			}
		}
	}
	outside := loopFunction(t, module, "Outside")
	if countOperations(outside, OpBoolNot) != 1 {
		t.Errorf("not in did not negate the membership result")
	}
	compare := outside.Blocks[outside.Entry].Operations[0]
	if compare.Kind != OpIntCompare || compare.Operands[0] != outside.Parameters[0].Value.ID || compare.Operands[1] != outside.Parameters[1].Value.ID {
		t.Errorf("first comparison is not value >= low: %+v", compare)
	}
	if loop := loopFunction(t, module, "Loop"); len(loop.Loops) != 1 || countOperations(loop, OpBoolNot) != 1 {
		t.Errorf("while condition with && and negated membership was not built")
	}
}

// Boolean operators outside the integer subset stay behind package gate 13,
// so the Sec MLIR path (package 12) keeps rejecting them explicitly.
func TestBooleanOperatorsAreRejectedBeforePackage13(t *testing.T) {
	_, err := analyzedModule(t, `module main

fn Not(ready: bool) bool {
    return !ready
}
`, 12)
	var unsupported *UnsupportedFeatureError
	if !errors.As(err, &unsupported) {
		t.Fatalf("err = %v, want an unsupported feature", err)
	}
}
