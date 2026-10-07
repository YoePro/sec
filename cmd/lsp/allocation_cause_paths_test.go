package main

import (
	"os"
	"path/filepath"
	"sec/internal/lexer"
	"sec/internal/parser"
	"sec/internal/sema"
	"strings"
	"testing"
)

// TestAllocationCausePathsLSP checks complete canonical witnesses, source links,
// stable owning diagnostic identity and terminal-location deduplication.
// Rules: rules/memory/allocation.md — §§28(4),29(4); rules/tooling/lsp.md — "Shared diagnostic model".
func TestAllocationCausePathsLSP(t *testing.T) {
	file := "../../testdata/sema/allocation_cause_paths_invalid.sec"
	data, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	p := parser.New(lexer.NewWithFile(string(data), file))
	program := p.ParseProgram()
	if len(p.Errors()) != 0 {
		t.Fatal(p.Errors())
	}
	errs := sema.NewAnalyzer().Analyze(program)
	if len(errs) != 4 {
		t.Fatal(errs)
	}
	for _, e := range errs {
		got := semaDiagnosticWithSources(e, 3, uriFromPath(file), string(data), nil)
		if got.Code != "S1108" || got.Severity != 1 || e.AllocationCause == nil || len(got.RelatedInformation) < 2 {
			t.Fatal(got)
		}
		for _, step := range e.AllocationCause.Steps {
			if !strings.Contains(got.Message, "note: "+step.Message) {
				t.Fatal(step, got)
			}
			point := diagnosticTokenStart(string(data), step.Source)
			found := false
			for _, link := range got.RelatedInformation {
				if link.Location.URI == uriFromPath(file) && link.Location.Range.Start == point {
					found = true
				}
			}
			if !found {
				t.Fatal("missing navigable witness", step, got)
			}
		}
		seen := map[position]bool{}
		for _, link := range got.RelatedInformation {
			if seen[link.Location.Range.Start] {
				t.Fatal("duplicate terminal link", got)
			}
			seen[link.Location.Range.Start] = true
		}
	}
}

// TestAllocationCausePathsLSPOverlay checks cross-file unsaved UTF-16 mapping
// and incomplete witnesses without pretending missing metadata is a source link.
// Rules: rules/memory/allocation.md — §29(4); rules/tooling/lsp.md — protocol position encoding.
func TestAllocationCausePathsLSPOverlay(t *testing.T) {
	file := filepath.Join(t.TempDir(), "callee.sec")
	root := filepath.Join(t.TempDir(), "root.sec")
	e := sema.Error{ID: "S1108", File: root, Line: 1, Column: 1, Message: "allocation", AllocationCause: &sema.AllocationCausePath{Incomplete: true, Steps: []sema.AllocationCauseStep{{Kind: "call", Source: lexer.Token{File: file, Line: 1, Column: 4}, Message: "call"}, {Kind: "operation", Message: "missing source"}}}}
	got := semaDiagnosticWithSources(e, 1, uriFromPath(root), "root", sourceOverlay{file: "😀abc"})
	if len(got.RelatedInformation) != 1 || got.RelatedInformation[0].Location.URI != uriFromPath(file) || got.RelatedInformation[0].Location.Range.Start.Character != 4 || !strings.Contains(got.Message, "allocation cause path is incomplete") {
		t.Fatal(got)
	}
}
