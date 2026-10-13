package main

import (
	"github.com/BurntSushi/toml"
	"sec/internal/sema"
)

// lspAnalysisDepth resolves the project budget for an on-disk request.
// Rules: rules/projects/projects.md — §23; rules/tooling/lsp.md — Configuration.
func lspAnalysisDepth(sourcePath string) sema.AnalysisDepth {
	return lspAnalysisDepthWithOverlay(sourcePath, sourceOverlay{})
}

// lspAnalysisDepthWithOverlay uses the nearest manifest snapshot, including
// unsaved settings. Missing or temporarily invalid settings keep the interactive
// budget; the compiler still runs required safety analysis at every depth.
// Rules: rules/tooling/lsp.md — Configuration, Snapshots;
// rules/compiler/compiler_analysis.md — §§60(1–5),62(1–3).
func lspAnalysisDepthWithOverlay(sourcePath string, overlay sourceOverlay) sema.AnalysisDepth {
	if sourcePath != "" {
		if _, text, found := lspProjectManifest(sourcePath, overlay); found {
			if depth, err := parseLSPAnalysisDepth(text); err == nil {
				return depth
			}
		}
	}
	return sema.AnalysisInteractive
}

// parseLSPAnalysisDepth accepts the normative manifest setting in every TOML
// spelling and delegates mode validation to Sema's canonical depth parser.
// Rules: rules/projects/projects.md — §23; rules/tooling/lsp.md — Configuration.
func parseLSPAnalysisDepth(text string) (sema.AnalysisDepth, error) {
	var settings struct {
		Analysis struct {
			Depth *string `toml:"lsp_depth"`
		} `toml:"analysis"`
	}
	if _, err := toml.Decode(text, &settings); err != nil {
		return "", err
	}
	if settings.Analysis.Depth == nil {
		return sema.AnalysisInteractive, nil
	}
	return sema.ParseAnalysisDepth(*settings.Analysis.Depth)
}
