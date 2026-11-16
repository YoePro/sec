package semantic

import (
	"errors"
	"reflect"
	"strings"
	"testing"
)

// P17-07 retains successful predecessor routes; an opt-in package number
// does not alter trivial semantics, reference/slice facts or the default.
// Rules: rules/mlir/packages/sec-mlir-dialect_package17.md — §§2,27,132,135;
// rules/compiler/compiler_pipeline.md — §§32–34.
func TestPackage17CapabilityPreservesPredecessors(t *testing.T) {
	for _, fixture := range []string{"package17_boundary_plain.sec", "package16_boundary_plain.sec", "slice_source_index.sec"} {
		program, a := sliceBoundaryInput(t, "semantic_ir/"+fixture)
		predecessor, err := Build(program, a, BuildOptions{RequestedModule: "main", MaxPackage: 16})
		if err != nil {
			t.Fatal(err)
		}
		upgraded, err := Build(program, a, BuildOptions{RequestedModule: "main", MaxPackage: 17})
		if err != nil {
			t.Fatal(err)
		}
		if err := Verify(upgraded); err != nil {
			t.Fatal(err)
		}
		if Format(predecessor) != Format(upgraded) {
			t.Fatalf("P17 changed predecessor %s", fixture)
		}
		if fixture == "package17_boundary_plain.sec" {
			ordinary, err := Build(program, a, BuildOptions{RequestedModule: "main"})
			if err != nil || Format(ordinary) != Format(upgraded) {
				t.Fatal("P17 changed default scalar route")
			}
		}
	}
}

// Unavailable ownership/cleanup IR fails before any module or earlier function
// can be published. Canonical Sema facts are consumed without query mutation.
// Rules: rules/mlir/packages/sec-mlir-dialect_package17.md — §§4,17–18,38,79,128,135.
func TestPackage17CapabilityRejectsMissingOwnershipIR(t *testing.T) {
	for _, test := range []struct{ fixture, module, feature string }{
		// Owned locals, scope cleanup and whole-local moves lower since
		// P17-35/P17-42, and whole fixed arrays since P17-38; these
		// fixtures now reach their later cleanup and P18 boundaries.
		{"sema/cleanup_initializations.sec", "main", "dynamic array type"},
		{"semantic_ir/package17_boundary_transfer.sec", "main", "dynamic array type"},
		{"semantic_ir/package17_boundary_discard.sec", "main", "dynamic array type"},
		// Option and Result lower since P17-37; defers in loops reading loop
		// owners or loop storage lower since P17-41 (TestPackage17DeferLoopFixturesLower).
	} {
		t.Run(test.feature, func(t *testing.T) {
			program, a := sliceBoundaryInput(t, "semantic_ir/package17_boundary_plain.sec", test.fixture)
			before := a.ResolvedTransferPlans()
			for attempt := 0; attempt < 2; attempt++ {
				module, err := Build(program, a, BuildOptions{RequestedModule: test.module, MaxPackage: 17})
				var unsupported *UnsupportedFeatureError
				if module != nil || !errors.As(err, &unsupported) || unsupported.Package != 17 || !strings.Contains(unsupported.Feature, test.feature) {
					t.Fatalf("partial IR or wrong boundary: module=%v err=%v", module, err)
				}
				capability := unsupported.ImplementationCapability()
				if capability.Boundary != "semantic IR package 17" || capability.File == "" || capability.Line <= 0 || capability.Column <= 0 {
					t.Fatalf("lost source capability identity: %+v", capability)
				}
			}
			if !reflect.DeepEqual(before, a.ResolvedTransferPlans()) {
				t.Fatal("preflight changed analysis facts")
			}
		})
	}
}

// Unselected modules do not activate a new P17 gate. Earlier package requests
// retain their existing named failure instead of being relabelled as P17.
// Rules: rules/compiler/compiler_pipeline.md — §§32–34;
// rules/mlir/packages/sec-mlir-dialect_package17.md — §§2,132.
func TestPackage17CapabilityModuleAndPredecessorBoundaries(t *testing.T) {
	program, a := sliceBoundaryInput(t, "semantic_ir/package17_boundary_plain.sec", "semantic_ir/package17_boundary_defer.sec")
	module, err := Build(program, a, BuildOptions{RequestedModule: "main", MaxPackage: 17})
	if err != nil || module == nil {
		t.Fatalf("unselected cleanup module blocked scalar IR: %v", err)
	}
	for _, pkg := range []uint8{14, 15, 16} {
		module, err := Build(program, a, BuildOptions{RequestedModule: "cleanup", MaxPackage: pkg})
		var unsupported *UnsupportedFeatureError
		if module != nil || !errors.As(err, &unsupported) || unsupported.Package != pkg || strings.Contains(unsupported.Feature, "P17") {
			t.Fatalf("P17 relabelled predecessor %d: %v", pkg, err)
		}
	}
}

// P17-41: the former defer-in-loop boundary fixtures lower: a loop owner a
// defer reads registers per execution and loop storage a defer reads becomes
// a per-declaration defer instance (defer.md §§ 8, 13).
func TestPackage17DeferLoopFixturesLower(t *testing.T) {
	for fixture, want := range map[string]string{
		"semantic_ir/package17_variants/defer_reads_loop_owner.sec":   "per-execution",
		"semantic_ir/package17_variants/defer_reads_loop_storage.sec": "defer-instance",
	} {
		program, a := sliceBoundaryInput(t, "semantic_ir/package17_boundary_plain.sec", fixture)
		module, err := Build(program, a, BuildOptions{RequestedModule: "main", MaxPackage: 17})
		if err != nil {
			t.Fatalf("%s: %v", fixture, err)
		}
		if err := Verify(module); err != nil {
			t.Fatalf("%s: %v", fixture, err)
		}
		if text := Format(module); !strings.Contains(text, want) {
			t.Errorf("%s lacks %q:\n%s", fixture, want, text)
		}
	}
}
