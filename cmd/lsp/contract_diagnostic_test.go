package main

import (
	"strings"
	"testing"

	"sec/internal/diagnostics"
)

// Duplicate in-contract values preserve Sema's stable identity, focused range,
// related first occurrence, and repair guidance at the LSP boundary.
//
// Rules:
//   - rules/types/contracts.md — "Ordered membership" and "Diagnostics"
//   - rules/tooling/diagnostics.md — § 15 "Language Server Protocol"
func TestDuplicateContractMembershipValueDiagnosticReachesLSP(t *testing.T) {
	items := analyze("", "module main\ntype Role string in [\"admin\", \"user\", \"admin\"]\n")
	for _, item := range items {
		if item.Code != diagnostics.DuplicateContractMembershipValue {
			continue
		}
		if item.Severity != 1 || item.Range.Start.Line != 1 ||
			!strings.Contains(item.Message, "duplicate membership value") ||
			!strings.Contains(item.Message, "previous declaration at") ||
			!strings.Contains(item.Message, "help: remove the duplicate") {
			t.Fatalf("duplicate membership LSP diagnostic = %+v", item)
		}
		return
	}
	t.Fatalf("missing %s in %+v", diagnostics.DuplicateContractMembershipValue, items)
}

// Empty in-contract diagnostics retain their canonical Sema identity and
// repair guidance when published by the language server.
//
// Rules:
//   - rules/types/contracts.md — "Ordered membership" and "Diagnostics"
//   - rules/tooling/diagnostics.md — § 15 "Language Server Protocol"
func TestEmptyContractMembershipDiagnosticReachesLSP(t *testing.T) {
	items := analyze("", "module main\ntype Impossible int in []\n")
	for _, item := range items {
		if item.Code != diagnostics.EmptyContractMembership {
			continue
		}
		if item.Severity != 1 || item.Range.Start.Line != 1 ||
			!strings.Contains(item.Message, "must contain at least one value") ||
			!strings.Contains(item.Message, "help: add at least one permitted") {
			t.Fatalf("empty membership LSP diagnostic = %+v", item)
		}
		return
	}
	t.Fatalf("missing %s in %+v", diagnostics.EmptyContractMembership, items)
}
