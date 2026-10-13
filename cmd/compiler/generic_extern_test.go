package main

import (
	"os"
	"path/filepath"
	"testing"

	"sec/internal/diagnostics"
)

// TestGenericExternStopsBeforeBackend ensures every semantic CLI entry rejects
// even unused generic externs before assigning a runtime ABI or emitting files.
// Rules: rules/compiler/monomorphization.md — §16.1, §16.4;
// rules/platform/ffi.md — §48, §49, §52.
func TestGenericExternStopsBeforeBackend(t *testing.T) {
	fixture := "../../testdata/tooling/generic_extern_invalid.sec"
	for _, command := range []string{"check", "sema", "analyse", "emit-ir", "emit-llvm", "emit-sec-mlir", "emit-mlir", "build"} {
		t.Run(command, func(t *testing.T) {
			output := filepath.Join(t.TempDir(), "output")
			args := []string{command, fixture, "--diagnostic-format=json"}
			if command != "check" && command != "sema" && command != "analyse" {
				args = append(args, "-o", output)
			}
			stdout, stderr, code := runCLIForDiagnostics(t, args...)
			if code != 3 {
				t.Fatalf("exit=%d stdout=%s stderr=%s", code, stdout, stderr)
			}
			doc := decodeOccurrenceDocument(t, stderr)
			if len(doc.Occurrences) != 3 {
				t.Fatal(doc)
			}
			for _, value := range doc.Occurrences {
				if value.ID == nil || *value.ID != diagnostics.UnresolvedGenericExtern || value.Arguments["proof_state"] != string(diagnostics.ProofInvalid) || len(value.Related) != 1 || value.Related[0].Message == nil || value.Related[0].Message.Text != "generic extern declaration" {
					t.Fatal(value)
				}
			}
			if _, err := os.Stat(output); !os.IsNotExist(err) {
				t.Fatal("unexpected backend artifact", err)
			}
		})
	}
	_, stderr, code := runCLIForDiagnostics(t, "check", "../../testdata/tooling/generic_extern_wrapper_valid.sec", "--diagnostic-format=json")
	if code != 0 {
		t.Fatalf("concrete provider and generic wrapper rejected: %s", stderr)
	}
}
