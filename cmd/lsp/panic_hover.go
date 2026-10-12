package main

import (
	"fmt"
	"sec/internal/diagnostics"
	"sec/internal/sema"
	"strings"
)

// panicHoverLines presents the compiler-owned panic summary of a callable:
// whether a synchronous path can panic, its direct panic sources with their
// registered reasons, and the cause path to the introducing callable.
//
// Rules:
//   - rules/errors/panic.md — § 21 "@noPanic", registered panic reasons
//   - rules/tooling/lsp.md — "Hover": effect and panic paths
func panicHoverLines(graph *sema.CallGraph, id sema.CallableID) []string {
	summary := graph.EffectSummary(id)
	if !summary.MayPanic {
		return []string{"May panic: `no` (no reachable panic source in the current analysis)"}
	}
	lines := []string{"May panic: `yes`"}
	for _, effect := range summary.DirectEffects {
		if !sema.IsPanicEffectKind(effect.Kind) {
			continue
		}
		reasons := make([]string, 0, len(effect.PanicReasonIDs))
		for _, reason := range effect.PanicReasonIDs {
			if definition, ok := diagnostics.PanicReasonByID(reason); ok {
				reasons = append(reasons, definition.Name)
			}
		}
		line := fmt.Sprintf("Panic source: `%s` at %d:%d", effect.Kind, effect.Source.Line, effect.Source.Column)
		if len(reasons) > 0 {
			line += " (" + strings.Join(reasons, ", ") + ")"
		}
		lines = append(lines, line)
	}
	for _, cause := range summary.PanicCauses {
		if len(cause.Path) == 0 {
			if cause.Effect.HasMessage {
				lines = append(lines, fmt.Sprintf("Panic message: %q", cause.Effect.Message))
			}
			continue
		}
		node, _ := graph.Node(cause.Callable)
		line := fmt.Sprintf("Panic cause in `%s`: `%s` at %s:%d:%d", node.Name, cause.Effect.Kind, cause.Source.File, cause.Source.Line, cause.Source.Column)
		for _, reason := range cause.Effect.PanicReasonIDs {
			if definition, ok := diagnostics.PanicReasonByID(reason); ok {
				line += " (`" + definition.Name + "`)"
			}
		}
		if cause.Effect.HasMessage {
			line += fmt.Sprintf("; message %q", cause.Effect.Message)
		}
		lines = append(lines, line)
		for _, site := range cause.Path {
			caller, _ := graph.Node(site.Caller)
			lines = append(lines, fmt.Sprintf("Called from `%s` at %s:%d:%d (%s, %s)", caller.Name, site.Source.File, site.Source.Line, site.Source.Column, site.Dispatch, site.Execution))
		}
	}
	if len(summary.PanicPath) > 1 {
		path := make([]string, 0, len(summary.PanicPath))
		for _, step := range summary.PanicPath {
			if node, ok := graph.Node(step); ok {
				path = append(path, node.Name)
			}
		}
		lines = append(lines, "Panic path: `"+strings.Join(path, "` -> `")+"`")
	}
	return lines
}
