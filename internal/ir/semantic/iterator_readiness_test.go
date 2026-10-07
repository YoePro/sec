package semantic

import (
	"errors"
	"os"
	"testing"

	"sec/internal/lexer"
	"sec/internal/parser"
	"sec/internal/sema"
)

// Rules: rules/compiler/compiler_pipeline.md — §§32(2–5), 33(1–3), 34(2–3).
func TestBuildIteratorReadinessBeforeRepresentation(t *testing.T) {
	data, err := os.ReadFile("../../../testdata/ir/iterator_readiness_valid.sec")
	if err != nil {
		t.Fatal(err)
	}
	p := parser.New(lexer.NewWithFile(string(data), "iterator.sec"))
	program := p.ParseProgram()
	if errs := p.Errors(); len(errs) != 0 {
		t.Fatal(errs)
	}
	analyzer := sema.NewAnalyzer()
	options := BuildOptions{RequestedModule: "main", MaxPackage: 14}
	module, err := Build(program, analyzer, options)
	var readiness *sema.IteratorLoweringReadinessError
	if module != nil || !errors.As(err, &readiness) || readiness.Issues[0].Classification != "Unproven" {
		t.Fatal("missing facts entered representation", module, err)
	}
	if errs := analyzer.Analyze(program); len(errs) != 0 {
		t.Fatal(errs)
	}
	if err := analyzer.ValidateIteratorLoweringReadiness(program, "main"); err != nil {
		t.Fatal(err)
	}
	module, err = Build(program, analyzer, options)
	var unsupported *UnsupportedFeatureError
	if module != nil || !errors.As(err, &unsupported) {
		t.Fatal("valid protocol confused with invalid source", module, err)
	}
	// P14 still lacks iterator CFG/reference/cleanup lowering. The readiness
	// gate must not claim implementation support after checking resolved facts.
	if errors.As(err, &readiness) {
		t.Fatal("complete plan rejected", err)
	}
	other := parser.New(lexer.NewWithFile(string(data), "iterator.sec")).ParseProgram()
	module, err = Build(other, analyzer, options)
	if module != nil || !errors.As(err, &readiness) {
		t.Fatal("stale snapshot entered representation", module, err)
	}
}

// Rules: rules/compiler/compiler_pipeline.md — §§33(1–3), 34(2–3).
func TestBuildRejectsInvalidIteratorBeforeIR(t *testing.T) {
	data, err := os.ReadFile("../../../testdata/sema/iterator_lowering_readiness_invalid.sec")
	if err != nil {
		t.Fatal(err)
	}
	p := parser.New(lexer.NewWithFile(string(data), "invalid-iterator.sec"))
	program := p.ParseProgram()
	if errs := p.Errors(); len(errs) != 0 {
		t.Fatal(errs)
	}
	analyzer := sema.NewAnalyzer()
	if errs := analyzer.Analyze(program); len(errs) == 0 {
		t.Fatal("invalid fixture accepted")
	}
	module, err := Build(program, analyzer, BuildOptions{RequestedModule: "main"})
	var readiness *sema.IteratorLoweringReadinessError
	if module != nil || !errors.As(err, &readiness) {
		t.Fatal("invalid iterator reached representation", module, err)
	}
	for _, issue := range readiness.Issues {
		if issue.Classification != "Invalid" {
			t.Fatal(issue)
		}
	}
}

// Rules: rules/compiler/compiler_pipeline.md — §33(1–2).
func TestBuildIteratorReadinessDoesNotRequireProtocolForOtherLoops(t *testing.T) {
	data, err := os.ReadFile("../../../testdata/ir/iterator_readiness_builtin_valid.sec")
	if err != nil {
		t.Fatal(err)
	}
	analyzer := sema.NewAnalyzer()
	p := parser.New(lexer.NewWithFile(string(data), "builtin.sec"))
	program := p.ParseProgram()
	if errs := p.Errors(); len(errs) != 0 {
		t.Fatal(errs)
	}
	if errs := analyzer.Analyze(program); len(errs) != 0 {
		t.Fatal(errs)
	}
	if err := analyzer.ValidateIteratorLoweringReadiness(program, "main"); err != nil {
		t.Fatal(err)
	}
	module, err := Build(program, analyzer, BuildOptions{RequestedModule: "main"})
	var unsupported *UnsupportedFeatureError
	if module != nil || !errors.As(err, &unsupported) {
		t.Fatal("built-in loop acquired a protocol requirement", module, err)
	}
}
