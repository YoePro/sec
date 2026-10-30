package sema

import (
	"os"
	"strings"
	"testing"
)

// TestDeferredArenaContinuation distinguishes registration-time validation from
// cleanup-time invalidation without hiding any ordinary immediate invalidation.
// Rules: rules/control-flow/defer.md — §§3,9,29;
// rules/memory/arena.md — §§2(6–7),35–36,40–44.
func TestDeferredArenaContinuation(t *testing.T) {
	for _, name := range []string{"valid", "immediate_invalid"} {
		t.Run(name, func(t *testing.T) {
			source, err := os.ReadFile("../../testdata/control_flow/defer_arena_continuation/" + name + ".sec")
			if err != nil {
				t.Fatal(err)
			}
			errors := analyzeSourceRaw(t, string(source))
			if name == "valid" {
				if len(errors) != 0 {
					t.Fatal(errors)
				}
				return
			}
			if len(errors) != 3 {
				t.Fatalf("want three immediate invalidations: %v", errors)
			}
			for _, err := range errors {
				if !strings.Contains(err.Message, "cannot use values after arena mem was reset") {
					t.Fatal(err)
				}
			}
		})
	}
}

// TestDeferredArenaBodyStillValidates keeps the deferred body's own generation
// transitions checked while restoring the ordinary continuation even on error.
// Rules: rules/control-flow/defer.md — §29;
// rules/memory/arena.md — §§35–36,38,44.
func TestDeferredArenaBodyStillValidates(t *testing.T) {
	data, err := os.ReadFile("../../testdata/control_flow/defer_arena_continuation/body_invalid.sec")
	if err != nil {
		t.Fatal(err)
	}
	errors := analyzeSourceRaw(t, string(data))
	if len(errors) != 1 || errors[0].Line != 8 || !strings.Contains(errors[0].Message, "cannot use values after arena mem was reset") {
		t.Fatalf("want only the deferred-body stale use, not the ordinary return: %v", errors)
	}
}
