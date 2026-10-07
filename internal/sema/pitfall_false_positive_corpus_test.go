package sema

import (
	"os"
	"reflect"
	"testing"

	"sec/internal/ast"
	"sec/internal/lexer"
	"sec/internal/parser"
)

// TestPitfallFalsePositiveCorpus protects intentional idioms across every
// depth and pairs supported families with real-source positive controls.
// Absent FFI metadata is uncertainty, never a guess from argument names.
// Rules: rules/analysis/pitfall_analysis.md — "False-positive regression corpus",
// "Required bounds and range tests", "Required collection-relation tests",
// "Evidence against a finding", "FFI pitfall analysis", and "Analysis states".
func TestPitfallFalsePositiveCorpus(t *testing.T) {
	source, err := os.ReadFile("../../testdata/sema/pitfall_false_positive_corpus_invalid.sec")
	if err != nil {
		t.Fatal(err)
	}
	safe := map[string]PitfallRuleID{
		"NeighborSuccessor":   PitfallOmittedLastElement,
		"NeighborPredecessor": PitfallSkippedFirstElement,
		"SentinelFirst":       PitfallSkippedFirstElement,
		"ManualPrefix":        PitfallSkippedFirstElement,
		"SeparateFinal":       PitfallOmittedLastElement,
		"ParallelEqual":       PitfallWrongBoundSource,
		"ParallelBoth":        "",
		"RingBuffer":          "",
		"CapacityBookkeeping": "",
		"CapacityLiveStorage": PitfallCapacityAsLength,
		"ProtocolRange":       "",
		"ProtocolPositions":   "",
		"FixedExtent":         "",
		"ByteBufferFFI":       "",
		"ByteBufferRenamed":   "",
		"CapacityStorageFFI":  "",
	}
	controls := map[string]PitfallRuleID{
		"MistakeUpper":             PitfallUpperNeighborIndex,
		"MistakeLower":             PitfallLowerNeighborIndex,
		"MistakeSentinelName":      PitfallSkippedFirstElement,
		"MistakeConditionalPrefix": PitfallSkippedFirstElement,
		"MistakeFinal":             PitfallOmittedLastElement,
		"MistakeParallel":          PitfallWrongBoundSource,
		"MistakeCapacity":          PitfallCapacityAsLength,
		"MistakeEndpoint":          PitfallInclusiveLengthIndex,
		"MistakeChangedTraversal":  PitfallFinalElementNeedsNonEmpty,
		"MistakeOpaqueTraversal":   PitfallFinalElementNeedsNonEmpty,
		"MistakeOtherProof":        PitfallFinalElementNeedsNonEmpty,
	}
	for _, depth := range []AnalysisDepth{AnalysisInteractive, AnalysisStandard, AnalysisDeep} {
		t.Run(string(depth), func(t *testing.T) {
			parsed := parser.New(lexer.NewWithFile(string(source), "corpus.sec"))
			program := parsed.ParseProgram()
			if errs := parsed.Errors(); len(errs) != 0 {
				t.Fatal(errs)
			}
			analyzer := NewAnalyzerWithDepth(depth)
			assertPitfallBoundsErrorCount(t, analyzer.Analyze(program), 1)
			analysis := analyzer.PitfallAnalysis()
			if analysis.Coverage().SkippedUnits != 0 {
				t.Fatal("corpus not fully inspected", analysis.Coverage())
			}
			byFunction := map[string][]PitfallFinding{}
			declarations := []*ast.FunctionDeclaration{}
			declared := map[string]bool{}
			for _, statement := range program.Statements {
				if function, ok := statement.(*ast.FunctionDeclaration); ok {
					declarations = append(declarations, function)
					declared[function.Name.Value] = true
				}
			}
			for name := range safe {
				if !declared[name] {
					t.Fatal("missing corpus case", name)
				}
			}
			for name := range controls {
				if !declared[name] {
					t.Fatal("missing control case", name)
				}
			}
			for _, finding := range analysis.Results() {
				owner := ""
				for _, declaration := range declarations {
					if declaration.Token.Line > finding.Subject.Source.Line {
						break
					}
					owner = declaration.Name.Value
				}
				if owner == "" {
					t.Fatal("unowned finding", finding)
				}
				byFunction[owner] = append(byFunction[owner], finding)
			}
			for name, suppressedRule := range safe {
				foundSuppression := false
				foundNonEmptyProof := false
				for _, result := range byFunction[name] {
					if result.State == PitfallStateFinding {
						t.Errorf("false positive in %s: %+v", name, result)
					}
					if result.Rule == suppressedRule && result.State == PitfallStateSuppressed {
						foundSuppression = true
						if result.Suppression == nil || len(result.EvidenceAgainst) == 0 || len(result.Suppression.Evidence) == 0 || result.Suppression.Reason == "" {
							t.Errorf("%s lost suppressing proof: %+v", name, result)
						}
					}
					if result.Rule == PitfallFinalElementNeedsNonEmpty && result.State == PitfallStateSuppressed && result.Suppression != nil && len(result.EvidenceAgainst) > 0 {
						foundNonEmptyProof = true
					}
				}
				if name == "SeparateFinal" && !foundNonEmptyProof {
					t.Error("read-only traversal lost the dominating nonempty proof")
				}
				if depth == AnalysisDeep && suppressedRule != "" && !foundSuppression {
					t.Errorf("%s did not exercise %s suppression", name, suppressedRule)
				}
			}
			for name, rule := range controls {
				if depth == AnalysisInteractive && (rule == PitfallOmittedLastElement || rule == PitfallSkippedFirstElement) {
					continue
				}
				matches := 0
				for _, result := range byFunction[name] {
					if result.Rule == rule && result.State == PitfallStateFinding {
						matches++
						if len(result.EvidenceFor) == 0 || result.Suppression != nil {
							t.Errorf("positive control lacks independent evidence: %+v", result)
						}
					}
				}
				if matches != 1 {
					t.Errorf("%s has %d %s findings; detector must remain active", name, matches, rule)
				}
			}
			for _, evaluation := range analysis.Evaluations() {
				if evaluation.Incomplete {
					t.Errorf("full corpus claims incomplete coverage: %+v", evaluation)
				}
			}
			// Reuse must not retain a prior suppression or publication order.
			before := analysis.Results()
			assertPitfallBoundsErrorCount(t, analyzer.Analyze(program), 1)
			if !reflect.DeepEqual(before, analyzer.PitfallAnalysis().Results()) {
				t.Fatal("corpus results changed on reanalysis")
			}
		})
	}
}
