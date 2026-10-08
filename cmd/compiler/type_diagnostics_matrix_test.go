package main

import (
	"reflect"
	"testing"

	"sec/internal/diagnostics"
)

// TestTypeDiagnosticsIndependentOfBackend checks that every lowering command
// stops at the same semantic errors with unchanged explanations.
// Rules: rules/types/types.md — Diagnostics; rules/tooling/diagnostics.md — Shared model.
func TestTypeDiagnosticsIndependentOfBackend(t *testing.T) {
	for _, file := range []string{"type_diagnostics/invalid.sec", "unit_parameters_invalid.sec"} {
		t.Run(file, func(t *testing.T) {
			var baseline []string
			for _, command := range []string{"sema", "emit-ir", "emit-llvm", "emit-sec-mlir", "emit-mlir"} {
				args := []string{command, "../../testdata/sema/" + file}
				if command != "sema" {
					args = append(args, "-o", "-")
				}
				_, output, code := runCLIForDiagnostics(t, append(args, "--diagnostic-format=json")...)
				doc := decodeOccurrenceDocument(t, output)
				expected := 6
				if file == "unit_parameters_invalid.sec" {
					expected = 5
				}
				if code != 3 || doc.Summary.Errors != expected || len(doc.Occurrences) != expected {
					t.Fatal(command, code, output)
				}
				var messages []string
				for _, e := range doc.Occurrences {
					if file == "unit_parameters_invalid.sec" && (e.ID == nil || *e.ID != diagnostics.UnitPolymorphismReserved || e.Severity != diagnostics.SeverityError || len(e.Help) != 1 || e.Primary == nil || e.Primary.Span.Start.Line == 0) {
						t.Fatal(e)
					}
					messages = append(messages, e.Message.Text)
				}
				if baseline == nil {
					baseline = messages
				} else if !reflect.DeepEqual(messages, baseline) {
					t.Fatalf("%s: %v differs from %v", command, messages, baseline)
				}
			}
		})
	}
}
