package main

import (
	"os"
	"strings"
	"testing"
)

// TestISRPanicDiagnosticsLSP retains the common cause-first occurrence and ISR
// root link, with repair help that never proposes weakening the implied promise.
// Rules: rules/errors/panic.md — §28(5)–(6); rules/platform/interrupts.md — §43.
func TestISRPanicDiagnosticsLSP(t *testing.T) {
	data, err := os.ReadFile("../../testdata/sema/isr_panic_paths_invalid.sec")
	if err != nil {
		t.Fatal(err)
	}
	uri := "file:///tmp/sec-isr-panic/main.sec"
	values := analyze(uri, string(data))
	count := 0
	for _, value := range values {
		if value.Code != "S1092" {
			continue
		}
		count++
		if value.Severity != 1 || !strings.Contains(value.Message, "unsafe does not waive") || len(value.RelatedInformation) != 1 || value.RelatedInformation[0].Location.URI != uri || value.Range.Start.Line == value.RelatedInformation[0].Location.Range.Start.Line {
			t.Fatal(value)
		}
		if strings.Contains(value.Message, "function UnsafeRoot ") && (value.Range.Start.Line != 1 || value.RelatedInformation[0].Location.Range.Start.Line != 10) {
			t.Fatal("explicit noPanic hid ISR root", value)
		}
	}
	if count != 7 {
		t.Fatal("missing canonical panic diagnostics", values)
	}
}
