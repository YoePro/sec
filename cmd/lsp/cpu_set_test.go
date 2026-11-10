package main

import (
	"os"
	"strings"
	"testing"
)

// TestCpuSetLSP shares nominal type, duplicate diagnostics and formatting with
// the compiler; it never presents synthetic positional keys as public fields.
// Rules: rules/concurrency/threads.md — §7; rules/tooling/lsp.md — Hover, Completion.
func TestCpuSetLSP(t *testing.T) {
	assertFormattingGoldenLSP(t, "cpu_set")
	source, err := os.ReadFile("../../testdata/sema/cpu_set_valid.sec")
	if err != nil {
		t.Fatal(err)
	}
	text := string(source)
	uri := "file:///cpu-set/main.sec"
	for _, d := range analyze(uri, text) {
		if d.Severity == 1 {
			t.Fatal(d)
		}
	}
	offset := strings.Index(text, "cpus :=")
	hover, ok := hoverForSource(uri, text, offsetPosition(text, offset))
	if !ok || !strings.Contains(hover.Contents.Value, "CpuSet") || strings.Contains(hover.Contents.Value, "set[uint]") {
		t.Fatal(hover)
	}
	invalid, err := os.ReadFile("../../testdata/sema/cpu_set_invalid.sec")
	if err != nil {
		t.Fatal(err)
	}
	count := 0
	duplicate := false
	for _, d := range analyze(uri, string(invalid)) {
		if d.Severity != 1 {
			continue
		}
		count++
		if strings.Contains(d.Message, "duplicate CpuSet index") {
			duplicate = true
			if d.Code != "S1214" || len(d.RelatedInformation) != 1 {
				t.Fatal(d)
			}
		}
	}
	if count != 8 || !duplicate {
		t.Fatal(count, duplicate)
	}
}
