package main

import (
	"os"
	"strings"
	"testing"
)

// LSP retains the compiler's exact-value classification and static overflow
// through immutable bindings instead of demanding try for proven constants.
// Rules: rules/declarations/enums.md — §12.1; rules/types/types.md — Exact integer conversion proof.
func TestKnownEnumValuesLSP(t *testing.T) {
	for _, name := range []string{"valid", "invalid"} {
		data, err := os.ReadFile("../../testdata/sema/enum_known_values/" + name + ".sec")
		if err != nil {
			t.Fatal(err)
		}
		errors := 0
		for _, d := range analyze("file:///tmp/sec-enum-known/main.sec", string(data)) {
			if d.Severity != 1 {
				continue
			}
			errors++
			if name == "invalid" && !strings.Contains(d.Message, "overflows") {
				t.Fatal(d)
			}
		}
		if name == "valid" && errors != 0 || name == "invalid" && errors != 4 {
			t.Fatalf("%s: errors=%d", name, errors)
		}
	}
}
