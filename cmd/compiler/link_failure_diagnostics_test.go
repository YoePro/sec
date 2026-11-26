package main

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"sec/internal/diagnostics"
	"sec/internal/linking"
	"sec/internal/testsupport"
)

// Human/JSON diagnostics preserve typed linking classes under wrapping/joins,
// with registered IDs and no fabricated source validity proof or location.
// Rules: rules/compiler/linking.md §64; rules/tooling/diagnostics.md §§8-10,14.
func TestLinkFailureDiagnostics(t *testing.T) {
	failures := errors.Join(
		fmt.Errorf("driver boundary: %w", &linking.Failure{Class: linking.ToolchainInvocationFailure, Tool: "missing-clang", Target: "linux-target", Output: "program", Cause: errors.New("cannot start")}),
		&linking.Failure{Class: linking.LinkResolutionFailure, Tool: "clang", Detail: "undefined native symbol"},
		&linking.Failure{Class: linking.ArtifactVerificationFailure, Tool: "clang", Detail: "empty output"},
		errors.New("unsupported linker text alone is not a capability or linking class"))
	for _, format := range []diagnosticOutputFormat{diagnosticFormatJSON, diagnosticFormatHuman} {
		var output bytes.Buffer
		reporter := &diagnosticReporter{format: format, output: &output}
		reporter.pipelineError("link", failures)
		reporter.finish()
		if format == diagnosticFormatHuman {
			for _, value := range []string{"S1262", "S1263", "S1264", "ToolchainInvocationFailure", "LinkResolutionFailure", "ArtifactVerificationFailure"} {
				if !strings.Contains(output.String(), value) {
					t.Fatal(output.String())
				}
			}
			continue
		}
		document := decodeOccurrenceDocument(t, output.String())
		if len(document.Occurrences) != 4 {
			t.Fatal(output.String())
		}
		for i, item := range document.Occurrences {
			if item.Primary != nil || item.Arguments["proof_state"] != "" {
				t.Fatal(item)
			}
			if i == 3 {
				if item.Arguments["diagnostic_class"] != "" {
					t.Fatal(item)
				}
				continue
			}
			id := []string{diagnostics.LinkToolchainInvocation, diagnostics.LinkResolution, diagnostics.LinkArtifactVerification}[i]
			if item.ID == nil || *item.ID != id || item.Arguments["diagnostic_class"] == "" || len(item.Help) != 1 || item.Arguments["category"] == "unsupported-lowering" {
				t.Fatal(item)
			}
		}
	}
}

// The real build command reports link-phase IDs and never replaces an existing
// artifact on a missing driver, unsuccessful link, or successful empty result.
// Rules: rules/compiler/linking.md §§50-51,64; rules/tooling/diagnostics.md §14.
func TestBuildLinkFailureDiagnostics(t *testing.T) {
	testsupport.RequireSecMLIROptPath(t)
	if runtime.GOOS == "windows" {
		t.Skip("shell driver fixtures require Unix")
	}
	for _, mode := range []string{"invocation", "resolution", "artifact"} {
		t.Run(mode, func(t *testing.T) {
			directory := t.TempDir()
			driver := filepath.Join(directory, "clang")
			if mode != "invocation" {
				fixture := "../../testdata/tooling/linking/" + mode + ".sh"
				data, err := os.ReadFile(fixture)
				if err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(driver, data, 0700); err != nil {
					t.Fatal(err)
				}
			}
			output := filepath.Join(directory, "program")
			if err := os.WriteFile(output, []byte("previous artifact"), 0600); err != nil {
				t.Fatal(err)
			}
			_, stderr, code := runCLIForDiagnostics(t, "build", "../../testdata/build/native/add.sec", "-o", output, "--clang", driver, "--diagnostic-format=json")
			wantID, wantClass, wantCode := diagnostics.LinkResolution, linking.LinkResolutionFailure, 5
			if mode == "invocation" {
				wantID, wantClass, wantCode = diagnostics.LinkToolchainInvocation, linking.ToolchainInvocationFailure, 1
			}
			if mode == "artifact" {
				wantID, wantClass = diagnostics.LinkArtifactVerification, linking.ArtifactVerificationFailure
			}
			if code != wantCode {
				t.Fatalf("exit %d, want %d: %s", code, wantCode, stderr)
			}
			document := decodeOccurrenceDocument(t, stderr)
			found := false
			for _, occurrence := range document.Occurrences {
				if occurrence.ID != nil && *occurrence.ID == wantID {
					found = true
					if occurrence.Arguments["diagnostic_class"] != string(wantClass) || occurrence.Primary != nil || len(occurrence.Help) == 0 {
						t.Fatal(occurrence)
					}
				}
			}
			if !found {
				t.Fatal(stderr)
			}
			data, err := os.ReadFile(output)
			if err != nil || string(data) != "previous artifact" {
				t.Fatalf("previous artifact lost: %q %v", data, err)
			}
		})
	}
}
