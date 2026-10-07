package main

import (
	"errors"
	"os"
	"testing"

	"sec/internal/lexer"
	"sec/internal/parser"
	"sec/internal/sema"
)

// Rules: rules/compiler/compiler_pipeline.md — §§23(3–4), 32–34.
func TestLegacyBackendIteratorPrerequisiteUsesRetainedAnalyzer(t *testing.T) {
	data, err := os.ReadFile("../../testdata/ir/iterator_readiness_valid.sec")
	if err != nil {
		t.Fatal(err)
	}
	p := parser.New(lexer.NewWithFile(string(data), "iterator.sec"))
	program := p.ParseProgram()
	if errs := p.Errors(); len(errs) != 0 {
		t.Fatal(errs)
	}
	analyzer := sema.NewAnalyzer()
	analyzed := analyzedProgram{Program: program, Analyzer: analyzer}
	var readiness *sema.IteratorLoweringReadinessError
	if err := validateIteratorReadiness(analyzed, "iterator.sec"); !errors.As(err, &readiness) {
		t.Fatal("fresh analyzer reached legacy backend", err)
	}
	if errs := analyzer.Analyze(program); len(errs) != 0 {
		t.Fatal(errs)
	}
	if err := validateIteratorReadiness(analyzed, "iterator.sec"); err != nil {
		t.Fatal(err)
	}
	if err := validateIteratorReadiness(analyzed, "different.sec"); !errors.As(err, &readiness) {
		t.Fatal("absent module skipped prerequisite", err)
	}
	other := parser.New(lexer.NewWithFile(string(data), "iterator.sec")).ParseProgram()
	if err := validateIteratorReadiness(analyzedProgram{Program: other, Analyzer: analyzer}, "iterator.sec"); !errors.As(err, &readiness) {
		t.Fatal("stale facts reached legacy backend", err)
	}
	if err := validateIteratorReadiness(analyzedProgram{}, "iterator.sec"); !errors.As(err, &readiness) {
		t.Fatal("nil analysis accepted", err)
	}
}
