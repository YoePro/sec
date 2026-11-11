package sema

import (
	"os"
	"strings"
	"testing"

	"sec/internal/diagnostics"
	"sec/internal/lexer"
	"sec/internal/parser"
)

// Blocked-operation inspection preserves canonical proof and complete source
// witnesses without replaying analysis or exposing mutable diagnostic slices.
// Rules: rules/memory/arena.md §§36,44,51,119–120;
// rules/tooling/diagnostics.md §9; rules/tooling/lsp.md — Hover.
func TestArenaInvalidationBlockerFacts(t *testing.T) {
	for _, file := range []string{"arena_invalidation_diagnostics_invalid.sec", "arena_nested_invalid.sec"} {
		data, err := os.ReadFile("../../testdata/sema/" + file)
		if err != nil {
			t.Fatal(err)
		}
		a, errors := analyzeSourceWithAnalyzerRaw(t, string(data))
		count := 0
		for _, problem := range errors {
			if problem.ID != diagnostics.ArenaResetLiveDependency && problem.ID != diagnostics.ArenaReleaseLiveDependency {
				continue
			}
			count++
			facts := a.ArenaInvalidationBlockersAt(problem.File, problem.Line, problem.Column)
			if len(facts) != 1 || facts[0].ID != problem.ID || facts[0].ProofState != diagnostics.ProofInvalid || len(facts[0].RelatedLocations()) < 2 {
				t.Fatal("missing causal blocker", facts)
			}
			links := facts[0].RelatedLocations()
			if links[0].Label != "live Arena dependency" || links[0].Line >= problem.Line {
				t.Fatal("bad source chain", links, problem)
			}
			if file == "arena_nested_invalid.sec" {
				if !strings.Contains(links[1].Label, "child Arena retains parent backing") {
					t.Fatal(links)
				}
			} else if !strings.Contains(links[1].Label, "keeps the Arena dependency live") {
				t.Fatal(links)
			}
			facts[0].Related[0].Label = "corrupted"
			if a.ArenaInvalidationBlockersAt(problem.File, problem.Line, problem.Column)[0].Related[0].Label == "corrupted" {
				t.Fatal("shared mutable facts")
			}
			if len(a.ArenaInvalidationBlockersAt("other.sec", problem.Line, problem.Column)) != 0 {
				t.Fatal("matched wrong source")
			}
		}
		want := 6
		if file == "arena_nested_invalid.sec" {
			want = 7
		}
		if count != want {
			t.Fatal(file, count, errors)
		}
		valid, err := os.ReadFile("../../testdata/sema/arena_invalidation_diagnostics_valid.sec")
		if err != nil {
			t.Fatal(err)
		}
		// Reuse the analyzer: rejected-operation facts cannot survive a new run.
		p := parser.New(lexer.New(string(valid)))
		program := p.ParseProgram()
		if len(p.Errors()) != 0 {
			t.Fatal(p.Errors())
		}
		if errs := a.Analyze(program); len(errs) != 0 {
			t.Fatal(errs)
		}
		for _, problem := range errors {
			if len(a.ArenaInvalidationBlockersAt(problem.File, problem.Line, problem.Column)) != 0 {
				t.Fatal("stale blocker")
			}
		}
	}
}
