package sema

import (
	"os"
	"strings"
	"testing"

	"sec/internal/lexer"
	"sec/internal/parser"
)

// Source-local facts preserve path sensitivity, alias origins and invalidation
// without treating a raw address as a spatial or lifetime proof.
// Rules: rules/memory/raw_pointers.md §§7,23–24,32,35(4–6).
func TestRawPointerToolingFacts(t *testing.T) {
	data, err := os.ReadFile("../../testdata/tooling/raw_pointer_facts.sec")
	if err != nil {
		t.Fatal(err)
	}
	p := parser.New(lexer.NewWithFile(string(data), "facts.sec"))
	program := p.ParseProgram()
	if len(p.Errors()) != 0 {
		t.Fatal(p.Errors())
	}
	a := NewAnalyzer()
	assertSemaErrors(t, a.Analyze(program), nil)
	facts := a.RawPointerFacts()
	for _, tc := range []struct {
		use     string
		null    bool
		origin  string
		invalid bool
	}{
		{"let empty:", true, "", false},
		{"let known := copied", true, "", false},
		{"let branch := input", true, "", false},
		{"let outside := input", false, "", false},
		{"let unknown := mutable", false, "", false},
		{"let before := alias", false, "session", false},
		{"let invalidated := alias", false, "session", true},
	} {
		prefix := string(data)[:strings.Index(string(data), tc.use)]
		line := strings.Count(prefix, "\n") + 1
		var found bool
		for _, fact := range facts {
			if fact.Source.Line != line {
				continue
			}
			found = true
			if fact.ProvenNull != tc.null || fact.Origin != tc.origin || (fact.InvalidatedAt.Line > 0) != tc.invalid {
				t.Fatalf("%s: %+v", tc.use, fact)
			}
			if !strings.Contains(fact.Display(), "Address space, range, alignment and lifetime: not established") {
				t.Fatal(fact.Display())
			}
			queried, ok := a.RawPointerFactAt(fact.Source.File, fact.Source.Line, fact.Source.Column)
			if !ok || queried != fact {
				t.Fatal("exact source query differs")
			}
		}
		if !found {
			t.Fatalf("missing facts at %s", tc.use)
		}
	}
	facts[0].Origin = "corrupted"
	if a.RawPointerFacts()[0].Origin == "corrupted" {
		t.Fatal("snapshot aliases mutable output")
	}
	empty := parser.New(lexer.NewWithFile("module empty\n", "empty.sec")).ParseProgram()
	a.Analyze(empty)
	if len(a.RawPointerFacts()) != 0 {
		t.Fatal("analyzer reuse retained stale facts")
	}
}
