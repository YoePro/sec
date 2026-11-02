package main

import (
	"os"
	"sec/internal/diagnostics"
	"strings"
	"testing"
)

// LSP projects source ownership conformance without requiring executable IR;
// a false match/try guard must not consume its prospective whole-error binding.
// Rules: rules/errors/errorhandling.md — §§20,29; MD-120 §11(4);
// rules/tooling/diagnostics.md — §§2(6–10),15.
func TestGenericErrorOwnershipLSP(t *testing.T) {
	for _, name := range []string{"valid", "invalid"} {
		data, err := os.ReadFile("../../testdata/sema/generic_error_ownership/" + name + ".sec")
		if err != nil {
			t.Fatal(err)
		}
		values := analyze("file:///tmp/sec-generic-error-ownership/main.sec", string(data))
		errors, guards := 0, 0
		for _, d := range values {
			if d.Severity != 1 {
				continue
			}
			errors++
			if d.Code == diagnostics.GuardConsumesBinding {
				guards++
				if !strings.Contains(d.Message, "help:") || d.Range.End.Character <= d.Range.Start.Character {
					t.Fatalf("guard diagnostic: %+v", d)
				}
			}
		}
		if name == "valid" && errors != 0 || name == "invalid" && (errors != 11 || guards != 2) {
			t.Fatalf("%s: %+v", name, values)
		}
	}
}
