package main

import (
	"bytes"
	"os"
	"sec/internal/lexer"
	"sec/internal/parser"
	"sec/internal/sema"
	"strings"
	"testing"
)

// TestAnalyseAllocationFacts checks all three canonical states, retained mixed
// evidence and callable selection without promoting uncertainty to an error.
// Rules: rules/memory/allocation.md — §§24(6),29(1)-(2),30.
func TestAnalyseAllocationFacts(t *testing.T) {
	file := "../../testdata/sema/allocation_facts_valid.sec"
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
	if errs := a.Analyze(program); len(errs) != 0 {
		t.Fatal(errs)
	}
	report := buildAnalyseReport(a, map[string]bool{file: true}, hostCompilerTarget())
	var output bytes.Buffer
	report.write(&output)
	names := map[sema.CallableID]string{}
	for _, node := range a.CallGraph().Nodes() {
		names[node.ID] = analyseCallableName(node)
	}
	for _, fact := range a.AllocationFacts() {
		want := names[fact.Callable] + ": allocation " + string(fact.Knowledge)
		if !strings.Contains(output.String(), want) {
			t.Fatal(want, output.String())
		}
	}
	for _, want := range []string{"allocation path:", "unknown allocation path:", "foreign body has no trusted allocation-free contract", "spawn control storage", "results: 0 errors"} {
		if !strings.Contains(output.String(), want) {
			t.Fatal(want, output.String())
		}
	}
	if report.counts[analyseClassError] != 0 {
		t.Fatal(report.counts)
	}
	selected := buildAnalyseReport(a, map[string]bool{"different.sec": true}, hostCompilerTarget())
	var other bytes.Buffer
	selected.write(&other)
	if strings.Contains(other.String(), ": allocation ") {
		t.Fatal("source selection ignored", other.String())
	}
}
