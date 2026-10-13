package sema

import (
	"os"
	"reflect"
	"strings"
	"testing"

	"sec/internal/diagnostics"
	"sec/internal/lexer"
	"sec/internal/parser"
)

// TestArenaMultiOriginValidity retains every continuing Arena dependency at
// if/switch/match/loop joins and checks both invalidation and later stale use.
// Rules: rules/memory/lifetime_analysis.md — §11.1–11.3;
// rules/memory/arena.md — §§36,38,44.
func TestArenaMultiOriginValidity(t *testing.T) {
	invalidPath := "../../testdata/sema/arena_multi_origin_invalid.sec"
	invalid, err := os.ReadFile(invalidPath)
	if err != nil {
		t.Fatal(err)
	}
	valid, err := os.ReadFile("../../testdata/sema/arena_multi_origin_valid.sec")
	if err != nil {
		t.Fatal(err)
	}
	var baseline []Error
	for _, depth := range []AnalysisDepth{AnalysisInteractive, AnalysisStandard, AnalysisDeep} {
		a := NewAnalyzerWithDepth(depth)
		for attempt := 0; attempt < 3; attempt++ {
			p := parser.New(lexer.NewWithFile(string(invalid), invalidPath))
			program := p.ParseProgram()
			if len(p.Errors()) != 0 {
				t.Fatal(p.Errors())
			}
			errors := a.Analyze(program)
			if len(errors) != 9 {
				t.Fatal(errors)
			}
			for i, line := range []int{16, 31, 45, 59, 77, 92, 107, 122, 142} {
				e := errors[i]
				if e.Line != line || e.File != invalidPath || e.PreviousFile != invalidPath || e.PreviousLine == 0 {
					t.Fatal(e)
				}
				if i == 6 || i == 7 {
					if e.ID != diagnostics.ArenaResetLiveDependency || e.ProofState != diagnostics.ProofInvalid {
						t.Fatal(e)
					}
				} else if !strings.Contains(e.Message, "after arena") {
					t.Fatal(e)
				}
			}
			if baseline == nil {
				baseline = errors
			} else if !reflect.DeepEqual(baseline, errors) {
				t.Fatal("unstable or depth-dependent required validity", errors)
			}
			p = parser.New(lexer.New(string(valid)))
			program = p.ParseProgram()
			if len(p.Errors()) != 0 {
				t.Fatal(p.Errors())
			}
			if errors := a.Analyze(program); len(errors) != 0 {
				t.Fatal("terminated/rebound/dead dependencies or analyzer reuse", errors)
			}
		}
	}
}

// TestReferenceOriginAlternatives checks the origin-set algebra and immutable
// propagation independently of branch traversal and exported snapshot mutation.
// Rules: rules/memory/lifetime_analysis.md — §11.2(1–2);
// rules/compiler/compiler_analysis.md — Immutable analysis results.
func TestReferenceOriginAlternatives(t *testing.T) {
	first := Type{Kind: ReferenceType, ReferenceOriginName: "domain-a", ReferenceOriginStorage: StorageOriginArena, ReferenceOriginGeneration: 1}
	second := Type{Kind: ReferenceType, ReferenceOriginName: "domain-b", ReferenceOriginStorage: StorageOriginArena, ReferenceOriginGeneration: 1}
	joined := joinedReferenceOrigin(first, second, first)
	if len(referenceOriginAlternatives(joined)) != 2 {
		t.Fatal(joined)
	}
	reversed := joinedReferenceOrigin(second, first)
	if !reflect.DeepEqual(referenceOriginAlternatives(joined), referenceOriginAlternatives(reversed)) {
		t.Fatal("traversal-dependent origin set")
	}
	if !sameReferenceOriginType(joined, joinedReferenceOrigin(joined, first, second)) {
		t.Fatal("join is not idempotent")
	}
	copy := referenceTypeWithOrigin(Type{Kind: SliceType}, joined)
	copy.ReferenceOriginAlternatives[0].Name = "changed"
	snapshot := semanticSnapshotType(joined)
	snapshot.ReferenceOriginAlternatives[0].Generation++
	if joined.ReferenceOriginAlternatives[0].Name != "domain-a" || joined.ReferenceOriginAlternatives[0].Generation != 1 {
		t.Fatal("origin-set storage escaped")
	}
	a := NewAnalyzer()
	a.symbols = map[string]Symbol{"holder": {Type: joined}}
	facts := a.referenceFactsForPlace(Place{Root: "holder"}, false, lexer.Token{Line: 1, Column: 1})
	if facts.Validity == ReferenceValidityProven || facts.Validity == ReferenceValidityDynamicEpoch {
		t.Fatal("single-domain ABI invented for joined origins", facts)
	}
}
