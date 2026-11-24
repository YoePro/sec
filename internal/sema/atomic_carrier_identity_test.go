package sema

import (
	"os"
	"sec/internal/layout"
	"sec/internal/lexer"
	"sec/internal/parser"
	"testing"
)

// MD-095 T02–T14 preserve nominal identity and contracts while sharing only
// the resolved carrier, across named layers, enum backing and pointer pointees.
// Rules: rules/concurrency/atomics.md §37A(4–6); types/types.md — Type identity.
func TestAtomicCarrierIdentity(t *testing.T) {
	data, err := os.ReadFile("../../testdata/sema/atomics/carrier_identity.sec")
	if err != nil {
		t.Fatal(err)
	}
	for _, width := range []uint16{32, 64} {
		p := parser.New(lexer.New(string(data)))
		program := p.ParseProgram()
		if len(p.Errors()) != 0 {
			t.Fatal(p.Errors())
		}
		for pass := 0; pass < 2; pass++ {
			a := NewAnalyzerWithScalarPlan(layout.ResolvedScalarPlan{PointerWidthBits: width})
			if errors := a.Analyze(program); len(errors) != 0 {

				t.Fatal(errors)
			}
			facts := a.AtomicMemoryOperations()
			if len(facts) != 16 {
				t.Fatalf("got %d operations", len(facts))
			}
			carriers := []string{"uint64", "uint64", "uint64", "uint64", "uint16", "uint16", "uint64", "uint", "int", "bool", "RawPtr", "RawPtr", "uint64", "uint16", "uint64", "uint16"}
			for i, fact := range facts {
				got := fact.TypeIdentity
				want := canonicalTypeIdentity(a.types[carriers[i]])
				if carriers[i] == "RawPtr" {
					base := a.types["RawPtr"]
					base.GenericParameters = nil
					want = canonicalTypeIdentity(base)
				}
				if got.Symbolic || got.CarrierTypeID != want || got.SemanticTypeID == "" {
					t.Fatalf("op %d width %d: %+v want %s", i, width, got, want)
				}
			}
			for _, i := range []int{1, 2, 3, 6} {
				if facts[0].TypeIdentity.SemanticTypeID == facts[i].TypeIdentity.SemanticTypeID {
					t.Fatal("nominal identities collapsed")
				}
			}
			if facts[10].TypeIdentity.SemanticTypeID == facts[11].TypeIdentity.SemanticTypeID || facts[10].TypeIdentity.CarrierTypeID != facts[11].TypeIdentity.CarrierTypeID {
				t.Fatal("pointer pointee/carrier confusion")
			}
			if facts[7].TypeIdentity.CarrierTypeID == facts[10].TypeIdentity.CarrierTypeID || facts[9].TypeIdentity.CarrierTypeID == facts[7].TypeIdentity.CarrierTypeID {
				t.Fatal("same-sized carriers collapsed")
			}
			if facts[12].TypeIdentity.SemanticTypeID == facts[14].TypeIdentity.SemanticTypeID || facts[12].TypeIdentity.CarrierTypeID != facts[14].TypeIdentity.CarrierTypeID {
				t.Fatal("generic identities collapsed")
			}
			for _, name := range []string{"int", "uint"} {
				if !LayoutRequirementOf(a.types[name]).TargetSized {
					t.Fatal("lost plan-sized carrier")
				}
			}
			// Mutating a detached returned fact cannot change the analyzer snapshot.
			facts[0].TypeIdentity.CarrierTypeID = "corrupt"
			if a.AtomicMemoryOperations()[0].TypeIdentity.CarrierTypeID == "corrupt" {
				t.Fatal("mutable identity fact")
			}
		}
	}
}

// Unresolved generics never acquire a concrete carrier before substitution;
// the single type service is also safe against missing/cyclic bases.
// Rules: rules/concurrency/atomics.md §37A(4); declarations/generics.md §17.
func TestAtomicCarrierSymbolicAndSubstitution(t *testing.T) {
	parameter := Type{Name: "T", Kind: GenericType}
	base := builtinType("uint64")
	named := Type{Name: "Tagged", Module: "example", Kind: UintType, Named: true, NamedBase: &base, GenericParameters: []string{"T"}, TypeArgs: []Type{parameter}}
	symbolic := AtomicValueTypeIdentity(named)
	if !symbolic.Symbolic || symbolic.CarrierTypeID != "" {
		t.Fatal(symbolic)
	}
	concrete := substituteGenericType(named, map[string]Type{"T": builtinType("int")})
	resolved := AtomicValueTypeIdentity(concrete)
	if resolved.Symbolic || resolved.CarrierTypeID != canonicalTypeIdentity(base) || resolved.SemanticTypeID == symbolic.SemanticTypeID {
		t.Fatal(resolved)
	}
	pointer := Type{Name: "RawPtr", Kind: RawPtrType, TypeArgs: []Type{parameter}}
	if got := AtomicValueTypeIdentity(pointer); !got.Symbolic || got.CarrierTypeID != "" {
		t.Fatal(got)
	}
	missing := Type{Name: "Missing", Kind: UintType, Named: true}
	if got := AtomicValueTypeIdentity(missing); got.CarrierTypeID != "" {
		t.Fatal(got)
	}
	cyclic := missing
	cyclic.NamedBase = &cyclic
	if got := AtomicValueTypeIdentity(cyclic); got.CarrierTypeID != "" {
		t.Fatal(got)
	}
	if got := AtomicValueTypeIdentity(builtinType("float64")); got.CarrierTypeID != "" {
		t.Fatal(got)
	}
}

// The canonical service retains module and instantiated enum identities and
// never treats a rejected source operation as a resolved carrier request.
// Rules: rules/concurrency/atomics.md §§37A(2–6),45.
func TestAtomicCarrierAuthorityAndInvalidCalls(t *testing.T) {
	base := builtinType("uint64")
	first := Type{Name: "Count", Module: "alpha", Kind: UintType, Named: true, NamedBase: &base}
	second := first
	second.Module = "beta"
	left, right := AtomicValueTypeIdentity(first), AtomicValueTypeIdentity(second)
	if left.SemanticTypeID == right.SemanticTypeID || left.CarrierTypeID != right.CarrierTypeID {
		t.Fatal(left, right)
	}
	// A fully resolved enum over multiple named backing layers uses no lookup.
	enum := Type{Name: "State", Module: "foreign", Kind: EnumType, Named: true, EnumCarrier: &first}
	named := Type{Name: "StateView", Module: "main", Kind: EnumType, Named: true, NamedBase: &enum}
	if got := AtomicValueTypeIdentity(named); got.CarrierTypeID != canonicalTypeIdentity(base) {
		t.Fatal(got)
	}
	parameter := Type{Name: "T", Kind: GenericType}
	template := Type{Name: "State", Module: "foreign", Kind: EnumType, Named: true, EnumCarrier: &parameter, TypeArgs: []Type{parameter}}
	specialized := substituteGenericType(template, map[string]Type{"T": base})
	if got := AtomicValueTypeIdentity(specialized); got.Symbolic || got.CarrierTypeID != canonicalTypeIdentity(base) || template.EnumCarrier.Kind != GenericType {
		t.Fatal(got)
	}
	data, err := os.ReadFile("../../testdata/sema/atomic_applicability_invalid.sec")
	if err != nil {
		t.Fatal(err)
	}
	p := parser.New(lexer.New(string(data)))
	program := p.ParseProgram()
	if len(p.Errors()) != 0 {
		t.Fatal(p.Errors())
	}
	a := NewAnalyzer()
	if errors := a.Analyze(program); len(errors) == 0 {
		t.Fatal("invalid atomic calls accepted")
	}
	if operations := a.AtomicMemoryOperations(); len(operations) != 0 {
		t.Fatalf("rejected operations got facts: %v", operations)
	}
}
