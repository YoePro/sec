package semantic

import (
	"os"
	"strings"
	"testing"

	"sec/internal/lexer"
	"sec/internal/parser"
	"sec/internal/sema"
)

// Integer range and infinite `for` loops lower to explicit loop CFG records
// of their own kind; each range bound is evaluated exactly once before the
// loop, and the progression lives in compiler-generated cursor/flag storage
// (flowcontrol_for.md §§ 2, 23, 29; semantic_ir.md § 65(2), § 66).
func TestForLoopsLowerToLoopCFG(t *testing.T) {
	module := buildForFixture(t, "../../../testdata/ir/for_loops_valid.sec")
	var ranges *Function
	for _, function := range module.Functions {
		if function.Name == "Ranges" {
			ranges = function
		}
	}
	if ranges == nil {
		t.Fatal("missing Ranges")
	}
	kinds := []LoopKind{}
	for _, loop := range ranges.Loops {
		kinds = append(kinds, loop.Kind)
	}
	if len(kinds) != 3 || kinds[0] != LoopForRange || kinds[1] != LoopForRange || kinds[2] != LoopForInfinite {
		t.Fatalf("loop kinds = %v", kinds)
	}
	if !ranges.Loops[2].ConditionKnown || !ranges.Loops[2].ConditionValue || !ranges.Loops[2].ContinuesAfter {
		t.Fatalf("infinite loop record = %+v", ranges.Loops[2])
	}
	text := Format(module)
	if calls := strings.Count(text, `call.direct "main::Bound(int)"`); calls != 2 {
		t.Fatalf("range bounds must be evaluated exactly once each, got %d calls:\n%s", calls, text)
	}
	for _, storage := range []string{"i.cursor", "i.more", "_.cursor", "_.more"} {
		if !strings.Contains(text, storage) {
			t.Fatalf("missing progression storage %s:\n%s", storage, text)
		}
	}
	if err := Verify(module); err != nil {
		t.Fatal(err)
	}
}

// An explicit step is not lowered yet and is named as an unsupported
// construct instead of being approximated.
func TestForRangeExplicitStepIsUnsupported(t *testing.T) {
	data, err := os.ReadFile("../../../testdata/ir/unsupported_for_step.sec")
	if err != nil {
		t.Fatal(err)
	}
	analyzer := sema.NewAnalyzer()
	p := parser.New(lexer.NewWithFile(string(data), "step.sec"))
	program := p.ParseProgram()
	if errs := analyzer.Analyze(program); len(errs) != 0 {
		t.Fatal(errs)
	}
	_, err = Build(program, analyzer, BuildOptions{RequestedModule: "main"})
	if err == nil || !strings.Contains(err.Error(), "for range with a step that is not a compile-time constant") {
		t.Fatalf("explicit step: %v", err)
	}
}

func buildForFixture(t *testing.T, path string) *Module {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	analyzer := sema.NewAnalyzer()
	p := parser.New(lexer.NewWithFile(string(data), path))
	program := p.ParseProgram()
	if errs := p.Errors(); len(errs) != 0 {
		t.Fatal(errs)
	}
	if errs := analyzer.Analyze(program); len(errs) != 0 {
		t.Fatal(errs)
	}
	module, err := Build(program, analyzer, BuildOptions{RequestedModule: "main"})
	if err != nil {
		t.Fatal(err)
	}
	return module
}
