package semantic

import (
	"strings"
	"testing"
)

// TestTypePackStaticMethodsSpecialize checks qualified identity, ordered sequence
// reuse, concrete signatures/lengths, no receiver and exactly-once argument calls.
// Rules: rules/declarations/impl.md §13; rules/declarations/functions.md §38A(4–6),(9);
// rules/compiler/semantic_ir.md §27A(2),(4).
func TestTypePackStaticMethodsSpecialize(t *testing.T) {
	module, err := typePackFixtureModule(t, "static_methods.sec")
	if err != nil {
		t.Fatal(err)
	}
	if err := Verify(module); err != nil {
		t.Fatal(err)
	}
	got := specializationIDs(module)
	wants := map[FunctionID][2]int{
		"main::Counter.Count[...Types<>]()":                               {0, 0},
		"main::Counter.Count[...Types<int,bool>](ref int,ref bool)":       {2, 0},
		"main::Counter.Count[...Types<bool,int>](ref bool,ref int)":       {2, 0},
		"main::Other.Count[...Types<int,bool>](ref int,ref bool)":         {2, 0},
		"main::Counter.Unused[...Types<>]()":                              {0, 0},
		"main::Counter.Unused[...Types<int>](ref int)":                    {1, 0},
		"main::Counter.Tagged[...Types<int,bool>](uint,ref int,ref bool)": {2, 1},
	}
	if len(got) != len(wants) {
		t.Fatalf("specializations = %v", got)
	}
	for id, counts := range wants {
		fn := got[id]
		if fn == nil {
			t.Fatalf("missing %s in %v", id, got)
		}
		if fn.TypePack == nil || len(fn.TypePack.Types) != counts[0] || fn.TypePack.FixedCount != counts[1] || len(fn.Parameters) != counts[0]+counts[1] {
			t.Fatalf("%s: %#v", id, fn)
		}
		for _, p := range fn.Parameters {
			if p.Name == "self" {
				t.Fatalf("static method %s has self", id)
			}
		}
	}
	for id, constant := range map[FunctionID]string{
		"main::Counter.Count[...Types<>]()":                         "const.int 0",
		"main::Counter.Count[...Types<int,bool>](ref int,ref bool)": "const.int 2",
		"main::Counter.Unused[...Types<int>](ref int)":              "const.int 7",
	} {
		if body := Format(&Module{Types: module.Types, Functions: []*Function{got[id]}}); !strings.Contains(body, constant) {
			t.Fatal(body)
		}
	}
	calls, produced := 0, 0
	for _, fn := range module.Functions {
		for _, block := range fn.Blocks {
			for _, op := range block.Operations {
				if op.Callee == "main::Counter.Count[...Types<int,bool>](ref int,ref bool)" {
					calls++
				}
				if strings.Contains(string(op.Callee), "::Produce(") {
					produced++
				}
			}
		}
	}
	if calls != 2 || produced != 1 {
		t.Fatalf("reused calls=%d, Produce calls=%d", calls, produced)
	}
}

// An undemanded method template has no executable body; non-shared pack
// positions still fail closed without a partial module.
// Rules: rules/compiler/semantic_ir.md §27A(2),(4).
func TestTypePackStaticMethodBoundaries(t *testing.T) {
	module, err := typePackFixtureModule(t, "static_template.sec")
	if err != nil || len(module.Functions) != 0 {
		t.Fatal(module, err)
	}
	for _, test := range []struct{ file, want string }{
		{"owned_cleanup_boundary.sec", "ownership-sensitive owned type-pack position"},
	} {
		t.Run(test.file, func(t *testing.T) {
			module, err := typePackFixtureModule(t, test.file)
			if module != nil || err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatal(module, err)
			}
		})
	}
}
