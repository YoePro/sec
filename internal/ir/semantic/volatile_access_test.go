package semantic

import (
	"strings"
	"testing"
)

// A copied local snapshot causes no new physical read; distinct reads remain
// distinct operations and never become own.move-from-place on external storage.
// Rules: rules/memory/copy_move.md §§19,30.6; rules/platform/volatile.md §§3,9.
func TestVolatileSnapshotBoundary(t *testing.T) {
	program, a := sliceBoundaryInput(t, "semantic_ir/volatile_snapshot/main.sec")
	module, err := Build(program, a, BuildOptions{RequestedModule: "main", MaxPackage: 23})
	if err != nil {
		t.Fatal(err)
	}
	if err := Verify(module); err != nil {
		t.Fatal(err)
	}
	serialized, err := Serialize(module)
	if err != nil {
		t.Fatal(err)
	}
	restored, err := Deserialize(serialized)
	if err != nil {
		t.Fatal(err)
	}
	again, err := Serialize(restored)
	if err != nil || string(again) != string(serialized) {
		t.Fatalf("volatile serialization is not stable: %v", err)
	}
	if Format(restored) != Format(module) {
		t.Fatal("volatile facts lost in serialization")
	}
	text := Format(module)
	for _, want := range []string{"volatile.read", "volatile.write", "physical-plan=unverified", "authorization=call-prefix@", "authorization=unsafe-block@"} {
		if !strings.Contains(text, want) {
			t.Fatalf("missing %s in %s", want, text)
		}
	}
	if countOperations(loopFunction(t, module, "User"), OpDirectCall) != 1 {
		t.Fatal("ordinary lookalike call was reinterpreted")
	}
	var ordered []OpKind
	for _, block := range loopFunction(t, module, "Ordered").Blocks {
		for _, op := range block.Operations {
			if op.Kind == OpDirectCall || op.Kind == OpVolatileWrite {
				ordered = append(ordered, op.Kind)
			}
		}
	}
	if len(ordered) != 3 || ordered[0] != OpDirectCall || ordered[1] != OpDirectCall || ordered[2] != OpVolatileWrite {
		t.Fatal("operand evaluation duplicated or reordered", ordered)
	}
	for _, tc := range []struct {
		name          string
		reads, writes int
	}{{"Snapshot", 2, 1}, {"Borrowed", 1, 0}, {"Discarded", 1, 0}, {"Unused", 1, 0}, {"Direct", 1, 0}, {"Ordered", 0, 1}, {"User", 0, 0}, {"Nominal", 1, 0}, {"Nested", 1, 0}, {"MovedSnapshot", 1, 0}} {
		fn := loopFunction(t, module, tc.name)
		reads, writes := 0, 0
		for _, block := range fn.Blocks {
			for _, op := range block.Operations {
				switch op.Kind {
				case OpVolatileRead:
					reads++
				case OpVolatileWrite:
					writes++
				}
				if (op.Kind == OpVolatileRead || op.Kind == OpVolatileWrite) && (op.Ownership != nil || op.Reference != nil || op.Storage != 0) {
					t.Fatal("physical access acquired ownership", op)
				}
			}
		}
		if reads != tc.reads || writes != tc.writes {
			t.Fatalf("%s: reads=%d writes=%d", tc.name, reads, writes)
		}
	}
}

// Malformed IR cannot recast hardware storage as an owned Place, attach a
// borrow, assert synchronization or claim an invented verified physical plan.
// Rules: rules/memory/copy_move.md §19; rules/platform/volatile.md §§9–12.
func TestVolatileAccessVerifierRejectsContradictions(t *testing.T) {
	cases := map[string]func(*Operation){
		"missing contract":          func(op *Operation) { op.Volatile = nil },
		"invented plan":             func(op *Operation) { op.Volatile.PhysicalPlan = "verified" },
		"missing unsafe provenance": func(op *Operation) { op.Volatile.Authorization = "" },
		"wrong member":              func(op *Operation) { op.Volatile.MemberID = "CKM-RAWPTR-READ" },
		"ownership move":            func(op *Operation) { op.Ownership = &OwnershipFacts{} },
		"borrow":                    func(op *Operation) { op.Reference = &ReferenceMetadata{} },
		"synchronization":           func(op *Operation) { op.AtomicOrder = "SeqCst" },
		"missing pointer":           func(op *Operation) { op.Operands = nil },
		"wrong pointee":             func(op *Operation) {},
		"extra result":              func(op *Operation) { op.Results = append(op.Results, op.Results[0]) },
		"metadata on ordinary op":   func(op *Operation) { op.Kind = OpOwnMoveFromPlace },
	}
	// Each mutation starts from an independently parsed, verified source module.
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			program, a := sliceBoundaryInput(t, "semantic_ir/volatile_snapshot/main.sec")
			module, err := Build(program, a, BuildOptions{RequestedModule: "main", MaxPackage: 23})
			if err != nil {
				t.Fatal(err)
			}
			fn := loopFunction(t, module, "Snapshot")
			var access *Operation
			for _, block := range fn.Blocks {
				for i := range block.Operations {
					if block.Operations[i].Kind == OpVolatileRead {
						access = &block.Operations[i]
						break
					}
				}
				if access != nil {
					break
				}
			}
			if access == nil {
				t.Fatal("missing volatile read")
			}
			mutate(access)
			if name == "wrong pointee" {
				access.Results[0].Type = module.Types.Intern(Type{Kind: TypeBool})
			}
			if err := Verify(module); err == nil {
				t.Fatal("accepted contradictory volatile access")
			}
		})
	}
}

// A raw read cannot manufacture managed ownership or borrowed string storage
// from unvalidated physical bits. No partial Semantic IR escapes this boundary.
// Rules: rules/memory/copy_move.md §19.3; rules/platform/volatile.md §§9–11.
func TestVolatileManagedDecodeBoundary(t *testing.T) {
	program, a := sliceBoundaryInput(t, "semantic_ir/volatile_snapshot/managed.sec")
	module, err := Build(program, a, BuildOptions{RequestedModule: "main", MaxPackage: 23})
	if module != nil || err == nil || !strings.Contains(err.Error(), "managed or unsupported value representation") {
		t.Fatalf("module=%v err=%v", module, err)
	}
}
