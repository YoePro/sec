package sema

import (
	"os"
	"sec/internal/diagnostics"
	"strings"
	"testing"
)

// Interface overload sets retain all finite signatures through substituted
// inheritance, diamonds, explicit conformance, views and generic constraints.
// Rules: rules/declarations/interfaces.md §§5–6,9;
// rules/declarations/functions.md §§19–20,24.
func TestInterfaceOverloadMatrix(t *testing.T) {
	for _, name := range []string{"valid", "contracts_invalid", "declarations_invalid"} {
		source, err := os.ReadFile("../../testdata/sema/interface_overload_matrix/" + name + ".sec")
		if err != nil {
			t.Fatal(err)
		}
		a, errors := analyzeSourceWithAnalyzerRaw(t, string(source))
		if name == "valid" {
			if len(errors) != 0 {
				t.Fatal(errors)
			}
			if len(a.types["Diamond"].InterfaceMethods) != 5 {
				t.Fatal("lost or duplicated diamond overloads")
			}
			reordered := strings.Replace(string(source), "Left[int], Right", "Right, Left[int]", 1)
			reordered = strings.Replace(reordered, "Combined, Left[int]", "Left[int], Combined", 1)
			b, errors := analyzeSourceWithAnalyzerRaw(t, reordered)
			if len(errors) != 0 || len(b.types["Diamond"].InterfaceMethods) != 5 {
				t.Fatalf("inheritance order changed overloads: %v", errors)
			}
			continue
		}
		if name == "declarations_invalid" {
			if len(errors) != 3 {
				t.Fatal(errors)
			}
			continue
		}
		if len(errors) != 2 {
			t.Fatal(errors)
		}
		for _, e := range errors {
			if e.ID != diagnostics.InterfaceMemberIncompatible {
				t.Fatal(e)
			}
		}
		if !strings.Contains(errors[1].Message, "result type is int, but the interface requires bool") {
			t.Fatalf("wrong overload explained: %v", errors[1])
		}
	}
}
