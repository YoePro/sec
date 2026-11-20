package main

import (
	"os"
	"strings"
	"testing"
)

// Unsaved documents retain canonical borrow origins and implementation-limit
// identities without a second editor borrow/provenance engine.
// Rules: rules/tooling/lsp.md — Snapshots; rules/declarations/spread.md §4.2.
func TestSpreadBorrowAuditLSP(t *testing.T) {
	data, err := os.ReadFile("../../testdata/sema/spread_borrow_audit_invalid.sec")
	if err != nil {
		t.Fatal(err)
	}
	uri := uriFromPath(t.TempDir() + "/main.sec")
	values := analyze(uri, string(data))
	want := []string{"S1161", "S1239", "S1239", "S1239", "S1161", "S1161", "S1239", "S1239"}
	if len(values) != len(want) {
		t.Fatalf("diagnostics: %+v", values)
	}
	for i, value := range values {
		if value.Code != want[i] || value.Severity != 1 || !strings.Contains(value.Message, "help:") {
			t.Fatalf("lost diagnostic: %+v", value)
		}
		if want[i] == "S1161" && (len(value.RelatedInformation) == 0 || value.RelatedInformation[0].Location.Range.Start.Character >= value.Range.Start.Character) {
			t.Fatalf("lost earlier reservation: %+v", value)
		}
	}
	data, err = os.ReadFile("../../testdata/sema/spread_borrow_audit_valid.sec")
	if err != nil {
		t.Fatal(err)
	}
	if clean := analyze(uri, string(data)); len(clean) != 0 {
		t.Fatalf("valid unsaved replacement: %+v", clean)
	}
}
