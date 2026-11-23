package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"sec/internal/diagnostics"
)

// The editor shares callable safety and conformance checks with compilation,
// including unannotated methods, generic owners and safe wrapper boundaries.
// Rules: rules/memory/unsafe.md — §§6–7,10–11,39–41;
// rules/tooling/lsp.md — Shared diagnostic model.
func TestUnsafeCallableConformanceLSP(t *testing.T) {
	uri := uriFromPath(filepath.Join(t.TempDir(), "main.sec"))
	for _, test := range []struct {
		name  string
		count int
	}{{"valid", 0}, {"contracts_invalid", 3}, {"calls_invalid", 12}} {
		source, err := os.ReadFile("../../testdata/sema/unsafe_callable_conformance/" + test.name + ".sec")
		if err != nil {
			t.Fatal(err)
		}
		items := analyze(uri, string(source))
		errors := 0
		for _, item := range items {
			if item.Severity != 1 {
				continue
			}
			errors++
			if test.name == "contracts_invalid" && (item.Code != diagnostics.InterfaceMemberIncompatible || !strings.Contains(item.Message, "unsafe caller obligations")) {
				t.Fatal(item)
			}
		}
		if errors != test.count {
			t.Fatal(test.name, items)
		}
	}
}
