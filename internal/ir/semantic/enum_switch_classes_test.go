package semantic

import (
	"strings"
	"testing"
)

// Rules: rules/control-flow/flowcontrol_switch.md §§ 24, 32, 39;
// rules/declarations/enums.md § 7. Aliases share a coverage class, each
// alternative tests same-enum equality, and only proven exhaustive switches
// without default acquire an unreachable residual path.
func TestEnumSwitchValueClasses(t *testing.T) {
	module := buildForFixture(t, "../../../testdata/build/native/enum_switch_classes.sec")
	if err := Verify(module); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name       string
		exhaustive bool
	}{
		{"AliasCase", true}, {"Grouped", true},
		{"Defaulted", false}, {"Dynamic", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			fn := loopFunction(t, module, tc.name)
			comparisons, residuals := 0, 0
			var matched []BlockID
			var firstFailed BlockID
			var echoBlock BlockID
			for _, block := range fn.Blocks {
				for index, op := range block.Operations {
					if op.Kind == OpEnumCompare {
						comparisons++
						if op.IntegerCompare != IntegerCompareEQ {
							t.Fatalf("enum alternative uses %s", op.IntegerCompare)
						}
						if index+1 >= len(block.Operations) {
							t.Fatal("comparison has no branch")
						}
						branch := block.Operations[index+1]
						if branch.Kind != OpCondBranch || len(branch.Successors) != 2 {
							t.Fatal("comparison does not select body or next alternative")
						}
						matched = append(matched, branch.Successors[0].Block)
						if comparisons == 1 {
							firstFailed = branch.Successors[1].Block
						}
					}
					if op.Kind == OpUnreachable && op.Reason == "switch-unmatched-excluded-by-sema" {
						if !op.Synthesized {
							t.Fatal("exhaustive residual lacks synthesized provenance")
						}
						residuals++
					}
					if op.Kind == OpDirectCall && strings.Contains(string(op.Callee), "Echo") {
						echoBlock = block.ID
					}
				}
			}
			wantComparisons := 2
			if tc.name == "Defaulted" {
				wantComparisons = 1
			}
			if comparisons != wantComparisons || (residuals == 1) != tc.exhaustive || residuals > 1 {
				t.Fatalf("comparisons=%d residuals=%d, want %d and exhaustive=%v", comparisons, residuals, wantComparisons, tc.exhaustive)
			}
			if tc.name == "Grouped" && matched[0] != matched[1] {
				t.Fatal("comma alternatives do not enter the same body")
			}
			if tc.name == "Dynamic" && echoBlock != firstFailed {
				t.Fatal("dynamic case is not evaluated on the earlier test's failure edge")
			}
		})
	}
}
