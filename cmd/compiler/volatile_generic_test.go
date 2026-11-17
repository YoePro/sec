package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Generic physical access must fail during check and before IR emission, while
// unrelated generic and concrete pointer operations retain their own checks.
// Rules: rules/platform/volatile.md — §§33,38;
// rules/tooling/diagnostics.md — §9.
func TestVolatileGenericEligibilityCLI(t *testing.T) {
	testVolatileEligibilityCLI(t, "volatile_generic", 6, "requires proven volatile eligibility for generic pointee T")
}

// Aggregate size and ordinary layout cannot supply a physical access contract.
// Rules: rules/platform/volatile.md — §37.
func TestVolatileAggregateEligibilityCLI(t *testing.T) {
	testVolatileEligibilityCLI(t, "volatile_aggregate", 20, "requires an explicit physical layout and access contract for aggregate")
}

// testVolatileEligibilityCLI verifies shared frontend rejection and suppresses
// IR artifacts for physical accesses missing a required type-eligibility proof.
// Rules: rules/platform/volatile.md — §§33,37–38;
// rules/tooling/diagnostics.md — §9.
func testVolatileEligibilityCLI(t *testing.T, fixture string, errorCount int, message string) {
	t.Helper()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "sec", "platform"), 0755); err != nil {
		t.Fatal(err)
	}
	for _, valid := range []bool{true, false} {
		name := fixture + "_invalid.sec"
		if valid {
			name = fixture + "_valid.sec"
		}
		source, err := os.ReadFile(filepath.Join("..", "..", "testdata", "sema", name))
		if err != nil {
			t.Fatal(err)
		}
		path := filepath.Join(root, "main.sec")
		if err := os.WriteFile(path, source, 0600); err != nil {
			t.Fatal(err)
		}
		stdout, stderr, code := runCLIForDiagnostics(t, "check", path, "--diagnostic-format=json")
		if valid {
			if code != 0 {
				t.Fatal(code, stdout, stderr)
			}
			continue
		}
		document := decodeOccurrenceDocument(t, stderr)
		if code != 3 || document.Summary.Errors != errorCount || !strings.Contains(stderr, message) {
			t.Fatal(code, stdout, stderr)
		}
		artifact := filepath.Join(root, "out.ir")
		_, stderr, code = runCLIForDiagnostics(t, "emit-ir", path, "-o", artifact)
		if code == 0 || !strings.Contains(stderr, message) {
			t.Fatal(code, stderr)
		}
		if _, err := os.Stat(artifact); !os.IsNotExist(err) {
			t.Fatalf("invalid volatile access left an IR artifact: %v", err)
		}
	}
}
