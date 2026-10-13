package main

import (
	"encoding/json"
	"sec/internal/lexer"
	"sec/internal/parser"
)

type testDiscoveryParams struct {
	TextDocument textDocumentIdentifier `json:"textDocument"`
	Run          string                 `json:"run,omitempty"`
}

type discoveredTest struct {
	Module   string   `json:"module"`
	Identity []string `json:"identity"`
	Name     string   `json:"name"`
	Location location `json:"location"`
}

type testDiscoveryResult struct {
	CompilationValid bool             `json:"compilationValid"`
	Tests            []discoveredTest `json:"tests"`
}

// discoverTests exposes the active file's validated compiler test view and
// compiler-owned name selection, without reconstructing identity from source.
// Rules: rules/tooling/testing.md — §§6,39.1–39.4;
// rules/compiler/compiler_pipeline.md — §11(3–5), §92(2).
func discoverTests(uri, text, pattern string, overlay sourceOverlay) (testDiscoveryResult, error) {
	result := testDiscoveryResult{Tests: []discoveredTest{}}
	path := pathFromURI(uri)
	p := parser.New(lexer.NewWithFile(text, path))
	parsed := p.Parse()
	if parsed.HasErrors {
		return result, nil
	}
	if errors := prepareProgramForLSP(parsed.Program, path, overlay); len(errors) != 0 {
		return result, nil
	}
	a := newLSPAnalyzerWithOverlay(uri, parsed.Program, overlay)
	if errors := a.Analyze(parsed.Program); len(errors) != 0 {
		return result, nil
	}
	selected, err := a.SelectTests(pattern)
	if err != nil {
		return result, err
	}
	result.CompilationValid = true
	for _, metadata := range selected {
		if normalizedSourcePath(metadata.Source.File) != normalizedSourcePath(path) {
			continue
		}
		point := diagnosticTokenStart(text, lexer.Token{Line: metadata.Source.Line, Column: metadata.Source.Column})
		end := diagnosticTokenStart(text, lexer.Token{Line: metadata.Source.EndLine, Column: metadata.Source.EndColumn})
		result.Tests = append(result.Tests, discoveredTest{Module: metadata.Identity.Module,
			Identity: append([]string(nil), metadata.Identity.Path...), Name: metadata.Name,
			Location: location{URI: uri, Range: lspRange{Start: point, End: end}}})
	}
	return result, nil
}

// handleTestDiscovery serves snapshot discovery/selection to thin clients.
// Execution remains the existing sec test pipeline's responsibility.
// Rules: rules/tooling/testing.md — §§39.1–39.7; rules/tooling/lsp.md — Snapshots.
func (s *server) handleTestDiscovery(message rpcMessage) error {
	var params testDiscoveryParams
	if err := json.Unmarshal(message.Params, &params); err != nil {
		return s.respondError(message.ID, -32602, err.Error())
	}
	snapshot, found := s.documentSnapshots.Snapshot(params.TextDocument.URI)
	if !found {
		return s.respondError(message.ID, -32602, "test document is not open")
	}
	result, err := discoverTests(params.TextDocument.URI, snapshot.Text, params.Run, s.sourceOverlay())
	if err != nil {
		return s.respondError(message.ID, -32602, "invalid test selection: "+err.Error())
	}
	return s.respond(message.ID, result)
}
