package sema

import (
	"os"
	"reflect"
	"strings"
	"testing"

	"sec/internal/ast"
	"sec/internal/diagnostics"
	"sec/internal/lexer"
	"sec/internal/parser"
)

// TestPitfallStateCorrelation exercises canonical Option/Result checks,
// admitted variant tests, path-local state, suppression and normative ownership
// on real sources. Ordinary optional inspection/handling is never mistaken for
// an operation that requires a proved state.
// Rules: rules/analysis/pitfall_analysis.md — "Option, Result, and state-correlation pitfalls",
// "Suppressing evidence", "Diagnostic ownership and coalescing";
// rules/errors/errorhandling.md — §§6.1–6.2.
func TestPitfallStateCorrelation(t *testing.T) {
	for _, depth := range []AnalysisDepth{AnalysisInteractive, AnalysisStandard, AnalysisDeep} {
		for _, valid := range []bool{false, true} {
			name := "pitfall_state_correlation_invalid.sec"
			if valid {
				name = "pitfall_state_correlation_valid.sec"
			}
			file := "../../testdata/sema/" + name
			data, err := os.ReadFile(file)
			if err != nil {
				t.Fatal(err)
			}
			p := parser.New(lexer.NewWithFile(string(data), file))
			program := p.ParseProgram()
			if len(p.Errors()) != 0 {
				t.Fatal(p.Errors())
			}
			a := NewAnalyzerWithDepth(depth)
			errors := a.Analyze(program)
			if valid && len(errors) != 0 {
				t.Fatal(errors)
			}
			if !valid && len(errors) != 18 {
				t.Fatalf("errors = %v (%d), want 18", errors, len(errors))
			}
			want := map[string]PitfallAnalysisState{}
			if valid {
				want["IndependentCheck"], want["ConstructedState"], want["IndependentConjunction"] = PitfallStateSuppressed, PitfallStateSuppressed, PitfallStateSuppressed
			} else {
				for _, fn := range []string{"WrongBorrowed", "WrongSome", "WrongElse", "WrongExit", "WrongMatch", "WrongVariant", "WrongNegatedVariant", "WrongErr", "Conjunction", "WrongWhile", "WrongRepeated"} {
					want[fn] = PitfallStateFinding
				}
			}
			seen := map[string]bool{}
			for _, statement := range program.Statements {
				fn, ok := statement.(*ast.FunctionDeclaration)
				if !ok {
					continue
				}
				walkASTValue(reflect.ValueOf(fn.Body), func(node any) {
					expression, ok := node.(ast.Expression)
					if !ok {
						return
					}
					fact, found := a.ResolvedStateRequirementOf(expression)
					if !found {
						return
					}
					if fact.Binding.ID == 0 || fact.Variant == "" {
						t.Fatal("unresolved canonical requirement", fact)
					}
					for _, finding := range a.PitfallAnalysis().Results() {
						if finding.Rule != PitfallWrongStateSubject || sourceTokenLocation(finding.Subject.Source) != sourceTokenLocation(expressionToken(expression)) {
							continue
						}
						state, expected := want[fn.Name.Value]
						if !expected || state != finding.State || seen[fn.Name.Value] {
							t.Fatal(fn.Name.Value, finding)
						}
						seen[fn.Name.Value] = true
						if finding.Family != PitfallOptionResultFlow || finding.Classification != PitfallLikelyMistake || finding.Confidence != PitfallConfidenceHigh || len(finding.EvidenceFor) != 2 || finding.EvidenceFor[0].Source.File != file || (finding.EvidenceFor[0].Source.Line > finding.Subject.Source.Line || finding.EvidenceFor[0].Source.Line == finding.Subject.Source.Line && finding.EvidenceFor[0].Source.Column >= finding.Subject.Source.Column) {
							t.Fatal(finding)
						}
						if state == PitfallStateFinding {
							if finding.DiagnosticID != diagnostics.NonDiscardableValue || len(finding.Actions) != 1 || finding.Actions[0].Kind != PitfallSuggestedEdit {
								t.Fatal(finding)
							}
						} else if finding.DiagnosticID != "" || finding.Suppression == nil || !fact.Proven {
							t.Fatal(finding, fact)
						}
					}
					if len(fact.Checks) > 0 {
						fact.Checks[0].Binding.Name = "corrupted"
						fact.Checks[0].Binding.Type.TypeArgs[0].Name = "corrupted"
						fresh, _ := a.ResolvedStateRequirementOf(expression)
						if fresh.Checks[0].Binding.Name == "corrupted" || fresh.Checks[0].Binding.Type.TypeArgs[0].Name == "corrupted" {
							t.Fatal("mutable state requirement snapshot")
						}
					}
				})
			}
			if len(seen) != len(want) {
				t.Fatal("missing state correlation results", seen, want, a.PitfallAnalysis().Results())
			}
			if !valid {
				for _, e := range errors {
					if e.ID != diagnostics.NonDiscardableValue {
						t.Fatal(e)
					}
				}
				if err := a.SetPitfallBudget(0, 0); err != nil {
					t.Fatal(err)
				}
				errors = a.Analyze(program)
				if len(errors) != 18 {
					t.Fatal("advisory budget disabled state errors", errors)
				}
				for _, e := range errors {
					if strings.Contains(e.Help, string(PitfallWrongStateSubject)) {
						t.Fatal("budget-disabled explanation survived", e)
					}
				}
			}
			empty := parser.New(lexer.New("module empty\nfn Empty() void {}\n")).ParseProgram()
			if errors := a.Analyze(empty); len(errors) != 0 || len(a.resolvedStateRequirements) != 0 {
				t.Fatal("stale state requirements survived reset", errors)
			}
		}
	}
	var nilAnalyzer *Analyzer
	if _, found := nilAnalyzer.ResolvedStateRequirementOf(nil); found {
		t.Fatal("nil analyzer returned state facts")
	}
}

// TestPitfallStateCorrelationRejectsInvalidProofs ensures a parsed state-test
// shape cannot replace a validated canonical state proof, even for the same
// subject. Rejected qualifiers cannot silence an independent obligation.
// Rules: rules/analysis/pitfall_analysis.md — "Semantic recognition, not syntax matching";
// rules/declarations/unions.md — §§8.1–8.3; rules/errors/errorhandling.md — §6.1.
func TestPitfallStateCorrelationRejectsInvalidProofs(t *testing.T) {
	file := "../../testdata/sema/pitfall_state_proof_invalid.sec"
	data, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	p := parser.New(lexer.NewWithFile(string(data), file))
	program := p.ParseProgram()
	if len(p.Errors()) != 0 {
		t.Fatal(p.Errors())
	}
	a := NewAnalyzer()
	errors := a.Analyze(program)
	if len(errors) != 4 {
		t.Fatal(errors)
	}
	stateErrors := 0
	for _, e := range errors {
		if e.ID == diagnostics.NonDiscardableValue {
			stateErrors++
		}
	}
	if stateErrors != 2 {
		t.Fatal("invalid tests established a state proof", errors)
	}
	for _, f := range a.PitfallAnalysis().Results() {
		if f.Rule == PitfallWrongStateSubject {
			t.Fatal("invalid test supplied advisory evidence", f)
		}
	}
	for _, fact := range a.resolvedStateRequirements {
		if fact.Proven || len(fact.Checks) != 0 {
			t.Fatal("invalid test published a proof", fact)
		}
	}
}
