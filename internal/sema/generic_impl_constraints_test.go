package sema

import (
	"os"
	"reflect"
	"sort"
	"strings"
	"testing"

	"sec/internal/ast"
	"sec/internal/lexer"
	"sec/internal/parser"
)

// TestGenericImplConstraintForwarding checks that primary and extended impl
// signatures may reuse their owner's constrained parameters as type arguments.
// Rules: rules/declarations/generics.md — §§8,12,14–15,25;
// rules/corrections/applied/generics-impl-fragments-correction-20260813.md —
// "Generic parameter rules".
func TestGenericImplConstraintForwarding(t *testing.T) {
	valid, err := os.ReadFile("../../testdata/sema/generic_impl_constraints_valid.sec")
	if err != nil {
		t.Fatal(err)
	}
	invalid, err := os.ReadFile("../../testdata/sema/generic_impl_constraints_invalid.sec")
	if err != nil {
		t.Fatal(err)
	}
	for _, depth := range []AnalysisDepth{AnalysisInteractive, AnalysisStandard, AnalysisDeep} {
		for _, test := range []struct {
			source  []byte
			invalid bool
		}{{valid, false}, {invalid, true}} {
			a := NewAnalyzerWithDepth(depth)
			p := parser.New(lexer.New(string(test.source)))
			program := p.ParseProgram()
			if len(p.Errors()) != 0 {
				t.Fatal(p.Errors())
			}
			errors := a.Analyze(program)
			if test.invalid {
				want := []string{
					"type OnlyFirst does not satisfy constraint Second for T",
					"type OnlySecond does not satisfy constraint First for T",
					"type StructuralOnlyType does not satisfy constraint First for T",
					"type StructuralOnlyType does not satisfy constraint Second for T",
					"type T does not satisfy constraint Second for T",
					"type T does not satisfy constraint Second for T",
					"type IntKey does not satisfy constraint Keyed[string] for K",
					"type K does not satisfy constraint Keyed[int] for K",
				}
				if len(errors) != len(want) {
					t.Fatal(depth, errors)
				}
				got := []string{}
				for _, diagnostic := range errors {
					got = append(got, diagnostic.Message)
				}
				sort.Strings(got)
				sort.Strings(want)
				if !reflect.DeepEqual(got, want) {
					t.Fatal(depth, got, want)
				}
				continue
			}
			if len(errors) != 0 {
				t.Fatal(depth, errors)
			}
			if errors := a.Analyze(program); len(errors) != 0 {
				t.Fatal("reanalyze", depth, errors)
			}
			for _, statement := range program.Statements {
				impl, ok := statement.(*ast.ImplStatement)
				if !ok || impl.Target.Name != "Box" {
					continue
				}
				fact, ok := a.GenericParameterFactAt(impl.Target.TypeArgs[0].Token)
				if !ok || fact.ImplTarget != "Box" || strings.Join(GenericConstraintDisplays(fact.Name, fact.Constraints), " & ") != "First & Second" {
					t.Fatal(depth, fact, ok)
				}
			}
			typ := a.functions["Use"][0].Parameters[0].Type
			if strings.Join(GenericConstraintDisplays("T", typ.GenericConstraints), " & ") != "First & Second" {
				t.Fatal(typ.GenericConstraints)
			}
			holder := a.functions["UseHolder"][0].Parameters[0].Type
			if strings.Join(GenericConstraintDisplays("K", holder.GenericConstraints), " & ") != "Keyed[int]" {
				t.Fatal(holder.GenericConstraints)
			}
		}
	}
}
