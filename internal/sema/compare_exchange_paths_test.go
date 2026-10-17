package sema

import (
	"os"
	"strconv"
	"testing"

	"sec/internal/ast"
	"sec/internal/ast/walk"
	"sec/internal/layout"
	"sec/internal/lexer"
	"sec/internal/parser"
)

// TestCompareExchangePaths checks independent path orders, nominal payloads,
// same-operation observation and RMW classification on both target widths.
// Rules: rules/concurrency/atomics.md — §§17,24–25,27,46(3–4).
func TestCompareExchangePaths(t *testing.T) {
	for _, width := range []uint16{32, 64} {
		for _, fixture := range []struct {
			name  string
			calls int
		}{
			{"../../testdata/sema/atomic_orders_valid.sec", 16},
			{"../../testdata/sema/atomics/compare_exchange_result/paths_valid.sec", 3},
		} {
			t.Run(fixture.name+"/"+strconv.Itoa(int(width)), func(t *testing.T) {
				program := parseCompareExchangePaths(t, fixture.name)
				a := NewAnalyzerWithScalarPlan(layout.ResolvedScalarPlan{PointerWidthBits: width})
				if errors := a.Analyze(program); len(errors) != 0 {
					t.Fatal(errors)
				}
				count := 0
				for _, call := range compareExchangePathCalls(t, program) {
					plan, ok := a.ResolvedCompareExchangeOf(call)
					if !ok {
						t.Fatal("missing plan", call.String())
					}
					count++
					if len(call.Arguments) == 4 && call.Arguments[2].String() == "success" && plan.ElementType.Name != "Count" {
						t.Fatal("lost nominal carrier", plan.ElementType)
					}
					member := call.Callee.(*ast.MemberExpression)
					if plan.Storage != member.Object || plan.Expected != call.Arguments[0] || plan.Desired != call.Arguments[1] || plan.Source != call.Token {
						t.Fatal("lost call provenance", plan)
					}
					success, failure := "SeqCst", "SeqCst"
					if len(call.Arguments) == 4 {
						// Alias fixture deliberately requests stronger failure ordering.
						success, failure = "Relaxed", "SeqCst"
						if order, ok := call.Arguments[2].(*ast.MemberExpression); ok {
							success = order.Property.Value
						}
						if order, ok := call.Arguments[3].(*ast.MemberExpression); ok {
							failure = order.Property.Value
						}
					}
					if plan.Success.Variant != "Exchanged" || plan.Success.Order != success || !plan.Success.ReadsStorage || !plan.Success.ReadModifyWrite || !plan.Success.ModifiesStorage || !plan.Success.MayExtendReleaseSequence || plan.Success.PayloadType != nil || plan.Success.ObservedBy != nil {
						t.Fatal("incorrect success path", plan.Success)
					}
					if plan.Failure.Variant != "NotExchanged" || plan.Failure.Order != failure || !plan.Failure.ReadsStorage || plan.Failure.ReadModifyWrite || plan.Failure.ModifiesStorage || plan.Failure.MayExtendReleaseSequence || plan.Failure.ObservedBy != call || plan.Failure.PayloadType == nil || !sameConcreteType(*plan.Failure.PayloadType, plan.ElementType) {
						t.Fatal("incorrect failure path", plan.Failure)
					}
					if !isCompareExchangeResult(plan.ResultType) || plan.ResultType.ErrorAssignable || !sameConcreteType(*plan.ResultType.UnionVariants[1].Payload, plan.ElementType) {
						t.Fatal("incorrect result", plan.ResultType)
					}
					// Returned mutable type graphs must not alter later queries.
					original := plan.ElementType.Name
					plan.Failure.PayloadType.Name = "changed"
					plan.ResultType.TypeArgs[0].Name = "changed"
					plan.ResultType.UnionVariants[1].Payload.Name = "changed"
					again, _ := a.ResolvedCompareExchangeOf(call)
					if again.Failure.PayloadType.Name != original || again.ResultType.TypeArgs[0].Name != original || again.ResultType.UnionVariants[1].Payload.Name != original {
						t.Fatal("mutable fact escaped")
					}
				}
				if count != fixture.calls {
					t.Fatal("call count", count)
				}
				calls := compareExchangePathCalls(t, program)
				// Reuse with a declaration-free fixture: Analyzer retains the
				// compilation's named type registry between analysis runs.
				if errors := a.Analyze(parseCompareExchangePaths(t, "../../testdata/sema/atomic_orders_valid.sec")); len(errors) != 0 {
					t.Fatal(errors)
				}
				if _, ok := a.ResolvedCompareExchangeOf(calls[0]); ok {
					t.Fatal("stale plan survived reanalysis")
				}
			})
		}
	}
}

// TestCompareExchangePathsRejectInvalid ensures failed value/order/name/arity
// checks never produce a validated path plan for downstream consumers.
// Rules: rules/concurrency/atomics.md — §§12,17,24,45.
func TestCompareExchangePathsRejectInvalid(t *testing.T) {
	for _, fixture := range []string{
		"../../testdata/sema/atomic_orders_invalid.sec",
		"../../testdata/sema/atomic_applicability_invalid.sec",
		"../../testdata/sema/atomics/compare_exchange_result/paths_invalid.sec",
	} {
		program := parseCompareExchangePaths(t, fixture)
		a := NewAnalyzer()
		if errors := a.Analyze(program); len(errors) == 0 {
			t.Fatal("invalid fixture passed", fixture)
		}
		calls := compareExchangePathCalls(t, program)
		if len(calls) == 0 {
			t.Fatal("no calls", fixture)
		}
		for _, call := range calls {
			if _, ok := a.ResolvedCompareExchangeOf(call); ok {
				t.Fatal("invalid call has plan", call.String())
			}
		}
	}
	var absent *Analyzer
	if _, ok := absent.ResolvedCompareExchangeOf(nil); ok {
		t.Fatal("nil query succeeded")
	}
	if _, ok := NewAnalyzer().ResolvedCompareExchangeOf(&ast.CallExpression{}); ok {
		t.Fatal("unanalyzed call has plan")
	}
}

func parseCompareExchangePaths(t *testing.T, filename string) *ast.Program {
	t.Helper()
	source, err := os.ReadFile(filename)
	if err != nil {
		t.Fatal(err)
	}
	p := parser.New(lexer.NewWithFile(string(source), filename))
	program := p.ParseProgram()
	if len(p.Errors()) != 0 {
		t.Fatal(p.Errors())
	}
	return program
}

func compareExchangePathCalls(t *testing.T, program *ast.Program) []*ast.CallExpression {
	t.Helper()
	var calls []*ast.CallExpression
	if err := walk.Inspect(program, func(node any) error {
		call, ok := node.(*ast.CallExpression)
		if !ok {
			return nil
		}
		member, ok := call.Callee.(*ast.MemberExpression)
		if ok && member.Property != nil && member.Property.Value == "CompareExchange" {
			calls = append(calls, call)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	return calls
}
