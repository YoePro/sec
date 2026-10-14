package semantic

import (
	"os"
	"testing"

	"sec/internal/lexer"
	"sec/internal/parser"
	"sec/internal/sema"
)

// Rules: rules/foundations/operators.md — "Logical negation", "Evaluation order".
func TestBooleanNegationKeepsOneTypedNonFallibleOperation(t *testing.T) {
	data, err := os.ReadFile("../../../testdata/semantic_ir/boolean_negation/valid.sec")
	if err != nil {
		t.Fatal(err)
	}
	for _, pkg := range []uint8{7, 12, 14} {
		module, err := analyzedModule(t, string(data), pkg)
		if err != nil {
			t.Fatalf("package %d: %v", pkg, err)
		}
		if err := Verify(module); err != nil {
			t.Fatal(err)
		}
		for name, want := range map[string]int{"Not": 1, "Double": 2, "Once": 1, "EqualNegated": 2, "Choice": 3, "Stored": 2} {
			fn := loopFunction(t, module, name)
			if got := countOperations(fn, OpBoolNot); got != want || countOperations(fn, OpArithmeticFailure) != 0 {
				t.Fatalf("%s negations = %d, want %d, no arithmetic failure", name, got, want)
			}
		}
		once := loopFunction(t, module, "Once")
		ops := once.Blocks[once.Entry].Operations
		if countOperations(once, OpDirectCall) != 1 || len(ops) < 3 || ops[0].Kind != OpDirectCall || ops[1].Kind != OpBoolNot || ops[1].Operands[0] != ops[0].Results[0].ID {
			t.Fatal("negation did not evaluate and consume its call operand exactly once")
		}
	}
}

func TestBooleanNegationRejectsNumericTruthiness(t *testing.T) {
	path := "../../../testdata/semantic_ir/boolean_negation/operand_invalid.sec"
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	p := parser.New(lexer.NewWithFile(string(data), path))
	program := p.ParseProgram()
	if len(p.Errors()) != 0 {
		t.Fatal(p.Errors())
	}
	if errors := sema.NewAnalyzer().Analyze(program); len(errors) == 0 {
		t.Fatal("numeric truthiness was accepted")
	}
}
