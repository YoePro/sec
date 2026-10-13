package main

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"sec/internal/diagnostics"
	"sec/internal/ir/semantic"
	"sec/internal/lowering/secmlir"
	"sec/internal/sema"
)

// TestCapabilityDiagnosticBoundaries transports typed limits separately from
// source errors and unclassified target/tool failures under wraps and joins.
// Rules: rules/compiler/compiler_pipeline.md — §32(1–5), §97(1);
// rules/compiler/compiler_analysis.md — §7(4–8).
func TestCapabilityDiagnosticBoundaries(t *testing.T) {
	failures := errors.Join(
		fmt.Errorf("first boundary: %w", &semantic.UnsupportedFeatureError{Feature: "iterator", Package: 14, Location: semantic.Location{File: "source.sec", Line: 2, Column: 3}}),
		fmt.Errorf("next boundary: %w", &secmlir.UnsupportedLoweringError{Feature: "guarded handler", Function: "main::Run"}),
		sema.Error{ID: diagnostics.IndexOutOfBounds, Severity: diagnostics.SeverityError, ProofState: diagnostics.ProofInvalid, Message: "source bounds violation"},
		errors.New("unsupported target or unavailable verifier is not a typed capability fact"))
	var output bytes.Buffer
	r := &diagnosticReporter{format: diagnosticFormatJSON, output: &output}
	r.pipelineError("lowering", failures)
	r.finish()
	doc := decodeOccurrenceDocument(t, output.String())
	if len(doc.Occurrences) != 4 {
		t.Fatal(doc)
	}
	capabilities, source, other := 0, 0, 0
	for _, item := range doc.Occurrences {
		if item.Arguments["diagnostic_class"] == "compiler-capability" {
			capabilities++
			if item.Arguments["category"] != "unsupported-lowering" || item.Arguments["proof_state"] != "" || item.Arguments["feature"] == "" || item.Arguments["boundary"] == "" || len(item.Help) != 1 || len(item.Notes) != 1 {
				t.Fatal(item)
			}
			if item.Arguments["callable"] != "" && item.Primary != nil {
				t.Fatal("fabricated source from callable ID", item)
			}
		} else if item.Arguments["proof_state"] == "Invalid" {
			source++
		} else {
			other++
		}
	}
	if capabilities != 2 || source != 1 || other != 1 {
		t.Fatal(doc)
	}
	output.Reset()
	r = &diagnosticReporter{format: diagnosticFormatHuman, output: &output}
	r.pipelineError("lowering", failures)
	if strings.Count(output.String(), "error [unsupported-lowering]") != 2 || !strings.Contains(output.String(), "source bounds violation") {
		t.Fatal(output.String())
	}
}

// TestCapabilitySecMLIRCLI exercises a valid source at the real schema limit,
// stopping before external tools or publishing output. Ordinary check succeeds.
// Rules: rules/compiler/compiler_pipeline.md — §32(2–4), §56(1–3), §59.
func TestCapabilitySecMLIRCLI(t *testing.T) {
	file := "../../testdata/tooling/capability_guarded_try.sec"
	_, stderr, code := runCLIForDiagnostics(t, "check", file)
	if code != 0 {
		t.Fatal(code, stderr)
	}
	output := filepath.Join(t.TempDir(), "result.mlir")
	_, stderr, code = runCLIForDiagnostics(t, "emit-sec-mlir", file, "-o", output, "--mlir-bin", filepath.Join(t.TempDir(), "missing-tools"), "--diagnostic-format=json")
	doc := decodeOccurrenceDocument(t, stderr)
	if code != 4 || len(doc.Occurrences) != 1 {
		t.Fatal(code, stderr)
	}
	item := doc.Occurrences[0]
	if item.Arguments["diagnostic_class"] != "compiler-capability" || item.Arguments["feature"] != "guarded try handlers" || item.Arguments["callable"] == "" || item.Arguments["proof_state"] != "" || !strings.HasPrefix(item.Arguments["boundary"], "Sec MLIR schema ") {
		t.Fatal(item)
	}
	if _, err := os.Stat(output); !os.IsNotExist(err) {
		t.Fatal("unsupported lowering published an artifact", err)
	}
}
