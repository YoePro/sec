package sema

import (
	"os"
	"reflect"
	"sec/internal/lexer"
	"sec/internal/parser"
	"testing"
)

// Rules: rules/tooling/testing.md — §§6,27,28,39.1–39.2.
func TestCompilerOwnedTestSelection(t *testing.T) {
	file := "../../testdata/tooling/identity_selection_valid_test.sec"
	data, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	p := parser.New(lexer.NewWithFile(string(data), file))
	program := p.Parse()
	if program.HasErrors {
		t.Fatal(p.Errors())
	}
	a := NewAnalyzer()
	if errors := a.Analyze(program.Program); len(errors) != 0 {
		t.Fatal(errors)
	}
	for _, test := range []struct {
		pattern string
		count   int
	}{{"", 3}, {"^one", 2}, {`^one\.\*$`, 1}, {"absent", 0}} {
		selected, err := a.SelectTests(test.pattern)
		if err != nil || len(selected) != test.count {
			t.Fatal(test, selected, err)
		}
		for _, metadata := range selected {
			if metadata.Identity.Module != "identity_selection" || len(metadata.Identity.Path) != 1 || metadata.Identity.Path[0] != metadata.Name || metadata.Source.EndColumn <= metadata.Source.Column {
				t.Fatal(metadata)
			}
		}
		if !reflect.DeepEqual(selected, mustSelectTests(t, a, test.pattern)) {
			t.Fatal("selection not deterministic")
		}
		if len(selected) > 0 {
			selected[0].Identity.Path[0] = "changed"
			if mustSelectTests(t, a, test.pattern)[0].Name != mustSelectTests(t, a, test.pattern)[0].Identity.Path[0] {
				t.Fatal("identity aliases caller")
			}
		}
	}
	if _, err := a.SelectTests("["); err == nil {
		t.Fatal("invalid pattern accepted")
	}
}

func mustSelectTests(t *testing.T, a *Analyzer, pattern string) []ResolvedTestMetadata {
	t.Helper()
	result, err := a.SelectTests(pattern)
	if err != nil {
		t.Fatal(err)
	}
	return result
}
