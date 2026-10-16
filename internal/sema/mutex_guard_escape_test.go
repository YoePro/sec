package sema

import (
	"os"
	"strings"
	"testing"

	"sec/internal/diagnostics"
	"sec/internal/lexer"
	"sec/internal/parser"
)

// TestMutexGuardExecutionEscape checks structural guards and canonical closure
// environments at concurrency/storage sinks without changing local transfers.
// Rules: rules/concurrency/mutex.md — §§28,39–40.
func TestMutexGuardExecutionEscape(t *testing.T) {
	for _, depth := range []AnalysisDepth{AnalysisInteractive, AnalysisStandard, AnalysisDeep} {
		a := NewAnalyzerWithDepth(depth)
		for _, test := range []struct {
			file   string
			errors int
		}{
			{"mutex_guard_escape_invalid.sec", 16},
			{"mutex_guard_escape_valid.sec", 0},
		} {
			data, err := os.ReadFile("../../testdata/sema/" + test.file)
			if err != nil {
				t.Fatal(err)
			}
			p := parser.New(lexer.NewWithFile(string(data), test.file))
			program := p.ParseProgram()
			if len(p.Errors()) != 0 {
				t.Fatal(p.Errors())
			}
			errors := a.Analyze(program)
			if len(errors) != test.errors {
				t.Fatalf("%s %v: %v", test.file, depth, errors)
			}
			for _, diagnostic := range errors {
				if !strings.Contains(diagnostic.Message, "bound to the current execution entity and cannot cross") || diagnostic.PreviousLine == 0 || diagnostic.RelatedLabel != "execution-bound guard origin" {
					t.Fatal(diagnostic)
				}
			}
			transfers := 0
			for _, fact := range a.EscapeAnalysis().Facts() {
				if fact.Mode != EscapeModeConcurrencyTransfer {
					continue
				}
				transfers++
				if fact.Unknown || fact.Sink.Line == 0 || len(fact.Sources) != 1 {
					t.Fatal(fact)
				}
				fact.Sources[0].Name = "corrupted"
			}
			if transfers != test.errors {
				t.Fatalf("%s: %d transfer facts, want %d", test.file, transfers, test.errors)
			}
			for _, fact := range a.EscapeAnalysis().Facts() {
				for _, source := range fact.Sources {
					if source.Name == "corrupted" {
						t.Fatal("escape snapshot aliases producer facts")
					}
				}
			}
		}
	}
}

// TestMutexGuardOpaqueEscapeIsUnproven keeps unknown value/capture graphs
// conservative without inventing an actual guard or an Invalid source proof.
// Rules: rules/memory/transferability.md — §§17,38–40.
func TestMutexGuardOpaqueEscapeIsUnproven(t *testing.T) {
	data, err := os.ReadFile("../../testdata/sema/mutex_guard_escape_unproven_invalid.sec")
	if err != nil {
		t.Fatal(err)
	}
	a, errors := analyzeSourceWithAnalyzerRaw(t, string(data))
	if len(errors) != 4 {
		t.Fatal(errors)
	}
	for _, diagnostic := range errors {
		if diagnostic.ID != diagnostics.RequiredProofUnavailable || diagnostic.ProofState != diagnostics.ProofUnproven || !strings.Contains(diagnostic.Message, "cannot prove value dependencies") {
			t.Fatal(diagnostic)
		}
	}
	unknown := 0
	for _, fact := range a.EscapeAnalysis().Facts() {
		if fact.Mode == EscapeModeConcurrencyTransfer && fact.Unknown {
			unknown++
		}
	}
	if unknown != 4 {
		t.Fatal(unknown)
	}
}
