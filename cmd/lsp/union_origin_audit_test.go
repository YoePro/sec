package main

import (
	"os"
	"testing"
)

// TestUnionOriginAuditLSP consumes the same constructor provenance as the CLI
// for unsaved source and clears diagnostics after replacement by valid code.
// Rules: rules/memory/lifetime_analysis.md §§14,28;
// rules/tooling/lsp.md — "Snapshots".
func TestUnionOriginAuditLSP(t *testing.T) {
	source, err := os.ReadFile("../../testdata/sema/union_origin_audit_invalid.sec")
	if err != nil {
		t.Fatal(err)
	}
	uri := uriFromPath(t.TempDir() + "/main.sec")
	diagnostics := analyze(uri, string(source))
	if len(diagnostics) != 12 {
		t.Fatalf("diagnostics: %+v", diagnostics)
	}
	for i, item := range diagnostics {
		if item.Code == "" || item.Severity != 1 || len(item.RelatedInformation) == 0 {
			t.Fatalf("lost referent origin: %+v", item)
		}
		if i%2 == 1 && item.Code != diagnostics[i-1].Code {
			t.Fatal("constructor forms disagree")
		}
	}
	source, err = os.ReadFile("../../testdata/sema/union_origin_audit_valid.sec")
	if err != nil {
		t.Fatal(err)
	}
	if diagnostics := analyze(uri, string(source)); len(diagnostics) != 0 {
		t.Fatalf("valid replacement: %+v", diagnostics)
	}
}
