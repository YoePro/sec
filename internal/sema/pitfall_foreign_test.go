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

// foreignUnitFixture binds trusted metadata to the actual resolved targets,
// keeping units and source syntax separate and testing native layout per plan.
// Rules: rules/analysis/pitfall_analysis.md — "FFI element count versus byte count".
func foreignUnitFixture(t *testing.T, depth AnalysisDepth, width uint16) (*Analyzer, *ast.Program) {
	t.Helper()
	file := "../../testdata/sema/pitfall_foreign_units_valid.sec"
	data, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	p := parser.New(lexer.NewWithFile(string(data), file))
	program := p.ParseProgram()
	if len(p.Errors()) != 0 {
		t.Fatal(p.Errors())
	}
	a := NewAnalyzerWithScalarPlanAndDepth(layout.ResolvedScalarPlan{PointerWidthBits: width}, depth)
	if errs := a.Analyze(program); len(errs) != 0 {
		t.Fatal(errs)
	}
	if len(a.PitfallAnalysis().Findings()) != 0 {
		t.Fatal("inferred units from function names")
	}
	var contracts ForeignBufferExtentContractStore
	seen := map[foreignExtentTarget]bool{}
	walkASTValue(reflect.ValueOf(program), func(node any) {
		call, ok := node.(*ast.CallExpression)
		if !ok || call == nil {
			return
		}
		target, ok := a.ResolvedCallTarget(call)
		if !ok || target.Kind != ResolvedForeignCall || seen[foreignExtentTargetOf(target.Function)] {
			return
		}
		unit := ForeignExtentBytes
		if target.Function.Name == "Elements" {
			unit = ForeignExtentElements
		}
		if err := contracts.Record(target.Function, []ForeignBufferExtentRelation{{PointerArgument: 0, ExtentArgument: 1, ExtentUnit: unit, Source: lexer.Token{File: "trusted-unit-contract", Line: 5, Column: 1}}}); err != nil {
			t.Fatal(err)
		}
		seen[foreignExtentTargetOf(target.Function)] = true
	})
	a.SetForeignBufferExtentContracts(&contracts)
	if errs := a.Analyze(program); len(errs) != 0 {
		t.Fatal(errs)
	}
	return a, program
}

// TestPitfallForeignUnits checks exact plan-dependent strides, both mismatch
// directions, correct units, byte/zero equivalence, opaque caller references,
// unknown layout/quantity/provenance, budget gating and metadata reset.
// Rules: rules/analysis/pitfall_analysis.md — "Required FFI tests", "Fix safety";
// rules/memory/layout.md — §18(1–4a).
func TestPitfallForeignUnits(t *testing.T) {
	for _, width := range []uint16{32, 64} {
		for _, depth := range []AnalysisDepth{AnalysisInteractive, AnalysisStandard, AnalysisDeep} {
			a, program := foreignUnitFixture(t, depth, width)
			found := 0
			for _, finding := range a.PitfallAnalysis().Findings() {
				if finding.Rule != PitfallForeignExtentUnit {
					continue
				}
				found++
				if finding.Classification != PitfallLikelyMistake || finding.Confidence != PitfallConfidenceHigh || len(finding.EvidenceFor) != 3 || finding.EvidenceFor[0].Source.File != "trusted-unit-contract" || len(finding.Actions) != 1 || finding.Actions[0].Kind != PitfallSuggestedEdit || finding.Actions[0].Replacement != "" {
					t.Fatal(finding)
				}
			}
			if found != 5 {
				t.Fatal(width, depth, found, a.PitfallAnalysis().Findings())
			}
			native := 0
			walkASTValue(reflect.ValueOf(program), func(node any) {
				call, ok := node.(*ast.CallExpression)
				if !ok || call == nil {
					return
				}
				target, ok := a.ResolvedCallTarget(call)
				if !ok || (target.Function.Name != "Native" && target.Function.Name != "NativeFloat") {
					return
				}
				facts, known := a.ResolvedForeignBufferExtentsOf(call)
				if !known || len(facts) != 1 || facts[0].ElementStorageBytes != int64(width)/8 {
					t.Fatal(width, facts)
				}
				native++
			})
			if native != 2 {
				t.Fatal("native layout cases not checked", native)
			}
			before := a.PitfallAnalysis().Results()
			inputs := a.PitfallAnalysis().ForeignExtentInputs()
			for i := range inputs {
				inputs[i].ElementStorageBytes = 999
				inputs[i].SuppliedExtentUnit = "invalid"
			}
			if errs := a.Analyze(program); len(errs) != 0 {
				t.Fatal(errs)
			}
			if !reflect.DeepEqual(before, a.PitfallAnalysis().Results()) {
				t.Fatal("unstable or mutable quantity facts")
			}
			if err := a.SetPitfallBudget(0, 0); err != nil {
				t.Fatal(err)
			}
			if errs := a.Analyze(program); len(errs) != 0 {
				t.Fatal(errs)
			}
			if len(a.PitfallAnalysis().Findings()) != 0 || len(a.resolvedForeignBufferExtents) == 0 {
				t.Fatal("budget altered mandatory contract facts")
			}
			a.SetForeignBufferExtentContracts(nil)
			if errs := a.Analyze(program); len(errs) != 0 {
				t.Fatal(errs)
			}
			if len(a.resolvedForeignBufferExtents) != 0 {
				t.Fatal("stale quantities")
			}
		}
	}
}

// TestForeignExactElementLayout rejects estimated or unspecified representation
// instead of reusing the parameter advice's approximate size model.
// Rules: rules/memory/layout.md — §§3,18; rules/analysis/pitfall_analysis.md — "FFI element count versus byte count".
func TestForeignExactElementLayout(t *testing.T) {
	a := NewAnalyzerWithScalarPlan(layout.ResolvedScalarPlan{PointerWidthBits: 32})
	for _, typ := range []Type{{Name: "int32", Kind: StructType}, {Name: "Opaque", Kind: IntType, Named: true}, {Name: "Odd", Kind: UintType, BitWidth: 5, Named: true}, {Name: "Unknown", Kind: FloatType, FloatBits: 80}} {
		if bytes, known := a.resolvedForeignElementBytes(typ); known {
			t.Fatal("guessed layout", typ, bytes)
		}
	}
	if bytes, known := a.resolvedForeignElementBytes(Type{Name: "foreign-long-double", Kind: FloatType, FloatBits: 80, BitWidth: 128, Intrinsic: true}); !known || bytes != 16 {
		t.Fatal(bytes, known)
	}
}
