package sema

import (
	"os"
	"testing"

	"sec/internal/layout"
	"sec/internal/lexer"
	"sec/internal/parser"
)

// TestAllocationPhysicalStorageSeparation preserves hardware storage as fixed,
// non-owning and independently volatile, never an Arena domain. Compiler-known
// volatile access contracts prove allocation freedom without treating unknown
// foreign mappings or similarly named user methods as physical intrinsics.
// Rules: rules/memory/allocation.md — §§2(7),20(1)-(5),24(3),(6);
// rules/platform/fixed-address-bindings.md — §§2,4,9;
// rules/platform/volatile.md — explicit raw volatile access.
func TestAllocationPhysicalStorageSeparation(t *testing.T) {
	file := "../../testdata/sema/allocation_physical_storage_valid.sec"
	data, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	expected := map[string]AllocationKnowledge{
		"Read": AllocationFree, "Write": AllocationFree, "Bind": AllocationFree,
		"Volatile": AllocationFree, "Forward": AllocationFree, "main": AllocationFree,
		"Mixed": AllocationMayAllocate, "Lookalike.VolatileRead": AllocationMayAllocate, "UserMember": AllocationMayAllocate,
		"MapRegion": AllocationUnknown, "UnknownMapping": AllocationUnknown,
	}
	for _, depth := range []AnalysisDepth{AnalysisInteractive, AnalysisStandard, AnalysisDeep} {
		for _, profile := range []string{"hosted", "embedded-arena", "noalloc", "freestanding"} {
			t.Run(string(depth)+"/"+profile, func(t *testing.T) {
				p := parser.New(lexer.NewWithFile(string(data), file))
				program := p.ParseProgram()
				if len(p.Errors()) != 0 {
					t.Fatal(p.Errors())
				}
				a := NewAnalyzerWithScalarPlanAndDepth(layout.ResolvedScalarPlan{Profile: profile, PointerWidthBits: 32}, depth)
				if errors := a.Analyze(program); len(errors) != 0 {
					t.Fatal(errors)
				}
				symbol := a.symbols["device"]
				if !symbol.Addressed || !symbol.Volatile || symbol.Storage != StorageOriginUnknown || symbol.AddressStability != AddressStabilityFixed || symbol.Type.ArenaDomainID != "" {
					t.Fatal("physical storage became owned allocation", symbol)
				}
				graph := a.CallGraph()
				for _, node := range graph.Nodes() {
					want, known := expected[node.Name]
					if !known {
						t.Fatal("unexpected node", node.Name)
					}
					fact := a.AllocationFact(node.ID)
					if fact.Knowledge != want {
						t.Fatalf("%s: %+v; want %s", node.Name, fact, want)
					}
					if want == AllocationFree {
						summary := graph.ArenaSummary(node.ID)
						if summary.MayAllocate || summary.AllocationUnknown || len(summary.DirectEffects) != 0 || len(a.AllocationContexts(node.ID)) != 0 {
							t.Fatal("hardware access acquired allocation context/effect", node.Name, summary)
						}
					}
					if node.Name == "Volatile" {
						effects := graph.EffectSummary(node.ID).DirectEffects
						if len(effects) != 2 || effects[0].Kind != EffectVolatileWrite || effects[1].Kind != EffectVolatileRead {
							t.Fatal("allocation freedom lost volatile effects", effects)
						}
					}
					if node.Name == "Forward" || node.Name == "Mixed" {
						volatile := false
						for _, domain := range graph.DomainSummary(node.ID).Effects {
							volatile = volatile || domain == SummaryMayAccessVolatile
						}
						if !volatile {
							t.Fatal("transitive volatile effect lost", node.Name)
						}
					}
				}
			})
		}
	}
}

// TestAllocationPhysicalStorageInvalidAccess retains validity/coverage guards;
// an allocation-free intrinsic contract cannot certify erroneous unsafe use.
// Rules: rules/memory/allocation.md — §§20(5),24(6);
// rules/platform/volatile.md — explicit raw volatile access.
func TestAllocationPhysicalStorageInvalidAccess(t *testing.T) {
	file := "../../testdata/sema/allocation_physical_storage_invalid.sec"
	data, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	p := parser.New(lexer.NewWithFile(string(data), file))
	program := p.ParseProgram()
	if len(p.Errors()) != 0 {
		t.Fatal(p.Errors())
	}
	a := NewAnalyzer()
	if errors := a.Analyze(program); len(errors) != 1 {
		t.Fatal("unsafe validity changed", errors)
	}
	for _, fact := range a.AllocationFacts() {
		if fact.Knowledge != AllocationUnknown || !fact.HasUnknown {
			t.Fatal("invalid source certified free", fact)
		}
	}
	pointer := Type{Kind: RawPtrType, Name: "RawPtr", TypeArgs: []Type{builtinTypes()["int"]}}
	// The pointee may own storage; raw-pointer cleanup still never reclaims it.
	pointer.GenericParameters = []string{"T"}
	pointer.TypeArgs = []Type{NewDynamicArrayType(builtinTypes()["int"])}
	if !allocationCleanupCovered(pointer) {
		t.Fatal("non-owning pointer acquired pointee cleanup")
	}
	invalid := pointer
	invalid.CustomFree = true
	if allocationCleanupCovered(invalid) {
		t.Fatal("custom cleanup was certified free")
	}

	count := 0
	for _, member := range CompilerKnownMembersForType(pointer, false) {
		if member.ID == "CKM-RAWPTR-VOLATILE-READ" || member.ID == "CKM-RAWPTR-VOLATILE-WRITE" {
			if member.AllocationBehavior != AllocationFree {
				t.Fatal("missing canonical operation contract", member)
			}
			count++
		} else if member.AllocationBehavior != "" {
			t.Fatal("unrelated operation acquired an allocation proof", member)
		}
	}
	if count != 2 {
		t.Fatal("volatile contracts missing", count)
	}
}
