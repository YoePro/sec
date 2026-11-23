package sema

import (
	"os"
	"reflect"
	"testing"

	"sec/internal/lexer"
	"sec/internal/parser"
)

// TestGenericToolingBindings verifies detached, declaration-ordered facts and
// ensures inspection creates no new semantic specialization demand or cache.
// Rules: rules/declarations/generics.md — §§18–20,25–26;
// rules/tooling/lsp.md — §18(2).
func TestGenericToolingBindings(t *testing.T) {
	data, err := os.ReadFile("../../testdata/lsp/generic_instantiation_hover/main.sec")
	if err != nil {
		t.Fatal(err)
	}
	p := parser.New(lexer.NewWithFile(string(data), "generic-hover.sec"))
	program := p.ParseProgram()
	if len(p.Errors()) != 0 {
		t.Fatal(p.Errors())
	}
	a := NewAnalyzer()
	if errs := a.Analyze(program); len(errs) != 0 {
		t.Fatal(errs)
	}
	functions, types := len(a.genericFuncInstances), len(a.genericTypeInstances)
	demands, err := a.InstantiationDemands()
	if err != nil {
		t.Fatal(err)
	}
	checked := 0
	for call := range a.resolvedCalls {
		before, ok := a.GenericCallTypeArgumentsOf(call)
		if !ok {
			continue
		}
		if before[0].Name == "Z" && len(before) == 2 && before[1].Name != "A" {
			t.Fatal("lost declaration order", before)
		}
		before[0].Name = "corrupted"
		before[0].Type.Name = "corrupted"
		after, ok := a.GenericCallTypeArgumentsOf(call)
		if !ok || after[0].Name == "corrupted" || after[0].Type.Name == "corrupted" {
			t.Fatal("query exposed mutable facts", after)
		}
		checked++
	}
	if checked < 6 {
		t.Fatal("missing selected call facts", checked)
	}
	checkedTypes := 0
	for _, fact := range a.resolvedSourceTypeUses {
		if fact.ambiguous {
			continue
		}
		arguments, ok := a.GenericTypeArgumentsOf(fact.typ)
		if !ok {
			continue
		}
		arguments[0].Name = "corrupted"
		arguments[0].Type.Name = "corrupted"
		after, ok := a.GenericTypeArgumentsOf(fact.typ)
		if !ok || after[0].Name == "corrupted" || after[0].Type.Name == "corrupted" {
			t.Fatal("type query exposed mutable facts", after)
		}
		checkedTypes++
	}
	if checkedTypes < 5 {
		t.Fatal("missing concrete type-use facts", checkedTypes)
	}
	afterDemands, err := a.InstantiationDemands()
	if err != nil || functions != len(a.genericFuncInstances) || types != len(a.genericTypeInstances) || !reflect.DeepEqual(demands, afterDemands) {
		t.Fatal("tooling query created specialization demand")
	}
}
