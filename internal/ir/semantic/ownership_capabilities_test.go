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
		{"sema/cleanup_initializations.sec", "main", "initialization"},
		{"semantic_ir/package17_boundary_defer.sec", "cleanup", "deferred cleanup"},
		{"semantic_ir/package17_boundary_transfer.sec", "main", "ownership transfer"},
		{"semantic_ir/package17_boundary_discard.sec", "main", "cleanup cancellation"},
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
