package main

import "sec/internal/diagnostics"

// diagnostic presents shared compiler occurrences using the LSP wire format.
// Rules: rules/tooling/lsp.md — Shared diagnostic model;
// rules/tooling/diagnostics.md — §§8–10.
type diagnostic struct {
	Range              lspRange                       `json:"range"`
	Severity           int                            `json:"severity"`
	Code               string                         `json:"code,omitempty"`
	Source             string                         `json:"source"`
	Message            string                         `json:"message"`
	RelatedInformation []diagnosticRelatedInformation `json:"relatedInformation,omitempty"`
	Data               *diagnosticData                `json:"data,omitempty"`
}

// diagnosticData retains optional compiler-owned proof metadata independently
// of severity. The key matches the CLI occurrence's proof_state argument.
// Rules: rules/compiler/compiler_analysis.md — §7(1–8), §58(3);
// rules/tooling/lsp.md — Shared diagnostic model (data payload).
type diagnosticData struct {
	ProofState diagnostics.ProofState `json:"proof_state"`
}

type diagnosticRelatedInformation struct {
	Location location `json:"location"`
	Message  string   `json:"message"`
}
