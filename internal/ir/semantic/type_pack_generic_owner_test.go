package semantic

import "testing"

// A static method of a generic owner called with explicit owner arguments,
// Box[int].Make(...), fixes the owner's arguments before the method's own
// parameters; for a pack method the ordered pack joins the specialization
// identity, for shared and owned packs and the empty pack alike.
// Rules: rules/compiler/monomorphization.md — §§ 8.2, 9.3; rules/declarations/functions.md — § 38A(2),(4),(9);
// rules/compiler/semantic_ir.md — § 27A(2),(4).
func TestTypePackStaticMethodsOfExplicitGenericOwner(t *testing.T) {
	module, err := typePackFixtureModule(t, "generic_owner_explicit_args.sec")
	if err != nil {
		t.Fatal(err)
	}
	if err := Verify(module); err != nil {
		t.Fatal(err)
	}
	got := specializationIDs(module)
	for _, id := range []FunctionID{
		"main::Box.Make[T=int](int)",
		"main::Box.Count[T=int,...Types<int,bool>](ref int,ref bool)",
		"main::Box.Take[T=bool,...Types<int,uint>](int,uint)",
		"main::Box.Count[T=int,...Types<>]()",
	} {
		if got[id] == nil {
			t.Fatalf("missing %s in %v", id, got)
		}
	}
}
