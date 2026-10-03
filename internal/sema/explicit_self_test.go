package sema

import (
	"strings"
	"testing"

	"sec/internal/diagnostics"
)

// An explicit self parameter in an impl method is recognized legacy syntax
// rejected with the registered migration diagnostic S1103 and a help naming
// the implicit receiver; ordinary methods without it stay valid.
//
// Rules:
//   - rules/corrections/applied/missing-decisions-md001-md009-correction-20261003.md — §§ 3–4
//   - rules/declarations/impl.md — implicit receiver
func TestExplicitSelfParameterIsRegisteredMigrationDiagnostic(t *testing.T) {
	errors := analyzeSource(t, `
module main

type Counter struct {
	value: int,
}

impl Counter {
	fn Legacy(self: Counter) int {
		return value
	}

	fn Current() int {
		return value
	}
}
`)
	found := 0
	for _, err := range errors {
		if err.ID == diagnostics.ExplicitSelfParameter {
			found++
			if !strings.Contains(err.Help, "implicitly") || !strings.Contains(err.Message, "remove self") {
				t.Fatalf("incomplete S1103 diagnostic: %+v", err)
			}
		}
	}
	if found != 1 {
		t.Fatalf("errors = %v, want one S1103", errors)
	}
}
