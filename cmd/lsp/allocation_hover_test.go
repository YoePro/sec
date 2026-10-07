package main

import (
	"os"
	"sec/internal/lexer"
	"sec/internal/parser"
	"sec/internal/sema"
	"strings"
	"testing"
)

// TestAllocationHoverCanonicalParity exercises real declaration hover against
// compiler facts, including mixed evidence and spawn's synchronous boundary.
// Rules: rules/memory/allocation.md — §§24,29(1)-(2),30; rules/tooling/lsp.md — "Hover".
func TestAllocationHoverCanonicalParity(t *testing.T) {
	file := "../../testdata/sema/allocation_facts_valid.sec"
	data, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	source := string(data)
	p := parser.New(lexer.NewWithFile(source, file))
	program := p.ParseProgram()
	if len(p.Errors()) != 0 {
		t.Fatal(p.Errors())
	}
	a := sema.NewAnalyzer()
	if errs := a.Analyze(program); len(errs) != 0 {
		t.Fatal(errs)
	}
	for _, fact := range a.AllocationFacts() {
		node, _ := a.CallGraph().Node(fact.Callable)
		if node.Name == "lambda" {
			continue
		}
		offset := strings.Index(source, "fn "+node.Name+"(") + 3
		hover, ok := hoverForSource("file:///tmp/sec-allocation-facts/main.sec", source, offsetPosition(source, offset))
		want := "Allocation behavior: `" + string(fact.Knowledge) + "`"
		if !ok || !strings.Contains(hover.Contents.Value, want) {
			t.Fatalf("%s: wanted %s, got %+v", node.Name, want, hover)
		}
		if fact.HasUnknown && !strings.Contains(hover.Contents.Value, "Unresolved allocation behavior: `yes`") {
			t.Fatal(node.Name, hover)
		}
		if node.Name == "Mixed" && !strings.Contains(hover.Contents.Value, "May allocate: `yes`") {
			t.Fatal(hover)
		}
		if node.Name == "Spawner" && strings.Contains(hover.Contents.Value, "May allocate: `yes`") {
			t.Fatal("spawn execution leaked", hover)
		}
	}
}
