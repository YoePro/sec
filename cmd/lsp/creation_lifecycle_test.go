package main

import (
	"os"
	"strings"
	"testing"
)

// TestCreationLifecycleLSP keeps raw creation ownership diagnostics aligned with
// compiler checking, including source ranges, registered code and mentor help.
// Rules: tasks.md §30; threads.md §34; lsp.md — One semantic source of truth.
func TestCreationLifecycleLSP(t *testing.T) {
	for _, mode := range []string{"valid", "invalid"} {
		source, err := os.ReadFile("../../testdata/sema/creation_lifecycle_" + mode + ".sec")
		if err != nil {
			t.Fatal(err)
		}
		count, creation := 0, 0
		for _, d := range analyze("file:///creation-lifecycle/main.sec", string(source)) {
			if d.Severity != 1 {
				continue
			}
			count++
			if d.Code == "S1247" {
				creation++
				if !strings.Contains(d.Message, "Creation failure produces no child") || d.Range.End.Character <= d.Range.Start.Character {
					t.Fatal(d)
				}
			}
		}
		if mode == "valid" && count != 0 || mode == "invalid" && (count != 9 || creation != 8) {
			t.Fatal(mode, count, creation)
		}
	}
}
