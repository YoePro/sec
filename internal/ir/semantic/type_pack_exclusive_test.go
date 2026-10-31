package semantic

import (
	"os"
	"strings"
	"testing"
)

// Exclusive pack positions retain concrete mutable reference types and ownership
// through the ordinary reference parameter/call producer; no holder is copied.
// Rules: rules/declarations/functions.md §38A(3)(2–5),(9);
// rules/compiler/semantic_ir.md §27A(1–2),(4).
func TestTypePackExclusivePositions(t *testing.T) {
	for _, name := range []string{"mutable_calls.sec", "static_mutable_boundary.sec", "exclusive_methods.sec"} {
		module, err := typePackFixtureModule(t, name)
		if err != nil {
			t.Fatal(name, err)
		}
		if err := Verify(module); err != nil {
			t.Fatal(name, err)
		}
		for _, fn := range specializationIDs(module) {
			borrows := map[uint32]bool{}
			for i, p := range fn.TypePack.Positions {
				parameter := fn.Parameters[fn.TypePack.FixedCount+i]
				if parameter.Reference == nil || borrows[parameter.Reference.Borrow] {
					t.Fatalf("%s reused a pack parameter grant", name)
				}
				borrows[parameter.Reference.Borrow] = true
				value := parameter.Value
				typ, _ := module.Types.Lookup(value.Type)
				if p.Mode != TypePackMutable || !typ.Mutable || value.Ownership != OwnershipMutableReference {
					t.Fatalf("%s/%d: %#v %#v", name, i, p, value)
				}
			}
		}
	}
}

// The verified mode must agree with reference authority and ownership; merely
// relabelling a shared grant as mutable (or vice versa) is invalid.
// Rules: rules/compiler/semantic_ir.md §27A(2);
// rules/declarations/functions.md §38A(3)(2–5).
func TestTypePackExclusiveAuthorityVerified(t *testing.T) {
	for _, mutate := range []func(*Module, *Function){
		func(_ *Module, f *Function) { f.TypePack.Positions[0].Mode = TypePackShared },
		func(_ *Module, f *Function) { f.TypePack.Positions[0].Mode = "unknown" },
		func(_ *Module, f *Function) {
			f.Parameters[f.TypePack.FixedCount].Value.Ownership = OwnershipSharedReference
		},
		func(m *Module, f *Function) {
			p := &f.Parameters[f.TypePack.FixedCount]
			typ, _ := m.Types.Lookup(p.Value.Type)
			typ.Mutable = false
			p.Value.Type = m.Types.Intern(typ)
		},
	} {
		m, err := typePackFixtureModule(t, "mutable_calls.sec")
		if err != nil {
			t.Fatal(err)
		}
		fn := specializationIDs(m)["main::Count[...Types<int,bool>](ref mut int,ref mut bool)"]
		if fn == nil {
			t.Fatal(specializationIDs(m))
		}
		mutate(m, fn)
		if err := Verify(m); err == nil {
			t.Fatal("accepted inconsistent exclusive pack facts")
		}
	}
}

// Exclusive grants retain target-independent semantic modes on both widths;
// the mutable holder is reborrowed for the call and remains usable afterwards.
// Rules: rules/declarations/functions.md §38A(3),(9);
// rules/memory/borrowing.md §11.
func TestTypePackExclusiveTargetWidths(t *testing.T) {
	data, err := os.ReadFile("../../../testdata/generics/type_pack_declarations/mutable_calls.sec")
	if err != nil {
		t.Fatal(err)
	}
	for _, width := range []uint16{32, 64} {
		m, err := buildPackage16Source(t, string(data), width)
		if err != nil {
			t.Fatal(width, err)
		}
		if err := Verify(m); err != nil {
			t.Fatal(width, err)
		}
		body := Format(m)
		if !strings.Contains(body, "ref mut int") {
			t.Fatal(body)
		}
	}
}
