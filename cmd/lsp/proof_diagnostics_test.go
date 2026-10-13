package main

import (
	"encoding/json"
	"strings"
	"testing"

	"sec/internal/diagnostics"
	"sec/internal/sema"
)

// TestProofDiagnosticWireData distinguishes owner proof from severity, ID and
// message text, preserving absent metadata rather than fabricating certainty.
// Rules: rules/compiler/compiler_analysis.md — §7(1–8), §58(3);
// rules/tooling/lsp.md — Shared diagnostic model.
func TestProofDiagnosticWireData(t *testing.T) {
	for _, state := range []diagnostics.ProofState{"", diagnostics.ProofValid, diagnostics.ProofInvalid, diagnostics.ProofUnproven} {
		for _, severity := range []diagnostics.Severity{diagnostics.SeverityError, diagnostics.SeverityWarning, diagnostics.SeverityInformation} {
			value := sema.Error{ID: diagnostics.RequiredProofUnavailable, Severity: severity,
				ProofState: state, Message: "Invalid and Unproven are words, not metadata", Line: 1, Column: 1,
				EndLine: 1, EndColumn: 7, Help: "Keep the required safety proof."}
			got := semaDiagnostic(value, 3, "Worker")
			if got.Severity != 1 {
				t.Fatal("mandatory severity changed", got)
			}
			if !strings.HasPrefix(got.Message, value.DisplayMessage()) || !strings.Contains(got.Message, value.Help) {
				t.Fatal(got)
			}
			wire, err := json.Marshal(got)
			if err != nil {
				t.Fatal(err)
			}
			var decoded map[string]json.RawMessage
			if err := json.Unmarshal(wire, &decoded); err != nil {
				t.Fatal(err)
			}
			data, exists := decoded["data"]
			if state == "" {
				if exists || got.Data != nil {
					t.Fatal("inferred proof from text/severity/ID", string(wire))
				}
				continue
			}
			var payload map[string]string
			if !exists || json.Unmarshal(data, &payload) != nil || payload["proof_state"] != string(state) {
				t.Fatal(string(wire))
			}
			var roundTrip diagnostic
			if err := json.Unmarshal(wire, &roundTrip); err != nil || roundTrip.Data == nil || roundTrip.Data.ProofState != state {
				t.Fatal(roundTrip, err)
			}
		}
	}
}

// TestCrossTargetProofStatesRemainDistinct guards against claiming a proven
// violation on every target when another target merely lacks a proof.
// Rules: rules/tooling/lsp.md — Multi-target diagnostics;
// rules/compiler/compiler_analysis.md — §7(4–8), §58(3).
func TestCrossTargetProofStatesRemainDistinct(t *testing.T) {
	invalid := diagnostic{Code: "shared-owner", Severity: 1, Source: "sec", Message: "required property",
		Data: &diagnosticData{ProofState: diagnostics.ProofInvalid}}
	unproven := invalid
	unproven.Data = &diagnosticData{ProofState: diagnostics.ProofUnproven}
	result := crossTargetResult{targets: []string{"linux-amd64", "linux-armv7"},
		perTarget: map[string][]diagnostic{"linux-amd64": {invalid}, "linux-armv7": {unproven}}}
	merged := mergeCrossTargetDiagnostics([]diagnostic{invalid}, result)
	if len(merged) != 2 {
		t.Fatal("distinct proof outcomes coalesced", merged)
	}
	seen := map[diagnostics.ProofState]bool{}
	for _, item := range merged {
		if item.Data == nil || !strings.Contains(item.Message, "1 of 2 targets") {
			t.Fatal(item)
		}
		seen[item.Data.ProofState] = true
	}
	if !seen[diagnostics.ProofInvalid] || !seen[diagnostics.ProofUnproven] {
		t.Fatal(seen)
	}
}
