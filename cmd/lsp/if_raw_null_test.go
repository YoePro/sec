package main

import (
	"os"
	"strings"
	"testing"

	"sec/internal/diagnostics"
)

// LSP projects compiler-owned null/unsafe diagnostics without making an if
// condition, its branches, or a preceding unsafe block an implicit unsafe scope.
// Rules: rules/control-flow/flowcontrol_if.md — §23;
// rules/memory/raw_pointers.md — §7(3–9a); MD-024 correction — §5.
func TestIfRawNullUnsafeBoundaryLSP(t *testing.T) {
	for _, name := range []string{"valid", "contexts_invalid"} {
		source, err := os.ReadFile("../../testdata/control_flow/if_raw_null/" + name + ".sec")
		if err != nil {
			t.Fatal(err)
		}
		counts := map[string]int{}
		errors, unsafeRead := 0, 0
		for _, d := range analyze("file:///tmp/sec-if-raw-null/"+name+".sec", string(source)) {
			if d.Severity != 1 {
				continue
			}
			errors++
			counts[d.Code]++
			if d.Message == "" || comparePosition(d.Range.Start, d.Range.End) >= 0 {
				t.Fatalf("incomplete diagnostic: %+v", d)
			}
			if strings.Contains(d.Message, "RawPtr.Read requires unsafe") {
				unsafeRead++
			}
		}
		if name == "valid" {
			if errors != 0 {
				t.Fatal(counts)
			}
			continue
		}
		if errors != 10 || counts[diagnostics.NullOutsideUnsafe] != 4 || counts[diagnostics.NullTestRequiresRawPointer] != 2 || counts[diagnostics.NullEquality] != 2 || counts[diagnostics.RawPointerNullDereference] != 1 || unsafeRead != 1 {
			t.Fatal(counts, errors, unsafeRead)
		}
	}
}
