package main

import (
	"os"
	"sec/internal/lexer"
	"sec/internal/parser"
	"sec/internal/sema"
	"strings"
	"testing"
)

// TestNoAllocTargetPolicyLSP consumes the compiler's selected-profile proof and
// root/call/operation navigation without deriving a separate editor policy.
// Rules: rules/memory/allocation.md — §§24(4),(6),28(4),29(1),(4).
func TestNoAllocTargetPolicyLSP(t *testing.T) {
	for _, name := range []string{"roots", "allocate", "spawn", "defer"} {
		file := "../../testdata/sema/allocation_noalloc_" + name + ".sec"
		data, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		p := parser.New(lexer.NewWithFile(string(data), file))
		program := p.ParseProgram()
		if len(p.Errors()) != 0 {
			t.Fatal(p.Errors())
		}
		a := sema.NewAnalyzer()
		a.SetAllocationProfile("noalloc")
		errors := a.Analyze(program)
		if len(errors) != 1 {
			t.Fatal(errors)
		}
		d := semaDiagnosticWithSources(errors[0], 3, uriFromPath(file), string(data), sourceOverlay{})
		if d.Code != "S1108" || d.Severity != 1 || d.Data == nil || d.Data.ProofState != errors[0].ProofState {
			t.Fatal(d)
		}
		for _, step := range errors[0].AllocationCause.Steps {
			if !strings.Contains(d.Message, step.Message) {
				t.Fatal(d)
			}
			point := diagnosticTokenStart(string(data), step.Source)
			if step.Kind == "root" && d.Range.Start == point {
				continue // The primary location already navigates to the root.
			}
			found := false
			for _, related := range d.RelatedInformation {
				if related.Location.URI == uriFromPath(file) && related.Location.Range.Start == point {
					found = true
				}
			}
			if !found {
				t.Fatal("missing policy navigation", step, d)
			}
		}
	}
}
