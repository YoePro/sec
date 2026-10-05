package sema

import (
	"fmt"
	"strings"
)

// AnalysisDepth selects an analysis resource budget, never language semantics.
// Required safety proofs remain enabled at every depth; cheaper depths may
// widen optional precision earlier.
type AnalysisDepth string

const (
	AnalysisInteractive AnalysisDepth = "interactive"
	AnalysisStandard    AnalysisDepth = "standard"
	AnalysisDeep        AnalysisDepth = "deep"
)

// AnalysisBudget contains finite limits used by the current analysis slices.
// Zero MaxSummaryIterations means to use the finite lattice-derived bound.
// MaxPitfallNodes bounds syntax preflight across the snapshot; MaxPitfallDepth
// bounds nesting within an admitted unit. Zero in either skips optional search.
type AnalysisBudget struct {
	MaxSummaryIterations int
	MaxPitfallNodes      int
	MaxPitfallDepth      int
}

func ParseAnalysisDepth(value string) (AnalysisDepth, error) {
	depth := AnalysisDepth(strings.ToLower(strings.TrimSpace(value)))
	switch depth {
	case AnalysisInteractive, AnalysisStandard, AnalysisDeep:
		return depth, nil
	default:
		return "", fmt.Errorf("unsupported analysis depth %q (want interactive, standard, or deep)", value)
	}
}

// analysisBudget assigns increasing optional search limits without changing safety proofs.
// Rules: rules/compiler/compiler_analysis.md — §§14–15;
// rules/analysis/pitfall_analysis.md — "Interactive, Standard, and Deep analysis".
func analysisBudget(depth AnalysisDepth) AnalysisBudget {
	switch depth {
	case AnalysisInteractive:
		// Editor analysis widens recursive/direct-call return-origin propagation
		// after a small number of global passes instead of monopolizing the UI.
		return AnalysisBudget{MaxSummaryIterations: 4, MaxPitfallNodes: 4096, MaxPitfallDepth: 64}
	case AnalysisDeep:
		return AnalysisBudget{MaxPitfallNodes: 262144, MaxPitfallDepth: 256}
	default:
		return AnalysisBudget{MaxPitfallNodes: 32768, MaxPitfallDepth: 128}
	}
}
