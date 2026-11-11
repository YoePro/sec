package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"sec/internal/diagnostics"
)

// Optional copy advice exposes canonical cost/use evidence without rewriting
// ownership or mistaking a large move-only value for a legal expensive copy.
// Rules: rules/memory/copy_move.md §22; rules/tooling/lsp.md — Hover;
// rules/analysis/parameter_usage_analysis.md §§63–65,92.
func TestLargeCopyAdviceLSP(t *testing.T) {
	data, err := os.ReadFile("../../testdata/tooling/large_copy_advice.sec")
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	uri := uriFromPath(filepath.Join(t.TempDir(), "main.sec"))
	for _, mode := range []string{"off", "concise", "detailed"} {
		hover, ok := hoverForSourceWithParameterInsight(uri, text, offsetPosition(text, strings.Index(text, "Read(value")+1), parameterInsightSettings{Hover: mode})
		if !ok {
			t.Fatal("missing hover")
		}
		if strings.Contains(hover.Contents.Value, "Estimated copy size:") != (mode == "detailed") {
			t.Fatalf("%s: %+v", mode, hover)
		}
		if mode == "detailed" && (!strings.Contains(hover.Contents.Value, "by-value copy is legal") || !strings.Contains(hover.Contents.Value, "call-local read access")) {
			t.Fatal(hover)
		}
	}
	owner, _ := hoverForSourceWithParameterInsight(uri, text, offsetPosition(text, strings.Index(text, "ObserveOwner(value")+1), parameterInsightSettings{Hover: "detailed"})
	if strings.Contains(owner.Contents.Value, "AvoidCopyCost") || strings.Contains(owner.Contents.Value, "Estimated copy size:") || !strings.Contains(owner.Contents.Value, "implicit copying is not proven") {
		t.Fatal("move-only copy advice", owner)
	}
	values := analyze(uri, text)
	for _, d := range values {
		if d.Severity == 1 {
			t.Fatal("legal copy acquired ownership error", d)
		}
	}
	for _, level := range []string{"off", "info", "warning", "error"} {
		count := 0
		for _, d := range parameterInsightDiagnostics(values, parameterInsightSettings{Advisories: level}) {
			if d.Code != diagnostics.LargeValueParameter {
				continue
			}
			count++
			if !strings.Contains(d.Message, "not an ownership repair") {
				t.Fatal(d)
			}
		}
		want := 2
		if level == "off" {
			want = 0
		}
		if count != want {
			t.Fatalf("%s: want %d advice, got %d", level, want, count)
		}
	}
	if actions := ownershipCodeActions(uri, text, values); len(actions) != 0 {
		t.Fatalf("performance advice rewrites ownership: %+v", actions)
	}
}
