package sema

import (
	"os"
	"reflect"
	"strings"
	"testing"

	"sec/internal/lexer"
	"sec/internal/parser"
)

// Rules: rules/analysis/pitfall_analysis.md — "Canonical idiom guidance",
// "Fix safety", "Required control-flow tests", "Analysis modes".
func TestPitfallCanonicalIdiomSuggestions(t *testing.T) {
	const file = "../../testdata/sema/pitfall_idioms_valid.sec"
	data, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	analyze := func(source string, depth AnalysisDepth) *Analyzer {
		t.Helper()
		parsed := parser.New(lexer.NewWithFile(source, file))
		program := parsed.ParseProgram()
		if errs := parsed.Errors(); len(errs) != 0 {
			t.Fatal(errs)
		}
		analyzer := NewAnalyzerWithDepth(depth)
		if errs := analyzer.Analyze(program); len(errs) != 0 {
			t.Fatal("guidance changed source validity", errs)
		}
		return analyzer
	}
	for _, depth := range []AnalysisDepth{AnalysisInteractive, AnalysisStandard, AnalysisDeep} {
		analyzer := analyze(string(data), depth)
		findings := analyzer.PitfallAnalysis().Findings()
		membership, traversal, proven, edits := 0, 0, 0, 0
		for _, finding := range findings {
			if finding.State == PitfallStateSuppressed {
				for _, action := range finding.Actions {
					if action.Idiom != "" {
						t.Fatal("suppressed intent retained preference", finding)
					}
				}
				continue
			}
			if finding.Classification == PitfallProvenInvalid {
				t.Fatal("optional canonical idiom became an error", finding)
			}
			for _, action := range finding.Actions {
				if action.Idiom == "" {
					continue
				}
				if action.Replacement == "" {
					t.Fatal("canonical idiom has no replacement", action)
				}
				var original string
				switch action.Idiom {
				case PitfallIdiomRangeMembership:
					membership++
					if action.Replacement != "value in 1..<9" {
						t.Fatal(action)
					}
					original = "value >= 1 && value < 9"
				case PitfallIdiomHalfOpenTraversal:
					traversal++
					switch finding.Rule {
					case PitfallFragileInclusiveLength:
						if strings.Contains(action.Replacement, "uint(0)") {
							original = "uint(0)..values.Len - 1"
						} else {
							original = "uint(1)..values.Len - 1"
						}
					case PitfallOmittedLastElement:
						original = "uint(0)..<values.Len - 1"
					default:
						t.Fatal("unexpected traversal producer", finding)
					}
				default:
					t.Fatal("unknown idiom", action)
				}
				if action.Kind == PitfallProvenFix {
					proven++
					if !action.Safety.verifiedFor(finding.Rule, action.Replacement) {
						t.Fatal("preference bypassed fix safety", action)
					}
				} else {
					edits++
				}
				rewritten := strings.Replace(string(data), original, action.Replacement, 1)
				if rewritten == string(data) {
					t.Fatal("source rewrite was not exercised", original, action)
				}
				analyze(rewritten, depth)
			}
		}
		wantMembership := 0
		if depth == AnalysisDeep {
			wantMembership = 1
		}
		wantTraversal, wantEdits := 3, 2
		if depth == AnalysisInteractive {
			wantTraversal, wantEdits = 2, 1
		}
		if membership != wantMembership || traversal != wantTraversal || proven != 1+wantMembership || edits != wantEdits {
			t.Fatal("incomplete idiom suggestions", depth, membership, traversal, proven, edits)
		}
		if !reflect.DeepEqual(findings, analyze(string(data), depth).PitfallAnalysis().Findings()) {
			t.Fatal("nondeterministic guidance")
		}
		for i := range findings {
			if len(findings[i].Actions) > 0 {
				findings[i].Actions[0].Idiom = "mutated"
			}
		}
		for _, finding := range analyzer.PitfallAnalysis().Findings() {
			for _, action := range finding.Actions {
				if action.Idiom == "mutated" {
					t.Fatal("mutable idiom snapshot")
				}
			}
		}
	}
}

// Rules: rules/analysis/pitfall_analysis.md — "Canonical idiom guidance", "Fix safety".
func TestPitfallCanonicalIdiomPreferenceDoesNotAuthorizeFix(t *testing.T) {
	finding := PitfallFinding{Rule: PitfallFragileInclusiveLength, Actions: []PitfallSuggestedAction{
		{Kind: PitfallSuggestedEdit, Title: "explanation"},
		{Kind: PitfallProvenFix, Replacement: "uint(0)..<values.Len"},
	}}
	requirePitfallFixSafety(&finding)
	preferPitfallCanonicalIdioms(&finding)
	if finding.Actions[0].Idiom != PitfallIdiomHalfOpenTraversal || finding.Actions[0].Kind != PitfallSuggestedEdit || finding.Actions[1].Idiom != "" {
		t.Fatal("preference lost safety or stable ordering", finding)
	}
	finding.State = PitfallStateSuppressed
	preferPitfallCanonicalIdioms(&finding)
	for _, action := range finding.Actions {
		if action.Idiom != "" {
			t.Fatal("suppressed preference", action)
		}
	}
}
