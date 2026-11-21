package sema

import (
	"os"
	"strings"
	"testing"

	"sec/internal/ast"
	"sec/internal/diagnostics"
	"sec/internal/lexer"
	"sec/internal/parser"
)

// TestDeferFallibleHandling checks ordinary local Result/Option recovery,
// must-use and discard rules and independent lambda propagation. Cleanup
// failures cannot escape even through a matching enclosing return type.
// Rules: rules/control-flow/defer.md §§3,15–16;
// rules/errors/errorhandling.md §§7,12,15–16,22,32.
func TestDeferFallibleHandling(t *testing.T) {
	for _, name := range []string{"valid", "invalid"} {
		source, err := os.ReadFile("../../testdata/control_flow/defer_fallible/" + name + ".sec")
		if err != nil {
			t.Fatal(err)
		}
		errors := analyzeSourceRaw(t, string(source))
		if name == "valid" {
			if len(errors) != 0 {
				t.Fatal(errors)
			}
			continue
		}
		if len(errors) != 7 {
			t.Fatalf("errors: %+v", errors)
		}
		counts := map[string]int{}
		for _, err := range errors {
			counts[err.ID]++
			if err.ID == "" || err.Help == "" {
				t.Errorf("missing structured diagnostic: %+v", err)
			}
			if strings.Contains(err.Help, "binding, return,") || strings.Contains(err.Help, "change the function return type") {
				t.Errorf("illegal cleanup repair: %+v", err)
			}
		}
		if counts[diagnostics.UnhandledMustUseResult] != 1 || counts[diagnostics.TryPropagationIncompatible] != 4 || counts[diagnostics.TryResidualUnpropagatable] != 2 {
			t.Fatalf("unexpected diagnostics: %v / %+v", counts, errors)
		}
	}
}

// TestDeferFallibleHandlingPlans keeps validated cleanup handlers local in the
// compiler facts, while a nested lambda may propagate within its own invocation.
// Rejected cleanup bodies do not publish registration or assignment plans.
// Rules: rules/control-flow/defer.md §§3,15–16,29;
// rules/compiler/semantic_ir.md — resolved semantic facts.
func TestDeferFallibleHandlingPlans(t *testing.T) {
	for _, name := range []string{"valid", "invalid"} {
		source, err := os.ReadFile("../../testdata/control_flow/defer_fallible/" + name + ".sec")
		if err != nil {
			t.Fatal(err)
		}
		p := parser.New(lexer.New(string(source)))
		program := p.ParseProgram()
		if len(p.Errors()) != 0 {
			t.Fatal(p.Errors())
		}
		a := NewAnalyzer()
		a.Analyze(program)
		if name == "invalid" {
			if len(a.resolvedDeferPlans) != 0 || len(a.resolvedTryAssignments) != 0 {
				t.Fatal("invalid cleanup published executable plans")
			}
			continue
		}
		if len(a.resolvedDeferPlans) != 4 || len(a.resolvedTryAssignments) != 1 {
			t.Fatalf("defer/assignment plans: %d/%d", len(a.resolvedDeferPlans), len(a.resolvedTryAssignments))
		}
		for statement := range a.resolvedTryAssignments {
			plan, ok := a.ResolvedTryAssignmentOf(statement)
			if !ok || plan.Kind != ResolvedTryAssignmentHandled || plan.HandlerPlan.ResidualPropagates {
				t.Fatalf("cleanup assignment escaped: %+v", plan)
			}
		}
		for _, statement := range program.Statements {
			fn, ok := statement.(*ast.FunctionDeclaration)
			if !ok {
				continue
			}
			for _, statement := range fn.Body.Statements {
				deferred, ok := statement.(*ast.DeferStatement)
				if !ok {
					continue
				}
				if _, ok := a.ResolvedDeferPlanOf(deferred); !ok {
					t.Fatal("missing validated cleanup plan")
				}
			}
		}
	}
}
