package semantic

import (
	"os"
	"strings"
	"testing"
)

// Pack methods preserve the ordinary receiver loan, followed by fixed source
// parameters and concrete pack positions; Len excludes self and fixed arguments.
// Rules: rules/declarations/functions.md §§22,38A(3–6),(9);
// rules/compiler/semantic_ir.md §§27A,60(3),61(4).
func TestTypePackInstanceMethodsSpecialize(t *testing.T) {
	module, err := typePackFixtureModule(t, "instance_methods.sec")
	if err != nil {
		t.Fatal(err)
	}
	if err := Verify(module); err != nil {
		t.Fatal(err)
	}
	got := specializationIDs(module)
	wants := map[FunctionID]struct {
		length, fixed int
		authority     OwnershipClass
	}{
		"main::Counter.Count[...Types<>]()":                         {0, 1, OwnershipSharedReference},
		"main::Counter.Count[...Types<int,bool>](ref int,ref bool)": {2, 1, OwnershipSharedReference},
		"main::Counter.Count[...Types<bool,int>](ref bool,ref int)": {2, 1, OwnershipSharedReference},
		"main::Counter.Add[...Types<>](uint)":                       {0, 2, OwnershipMutableReference},
		"main::Counter.Add[...Types<bool>](uint,ref bool)":          {1, 2, OwnershipMutableReference},
	}
	if len(got) != len(wants) {
		t.Fatalf("specializations = %v", got)
	}
	for id, want := range wants {
		fn := got[id]
		if fn == nil {
			t.Fatalf("missing %s in %v", id, got)
		}
		if fn.TypePack.FixedCount != want.fixed || len(fn.TypePack.Types) != want.length || len(fn.Parameters) != want.fixed+want.length {
			t.Fatalf("%s: %#v", id, fn)
		}
		receiver := fn.Parameters[0]
		if receiver.Name != "self" || receiver.Value.Ownership != want.authority || receiver.Reference == nil {
			t.Fatalf("%s receiver=%#v", id, receiver)
		}
		if want.fixed == 2 && fn.Parameters[1].Name != "base" {
			t.Fatal(fn.Parameters)
		}
		for i := want.fixed; i < len(fn.Parameters); i++ {
			if !strings.HasPrefix(fn.Parameters[i].Name, "values[") {
				t.Fatal(fn.Parameters)
			}
		}
	}
	for id, want := range map[FunctionID]string{
		"main::Counter.Count[...Types<>]()":                         "const.int 0",
		"main::Counter.Count[...Types<int,bool>](ref int,ref bool)": "const.int 2",
	} {
		body := Format(&Module{Types: module.Types, Functions: []*Function{got[id]}})
		if !strings.Contains(body, want) {
			t.Fatal(body)
		}
	}
	// Implicit shared self copies its reference; mutable self is reborrowed.
	for name, want := range map[string]OpKind{
		"Counter.ThroughSelf":        OpReferenceCopyShared,
		"Counter.ThroughMutableSelf": OpReferenceReborrowMutable,
	} {
		seen := false
		for _, fn := range module.Functions {
			if fn.Name == name {
				for _, block := range fn.Blocks {
					for _, op := range block.Operations {
						if op.Kind == want {
							seen = true
						}
					}
				}
			}
		}
		if !seen {
			t.Fatalf("%s missing %s", name, want)
		}
	}

}

// A missing receiver offset must fail verification rather than shift pack
// positions onto self or a fixed argument, including the empty-pack case.
// Rules: rules/compiler/semantic_ir.md §27A(2),§60(3).
func TestTypePackInstanceReceiverOffsetVerified(t *testing.T) {
	for _, id := range []FunctionID{"main::Counter.Count[...Types<>]()", "main::Counter.Add[...Types<bool>](uint,ref bool)"} {
		module, err := typePackFixtureModule(t, "instance_methods.sec")
		if err != nil {
			t.Fatal(err)
		}
		specializationIDs(module)[id].TypePack.FixedCount--
		if err := Verify(module); err == nil || !strings.Contains(err.Error(), "type-pack") {
			t.Fatal(err)
		}
	}
}

// Receiver-dependent return origins and explicit receiver offsets survive both
// target widths without modifying source pack bindings or specialization keys.
// Rules: rules/declarations/functions.md §38A(8–9);
// rules/compiler/semantic_ir.md §§27A,60(3),61(4).
func TestTypePackInstanceTargetsAndReturnOrigins(t *testing.T) {
	for _, name := range []string{"instance_methods.sec", "method_reference_return.sec"} {
		data, err := os.ReadFile("../../../testdata/generics/type_pack_declarations/" + name)
		if err != nil {
			t.Fatal(err)
		}
		for _, width := range []uint16{32, 64} {
			module, err := buildPackage16Source(t, string(data), width)
			if err != nil {
				t.Fatal(name, width, err)
			}
			if err := Verify(module); err != nil {
				t.Fatal(name, width, err)
			}
			if name == "method_reference_return.sec" {
				fn := specializationIDs(module)["main::Counter.Borrow[...Types<int>](ref int)"]
				if fn == nil || len(fn.ReturnOrigins) != 1 || fn.ReturnOrigins[0].Kind != ReturnOriginReceiver || len(fn.ReturnOrigins[0].Path) != 1 {
					t.Fatalf("%s/%d: %#v", name, width, fn)
				}
			}
		}
	}
}
