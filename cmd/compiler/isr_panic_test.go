package main

import (
	"strings"
	"testing"
)

// TestISRPanicDiagnosticsCLI projects Sema's cause and ISR root consistently at
// every frontend gate; none of these diagnostics certifies complete ISR support.
// Rules: rules/errors/panic.md — §28(5)–(6); rules/platform/interrupts.md — §43.
func TestISRPanicDiagnosticsCLI(t *testing.T) {
	for _, command := range []string{"sema", "analyse", "emit-ir", "emit-llvm", "emit-sec-mlir", "emit-mlir"} {
		args := []string{command, "../../testdata/sema/isr_panic_paths_invalid.sec", "--diagnostic-format=json"}
		if strings.HasPrefix(command, "emit-") {
			args = append(args, "-o", "-")
		}
		stdout, output, code := runCLIForDiagnostics(t, args...)
		if code != 3 || stdout != "" {
			t.Fatal(command, code, stdout, output)
		}
		doc := decodeOccurrenceDocument(t, output)
		count := 0
		for _, occurrence := range doc.Occurrences {
			if occurrence.ID == nil || *occurrence.ID != "S1092" {
				continue
			}
			count++
			if len(occurrence.Related) != 1 || len(occurrence.Help) != 1 || !strings.Contains(occurrence.Help[0].Text, "unsafe does not waive") || occurrence.Primary.Span.Start.Line == occurrence.Related[0].Span.Start.Line {
				t.Fatal(command, occurrence)
			}
		}
		if count != 7 {
			t.Fatal(command, count, output)
		}
	}
}
