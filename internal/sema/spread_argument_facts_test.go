package sema

import (
	"os"
	"reflect"
	"testing"

	"sec/internal/ast"
	"sec/internal/lexer"
	"sec/internal/parser"
)

// TestResolvedSpreadArguments checks the public post-selection facts rather
// than re-expanding AST arguments as a downstream consumer would have to do.
// Rules: rules/declarations/spread.md §§3,4.1–4.4; functions.md §38A.
func TestResolvedSpreadArguments(t *testing.T) {
	data, err := os.ReadFile("../../testdata/sema/spread_argument_facts_valid.sec")
	if err != nil {
		t.Fatal(err)
	}
	a, errors := analyzeSourceWithAnalyzerRaw(t, string(data))
	assertSemaErrors(t, errors, nil)
	counts := map[string]int{}
	for call := range a.resolvedCalls {
		fact, known := a.ResolvedCallTarget(call)
		if !known || fact.Arguments == nil {
			continue
		}
		plan := fact.Arguments
		counts[call.Callee.String()]++
		switch call.Callee.String() {
		case "Pair", "Consume", "Generic", "worker.Run":
			if len(plan.Sources) != 1 || len(plan.Expanded) != 2 {
				t.Fatalf("%s: %+v", call, plan)
			}
			for i, slot := range plan.Expanded {
				if slot.Source != 0 || slot.ElementIndex != int64(i) || slot.RuntimeSpread || !slot.Prepared || slot.ValueType.Kind != IntType || slot.ParameterIndex != i || slot.Operation != ArgumentCopyTrivial || slot.Delivery != ArgumentMove {
					t.Fatalf("%s slot %d: %+v", call, i, slot)
				}
				if slot.Parameter.Consuming != (call.Callee.String() == "Consume") {
					t.Fatalf("destination ownership lost: %+v", slot)
				}
			}
			if call.Arguments[0].(*ast.SpreadExpression).Value != plan.Sources[0].Expression {
				t.Fatal("source node identity changed")
			}
		case "Empty":
			if len(plan.Sources) != 1 || !plan.Sources[0].Spread || len(plan.Expanded) != 0 {
				t.Fatalf("empty spread source lost: %+v", plan)
			}
		case "Floats":
			if len(plan.Expanded) != 2 || plan.Sources[0].Type.Element.FloatBits != 32 {
				t.Fatalf("source scalar format lost: %+v", plan)
			}
			for _, slot := range plan.Expanded {
				if slot.ValueType.FloatBits != 32 || slot.Parameter.Type.FloatBits != 32 {
					t.Fatalf("prepared scalar format lost: %+v", slot)
				}
			}
		case "reader.Read":
			if fact.Kind != ResolvedInterfaceMethodCall || len(plan.Expanded) != 2 || plan.Expanded[1].Parameter.Name != "second" {
				t.Fatalf("interface destination lost: %+v", fact)
			}
		case "Mixed":
			wantSources := []int{0, 1, 1, 2, 3, 3}
			wantIndices := []int64{-1, 0, 1, -1, 0, 1}
			if len(plan.Sources) != 4 || len(plan.Expanded) != 6 {
				t.Fatalf("mixed plan %+v", plan)
			}
			for i, slot := range plan.Expanded {
				parameter := 1
				if i == 0 {
					parameter = 0
				}
				if slot.Source != wantSources[i] || slot.ElementIndex != wantIndices[i] || slot.ParameterIndex != parameter {
					t.Fatalf("mixed slot %d: %+v", i, slot)
				}
			}
		case "Sum":
			if len(plan.Expanded) != 1 || len(plan.Sources) != 1 {
				t.Fatalf("runtime plan %+v", plan)
			}
			slot := plan.Expanded[0]
			if !slot.RuntimeSpread || slot.ElementIndex != -1 || slot.ParameterIndex != 0 || slot.ValueType.Kind != IntType || !slot.Parameter.Variadic {
				t.Fatalf("runtime segment: %+v", slot)
			}
			want := ArgumentCopyTrivial
			if plan.Sources[0].Consuming {
				want = ArgumentWholePackTransfer
			}
			if slot.Operation != want {
				t.Fatalf("runtime ownership: %+v", slot)
			}
		default:
			t.Fatalf("unexpected spread %s", call)
		}
		// Returned slices, parameter types and nested source types are detached.
		plan.Sources[0].Type.Name = "changed"
		if plan.Sources[0].Type.Element != nil {
			plan.Sources[0].Type.Element.Name = "changed"
		}
		if len(plan.Expanded) != 0 {
			plan.Expanded[0].Parameter.Type.Name = "changed"
			plan.Expanded[0].ValueType.Name = "changed"
		}
		again, _ := a.ResolvedCallTarget(call)
		if reflect.DeepEqual(plan, again.Arguments) || again.Arguments.Sources[0].Type.Name == "changed" {
			t.Fatal("public facts alias stored plan")
		}
	}
	if !reflect.DeepEqual(counts, map[string]int{"Pair": 2, "Consume": 1, "Mixed": 1, "Empty": 1, "Generic": 1, "worker.Run": 1, "Sum": 4, "Floats": 1, "reader.Read": 1}) {
		t.Fatalf("missing accepted spread calls: %v", counts)
	}
	for call, forward := range a.typePackForwards {
		outer := map[string]Type{forward.pack: {Kind: GenericTypePackType, TypeArgs: []Type{{Kind: IntType, Name: "int"}, {Kind: BoolType, Name: "bool"}}}}
		resolved, problem := a.ResolvedTypePackForwardOf(call, outer)
		if problem != "" {
			t.Fatalf("%s: %s", call, problem)
		}
		if resolved.Arguments == nil || len(resolved.Arguments.Expanded) != len(resolved.Expanded) {
			t.Fatalf("missing specialization plan: %s", call)
		}
		for i, slot := range resolved.Arguments.Expanded {
			previous := resolved.Expanded[i]
			if slot.Source != previous.Source || slot.ElementIndex != int64(previous.Position) || slot.RuntimeSpread {
				t.Fatalf("specialization index lost: %+v", slot)
			}
			if previous.Position < 0 {
				continue
			}
			want := ArgumentCopyTrivial
			if forward.consuming {
				want = ArgumentWholePackTransfer
			} else if forward.mode == typePackSourceShared {
				want = ArgumentBorrowShared
			} else if forward.mode == typePackSourceMutable {
				want = ArgumentBorrowMutable
			}
			if slot.Operation != want {
				t.Fatalf("%s: %+v, want %s", call, slot, want)
			}
		}
		resolved.Arguments.Expanded[0].ValueType.Name = "changed"
		again, problem := a.ResolvedTypePackForwardOf(call, outer)
		if problem != "" || again.Arguments.Expanded[0].ValueType.Name == "changed" {
			t.Fatal("specialized facts alias an earlier query")
		}
	}
	if len(a.typePackForwards) != 4 {
		t.Fatalf("missing pack forwards: %d", len(a.typePackForwards))
	}
	var oldCall *ast.CallExpression
	for call, fact := range a.resolvedCalls {
		if fact.Arguments != nil {
			oldCall = call
			break
		}
	}
	p := parser.New(lexer.New("module main\nfn Clean() void {}\n"))
	program := p.ParseProgram()
	assertSemaErrors(t, a.Analyze(program), nil)
	if _, known := a.ResolvedCallTarget(oldCall); known {
		t.Fatal("reanalysis retained stale spread facts")
	}
}

// Invalid spread calls cannot publish a prepared-value plan. A later analysis
// replaces earlier call facts rather than retaining stale AST-keyed entries.
// Rules: rules/declarations/spread.md §§4.1–4.3; compiler_analysis.md §55.
func TestInvalidSpreadHasNoArgumentPlan(t *testing.T) {
	data, err := os.ReadFile("../../testdata/sema/spread_invalid.sec")
	if err != nil {
		t.Fatal(err)
	}
	a, errors := analyzeSourceWithAnalyzerRaw(t, string(data))
	if len(errors) == 0 {
		t.Fatal("invalid fixture was accepted")
	}
	for call := range a.resolvedCalls {
		fact, _ := a.ResolvedCallTarget(call)
		if fact.Arguments != nil {
			t.Fatalf("invalid spread published facts: %s", call)
		}
	}
}
