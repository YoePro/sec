package sema

import (
	"os"
	"reflect"
	"sec/internal/ast"
	"sec/internal/lexer"
	"sec/internal/parser"
	"testing"
)

// TestPitfallForeignOrigin validates real foreign calls with trusted relations,
// disjoint origins, known reference aliases, equal/unknown provenance, absent
// metadata, deterministic snapshots, budget gating and contract removal.
// Rules: rules/analysis/pitfall_analysis.md — "FFI pointer/extent provenance mismatch",
// "FFI pointer/size origin mismatch", "Required FFI tests", "Determinism".
func TestPitfallForeignOrigin(t *testing.T) {
	for _, depth := range []AnalysisDepth{AnalysisInteractive, AnalysisStandard, AnalysisDeep} {
		t.Run(string(depth), func(t *testing.T) {
			path := "../../testdata/sema/pitfall_foreign_origin_valid.sec"
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
			if errs := a.Analyze(program); len(errs) != 0 {
				t.Fatal(errs)
			}
			if len(a.PitfallAnalysis().Findings()) != 0 {
				t.Fatal("inferred contract from names")
			}
			var store ForeignBufferExtentContractStore
			recorded := map[string]bool{}
			walkASTValue(reflect.ValueOf(program), func(node any) {
				call, ok := node.(*ast.CallExpression)
				if !ok || call == nil {
					return
				}
				resolved, ok := a.ResolvedCallTarget(call)
				if !ok || (resolved.Function.Name != "Ship" && resolved.Function.Name != "ShipBytes") || recorded[resolved.Function.Name] {
					return
				}
				err := store.Record(resolved.Function, []ForeignBufferExtentRelation{{PointerArgument: 0, ExtentArgument: 1, ExtentUnit: ForeignExtentElements, Source: lexer.Token{File: "trusted-contract", Line: 7, Column: 1}}})
				if err != nil {
					t.Fatal(err)
				}
				recorded[resolved.Function.Name] = true
			})
			if len(recorded) != 2 {
				t.Fatal("missing selected foreign declaration")
			}
			a.SetForeignBufferExtentContracts(&store)
			if errs := a.Analyze(program); len(errs) != 0 {
				t.Fatal(errs)
			}
			results := a.PitfallAnalysis().Findings()
			if len(results) != 4 {
				t.Fatal(results)
			}
			for _, result := range results {
				if result.Rule != PitfallForeignExtentOrigin || result.Classification != PitfallLikelyMistake || result.Confidence != PitfallConfidenceHigh || len(result.EvidenceFor) != 3 || result.EvidenceFor[0].Source.File != "trusted-contract" || result.Subject.Source.File != path || len(result.Actions) != 1 || result.Actions[0].Kind != PitfallSuggestedEdit || result.Actions[0].Replacement != "" {
					t.Fatal(result)
				}
			}
			before := a.PitfallAnalysis().ForeignExtentInputs()
			changed := a.PitfallAnalysis().ForeignExtentInputs()
			for _, fact := range changed {
				if fact.PointerOrigin != nil {
					fact.PointerOrigin.Root = "corrupted"
					if len(fact.PointerOrigin.Projections) > 0 {
						fact.PointerOrigin.Projections[0].Name = "corrupted"
					}
				}
			}
			if !reflect.DeepEqual(before, a.PitfallAnalysis().ForeignExtentInputs()) {
				t.Fatal("snapshot aliases origin")
			}
			walkASTValue(reflect.ValueOf(program), func(node any) {
				call, ok := node.(*ast.CallExpression)
				if !ok || call == nil {
					return
				}
				facts, _ := a.ResolvedForeignBufferExtentsOf(call)
				for _, fact := range facts {
					if fact.ExtentOrigin != nil {
						fact.ExtentOrigin.Root = "corrupted"
					}
				}
			})
			if errs := a.Analyze(program); len(errs) != 0 {
				t.Fatal(errs)
			}
			if !reflect.DeepEqual(results, a.PitfallAnalysis().Findings()) {
				t.Fatal("nondeterministic findings")
			}
			if err := a.SetPitfallBudget(0, 0); err != nil {
				t.Fatal(err)
			}
			if errs := a.Analyze(program); len(errs) != 0 {
				t.Fatal(errs)
			}
			if len(a.PitfallAnalysis().Findings()) != 0 || len(a.resolvedForeignBufferExtents) == 0 {
				t.Fatal("budget affected mandatory facts")
			}
			a.SetForeignBufferExtentContracts(nil)
			if errs := a.Analyze(program); len(errs) != 0 {
				t.Fatal(errs)
			}
			if len(a.resolvedForeignBufferExtents) != 0 {
				t.Fatal("stale contracts")
			}
		})
	}
}
