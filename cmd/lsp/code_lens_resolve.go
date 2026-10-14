package main

import (
	"encoding/json"

	"sec/internal/ast"
)

// codeLensResolveData ties a pending lens to one exact interface declaration
// and source snapshot. It contains no callable or client-selected command.
type codeLensResolveData struct {
	Version     int    `json:"version"`
	URI         string `json:"uri"`
	Fingerprint string `json:"fingerprint"`
	Interface   string `json:"interface"`
}

// unresolvedInterfaceCodeLenses lists only source declarations. Parsing the
// active snapshot does not load sibling targets or invoke semantic analysis;
// conformance remains entirely owned by the existing compiler-fact projection.
// Rules: rules/tooling/lsp.md — "Interface conformance code lens", "Code lens";
// rules/declarations/interfaces.md — §§6,12.
func unresolvedInterfaceCodeLenses(uri, text string) []codeLens {
	result := []codeLens{}
	program := parseProgramForLSP(uri, text)
	if program == nil {
		return result
	}
	fingerprint := sourceContentFingerprint(text)
	for _, stmt := range program.Statements {
		declaration, ok := stmt.(*ast.InterfaceDeclaration)
		if !ok || declaration == nil || declaration.Name == nil {
			continue
		}
		result = append(result, codeLens{Range: tokenRange(text, declaration.Name.Token),
			Data: &codeLensResolveData{Version: 1, URI: uri, Fingerprint: fingerprint, Interface: declaration.Name.Value}})
	}
	return result
}

// handleCodeLensResolve validates the pending declaration and runs the existing
// cross-target conformance service on a read-only worker. Current dependency
// snapshots feed the existing cache; changed active text cannot relocate a lens
// by name. Commands from client input are ignored and recomputed from Sema facts.
// Rules: rules/tooling/lsp.md — "Interface conformance code lens", "Document snapshots",
// "Responsiveness model"; rules/declarations/interfaces.md — §§6,12.
func (s *server) handleCodeLensResolve(message rpcMessage) error {
	var lens codeLens
	if err := json.Unmarshal(message.Params, &lens); err != nil || lens.Data == nil || lens.Data.Version != 1 || lens.Data.URI == "" || lens.Data.Fingerprint == "" || lens.Data.Interface == "" {
		return s.respondError(message.ID, -32602, "invalid code lens resolve data")
	}
	if s.documentSnapshots == nil {
		return s.respondError(message.ID, -32801, "code lens document is no longer open")
	}
	snapshot, ok := s.documentSnapshots.Snapshot(lens.Data.URI)
	if !ok || sourceContentFingerprint(snapshot.Text) != lens.Data.Fingerprint {
		return s.respondError(message.ID, -32801, "code lens source changed; repeat textDocument/codeLens")
	}
	valid := false
	for _, current := range unresolvedInterfaceCodeLenses(lens.Data.URI, snapshot.Text) {
		if current.Range == lens.Range && current.Data.Interface == lens.Data.Interface {
			valid = true
			break
		}
	}
	if !valid {
		return s.respondError(message.ID, -32602, "code lens does not match an interface declaration")
	}
	for _, summary := range crossTargetInterfaceConformance(lens.Data.URI, snapshot.Text, s.sourceOverlay()) {
		if summary.name == lens.Data.Interface && summary.rng == lens.Range {
			lens.Command = interfaceConformanceCommand(lens.Data.URI, summary)
			// Reject a result if an edit or close arrived while a worker analyzed it.
			latest, open := s.documentSnapshots.Snapshot(lens.Data.URI)
			if !open || latest.Version != snapshot.Version || latest.Text != snapshot.Text {
				return s.respondError(message.ID, -32801, "code lens source changed during analysis")
			}
			return s.respond(message.ID, lens)
		}
	}
	return s.respondError(message.ID, -32801, "interface conformance result is unavailable")
}
