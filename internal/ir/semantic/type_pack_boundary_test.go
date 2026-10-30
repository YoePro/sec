package semantic

import (
	"errors"
	"os"
	"strings"
	"testing"
)

func typePackFixtureModule(t *testing.T, name string) (*Module, error) {
	t.Helper()
	data, err := os.ReadFile("../../../testdata/generics/type_pack_declarations/" + name)
	if err != nil {
		t.Fatal(err)
	}
	return analyzedModule(t, string(data), 15)
}

// Each selected call demands one concrete specialization: order and arity
// select distinct functions, the same ordered sequence shares one, Len is the
// exact length per specialization and a template alone builds nothing.
// Rules: rules/compiler/semantic_ir.md § 27A; rules/declarations/functions.md § 38A(2),(5),(9);
// rules/compiler/monomorphization.md § 9.1.
func TestTypePackCallsBuildConcreteSpecializations(t *testing.T) {
	module, err := typePackFixtureModule(t, "shared_specialization.sec")
	if err != nil {
		t.Fatal(err)
	}
	if err := Verify(module); err != nil {
		t.Fatal(err)
	}
	specializations := map[FunctionID]*Function{}
	for _, function := range module.Functions {
		if function.TypePack != nil {
			specializations[function.ID] = function
		}
	}
	for _, test := range []struct {
		id            FunctionID
		length, fixed int
		parameters    []string
	}{
		{"main::Count[...Types<>]()", 0, 0, nil},
		{"main::Count[...Types<int,bool>](ref int,ref bool)", 2, 0, []string{"values[0]", "values[1]"}},
		{"main::Count[...Types<bool,int>](ref bool,ref int)", 2, 0, []string{"values[0]", "values[1]"}},
		{"main::Unused[...Types<int>](ref int)", 1, 0, []string{"values[0]"}},
		{"main::Tagged[...Types<int,int,int>](uint,ref int,ref int,ref int)", 3, 1, []string{"base", "values[0]", "values[1]", "values[2]"}},
		{"main::Nested[...Types<int>](ref int)", 1, 0, []string{"values[0]"}},
	} {
		function := specializations[test.id]
		if function == nil {
			t.Fatalf("missing %s in %v", test.id, specializations)
		}
		if len(function.TypePack.Types) != test.length || function.TypePack.FixedCount != test.fixed || function.TypePack.Parameter != "Types" {
			t.Fatalf("%s facts = %#v", test.id, function.TypePack)
		}
		if len(function.Parameters) != len(test.parameters) {
			t.Fatalf("%s parameters = %#v", test.id, function.Parameters)
		}
		for index, name := range test.parameters {
			if function.Parameters[index].Name != name {
				t.Fatalf("%s parameter %d = %s", test.id, index, function.Parameters[index].Name)
			}
		}
	}
	if len(specializations) != 6 {
		t.Fatalf("specializations = %v", specializations)
	}
	lengths := map[FunctionID]string{
		"main::Count[...Types<>]()":                                         "const.int 0",
		"main::Count[...Types<int,bool>](ref int,ref bool)":                 "const.int 2",
		"main::Tagged[...Types<int,int,int>](uint,ref int,ref int,ref int)": "const.int 3",
	}
	for id, want := range lengths {
		text := Format(&Module{Types: module.Types, Functions: []*Function{specializations[id]}})
		if !strings.Contains(text, want) {
			t.Fatalf("%s lacks %s:\n%s", id, want, text)
		}
	}
	calls := 0
	for _, function := range module.Functions {
		if function.Name != "Use" {
			continue
		}
		for _, block := range function.Blocks {
			for _, operation := range block.Operations {
				if operation.Callee == "main::Count[...Types<int,bool>](ref int,ref bool)" {
					calls++
				}
			}
		}
	}
	if calls != 2 {
		t.Fatalf("calls sharing the ordered sequence = %d", calls)
	}
}

// The § 27A facts must agree with the concrete signature.
func TestTypePackFactsAreVerified(t *testing.T) {
	for name, mutate := range map[string]func(*TypePackFacts){
		"swapped element": func(facts *TypePackFacts) { facts.Types[0], facts.Types[1] = facts.Types[1], facts.Types[0] },
		"short sequence":  func(facts *TypePackFacts) { facts.Types = facts.Types[:1] },
		"short positions": func(facts *TypePackFacts) { facts.Positions = facts.Positions[:1] },
		"fixed count":     func(facts *TypePackFacts) { facts.FixedCount = 1 },
		"unnamed":         func(facts *TypePackFacts) { facts.Parameter = "" },
		"mode":            func(facts *TypePackFacts) { facts.Positions[0].Mode = "owned" },
	} {
		t.Run(name, func(t *testing.T) {
			module, err := typePackFixtureModule(t, "shared_specialization.sec")
			if err != nil {
				t.Fatal(err)
			}
			mutated := false
			for _, function := range module.Functions {
				if function.ID == "main::Count[...Types<int,bool>](ref int,ref bool)" {
					mutate(function.TypePack)
					mutated = true
				}
			}
			if !mutated {
				t.Fatal("specialization not found")
			}
			if err := Verify(module); err == nil || !strings.Contains(err.Error(), "type-pack") {
				t.Fatalf("err = %v", err)
			}
		})
	}
}

// A template without a call builds nothing, and forms whose body needs
// monomorphization that Semantic IR lacks return no partial module.
func TestTypePackBoundariesFailClosed(t *testing.T) {
	module, err := analyzedModule(t, "module main\nfn Count[...Types](values: ...ref Types) uint { return values.Len }\n", 15)
	if err != nil || len(module.Functions) != 0 {
		t.Fatal(err, module)
	}
	built, err := typePackFixtureModule(t, "shared_scalar_prefix.sec")
	var unsupported *UnsupportedFeatureError
	if built != nil || !errors.As(err, &unsupported) || !strings.Contains(unsupported.Feature, "type-pack callable with a scalar generic prefix") {
		t.Fatal(built, err)
	}
}
