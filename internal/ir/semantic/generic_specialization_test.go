package semantic

import (
	"errors"
	"os"
	"strings"
	"testing"
)

func specializationIDs(module *Module) map[FunctionID]*Function {
	out := map[FunctionID]*Function{}
	for _, function := range module.Functions {
		if function.Specialization != nil || function.TypePack != nil {
			out[function.ID] = function
		}
	}
	return out
}

// Each selected call of a generic free function demands one concrete
// specialization identified by its ordered arguments. Calls inside a
// specialization forward their arguments through the enclosing substitution
// and reuse existing specializations; a scalar prefix combines with a type pack.
// Rules: rules/compiler/monomorphization.md §§ 4, 6.1–6.2, 8.3, 9.1, 13.3;
// rules/compiler/semantic_ir.md § 27A.
func TestGenericFreeFunctionsSpecialize(t *testing.T) {
	data, err := os.ReadFile("../../../testdata/generics/specialization/free_functions.sec")
	if err != nil {
		t.Fatal(err)
	}
	// Package 17 lowers the explicit `<-` moves the templates need (§ 11.3).
	module, err := analyzedModule(t, string(data), 17)
	if err != nil {
		t.Fatal(err)
	}
	if err := Verify(module); err != nil {
		t.Fatal(err)
	}
	got := specializationIDs(module)
	for _, want := range []FunctionID{
		"main::Id[T=int](int)",
		"main::Id[T=bool](bool)",
		"main::Pick[T=int](int,int,bool)",
		"main::Pick[T=bool](bool,bool,bool)",
		"main::Twice[T=int](int)",
		"main::Pair[A=bool,B=int](bool,int)",
		"main::Head[T=int,...Types<bool,int>](int,ref bool,ref int)",
	} {
		if got[want] == nil {
			t.Fatalf("missing %s in %v", want, got)
		}
	}
	if len(got) != 7 {
		t.Fatalf("specializations = %v", got)
	}
	pair := got["main::Pair[A=bool,B=int](bool,int)"].Specialization
	if pair.Template != "Pair" || len(pair.Arguments) != 2 || pair.Arguments[0].Name != "A" || pair.Arguments[1].Name != "B" {
		t.Fatalf("Pair facts = %#v", pair)
	}
	head := got["main::Head[T=int,...Types<bool,int>](int,ref bool,ref int)"]
	if head.Specialization == nil || head.TypePack == nil || head.TypePack.FixedCount != 1 || len(head.TypePack.Types) != 2 {
		t.Fatalf("Head facts = %#v %#v", head.Specialization, head.TypePack)
	}
	// Twice[int] forwards T and reuses Id[T=int] instead of a new function.
	twice := Format(&Module{Types: module.Types, Functions: []*Function{got["main::Twice[T=int](int)"]}})
	if strings.Count(twice, `call.direct "main::Id[T=int](int)"`) != 2 {
		t.Fatal(twice)
	}
}

// Generic structs, Option payloads and recursion specialize with concrete
// ownership: a template's conservative plan for an unknown parameter becomes
// a trivial copy for a trivially copyable argument (monomorphization.md § 11.2).
func TestGenericSpecializationConcreteTypes(t *testing.T) {
	for _, test := range []struct {
		name, source string
		want         []FunctionID
	}{
		{"struct", "module main\ntype Box[T] struct { Value: T }\nfn Make[T](value: T) Box[T] { return Box[T] { Value: value } }\nfn Get[T](box: Box[T]) T { return box.Value }\nfn Use() int { return Get(Make(3)) }\n",
			[]FunctionID{"main::Make[T=int](int)", "main::Get[T=int](main::Box<int>)"}},
		{"option", "module main\nfn Wrap[T](value: T) Option[T] { return Some(value) }\nfn Use() int {\n  match Wrap(3) {\n    Some(v) => return v\n    None => return 0\n  }\n}\n",
			[]FunctionID{"main::Wrap[T=int](int)"}},
		{"recursion", "module main\nfn Down[T](value: T, n: int) T {\n  if n <= 0 { return value }\n  return Down(<-value, n - 1)\n}\nfn Use() int { return Down(1, 3) }\n",
			[]FunctionID{"main::Down[T=int](int,int)"}},
	} {
		t.Run(test.name, func(t *testing.T) {
			module, err := analyzedModule(t, test.source, 17)
			if err != nil {
				t.Fatal(err)
			}
			if err := Verify(module); err != nil {
				t.Fatal(err)
			}
			got := specializationIDs(module)
			if len(got) != len(test.want) {
				t.Fatalf("specializations = %v", got)
			}
			for _, want := range test.want {
				if got[want] == nil {
					t.Fatalf("missing %s in %v", want, got)
				}
			}
		})
	}
}

// Methods of a generic implementation specialize per receiver instance: the
// receiver reference targets the concrete instance and its field types are
// substituted, while each instance's specialization stays distinct.
// Rules: rules/compiler/monomorphization.md § 9.3; package15-todo.md P15-47.
func TestGenericImplMethodsSpecialize(t *testing.T) {
	data, err := os.ReadFile("../../../testdata/generics/specialization/impl_methods.sec")
	if err != nil {
		t.Fatal(err)
	}
	module, err := analyzedModule(t, string(data), 15)
	if err != nil {
		t.Fatal(err)
	}
	if err := Verify(module); err != nil {
		t.Fatal(err)
	}
	got := specializationIDs(module)
	for id, receiver := range map[FunctionID]OwnershipClass{
		"main::Box.Get[T=int]()":  OwnershipSharedReference,
		"main::Box.Get[T=bool]()": OwnershipSharedReference,
		"main::Box.Bump[T=int]()": OwnershipMutableReference,
	} {
		function := got[id]
		if function == nil || len(function.Parameters) != 1 || function.Parameters[0].Name != "self" || function.Parameters[0].Value.Ownership != receiver {
			t.Fatalf("%s = %#v in %v", id, function, got)
		}
		self, _ := module.Types.Lookup(function.Parameters[0].Value.Type)
		referent, _ := module.Types.Lookup(self.Element)
		if referent.Kind != TypeStruct || referent.Name != "Box" || len(referent.TypeArgs) != 1 {
			t.Fatalf("%s receiver referent = %#v", id, referent)
		}
	}
	if len(got) != 3 {
		t.Fatalf("specializations = %v", got)
	}
}

// A template alone builds nothing, and an argument whose ownership the
// template plans cannot express stays a named boundary rather than reusing
// plans computed for an unknown parameter.
func TestGenericSpecializationBoundaries(t *testing.T) {
	module, err := analyzedModule(t, "module main\nfn Id[T](value: T) T { return value }\n", 17)
	if err != nil || len(module.Functions) != 0 {
		t.Fatal(err, module)
	}
	_, err = analyzedModule(t, `module main
type Handle struct { Code: int }
impl Handle {
    free {
    }
}
fn Keep[T](value: T) T {
    return value
}
fn Use() int {
    let handle := Handle { Code: 1 }
    let kept := Keep(<-handle)
    return 0
}
`, 17)
	var unsupported *UnsupportedFeatureError
	if !errors.As(err, &unsupported) || !strings.Contains(unsupported.Feature, "ownership-sensitive argument T=main::Handle") {
		t.Fatalf("err = %v", err)
	}
}

// Specialization facts must name their template and each argument once.
func TestSpecializationFactsAreVerified(t *testing.T) {
	for name, mutate := range map[string]func(*SpecializationFacts){
		"no template":        func(facts *SpecializationFacts) { facts.Template = "" },
		"no arguments":       func(facts *SpecializationFacts) { facts.Arguments = nil },
		"duplicate argument": func(facts *SpecializationFacts) { facts.Arguments = append(facts.Arguments, facts.Arguments[0]) },
		"unnamed argument":   func(facts *SpecializationFacts) { facts.Arguments[0].Name = "" },
		"invalid type":       func(facts *SpecializationFacts) { facts.Arguments[0].Type = 9999 },
	} {
		t.Run(name, func(t *testing.T) {
			module, err := analyzedModule(t, "module main\nfn Id[T](value: T) T { return value }\nfn Use() int { return Id(1) }\n", 17)
			if err != nil {
				t.Fatal(err)
			}
			for _, function := range module.Functions {
				if function.Specialization != nil {
					mutate(function.Specialization)
				}
			}
			if err := Verify(module); err == nil || !strings.Contains(err.Error(), "specialization") {
				t.Fatalf("err = %v", err)
			}
		})
	}
}
