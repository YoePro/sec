package sema

import (
	"os"
	"sec/internal/lexer"
	"sec/internal/parser"
	"strings"
	"testing"
)

// TestArenaReferenceAliasRetainsDomain checks that changing the local holder
// of a borrowed Arena does not create a new symbolic allocation domain.
// Rules: rules/memory/arena.md — §§4.2,44,68(3)-(4).
func TestArenaReferenceAliasRetainsDomain(t *testing.T) {
	path := "../../testdata/sema/arena_domain_alias_invalid.sec"
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	p := parser.New(lexer.NewWithFile(string(data), path))
	program := p.ParseProgram()
	if len(p.Errors()) != 0 {
		t.Fatal(p.Errors())
	}
	a := NewAnalyzer()
	errors := a.Analyze(program)
	if len(errors) != 2 {
		t.Fatal(errors)
	}
	for _, e := range errors {
		if !strings.Contains(e.Message, "cannot use values after arena") {
			t.Fatal(e)
		}
	}
	if got := a.functions["MovedReset"][0].ArenaParameterResets; len(got) != 1 || got[0] != 0 {
		t.Fatal(got)
	}
}

// TestArenaStableDomainCompletion keeps domain identity through repeated
// allocation, reset, owner moves/assignment, parameter moves and repeated names in distinct callables.
// Reanalysis preserves the same compiler-owned identities at every depth.
// Rules: rules/memory/arena.md — §§4.2(1)-(5),4.5,7(6),44,68.
func TestArenaStableDomainCompletion(t *testing.T) {
	path := "../../testdata/sema/arena_domain_stable_valid.sec"
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var previous string
	for _, depth := range []AnalysisDepth{AnalysisInteractive, AnalysisStandard, AnalysisDeep} {
		p := parser.New(lexer.NewWithFile(string(data), path))
		program := p.ParseProgram()
		if len(p.Errors()) != 0 {
			t.Fatal(p.Errors())
		}
		a := NewAnalyzerWithDepth(depth)
		for run := 0; run < 2; run++ {
			if errors := a.Analyze(program); len(errors) != 0 {
				t.Fatal(errors)
			}
			key := ""
			domains := map[string]string{}
			for _, node := range a.CallGraph().Nodes() {
				sites := a.CallGraph().ArenaSummary(node.ID).DirectEffects
				if len(sites) == 0 {
					t.Fatal(node.Name, "missing effects")
				}
				for _, site := range sites {
					if site.Domain == "" || site.Domain == site.Arena {
						t.Fatal(site)
					}
					key += site.Domain + "/"
				}
				for _, site := range sites {
					if site.Domain != sites[0].Domain {
						t.Fatal(node.Name, sites)
					}
				}
				for other, domain := range domains {
					if domain == sites[0].Domain {
						t.Fatal("distinct owners share a domain", other, node.Name)
					}
				}
				domains[node.Name] = sites[0].Domain

			}
			if previous != "" && key != previous {
				t.Fatal("unstable domain identity", previous, key)
			}
			previous = key
		}
	}
}
