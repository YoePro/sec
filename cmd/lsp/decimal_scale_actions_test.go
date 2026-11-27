package main

import (
	"os"
	"path/filepath"
	"sec/internal/diagnostics"
	"sec/internal/lexer"
	"sec/internal/parser"
	"sec/internal/sema"
	"strings"
	"testing"
)

// A2012 fixes preserve literal/trivia bytes, canonical range/default facts,
// negative/capacity-reduced scales and inherited carrier constraints. Explicit
// and inherited scales, stale diagnostics and source errors offer no action.
// Rules: rules/tooling/diagnostics.md — §31A; MD-049 — §15(6).
func TestDecimalScaleCodeActions(t *testing.T) {
	data, err := os.ReadFile("../../testdata/lsp/decimal_scale/actions.sec")
	if err != nil {
		t.Fatal(err)
	}
	source := string(data)
	uri := uriFromPath(filepath.Join(t.TempDir(), "main.sec"))
	reports := analyze(uri, source)
	count := 0
	for _, report := range reports {
		if report.Code != diagnostics.InferredDecimalScale {
			if report.Severity == 1 {
				t.Fatal(report)
			}
			continue
		}
		count++
		if report.Data == nil || report.Data.Arguments["selected_scale"] == "" {
			t.Fatal("missing structured scale facts", report)
		}
		actions := decimalScaleCodeActions(uri, source, []diagnostic{report}, sourceOverlay{})
		if len(actions) != 1 {
			t.Fatalf("%s: actions=%+v", report.Message, actions)
		}
		edits := actions[0].Edit.Changes[uri]
		if len(edits) != 1 || edits[0].NewText != "scale "+report.Data.Arguments["selected_scale"]+" " {
			t.Fatal(edits)
		}
		at := lineCharToOffset(source, edits[0].Range.Start.Line, edits[0].Range.Start.Character)
		if edits[0].Range.Start != edits[0].Range.End || source[at:at+5] != "range" {
			t.Fatal("not before first range", edits)
		}
		updated := applyTextEdits(source, edits)
		if updated[:at] != source[:at] || updated[at+len(edits[0].NewText):] != source[at:] {
			t.Fatal("changed existing source")
		}
		// Exact current diagnostic must disappear; other declarations retain their advisory.
		for _, d := range analyze(uri, updated) {
			if d.Severity == 1 || (d.Code == report.Code && d.Range.Start == report.Range.Start) {
				t.Fatal(d)
			}
		}
		if got := decimalScaleCodeActions(uri, updated, []diagnostic{report}, sourceOverlay{}); len(got) != 0 {
			t.Fatal("stale action", got)
		}
	}
	if count != 6 {
		t.Fatalf("got %d inferred declarations: %+v", count, reports)
	}
	// Forged client text is not authority for the selected scale.
	for _, report := range reports {
		if report.Code == diagnostics.InferredDecimalScale {
			report.Message = "forged"
			if got := decimalScaleCodeActions(uri, source, []diagnostic{report}, sourceOverlay{}); len(got) != 0 {
				t.Fatal(got)
			}
			break
		}
	}
	if got := decimalScaleCodeActions(uri, source+"\nfn Broken() int { return false }\n", reports, sourceOverlay{}); len(got) != 0 {
		t.Fatal("unsafe fix on erroneous snapshot", got)
	}
	if strings.Contains(source, "scale -1") {
		t.Fatal("fixture already explicit")
	}
}

// The equivalence gate rejects otherwise valid edits that change a grid,
// bound or explicit default, even when the resulting program still compiles.
// Rules: rules/tooling/diagnostics.md — §31A; MD-049 — §15(6).
func TestDecimalScaleFixEquivalence(t *testing.T) {
	source, err := os.ReadFile("../../testdata/lsp/decimal_scale/proof.sec")
	if err != nil {
		t.Fatal(err)
	}
	analyzeType := func(text string) sema.Type {
		p := parser.New(lexer.New(text))
		program := p.ParseProgram()
		if len(p.Errors()) != 0 {
			t.Fatal(p.Errors())
		}
		a := sema.NewAnalyzer()
		if errors := a.Analyze(program); len(errors) != 0 {
			t.Fatal(errors)
		}
		return a.Types()["Amount"]
	}
	before := analyzeType(string(source))
	for _, test := range []struct {
		old, new   string
		equivalent bool
	}{
		{"range", "scale 2 range", true},
		{"range", "scale 1 range", false},
		{"10.00", "20.00", false},
		{"default 1.00", "default 2.00", false},
	} {
		after := analyzeType(strings.Replace(string(source), test.old, test.new, 1))
		if got := decimalScaleFixEquivalent(before, after); got != test.equivalent {
			t.Fatalf("%s -> %s: %v", test.old, test.new, got)
		}
	}
}
