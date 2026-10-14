package main

import (
	"bytes"
	"os"
	"sec/internal/lexer"
	"sec/internal/parser"
	"sec/internal/sema"
	"testing"
)

// TestNoAllocTargetPolicyCLI transports the canonical selected-profile root
// proof through the CLI occurrence reporter, including every cause location.
// Rules: rules/memory/allocation.md — §§24(4),(6),28(4),29(1),(4).
func TestNoAllocTargetPolicyCLI(t *testing.T) {
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
		var output bytes.Buffer
		r := &diagnosticReporter{format: diagnosticFormatJSON, output: &output}
		r.semaDiagnostic(errors[0], errors[0].Error())
		r.finish()
		doc := decodeOccurrenceDocument(t, output.String())
		if len(doc.Occurrences) != 1 || doc.Summary.Errors != 1 {
			t.Fatal(doc)
		}
		item := doc.Occurrences[0]
		if item.ID == nil || *item.ID != "S1108" || item.Arguments["proof_state"] != string(errors[0].ProofState) || len(item.Notes) != len(errors[0].AllocationCause.Steps) {
			t.Fatal(item)
		}
		for _, note := range item.Notes {
			if note.Arguments["file"] != file || note.Arguments["incomplete"] != "false" {
				t.Fatal(note)
			}
		}
	}
}
