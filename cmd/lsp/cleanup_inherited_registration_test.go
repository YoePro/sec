package main

import (
	"os"
	"path/filepath"
	"testing"
)

// Editor diagnostics accept current-value deferred reads after replacement,
// while retaining borrow, pending-defer transfer/discard and invalid-RHS errors.
// Rules: rules/memory/destruction.md — §13.2(3–8); rules/control-flow/defer.md — §§9–11;
// rules/tooling/lsp.md — "Shared diagnostic model" (MD-125).
func TestInheritedCleanupRegistrationLSP(t *testing.T) {
	for _, name := range []string{"cleanup_inherited_registration", "cleanup_inherited_registration_invalid"} {
		t.Run(name, func(t *testing.T) {
			source, err := os.ReadFile("../../testdata/sema/" + name + ".sec")
			if err != nil {
				t.Fatal(err)
			}
			uri := uriFromPath(filepath.Join(t.TempDir(), "main.sec"))
			diagnostics := analyze(uri, string(source))
			errors := 0
			for _, diagnostic := range diagnostics {
				if diagnostic.Severity == 1 {
					errors++
				}
			}
			want := 0
			if name == "cleanup_inherited_registration_invalid" {
				want = 4
			}
			if errors != want {
				t.Fatalf("errors = %d, want %d: %+v", errors, want, diagnostics)
			}
		})
	}
}
