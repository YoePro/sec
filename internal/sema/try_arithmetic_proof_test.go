package sema

import (
	"os"
	"testing"

	"sec/internal/lexer"
	"sec/internal/parser"
)

// A Result call retains its own error channel when argument checks are proven
// impossible from constants, widening, a narrow property or a dominating guard.
// Unbounded int64 parameters/fields still contribute ArithmeticError.
// Rules: rules/errors/errorhandling.md — §§11,13.1;
// rules/errors/runtime_checks.md — Static proof and check elimination.
func TestTryArgumentArithmeticProof(t *testing.T) {
	for _, test := range []struct {
		name   string
		errors int
	}{{"valid", 0}, {"unproven_invalid", 3}} {
		source, err := os.ReadFile("../../testdata/sema/try_arithmetic_proof/" + test.name + ".sec")
		if err != nil {
			t.Fatal(err)
		}
		errors := analyzeSourceRaw(t, string(source))
		if len(errors) != test.errors {
			t.Fatalf("%s: %v", test.name, errors)
		}
	}
}

// The canonical proof excludes impossible failure points without changing the
// checked operator representation or the Result call's actual error channel.
// Rules: rules/errors/errorhandling.md — §§11,13.1;
// rules/compiler/semantic_ir.md — resolved semantic facts.
func TestTryArgumentArithmeticProofFacts(t *testing.T) {
	source, err := os.ReadFile("../../testdata/sema/try_arithmetic_proof/valid.sec")
	if err != nil {
		t.Fatal(err)
	}
	p := parser.New(lexer.New(string(source)))
	program := p.ParseProgram()
	if len(p.Errors()) != 0 {
		t.Fatal(p.Errors())
	}
	a := NewAnalyzer()
	if errs := a.Analyze(program); len(errs) != 0 {
		t.Fatal(errs)
	}
	proven := 0
	for _, operator := range a.resolvedOperators {
		if operator.ArithmeticProvenSafe {
			proven++
			if !operator.RuntimeCheck {
				t.Fatal("proof unexpectedly removed checked representation")
			}
		}
	}
	if proven != 6 || len(a.resolvedTries) != 6 {
		t.Fatal("missing argument proofs or try facts", proven, len(a.resolvedTries))
	}
	for _, plan := range a.resolvedTries {
		if plan.ErrorType.Name != "BusinessError" {
			t.Fatal("call error channel changed", plan.ErrorType)
		}
	}
}
