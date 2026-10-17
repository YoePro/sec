package semantic

import (
	"strings"
	"testing"
)

// A subject switch lowers to explicit comparisons and branches: the subject
// is evaluated exactly once before the tests, a dynamic case value is
// evaluated only in its own test block (after earlier alternatives failed),
// range alternatives test both bounds, and fallthrough enters the next body
// directly (flowcontrol_switch.md §§ 4–8, 16, 39; semantic_ir.md § 68(2)).
func TestSwitchLowersToOrderedTests(t *testing.T) {
	module := buildForFixture(t, "../../../testdata/ir/switch_valid.sec")
	if err := Verify(module); err != nil {
		t.Fatal(err)
	}
	var classify, fall *Function
	for _, function := range module.Functions {
		switch function.Name {
		case "Classify":
			classify = function
		case "Fall":
			fall = function
		}
	}
	if classify == nil || fall == nil {
		t.Fatal("missing functions")
	}
	subjectCalls, limitBlock, firstCompareBlock := 0, -1, -1
	for index, block := range classify.Blocks {
		for _, op := range block.Operations {
			if op.Kind == OpDirectCall && strings.Contains(string(op.Callee), "Subject") {
				subjectCalls++
			}
			if op.Kind == OpDirectCall && strings.Contains(string(op.Callee), "Limit") {
				limitBlock = index
			}
			if op.Kind == OpIntCompare && firstCompareBlock < 0 {
				firstCompareBlock = index
			}
		}
	}
	if subjectCalls != 1 {
		t.Fatalf("subject evaluated %d times", subjectCalls)
	}
	if limitBlock <= firstCompareBlock {
		t.Fatalf("dynamic case value evaluated before the earlier alternative failed (blocks %d, %d)", limitBlock, firstCompareBlock)
	}
	text := Format(module)
	if !strings.Contains(text, "int.compare ge") || !strings.Contains(text, "int.compare lt") {
		t.Fatalf("range alternative lost a bound test:\n%s", text)
	}
	// Fall: the case-1 body ends in a branch straight into the case-2 body.
	adds := 0
	for _, block := range fall.Blocks {
		for _, op := range block.Operations {
			if op.Kind == OpIntBinaryChecked {
				adds++
			}
		}
	}
	if adds != 2 {
		t.Fatalf("fallthrough bodies: %d additions", adds)
	}
}
