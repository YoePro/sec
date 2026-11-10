package sema

import (
	"os"
	"reflect"
	"sec/internal/lexer"
	"sec/internal/parser"
	"testing"
)

// TestForBindingModeFacts exposes checked modes as detached per-snapshot facts;
// invalid reference binding headers never become positive tooling evidence.
// Rules: rules/control-flow/flowcontrol_for.md — §§4–10;
// rules/compiler/compiler_analysis.md — §18.
func TestForBindingModeFacts(t *testing.T) {
	data, err := os.ReadFile("../../testdata/formatter/for_binding_headers.sec")
	if err != nil {
		t.Fatal(err)
	}
	a, errors := analyzeSourceWithAnalyzerRaw(t, string(data))
	if len(errors) != 0 {
		t.Fatal(errors)
	}
	facts := a.ForBindingModes()
	modes := []string{}
	types := []string{}
	for _, fact := range facts {
		modes = append(modes, fact.Mode)
		types = append(types, fact.Type)
	}
	if !reflect.DeepEqual(modes, []string{"value", "ref", "value", "ref mut", "discard", "value", "ref"}) || !reflect.DeepEqual(types, []string{"int", "ref int", "int", "ref mut int", "int", "int", "ref int"}) {
		t.Fatal(facts)
	}
	facts[0].Mode = "changed"
	if a.ForBindingModes()[0].Mode != "value" {
		t.Fatal("mutable snapshot")
	}
	data, err = os.ReadFile("../../testdata/tooling/for_binding_empty.sec")
	if err != nil {
		t.Fatal(err)
	}
	p := parser.New(lexer.New(string(data)))
	program := p.ParseProgram()
	if errors := a.Analyze(program); len(errors) != 0 {
		t.Fatal(errors)
	}
	if len(a.ForBindingModes()) != 0 || len((*Analyzer)(nil).ForBindingModes()) != 0 {
		t.Fatal("stale facts")
	}
	data, err = os.ReadFile("../../testdata/sema/for_binding_tooling_invalid.sec")
	if err != nil {
		t.Fatal(err)
	}
	invalid, errors := analyzeSourceWithAnalyzerRaw(t, string(data))
	if len(errors) == 0 || len(invalid.ForBindingModes()) != 0 {
		t.Fatal("invalid header advertised", errors, invalid.ForBindingModes())
	}

}
