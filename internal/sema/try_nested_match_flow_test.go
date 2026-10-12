package sema

import (
	"os"
	"strings"
	"testing"

	"sec/internal/lexer"
	"sec/internal/parser"
)

// TestTryNestedMatchReturns uses normalized match coverage and arm flow for
// a value-position handler that remaps every variant and returns the error.
// Rules: rules/errors/errorhandling.md — §§21,21.1;
// rules/control-flow/flowcontrol_match.md — §23 "Return and termination analysis".
func TestTryNestedMatchReturns(t *testing.T) {
	data, err := os.ReadFile("../../testdata/sema/try_handler_nested_match_valid.sec")
	if err != nil {
		t.Fatal(err)
	}
	p := parser.New(lexer.NewWithFile(string(data), "try_handler_nested_match_valid.sec"))
	program := p.ParseProgram()
	if len(p.Errors()) != 0 {
		t.Fatal(p.Errors())
	}
	a := NewAnalyzer()
	if errors := a.Analyze(program); len(errors) != 0 {
		t.Fatal(errors)
	}
	count := 0
	for _, plan := range a.resolvedTryPlans {
		for _, handler := range plan.Handlers {
			count++
			if handler.Flow != TryHandlerReturns {
				t.Fatalf("exhaustive returning match classified as %s", handler.Flow)
			}
		}
	}
	if count == 0 {
		t.Fatal("missing handler flow")
	}
}

// TestTryNestedMatchInvalidFlow distinguishes continuing match branches from
// an unreachable final recovery value after an exhaustive returning match.
// Rules: rules/errors/errorhandling.md — §§21–21.1;
// rules/control-flow/flowcontrol_match.md — §23.
func TestTryNestedMatchInvalidFlow(t *testing.T) {
	data, err := os.ReadFile("../../testdata/sema/try_handler_nested_match_invalid.sec")
	if err != nil {
		t.Fatal(err)
	}
	p := parser.New(lexer.New(string(data)))
	program := p.ParseProgram()
	if len(p.Errors()) != 0 {
		t.Fatal(p.Errors())
	}
	errors := NewAnalyzer().Analyze(program)
	if len(errors) != 2 || !strings.Contains(errors[0].Message, "try handler must return") || !strings.Contains(errors[1].Message, "unreachable recovery value") {
		t.Fatal(errors)
	}
}
