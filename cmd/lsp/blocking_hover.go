package main

import (
	"fmt"
	"sec/internal/sema"
	"strings"
)

// blockingHoverLine presents the compiler-owned blocking summary.
//
// Rules:
//   - rules/analysis/effect_analysis.md — §§5,30
//   - rules/foundations/attributes.md — §81(1)
//   - rules/tooling/lsp.md — §4.1(7)
func blockingHoverLine(graph *sema.CallGraph, id sema.CallableID) string {
	summary := graph.BlockSummary(id)
	if !summary.MayBlock {
		return "May block: `no`"
	}
	names := []string{}
	for _, step := range summary.BlockPath {
		if node, ok := graph.Node(step); ok {
			names = append(names, node.Name)
		}
	}
	line := "May block: `yes` via `" + strings.Join(names, "` -> `") + "`"
	if direct := graph.BlockSummary(summary.BlockPath[len(summary.BlockPath)-1]).DirectEffects; len(direct) > 0 {
		line += " (" + blockingEffectHoverCause(direct[0]) + ")"
		if len(direct) > 1 {
			evidence := make([]string, 0, len(direct))
			for _, effect := range direct {
				evidence = append(evidence, blockingEffectHoverCause(effect))
			}
			line += "\n\nBlocking evidence: " + strings.Join(evidence, "; ")
		}
	}
	return line
}

// blockingEffectHoverCause renders existing conservative effect kinds in
// readable terms, without upgrading an unknown call into a proven physical wait.
// Rules: rules/analysis/effect_analysis.md — §§5,30;
// rules/foundations/attributes.md — §81(1); rules/tooling/lsp.md — §4.1(7).
func blockingEffectHoverCause(effect sema.BlockEffectSite) string {
	cause := effect.Operation
	switch effect.Kind {
	case sema.BlockEffectForeign:
		cause = "unverified foreign call (blocking behavior unknown)"
	case sema.BlockEffectUnknownCallee:
		cause = "unresolved call target (blocking behavior unknown)"
	default:
		if cause == "" {
			cause = "potentially blocking operation"
		}
	}
	return fmt.Sprintf("%s at %d:%d", cause, effect.Source.Line, effect.Source.Column)
}
