package sema

import (
	"os"
	"strings"
	"testing"

	"sec/internal/diagnostics"
)

// TestCreationLifecycleScope preserves the child obligation through creation,
// continuing-path joins and early returns, without consuming failure carriers.
// Rules: tasks.md §§4–5,30; threads.md §§2,34; ownership.md §§17,20–21.
func TestCreationLifecycleScope(t *testing.T) {
	valid, err := os.ReadFile("../../testdata/sema/creation_lifecycle_valid.sec")
	if err != nil {
		t.Fatal(err)
	}
	if errors := analyzeSourceRaw(t, string(valid)); len(errors) != 0 {
		t.Fatal(errors)
	}
	invalid, err := os.ReadFile("../../testdata/sema/creation_lifecycle_invalid.sec")
	if err != nil {
		t.Fatal(err)
	}
	errors := analyzeSourceRaw(t, string(invalid))
	for _, name := range []string{"lostThread", "lostTask", "lostEarly", "lostConditional", "lostReplacement", "lostPropagation", "lostParameter", "lostAlias"} {
		count := 0
		for _, e := range errors {
			if e.ID == diagnostics.UnresolvedCreationLifecycle && strings.Contains(e.Message, "creation result "+name+" ") {
				count++
				if e.Help == "" || e.Line == 0 || e.Column == 0 {
					t.Errorf("incomplete diagnostic: %+v", e)
				}
			}
		}
		if count != 1 {
			t.Errorf("%s: want one lifecycle diagnostic, got %d: %v", name, count, errors)
		}
	}
	found := false
	for _, e := range errors {
		found = found || strings.Contains(e.Message, "owned thread unresolvedWorker is unresolved")
	}
	if !found {
		t.Errorf("extracted owner lost its obligation: %v", errors)
	}
	if len(errors) != 9 {
		t.Fatalf("want nine errors: %v", errors)
	}
}
