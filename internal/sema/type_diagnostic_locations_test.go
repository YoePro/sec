package sema

import (
	"os"
	"testing"

	"sec/internal/diagnostics"
)

// TestTypeDiagnosticSourceLocations verifies local/inherited contracts,
// explicit defaults, ambiguous defaults, omitted fields and missing
// initialization across independently parsed files.
// Rules: rules/types/contracts.md — Diagnostics;
// rules/types/default_values.md — Diagnostics and Diagnostic examples.
func TestTypeDiagnosticSourceLocations(t *testing.T) {
	const base = "../../testdata/sema/type_locations/definitions.sec"
	const use = "../../testdata/sema/type_locations/use_invalid.sec"
	files := []sourceFile{}
	for _, path := range []string{base, use} {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		files = append(files, sourceFile{name: path, source: string(data)})
	}
	_, errors := analyzeSourceFilesRaw(t, files...)
	want := map[string]int{diagnostics.DefaultViolatesContract: 1, diagnostics.InvalidMembershipValue: 2, diagnostics.DefaultNotRepresentable: 1, diagnostics.AmbiguousImplicitDefault: 2, diagnostics.InvalidDefaultedField: 2, diagnostics.MissingNonDefaultableField: 1, diagnostics.ValueViolatesContract: 2, diagnostics.ConstrainedAssignmentRequiresTry: 1, diagnostics.ImmutableRequiresInitializer: 1}
	counts := map[string]int{}
	for _, e := range errors {
		counts[e.ID]++
		if e.PreviousLine == 0 || e.PreviousColumn == 0 || e.RelatedLabel == "" {
			t.Fatalf("missing source relation: %+v", e)
		}
		if e.ID == diagnostics.ImmutableRequiresInitializer {
			if e.PreviousFile != use || e.PreviousLine != 10 {
				t.Fatal(e)
			}
			continue
		}
		if e.PreviousFile != base {
			t.Fatal("lost definition file", e)
		}
		if e.ID == diagnostics.InvalidMembershipValue && (e.File != base || e.Line != 4 || e.PreviousLine != 6) {
			t.Fatal("membership value must link to derived constraint", e)
		}
		if e.ID == diagnostics.DefaultViolatesContract && (e.Line != 5 || e.PreviousLine != 3) {
			t.Fatal("explicit default must link to inherited contract", e)
		}
	}
	if len(counts) != len(want) {
		t.Fatal(counts, errors)
	}
	for id, count := range want {
		if counts[id] != count {
			t.Fatalf("%s: got %d want %d: %v", id, counts[id], count, errors)
		}
	}
}
