package sema

import (
	"os"
	"reflect"
	"strings"
	"testing"

	"sec/internal/lexer"
	"sec/internal/parser"
)

// TestIteratorResolutionsSnapshot checks canonical target identities, generic
// constraints, deterministic publication, detached data and snapshot replacement.
// Rules: rules/compiler/semantic_ir.md — §109(1–4),§115(2);
// rules/control-flow/flowcontrol_for.md — §37.
func TestIteratorResolutionsSnapshot(t *testing.T) {
	a := NewAnalyzer()
	load := func(name string, valid bool) {
		data, err := os.ReadFile("../../testdata/tooling/" + name)
		if err != nil {
			t.Fatal(err)
		}
		p := parser.New(lexer.NewWithFile(string(data), name))
		result := p.Parse()
		if result.HasErrors {
			t.Fatal(p.Errors())
		}
		errors := a.Analyze(result.Program)
		if (len(errors) == 0) != valid {
			t.Fatalf("valid=%t: %v", valid, errors)
		}
	}
	load("iterator_resolution.sec", true)
	facts := a.IteratorResolutions()
	if len(facts) != 3 {
		t.Fatalf("facts: %+v", facts)
	}
	for _, fact := range facts[:2] {
		node, ok := a.CallGraph().Node(fact.NextCallable)
		if !ok || node.Name != fact.NextTarget || node.Declaration != fact.NextDeclaration {
			t.Fatalf("canonical target mismatch: %+v / %+v", fact, node)
		}
	}
	if facts[0].Conformance != "Iterator[int]" || facts[0].NextTarget != "Counter.Next" || facts[1].Conformance != "Iterator[string]" || facts[1].NextTarget != "Words.Next" {
		t.Fatalf("facts: %+v", facts)
	}
	if !facts[2].GenericTemplate || facts[2].NextCallable != "" || facts[2].NextTarget != "" || !strings.Contains(facts[2].Display(), "resolved per specialization") {
		t.Fatalf("invented generic target: %+v", facts[2])
	}
	for i := 0; i < 25; i++ {
		if !reflect.DeepEqual(facts, a.IteratorResolutions()) {
			t.Fatal("nondeterministic facts")
		}
	}
	facts[0].NextTarget = "changed"
	if a.IteratorResolutions()[0].NextTarget != "Counter.Next" {
		t.Fatal("published facts alias analyzer")
	}
	load("iterator_resolution_invalid.sec", false)
	if got := a.IteratorResolutions(); len(got) != 0 {
		t.Fatalf("invalid or stale resolution: %+v", got)
	}
	if got := (*Analyzer)(nil).IteratorResolutions(); len(got) != 0 {
		t.Fatal(got)
	}
}
