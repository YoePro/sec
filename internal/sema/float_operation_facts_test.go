package sema

import (
	"os"
	"reflect"
	"testing"

	"sec/internal/ast"
	"sec/internal/layout"
	"sec/internal/lexer"
	"sec/internal/parser"
)

// All primitive operator facts preserve source identity, target width, strict
// policies and the NaN-aware comparison mapping independently of analysis depth.
// Queries are read-only and detached, including the compound assignment API.
// Rules: rules/mlir/packages/sec-mlir-dialect_package20.md — §§6–9,20–27;
// rules/mlir/normative-sync/sec_p20_october_compatibility_package21.md — §2.
func TestResolvedFloatOperationFacts(t *testing.T) {
	data, err := os.ReadFile("../../testdata/sema/float_operation_facts.sec")
	if err != nil {
		t.Fatal(err)
	}
	predicates := map[ResolvedOperatorKind]string{
		ResolvedFloatCompareEQ: "oeq", ResolvedFloatCompareNE: "une",
		ResolvedFloatCompareLT: "olt", ResolvedFloatCompareLE: "ole",
		ResolvedFloatCompareGT: "ogt", ResolvedFloatCompareGE: "oge",
	}
	operators := map[ResolvedOperatorKind]string{
		ResolvedFloatUnaryPlus: "+", ResolvedFloatNegate: "-", ResolvedFloatAdd: "+",
		ResolvedFloatSubtract: "-", ResolvedFloatMultiply: "*", ResolvedFloatDivide: "/",
		ResolvedFloatRemainder: "%", ResolvedFloatCompareEQ: "==", ResolvedFloatCompareNE: "!=",
		ResolvedFloatCompareLT: "<", ResolvedFloatCompareLE: "<=", ResolvedFloatCompareGT: ">", ResolvedFloatCompareGE: ">=",
	}
	for _, width := range []uint16{0, 32, 64} {
		for _, depth := range []AnalysisDepth{AnalysisInteractive, AnalysisStandard, AnalysisDeep} {
			p := parser.New(lexer.New(string(data)))
			program := p.ParseProgram()
			if len(p.Errors()) != 0 {
				t.Fatal(p.Errors())
			}
			a := NewAnalyzerWithScalarPlanAndDepth(layout.ResolvedScalarPlan{PointerWidthBits: width}, depth)
			if errors := a.Analyze(program); len(errors) != 0 {
				t.Fatal(errors)
			}
			seen := map[string]map[ResolvedOperatorKind]bool{}
			for expression, resolved := range a.resolvedOperators {
				facts, ok := a.ResolvedFloatOperationOf(expression)
				if resolved.LeftType.Kind != FloatType {
					if ok {
						t.Fatalf("integer operator acquired float facts: %+v", facts)
					}
					continue
				}
				format := uint16(resolved.LeftType.FloatBits)
				if resolved.LeftType.Name == "float" {
					format = width
				}
				want := ResolvedFloatOperation{Kind: resolved.Kind, CommonType: resolved.LeftType.Name,
					FormatBits: format, PlanWidthBits: width, SourceOperator: operators[resolved.Kind],
					ComparisonPredicate: predicates[resolved.Kind], Rounding: "nearest-even", FastMath: "none",
					NoContraction: true, PreserveSubnormals: true}
				if resolved.Kind == ResolvedFloatRemainder {
					want.RemainderKind = "truncation"
				}
				if !ok || facts != want {
					t.Fatalf("width %d: facts %+v, want %+v", width, facts, want)
				}
				if resolved.RuntimeCheck || resolved.FailureBehavior != OperatorDoesNotFail {
					t.Fatal(resolved)
				}
				if seen[facts.CommonType] == nil {
					seen[facts.CommonType] = map[ResolvedOperatorKind]bool{}
				}
				seen[facts.CommonType][facts.Kind] = true
				facts.CommonType, facts.Rounding = "corrupt", "toward-zero"
				again, _ := a.ResolvedFloatOperationOf(expression)
				published, _ := a.ResolvedOperatorOf(expression)
				published.Float.FastMath = "fast"
				if again != want || a.resolvedOperators[expression].Float != want {
					t.Fatal("query aliases stored facts")
				}
			}
			for _, typ := range []string{"float", "float32", "float64"} {
				if len(seen[typ]) != len(operators) {
					t.Fatalf("%s missing operator kinds: %v", typ, seen[typ])
				}
			}
			if len(a.resolvedCompoundAssignments) != 15 {
				t.Fatal("compound fixture coverage changed")
			}
			for statement, resolved := range a.resolvedCompoundAssignments {
				facts, ok := a.ResolvedFloatCompoundAssignmentOf(statement)
				if !ok || facts != resolved.Float || facts.SourceOperator != operators[resolved.Kind] {
					t.Fatal(facts)
				}
				facts.PreserveSubnormals = false
				again, _ := a.ResolvedFloatCompoundAssignmentOf(statement)
				if !again.PreserveSubnormals {
					t.Fatal("compound query aliases stored facts")
				}
			}
			before := len(a.resolvedOperators)
			if _, ok := a.ResolvedFloatOperationOf(&ast.InfixExpression{}); ok {
				t.Fatal("unanalysed expression acquired facts")
			}
			if _, ok := a.ResolvedFloatCompoundAssignmentOf(&ast.AssignmentStatement{}); ok {
				t.Fatal("unanalysed assignment acquired facts")
			}
			if len(a.resolvedOperators) != before {
				t.Fatal("query mutated resolution state")
			}
		}
	}
}

// Nil, unanalysed and invalid operations cannot acquire a plan by querying.
// Rules: rules/mlir/packages/sec-mlir-dialect_package20.md — §21 common type.
func TestResolvedFloatOperationUnavailable(t *testing.T) {
	var absent *Analyzer
	if _, ok := absent.ResolvedFloatOperationOf(nil); ok {
		t.Fatal("nil has facts")
	}
	if _, ok := absent.ResolvedFloatCompoundAssignmentOf(nil); ok {
		t.Fatal("nil has facts")
	}
	data, err := os.ReadFile("../../testdata/sema/float_operation_facts_invalid.sec")
	if err != nil {
		t.Fatal(err)
	}
	p := parser.New(lexer.New(string(data)))
	program := p.ParseProgram()
	if len(p.Errors()) != 0 {
		t.Fatal(p.Errors())
	}
	a := NewAnalyzer()
	if errors := a.Analyze(program); len(errors) == 0 {
		t.Fatal("mixed float types accepted")
	}
	for expression := range a.expressionTypes {
		if _, ok := a.ResolvedFloatOperationOf(expression); ok {
			t.Fatal("invalid operator has a float plan")
		}
	}
	if _, ok := a.ResolvedFloatOperationOf(nil); ok {
		t.Fatal("nil has facts")
	}
	if _, ok := a.ResolvedFloatCompoundAssignmentOf(nil); ok {
		t.Fatal("nil has facts")
	}
	if !reflect.DeepEqual(ResolvedFloatOperation{}, a.resolvedFloatOperationFacts(ResolvedOperator{})) {
		t.Fatal("non-float operator has facts")
	}
}
