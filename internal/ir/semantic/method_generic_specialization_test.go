package semantic

import (
	"fmt"
	"os"
	"strings"
	"testing"
)

// TestMethodGenericSpecializations builds static/instance method templates only
// on demand, preserving independent owner substitutions and ordinary receivers.
// Rules: rules/compiler/monomorphization.md — §§4,6.4,8–9;
// rules/compiler/semantic_ir.md — §§60–61,75.
func TestMethodGenericSpecializations(t *testing.T) {
	source, err := os.ReadFile("../../../testdata/generics/specialization/method_generics.sec")
	if err != nil {
		t.Fatal(err)
	}
	module, err := analyzedModule(t, string(source), 17)
	if err != nil {
		t.Fatal(err)
	}
	if err := Verify(module); err != nil {
		t.Fatal(err)
	}
	specializations := specializationIDs(module)
	if len(specializations) != 9 {
		t.Fatalf("got %d specializations: %+v", len(specializations), specializations)
	}
	owners := map[string]bool{}
	shared, mutable, static := 0, 0, 0
	for _, function := range specializations {
		if function.Specialization.Template == "Transfer" {
			continue
		}
		if function.Specialization.Identity == "" {
			t.Fatal("missing canonical method identity", function.ID)
		}
		if strings.Contains(function.Specialization.Template, "Unused") {
			t.Fatal("unused method emitted")
		}
		if strings.HasSuffix(function.Specialization.Template, "Static") {
			static++
			continue
		}
		if len(function.Parameters) == 0 || function.Parameters[0].Name != "self" {
			t.Fatalf("missing receiver: %+v", function)
		}
		if function.Parameters[0].Value.Ownership == OwnershipMutableReference {
			mutable++
		} else {
			shared++
		}
		if function.Specialization.Template == "Box.Pick" {
			self, _ := module.Types.Lookup(function.Parameters[0].Value.Type)
			owner, _ := module.Types.Lookup(self.Element)
			owners[fmt.Sprint(owner.TypeArgs[0])] = true
		}
	}
	if len(owners) != 2 || shared != 4 || mutable != 1 || static != 3 {
		t.Fatalf("owners/shared/mutable/static = %d/%d/%d/%d", len(owners), shared, mutable, static)
	}
}

// TestMethodGenericSpecializationModes preserves owned cleanup, borrowed return
// origins and exact overload identities through the existing concrete builder.
// Rules: rules/compiler/monomorphization.md — §§4,9.1,11.2–11.4;
// rules/compiler/semantic_ir.md — §§60–61,75(4).
func TestMethodGenericSpecializationModes(t *testing.T) {
	source, err := os.ReadFile("../../../testdata/generics/specialization/method_generics_modes.sec")
	if err != nil {
		t.Fatal(err)
	}
	module, err := analyzedModule(t, string(source), 17)
	if err != nil {
		t.Fatal(err)
	}
	if err := Verify(module); err != nil {
		t.Fatal(err)
	}
	got := specializationIDs(module)
	if len(got) != 4 {
		t.Fatalf("want four distinct specializations: %+v", got)
	}
	for _, function := range got {
		if function.Specialization.Identity == "" {
			t.Fatal("missing canonical identity", function.ID)
		}
		if function.Specialization.Template == "Helper.Keep" {
			text := Format(&Module{Types: module.Types, Functions: []*Function{function}})
			if !strings.Contains(text, "cleanup.track-owned") || !strings.Contains(text, "reason=transferred") {
				t.Fatal("lost ownership obligations", text)
			}
		}
	}
	for _, mutate := range []func(*SpecializationFacts){func(f *SpecializationFacts) { f.Identity = "" }, func(f *SpecializationFacts) { f.Identity = "wrong" }} {
		for _, function := range got {
			saved := function.Specialization.Identity
			mutate(function.Specialization)
			if err := Verify(module); err == nil {
				t.Fatal("inconsistent identity accepted")
			}
			function.Specialization.Identity = saved
			break
		}
	}
}
