package sema

import (
	"math/big"
	"os"
	"reflect"
	"testing"

	"sec/internal/ast"
	"sec/internal/lexer"
	"sec/internal/parser"
)

// TestEmptyDynamicArrayDefault separates an empty owning value from fixed-array
// construction and safe slices, including elements that have no default.
// Rules: rules/collections/collections.md — §6.2;
// rules/types/default_values.md — "Owning dynamic arrays", "Slices".
func TestEmptyDynamicArrayDefault(t *testing.T) {
	element := Type{Kind: ReferenceType, Name: "ref int"}
	owner := NewDynamicArrayType(element)
	value := DefaultValueOf(owner)
	if value.Kind != EmptyDynamicArrayDefault || value.ArrayElementDefault != nil || len(value.Elements) != 0 {
		t.Fatal(value)
	}
	if fixed := DefaultValueOf(NewFixedArrayType(element, big.NewInt(0))); fixed.Kind != ArrayDefault {
		t.Fatal("fixed zero-length array lost its distinct default", fixed)
	}
	for _, display := range []func(Type) (string, DefaultKind, bool){DefaultValueDisplay, func(typ Type) (string, DefaultKind, bool) { return DefaultValuePreview(typ, 1) }} {
		text, kind, ok := display(owner)
		if !ok || text != "[]" || kind != EmptyDynamicArrayDefault {
			t.Fatal(text, kind, ok)
		}
	}
}

// TestEmptyDynamicArrayDefaultAnalysis exercises real omitted initializers at
// every depth, aggregate recursion, absence of allocation requests and reanalysis.
// Rules: rules/collections/collections.md — §6.2;
// rules/types/default_values.md — "Mutable declarations", "Structs";
// rules/memory/allocation.md — §24.
func TestEmptyDynamicArrayDefaultAnalysis(t *testing.T) {
	for _, depth := range []AnalysisDepth{AnalysisInteractive, AnalysisStandard, AnalysisDeep} {
		for _, invalid := range []bool{false, true} {
			suffix, want := "valid", 0
			if invalid {
				suffix, want = "invalid", 2
			}
			path := "../../testdata/sema/dynamic_array_default_" + suffix + ".sec"
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			p := parser.New(lexer.NewWithFile(string(data), path))
			program := p.ParseProgram()
			if len(p.Errors()) != 0 {
				t.Fatal(p.Errors())
			}
			a := NewAnalyzerWithDepth(depth)
			for attempt := 0; attempt < 2; attempt++ {
				if errs := a.Analyze(program); len(errs) != want {
					t.Fatal(depth, invalid, attempt, errs)
				}
				if invalid {
					break
				}
				count := 0
				walkASTValue(reflect.ValueOf(program), func(node any) {
					binding, ok := node.(*ast.LetStatement)
					if !ok || !binding.SynthesizedDefault {
						return
					}
					count++
					typ, found := a.ResolvedTypeOf(binding.Value)
					if !found {
						t.Fatal("missing default type", binding.Name.Value)
					}
					resolution := DefaultValueOf(typ)
					switch binding.Name.Value {
					case "values":
						if arrayShapeOf(typ) == ArrayShapeDynamic {
							if resolution.Kind != EmptyDynamicArrayDefault {
								t.Fatal(resolution)
							}
						} else if resolution.ArrayElementDefault == nil || resolution.ArrayElementDefault.Kind != EmptyDynamicArrayDefault {
							t.Fatal(resolution)
						}
					case "box":
						if len(resolution.Fields) != 1 || resolution.Fields[0].Value.Kind != EmptyDynamicArrayDefault {
							t.Fatal(resolution)
						}
					}
				})
				if count != 3 {
					t.Fatal("synthesized bindings", count)
				}
				for _, fact := range a.AllocationFacts() {
					if fact.Knowledge == AllocationMayAllocate {
						t.Fatal("empty default allocated", fact)
					}
				}
				if facts := a.AllocationContextFacts(); len(facts) != 0 {
					t.Fatal("empty owner requested allocation context", facts)
				}
			}
		}
	}
}
