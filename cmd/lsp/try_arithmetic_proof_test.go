package main

import (
	"os"
	"path/filepath"
	"testing"

	"sec/internal/diagnostics"
)

// The editor consumes the compiler's impossible-overflow proof while keeping
// unknown wide values, wide fields and a failing nested argument protected.
// Rules: rules/errors/errorhandling.md — §§11,13.1;
// rules/errors/runtime_checks.md — Static proof and check elimination.
func TestTryArgumentArithmeticProofLSP(t *testing.T) {
	uri := uriFromPath(filepath.Join(t.TempDir(), "main.sec"))
	for _, test := range []struct {
		name   string
		errors int
	}{{"valid", 0}, {"unproven_invalid", 3}} {
		source, err := os.ReadFile("../../testdata/sema/try_arithmetic_proof/" + test.name + ".sec")
		if err != nil {
			t.Fatal(err)
		}
		errors := 0
		for _, item := range analyze(uri, string(source)) {
			if item.Severity != 1 {
				continue
			}
			errors++
			if item.Code != diagnostics.TryPropagationIncompatible {
				t.Fatal(item)
			}
		}
		if errors != test.errors {
			t.Fatal(test.name, errors)
		}
	}
}
