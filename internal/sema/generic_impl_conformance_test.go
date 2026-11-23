package sema

import (
	"os"
	"strings"
	"testing"

	"sec/internal/diagnostics"
	"sec/internal/lexer"
	"sec/internal/parser"
)

// TestGenericImplConformance validates the complete explicit contract in its
// owning generic scope, then requires that conformance at a concrete call.
// Rules: rules/declarations/generics.md — §§8,14,25,30;
// rules/declarations/interfaces.md — §§4–6,9.
func TestGenericImplConformance(t *testing.T) {
	data, err := os.ReadFile("../../testdata/sema/generic_impl_conformance/valid.sec")
	if err != nil {
		t.Fatal(err)
	}
	p := parser.New(lexer.NewWithFile(string(data), "generic-conformance.sec"))
	program := p.ParseProgram()
	if len(p.Errors()) != 0 {
		t.Fatal(p.Errors())
	}
	a := NewAnalyzer()
	if errors := a.Analyze(program); len(errors) != 0 {
		t.Fatal(errors)
	}
	for _, value := range []string{"int", "bool"} {
		instance := a.types["Box"]
		instance.TypeArgs = []Type{a.types[value]}
		instance = a.instantiateGenericType(instance)
		if typeDisplayName(instance.Properties[0].Type) != "ref "+value || typeDisplayName(instance.Events[0].Payload) != value || typeDisplayName(instance.Implements[0].InterfaceEvents[0].Payload) != value {
			t.Fatalf("incomplete %s member substitution: %+v", value, instance)
		}
	}
	if typeDisplayName(a.types["Box"].Events[0].Payload) != "T" {
		t.Fatal("mutated template")
	}
	facts := a.InterfaceConformances()
	if len(facts) != 1 || facts[0].State != diagnostics.ProofValid || len(facts[0].Members) != 4 {
		t.Fatalf("conformance facts: %+v", facts)
	}
}

// TestGenericImplSetterConformance keeps a generic error parameter in the
// declaration's scope and substitutes both the value and error contracts.
// Rules: rules/declarations/generics.md — §§8,14,15A,22,25;
// rules/errors/errorhandling.md — §24.1.
func TestGenericImplSetterConformance(t *testing.T) {
	data, err := os.ReadFile("../../testdata/sema/generic_impl_conformance/setters.sec")
	if err != nil {
		t.Fatal(err)
	}
	p := parser.New(lexer.NewWithFile(string(data), "generic-setters.sec"))
	program := p.ParseProgram()
	if len(p.Errors()) != 0 {
		t.Fatal(p.Errors())
	}
	a := NewAnalyzer()
	if errors := a.Analyze(program); len(errors) != 0 {
		t.Fatal(errors)
	}
	for _, item := range []struct{ owner, value, errorName string }{
		{"Editable", "int", "WriteError"}, {"Inherited", "bool", "SecondError"},
	} {
		instance := a.types[item.owner]
		if item.owner == "Editable" {
			instance.TypeArgs = []Type{a.types[item.value], a.types[item.errorName]}
		} else {
			instance.TypeArgs = []Type{a.types[item.errorName], a.types[item.value]}
		}
		instance = a.instantiateGenericType(instance)
		property := instance.Properties[0]
		required := instance.Implements[0].InterfaceProperties[0]
		contract := a.interfaceSetterErrorContract(instance.Implements[0], required)
		if contract == nil || typeDisplayName(*contract) != item.errorName || property.Error == nil || typeDisplayName(*property.Error) != item.errorName || typeDisplayName(property.Type) != item.value {
			t.Fatalf("%s: property=%+v contract=%+v", item.owner, property, contract)
		}
	}
	empty := parser.New(lexer.NewWithFile("module main", "empty.sec")).ParseProgram()
	a.Analyze(empty)
	if len(a.interfaceSetterErrorRefs) != 0 || len(a.interfaceSetterErrors) != 0 {
		t.Fatal("stale setter scope")
	}
}

// TestGenericImplInvalidConformance rejects incompatible and missing members,
// including an unrelated setter error and structural-only conformance.
// Rules: rules/declarations/generics.md — §§8,14,15A,25;
// rules/declarations/interfaces.md — §§4–6,9,12.
func TestGenericImplInvalidConformance(t *testing.T) {
	data, err := os.ReadFile("../../testdata/sema/generic_impl_conformance/contract_invalid.sec")
	if err != nil {
		t.Fatal(err)
	}
	p := parser.New(lexer.NewWithFile(string(data), "contract_invalid.sec"))
	program := p.ParseProgram()
	if len(p.Errors()) != 0 {
		t.Fatal(p.Errors())
	}
	a := NewAnalyzer()
	errors := a.Analyze(program)
	if len(errors) != 10 {
		t.Fatalf("errors: %+v", errors)
	}
	duckRejected := false
	for _, diagnostic := range errors {
		duckRejected = duckRejected || strings.Contains(diagnostic.Message, "type Duck[int] does not satisfy constraint Contract[int] for X")
	}
	if !duckRejected {
		t.Fatal("structural-only conformance accepted")
	}
	counts := map[string]int{}
	for _, fact := range a.InterfaceConformances() {
		if fact.State != diagnostics.ProofInvalid {
			t.Fatalf("invalid proof: %+v", fact)
		}
		counts[fact.TypeName] = len(fact.Failures)
		for _, failure := range fact.Failures {
			if failure.Diagnostic.PreviousLine == 0 {
				t.Fatalf("missing requirement location: %+v", failure)
			}
		}
	}
	if counts["Broken"] != 4 || counts["Missing"] != 4 || counts["WrongSetter"] != 1 || len(counts) != 3 {
		t.Fatal(counts)
	}
}
