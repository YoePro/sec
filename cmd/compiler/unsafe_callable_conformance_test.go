package main

import (
	"strings"
	"testing"

	"sec/internal/diagnostics"
)

// Unsafe callable errors stop every requested frontend output boundary;
// conformance reports incompatibility instead of hiding behind a missing method.
// Rules: rules/memory/unsafe.md — §§6–7,10–11,39–41;
// rules/compiler/compiler_pipeline.md — §§32–34.
func TestUnsafeCallableConformanceCLI(t *testing.T) {
	for _, test := range []struct {
		name  string
		count int
	}{{"contracts_invalid", 3}, {"calls_invalid", 12}} {
		for _, command := range []string{"check", "emit-ir", "emit-mlir"} {
			args := []string{command, "../../testdata/sema/unsafe_callable_conformance/" + test.name + ".sec", "--diagnostic-format=json"}
			if command != "check" {
				args = append(args, "-o", "-")
			}
			stdout, stderr, code := runCLIForDiagnostics(t, args...)
			doc := decodeOccurrenceDocument(t, stderr)
			if code != 3 || stdout != "" || doc.Summary.Errors != test.count {
				t.Fatal(command, test.name, code, stdout, stderr)
			}
			if test.name == "contracts_invalid" {
				for _, item := range doc.Occurrences {
					if item.ID == nil || *item.ID != diagnostics.InterfaceMemberIncompatible || !strings.Contains(item.Message.Text, "unsafe caller obligations") {
						t.Fatal(item)
					}
				}
			}
		}
	}
	_, stderr, code := runCLIForDiagnostics(t, "check", "../../testdata/sema/unsafe_callable_conformance/valid.sec")
	if code != 0 {
		t.Fatal(code, stderr)
	}
}
