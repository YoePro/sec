package main

import (
	"bytes"
	"errors"
	"fmt"
	"math/big"
	"os"
	"strings"
	"testing"

	"sec/internal/diagnostics"
	"sec/internal/ir/semantic"
	"sec/internal/lexer"
	"sec/internal/parser"
	"sec/internal/sema"
)

// TestPipelineProofDiagnosticTransport exercises the actual Sema -> Semantic
// IR failure -> command reporter boundary, not a text-derived classification.
// Rules: rules/compiler/compiler_analysis.md — §§7, 58(3);
// rules/compiler/compiler_pipeline.md — §§32(2–4), 33(3).
func TestPipelineProofDiagnosticTransport(t *testing.T) {
	for _, test := range []struct {
		name, file       string
		analyze, invalid bool
		state            diagnostics.ProofState
		id               string
	}{
		{"missing snapshot", "../../testdata/ir/iterator_readiness_valid.sec", false, false, diagnostics.ProofUnproven, diagnostics.RequiredProofUnavailable},
		{"invalid source", "../../testdata/sema/iterator_lowering_readiness_invalid.sec", true, true, diagnostics.ProofInvalid, diagnostics.RequiredAnalysisInvalid},
		{"unsupported lowering", "../../testdata/ir/iterator_readiness_valid.sec", true, false, "", ""},
	} {
		t.Run(test.name, func(t *testing.T) {
			data, err := os.ReadFile(test.file)
			if err != nil {
				t.Fatal(err)
			}
			p := parser.New(lexer.NewWithFile(string(data), test.file))
			program := p.ParseProgram()
			if len(p.Errors()) != 0 {
				t.Fatal(p.Errors())
			}
			analyzer := sema.NewAnalyzer()
			if test.analyze {
				errs := analyzer.Analyze(program)
				if (len(errs) > 0) != test.invalid {
					t.Fatal(errs)
				}
			}
			module, failure := semantic.Build(program, analyzer, semantic.BuildOptions{RequestedModule: "main"})
			if module != nil || failure == nil {
				t.Fatal(module, failure)
			}
			failure = fmt.Errorf("compiling requested module: %w", failure)
			var human, json bytes.Buffer
			(&diagnosticReporter{format: diagnosticFormatHuman, output: &human}).pipelineError("semantic IR", failure)
			reporter := &diagnosticReporter{format: diagnosticFormatJSON, output: &json}
			reporter.pipelineError("semantic IR", failure)
			reporter.finish()
			doc := decodeOccurrenceDocument(t, json.String())
			if len(doc.Occurrences) == 0 || doc.Summary.Errors != len(doc.Occurrences) {
				t.Fatal(doc)
			}
			for _, value := range doc.Occurrences {
				if value.Severity != diagnostics.SeverityError || len(value.Notes) != 1 || !strings.Contains(value.Notes[0].Text, "compiling requested module") {
					t.Fatal(value)
				}
				if test.state == "" {
					if value.Arguments["proof_state"] != "" || value.Arguments["category"] != "unsupported-lowering" || strings.Contains(human.String(), "Invalid:") || strings.Contains(human.String(), "Unproven:") {
						t.Fatal(value, human.String())
					}
				} else {
					if value.ID == nil || *value.ID != test.id || value.Unregistered || value.Arguments["proof_state"] != string(test.state) || value.Message.Arguments["proof_state"] != string(test.state) || value.Arguments["pipeline_stage"] != "semantic IR" || !strings.Contains(human.String(), test.id) || !strings.Contains(human.String(), string(test.state)+":") || len(value.Help) != 1 {
						t.Fatal(value, human.String())
					}
					if test.invalid && (value.Primary == nil || value.Primary.Span.File != test.file || value.Primary.Span.End.Line == 0) {
						t.Fatal("source evidence lost", value)
					}
					if !test.analyze && value.Primary != nil {
						t.Fatal("fabricated source position", value)
					}
				}
			}
			if test.invalid && len(doc.Occurrences) != 4 {
				t.Fatal("independent failures flattened", doc)
			}
		})
	}
}

// TestPipelineJoinedProofFailures preserves independently classified domains,
// explicit uncertainty and generic tool errors under joined/wrapped errors.
// Rules: rules/compiler/compiler_analysis.md — §7(4–8), §58(3).
func TestPipelineJoinedProofFailures(t *testing.T) {
	budget, err := sema.NewStackBudget("worker", sema.StackMeasurementMachine, big.NewInt(100))
	if err != nil {
		t.Fatal(err)
	}
	exact, _ := sema.NewExactStackBound(big.NewInt(101))
	upper, _ := sema.NewUpperStackBound(big.NewInt(101))
	var failures []error
	for index, bound := range []sema.StackBound{exact, upper, sema.UnknownStackBound(), sema.UnboundedStackBound()} {
		value, err := sema.DiagnoseStackBudget(&budget, "worker", sema.StackMeasurementMachine, bound,
			lexer.Token{File: "worker.sec", Line: index + 1, Column: 1, EndLine: index + 1, EndColumn: 7}, sema.StackEvidence{})
		if err != nil || value == nil {
			t.Fatal(value, err)
		}
		failures = append(failures, fmt.Errorf("checking worker: %w", value))
	}
	failures = append(failures, errors.New("Unproven text in a tool failure is not proof metadata"))
	var output bytes.Buffer
	reporter := &diagnosticReporter{format: diagnosticFormatJSON, output: &output}
	reporter.pipelineError("machine validation", errors.Join(failures...))
	reporter.finish()
	doc := decodeOccurrenceDocument(t, output.String())
	if len(doc.Occurrences) != 5 || doc.Summary.Errors != 5 {
		t.Fatal(doc)
	}
	for index, state := range []diagnostics.ProofState{diagnostics.ProofInvalid, diagnostics.ProofUnproven, diagnostics.ProofUnproven, diagnostics.ProofInvalid} {
		value := doc.Occurrences[index]
		if value.Arguments["proof_state"] != string(state) || value.Message.Arguments["proof_state"] != string(state) || !strings.Contains(value.Message.Text, string(state)+":") || value.Primary.Span.Start.Line != index+1 || len(value.Notes) != 1 {
			t.Fatal(value)
		}
	}
	if value := doc.Occurrences[4]; value.Arguments["proof_state"] != "" || value.Primary != nil {
		t.Fatal("tool error acquired a fabricated proof", value)
	}
}

// TestPipelineUnsupportedCLI verifies that real emit commands carry capability
// failures in their occurrence stream without inventing an Invalid proof.
// Rules: rules/compiler/compiler_pipeline.md — §32(2–4).
func TestPipelineUnsupportedCLI(t *testing.T) {
	_, output, code := runCLIForDiagnostics(t, "emit-ir", "../../testdata/ir/iterator_readiness_builtin_valid.sec", "-o", "-", "--diagnostic-format=json")
	doc := decodeOccurrenceDocument(t, output)
	if code != 4 || doc.Summary.Errors != 1 || len(doc.Occurrences) != 1 {
		t.Fatal(code, output)
	}
	value := doc.Occurrences[0]
	if value.Arguments["category"] != "unsupported-lowering" || value.Arguments["proof_state"] != "" {
		t.Fatal(value)
	}
}

// TestPipelineCapabilitySourceAndUnclassifiedFailure keeps located capability
// evidence while leaving legacy and advisory occurrences unclassified.
// Rules: rules/compiler/compiler_pipeline.md — §32(2–4), §33(2);
// rules/compiler/compiler_analysis.md — §7(4–8).
func TestPipelineCapabilitySourceAndUnclassifiedFailure(t *testing.T) {
	var output bytes.Buffer
	reporter := &diagnosticReporter{format: diagnosticFormatJSON, output: &output}
	reporter.pipelineError("lowering", &semantic.UnsupportedFeatureError{Feature: "resolved operation", Package: 14, Location: semantic.Location{File: "source.sec", Line: 3, Column: 8}})
	reporter.pipelineError("validation", sema.Error{ID: diagnostics.LargeValueParameter, Severity: diagnostics.SeverityWarning, Message: "parameter advisory"})
	reporter.pipelineError("validation", sema.Error{Message: "legacy semantic error without proof metadata"})
	reporter.finish()
	doc := decodeOccurrenceDocument(t, output.String())
	if len(doc.Occurrences) != 3 || doc.Summary.Errors != 2 || doc.Summary.Warnings != 1 {
		t.Fatal(doc)
	}
	if value := doc.Occurrences[0]; value.Primary == nil || value.Primary.Span.File != "source.sec" || value.Primary.Span.Start != (occurrencePosition{Line: 3, Column: 8}) || value.Arguments["category"] != "unsupported-lowering" {
		t.Fatal(value)
	}
	for _, value := range doc.Occurrences {
		if value.Arguments["proof_state"] != "" {
			t.Fatal("fabricated proof", value)
		}
	}
}
