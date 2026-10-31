package semantic

import (
	"strings"
	"testing"
)

// Trivial owned pack positions are ordinary by-value parameters, with no
// reference grant or runtime container; their concrete destruction is a no-op.
// Rules: rules/declarations/functions.md §38A(3),(6),(9);
// rules/compiler/monomorphization.md §11.2,11.4; rules/compiler/semantic_ir.md §27A.
func TestTypePackTrivialOwnedSpecializations(t *testing.T) {
	for _, name := range []string{"owned_scalar.sec", "static_owned_boundary.sec", "owned_trivial.sec"} {
		m, err := typePackFixtureModule(t, name)
		if err != nil {
			t.Fatal(name, err)
		}
		if err := Verify(m); err != nil {
			t.Fatal(name, err)
		}
		specs := specializationIDs(m)
		if len(specs) == 0 {
			t.Fatal(name, "no specializations")
		}
		for _, fn := range specs {
			for i, p := range fn.TypePack.Positions {
				parameter := fn.Parameters[fn.TypePack.FixedCount+i]
				if p.Mode != TypePackOwned || parameter.Value.Type != fn.TypePack.Types[i] || parameter.Value.Ownership != OwnershipImmediate || parameter.Reference != nil {
					t.Fatalf("%s: %#v %#v", name, p, parameter)
				}
			}
		}
		if name == "owned_trivial.sec" {
			for _, id := range []FunctionID{"main::Count[...Types<>]()", "main::Count[...Types<int,bool>](int,bool)", "main::Count[...Types<bool,int>](bool,int)", "main::Tagged[...Types<int,bool>](uint,int,bool)", "main::Counter.Count[...Types<int,bool>](int,bool)"} {
				if specs[id] == nil {
					t.Fatalf("missing %s in %v", id, specs)
				}
			}
		}
	}
}

// The owned mode cannot hide reference authority or a mismatching element type.
// Rules: rules/compiler/semantic_ir.md §27A(2).
func TestTypePackTrivialOwnedFactsVerified(t *testing.T) {
	for _, mutate := range []func(*Function){
		func(f *Function) { f.TypePack.Positions[0].Mode = TypePackShared },
		func(f *Function) { f.Parameters[0].Value.Ownership = OwnershipSharedReference },
		func(f *Function) { f.TypePack.Types[0] = f.TypePack.Types[1] },
	} {
		m, err := typePackFixtureModule(t, "owned_scalar.sec")
		if err != nil {
			t.Fatal(err)
		}
		fn := specializationIDs(m)["main::Count[...Types<int,bool>](int,bool)"]
		if fn == nil {
			t.Fatal(specializationIDs(m))
		}
		mutate(fn)
		if err := Verify(m); err == nil {
			t.Fatal("accepted invalid owned pack facts")
		}
	}
	m, err := typePackFixtureModule(t, "owned_cleanup_boundary.sec")
	if m != nil || err == nil || !strings.Contains(err.Error(), "ownership-sensitive owned type-pack position") {
		t.Fatal(m, err)
	}
}
