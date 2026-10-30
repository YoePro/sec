package sema

import (
	"os"
	"strings"
	"testing"

	"sec/internal/diagnostics"
)

// TestMatchTerminationMatrix checks normalized union/Option/alias/open-bit
// coverage and nested return flow, including an ordinary unsafe block wrapper.
// Rules: rules/control-flow/flowcontrol_match.md — §§5,23–24;
// rules/declarations/enums.md — §§7,11; rules/memory/unsafe.md — §4.
func TestMatchTerminationMatrix(t *testing.T) {
	for _, name := range []string{"valid", "continuing_invalid", "unreachable_invalid"} {
		t.Run(name, func(t *testing.T) {
			data, err := os.ReadFile("../../testdata/control_flow/match_termination/" + name + ".sec")
			if err != nil {
				t.Fatal(err)
			}
			errors := analyzeSourceRaw(t, string(data))
			if name == "valid" {
				if len(errors) != 0 {
					t.Fatal(errors)
				}
				return
			}
			if len(errors) != 6 {
				t.Fatalf("want six independent flow errors, got %v", errors)
			}
			for _, err := range errors {
				if name == "unreachable_invalid" {
					if err.ID != diagnostics.UnreachableStatement {
						t.Fatal(err)
					}
				} else if !strings.Contains(err.Message, "return") {
					t.Fatal(err)
				}
			}
			if name == "continuing_invalid" {
				for _, fn := range []string{"Union", "Optional", "Aliases", "Bits", "Nested", "NestedUnsafe"} {
					found := false
					for _, err := range errors {
						found = found || strings.Contains(err.Message, fn)
					}
					if !found {
						t.Errorf("missing continuing-path return error for %s", fn)
					}
				}
			}
		})
	}
}
