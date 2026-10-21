package sema

import (
	"os"
	"strings"
	"testing"

	"sec/internal/ast"
	"sec/internal/ast/walk"
	"sec/internal/lexer"
	"sec/internal/parser"
)

// Final-use advice is optional, binding-aware, and consumes selected function
// and initializer copies. Its manual edits must themselves pass ownership Sema.
// Rules: rules/analysis/pitfall_analysis.md — "Large by-value argument at its final use";
// rules/memory/copy_move.md — §§6.3,8.1–8.3.
func TestPitfallLastUseArgumentCopies(t *testing.T) {
	source, err := os.ReadFile("../../testdata/sema/pitfall_move/arguments.sec")
	if err != nil {
		t.Fatal(err)
	}
	for _, depth := range []AnalysisDepth{AnalysisInteractive, AnalysisStandard, AnalysisDeep} {
		t.Run(string(depth), func(t *testing.T) {
			p := parser.New(lexer.NewWithFile(string(source), "arguments.sec"))
			program := p.ParseProgram()
			if errs := p.Errors(); len(errs) != 0 {
				t.Fatal(errs)
			}
			a := NewAnalyzerWithDepth(depth)
			if errs := a.Analyze(program); len(errs) != 0 {
				t.Fatal(errs)
			}
			functions := map[int]string{}
			for _, stmt := range program.Statements {
				if fn, ok := stmt.(*ast.FunctionDeclaration); ok {
					_ = walk.Inspect(fn.Body, func(node any) error {
						if id, ok := node.(*ast.Identifier); ok {
							functions[id.Token.Line] = fn.Name.Value
						}
						return nil
					})
				}
			}
			counts := map[string]int{}
			for _, finding := range a.PitfallAnalysis().Findings() {
				if finding.Rule != PitfallLastUseArgumentCopy {
					continue
				}
				name := functions[finding.Subject.Source.Line]
				counts[name]++
				if finding.Confidence != PitfallConfidenceHigh || finding.Classification != PitfallLikelyMistake || len(finding.Actions) != 1 || finding.Actions[0].Kind != PitfallSuggestedEdit || finding.Actions[0].Replacement != "<-files" || len(finding.EvidenceFor) != 3 {
					t.Fatalf("bad advice: %+v", finding)
				}
			}
			for _, name := range []string{"Final", "PreviousRead", "Construct", "FallibleConstruct", "StringArray", "LaterArgument", "DistinctBindings"} {
				if counts[name] != 1 {
					t.Errorf("%s findings=%d want 1", name, counts[name])
				}
				delete(counts, name)
			}
			if len(counts) != 0 {
				t.Fatalf("unsafe/unhelpful advice: %v", counts)
			}
			// Applying a proposed local edit requires no parameter signature
			// change and marks only the argument source as transferred.
			moved := strings.Replace(string(source), "Accept(files)", "Accept(<-files)", 1)
			mp := parser.New(lexer.NewWithFile(moved, "moved.sec"))
			ma := NewAnalyzer()
			if errs := ma.Analyze(mp.ParseProgram()); len(errs) != 0 {
				t.Fatalf("recommended explicit move is invalid: %v", errs)
			}
		})
	}
}

// Budget exhaustion never creates a last-use proof from a truncated body.
// Rules: rules/analysis/pitfall_analysis.md — "Analysis states", "Large by-value argument at its final use".
func TestPitfallLastUseMoveBudget(t *testing.T) {
	source, err := os.ReadFile("../../testdata/sema/pitfall_move/arguments.sec")
	if err != nil {
		t.Fatal(err)
	}
	p := parser.New(lexer.NewWithFile(string(source), "arguments.sec"))
	a := NewAnalyzer()
	if err := a.SetPitfallBudget(0, 0); err != nil {
		t.Fatal(err)
	}
	if errs := a.Analyze(p.ParseProgram()); len(errs) != 0 {
		t.Fatal(errs)
	}
	for _, finding := range a.PitfallAnalysis().Findings() {
		if finding.Rule == PitfallLastUseArgumentCopy {
			t.Fatal("zero budget produced advice")
		}
	}
}

// Non-copyable ordinary arguments keep the mandatory ownership diagnostic;
// the newly clarified explicit-move alternative appears in its help, not as
// a duplicate optional pitfall warning.
// Rules: rules/memory/copy_move.md — §§8.1(1–5),28.1;
// rules/analysis/pitfall_analysis.md — "Large by-value argument at its final use".
func TestPitfallLastUseMoveDoesNotDuplicateOwnershipError(t *testing.T) {
	data, err := os.ReadFile("../../testdata/sema/pitfall_move/move_only_invalid.sec")
	if err != nil {
		t.Fatal(err)
	}
	p := parser.New(lexer.NewWithFile(string(data), "move_only_invalid.sec"))
	a := NewAnalyzer()
	errs := a.Analyze(p.ParseProgram())
	if len(errs) != 1 || !strings.Contains(errs[0].Message, "cannot copy reusable source files") || !strings.Contains(errs[0].Help, "<-files") {
		t.Fatal(errs)
	}
	for _, finding := range a.PitfallAnalysis().Findings() {
		if finding.Rule == PitfallLastUseArgumentCopy {
			t.Fatal("invalid copy received duplicate optional advice")
		}
	}
	moved := strings.Replace(string(data), "Accept(files)", "Accept(<-files)", 1)
	mp := parser.New(lexer.NewWithFile(moved, "move_only_moved.sec"))
	if errs := NewAnalyzer().Analyze(mp.ParseProgram()); len(errs) != 0 {
		t.Fatal("explicit move to ordinary value parameter failed", errs)
	}
}

// Warm reuse preserves complete findings; new binding uses invalidate the
// body rather than retaining stale final-use guidance from an earlier snapshot.
// Rules: rules/analysis/pitfall_analysis.md — "Large by-value argument at its final use", "Incremental analysis".
func TestPitfallLastUseMoveCache(t *testing.T) {
	data, err := os.ReadFile("../../testdata/sema/pitfall_move/arguments.sec")
	if err != nil {
		t.Fatal(err)
	}
	scope := PitfallCacheScope{Module: "main", CompilationPlan: "test-plan", CompilerModel: "last-use-v1"}
	var cache PitfallCache
	a := NewAnalyzerWithDepth(AnalysisStandard)
	checkPitfallCacheRun(t, a, &cache, scope, string(data))
	if usage := checkPitfallCacheRun(t, a, &cache, scope, string(data)); usage.Hits == 0 {
		t.Fatal("body advice did not reuse canonical cache", usage)
	}
	changed := strings.Replace(string(data), "Accept(files) }", "Accept(files)\n    Read(files) }", 1)
	if changed == string(data) {
		t.Fatal("last-use mutation did not change fixture")
	}
	checkPitfallCacheRun(t, a, &cache, scope, changed)
}
