package sema

import (
	"os"
	"reflect"
	"testing"

	"sec/internal/ast"
	"sec/internal/lexer"
	"sec/internal/parser"
)

// TestMapEntryIterationFacts verifies key/value positions, nominal types,
// unspecified order and per-component modes/discard without inferring an index
// or copying discarded noncopyable components. Facts are detached and stable
// across depths/reanalysis; ordinary sequential loops acquire no map contract.
// Rules: rules/collections/collections.md — §§14.2,14.6;
// rules/control-flow/flowcontrol_for.md — §21.
func TestMapEntryIterationFacts(t *testing.T) {
	for _, depth := range []AnalysisDepth{AnalysisInteractive, AnalysisStandard, AnalysisDeep} {
		path := "../../testdata/sema/map_iteration_valid.sec"
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
			if errs := a.Analyze(program); len(errs) != 0 {
				t.Fatal(depth, errs)
			}
			count := 0
			for _, statement := range program.Statements {
				fn, ok := statement.(*ast.FunctionDeclaration)
				if !ok {
					continue
				}
				for _, statement := range fn.Body.Statements {
					loop, ok := statement.(*ast.ForStatement)
					if !ok {
						continue
					}
					fact, known := a.ForIterationDependenciesOf(loop)
					if !known || !fact.BodyValid {
						t.Fatal(fn.Name.Value, fact, known)
					}
					if fn.Name.Value == "Sequential" {
						if fact.MapEntries != nil {
							t.Fatal("sequential loop got map entries", fact)
						}
						continue
					}
					count++
					entries := fact.MapEntries
					if entries == nil || entries.Order != MapIterationOrderUnspecified {
						t.Fatal(fn.Name.Value, entries)
					}
					source := dereferenceType(fact.SourceType)
					for i, component := range entries.Components {
						binding := loop.Bindings[i]
						if !sameConcreteType(component.Type, source.TypeArgs[i]) || component.Mode != binding.Mode || component.Discard != binding.Discard || component.Source != binding.Token {
							t.Fatal(fn.Name.Value, i, component)
						}
					}
					if fn.Name.Value == "Plain" && (entries.Components[0].Type.Name != "EntryKey" || entries.Components[1].Type.Name != "Payload") {
						t.Fatal(entries)
					}
					if fn.Name.Value == "Bounded" && (entries.Components[0].Mode != ast.ForBindingRef || entries.Components[1].Mode != ast.ForBindingRefMut) {
						t.Fatal(entries)
					}
					if fn.Name.Value == "Bounded" && (len(source.ConstArgs) != 1 || source.ConstArgs[0] != 8) {
						t.Fatal("lost bounded map identity", source)
					}
					if fn.Name.Value == "BoundedComponent" && (len(entries.Components[1].Type.ConstArgs) != 1 || entries.Components[1].Type.ConstArgs[0] != 4) {
						t.Fatal("lost bounded component identity", entries)
					}
					at, found := a.MapIterationAt(loop.Token.File, loop.Token.Line, loop.Token.Column)
					if !found || !reflect.DeepEqual(at.MapEntries, entries) {
						t.Fatal("source query disagrees with loop query", at, entries)
					}
					baseline := cloneMapIterationEntries(entries)
					if args := entries.Components[1].Type.ConstArgs; len(args) > 0 {
						args[0] = 99
					}
					entries.Order = "changed"
					entries.Components[0].Type.Name = "changed"
					if fields := entries.Components[1].Type.Fields; len(fields) > 0 {
						fields[0].Type.Name = "changed"
					}
					fresh, _ := a.ForIterationDependenciesOf(loop)
					if !reflect.DeepEqual(fresh.MapEntries, baseline) {
						t.Fatal("query changed compiler facts", fresh.MapEntries, baseline)
					}
				}
			}
			if count != 7 {
				t.Fatal("map plan count", count)
			}
		}
	}
}

// TestMapEntryIterationRejectsInvalidHeaders covers arity, copy and mutable
// authority without publishing valid header facts, and keeps body errors
// distinct from a valid entry contract.
// Rules: rules/collections/collections.md — §14.6;
// rules/control-flow/flowcontrol_for.md — §§8,21.
func TestMapEntryIterationRejectsInvalidHeaders(t *testing.T) {
	data, err := os.ReadFile("../../testdata/sema/map_iteration_invalid.sec")
	if err != nil {
		t.Fatal(err)
	}
	for _, depth := range []AnalysisDepth{AnalysisInteractive, AnalysisStandard, AnalysisDeep} {
		p := parser.New(lexer.New(string(data)))
		program := p.ParseProgram()
		if len(p.Errors()) != 0 {
			t.Fatal(p.Errors())
		}
		a := NewAnalyzerWithDepth(depth)
		if errs := a.Analyze(program); len(errs) != 8 {
			t.Fatal(depth, errs)
		}
		for _, statement := range program.Statements {
			fn, ok := statement.(*ast.FunctionDeclaration)
			if !ok {
				continue
			}
			var loop *ast.ForStatement
			for _, statement := range fn.Body.Statements {
				if candidate, ok := statement.(*ast.ForStatement); ok {
					loop = candidate
				}
			}
			if loop == nil {
				t.Fatal("missing loop", fn.Name.Value)
			}
			fact, known := a.ForIterationDependenciesOf(loop)
			if fn.Name.Value == "Structural" {
				if !known || fact.MapEntries == nil || fact.BodyValid {
					t.Fatal(fn.Name.Value, fact, known)
				}
			} else if known {
				t.Fatal("invalid header published dependency proof", fn.Name.Value, fact)
			}
		}
	}
}
