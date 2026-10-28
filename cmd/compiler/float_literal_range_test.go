package main

import (
	"strings"
	"testing"

	"sec/internal/diagnostics"
)

// Literal validity is checked before backend discovery, using the selected
// target width, and keeps its registered diagnostic and source span in JSON.
// Rules: rules/compiler/compiler_pipeline.md — §56(1);
// rules/mlir/packages/sec-mlir-dialect_package20.md — §16;
// rules/corrections/md066-md067-md068-decimal-correction-20261028.md — §3(7).
func TestCheckFloatLiteralRangeBoundary(t *testing.T) {
	for _, tc := range []struct {
		target string
		count  int
	}{
		{"linux-amd64", 11}, {"linux-armv7", 12},
	} {
		t.Run(tc.target, func(t *testing.T) {
			_, output, code := runCheckWithoutTools(t, "check", "../../testdata/sema/md066_literals/decimals.sec", "--target", tc.target)
			if code != 0 {
				t.Fatal(code, output)
			}
			_, output, code = runCheckWithoutTools(t, "check", "../../testdata/sema/md066_literals/float_ranges.sec", "--target", tc.target, "--diagnostic-format=json")
			document := decodeOccurrenceDocument(t, output)
			if code != 3 || document.Summary.Errors != tc.count || len(document.Occurrences) != tc.count {
				t.Fatal(code, output)
			}
			for _, occurrence := range document.Occurrences {
				if occurrence.ID == nil || *occurrence.ID != diagnostics.FloatLiteralOutOfRange || !strings.HasSuffix(occurrence.Primary.Span.File, "float_ranges.sec") || occurrence.Primary.Span.End.Column <= occurrence.Primary.Span.Start.Column {
					t.Fatalf("lost literal diagnostic: %+v", occurrence)
				}
			}
		})
	}
}
