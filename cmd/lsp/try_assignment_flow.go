package main

import (
	"fmt"
	"strings"

	"sec/internal/sema"
)

// withTryAssignmentDetail projects resolved assignment counts on prepared,
// incoming and outgoing hierarchy items, without creating synthetic callees
// for propagation edges or guessing whether an assignment has local handlers.
// Rules: rules/errors/errorhandling.md — §§23,36; rules/tooling/lsp.md — "Call hierarchy".
func withTryAssignmentDetail(item callHierarchyItem, analyzer *sema.Analyzer, id sema.CallableID) callHierarchyItem {
	propagated, handled := 0, 0
	for _, flow := range analyzer.TryAssignmentFlows() {
		if flow.Callable != id {
			continue
		}
		if flow.FailurePropagates {
			propagated++
		}
		if flow.Kind == sema.ResolvedTryAssignmentHandled {
			handled++
		}
	}
	if propagated+handled != 0 {
		if item.Detail != "" {
			item.Detail += " · "
		}
		item.Detail += fmt.Sprintf("fallible assignments: %d propagate, %d local handlers", propagated, handled)
	}
	return item
}

// tryAssignmentFlowHoverLines renders the callable's compiler-resolved flow
// view, including success, continuing handlers and unmatched propagation.
// Rules: rules/errors/errorhandling.md — §§16,21,23,36; rules/tooling/lsp.md — "Hover".
func tryAssignmentFlowHoverLines(analyzer *sema.Analyzer, id sema.CallableID) []string {
	var lines []string
	for _, flow := range analyzer.TryAssignmentFlows() {
		if flow.Callable != id {
			continue
		}
		line := fmt.Sprintf("Fallible assignment at %s:%d:%d: `%s`", flow.Source.File, flow.Source.Line, flow.Source.Column, flow.ErrorType)
		if flow.SuccessContinues {
			line += "; success continues"
		}
		if flow.Kind == sema.ResolvedTryAssignmentHandled {
			coverage := "partial"
			if flow.Exhaustive {
				coverage = "exhaustive"
			}
			line += "; " + coverage + " local handlers"
			if flow.FailureContinues {
				line += "; handled failure may continue"
			} else {
				line += "; selected handlers leave control flow"
			}
			flows := make([]string, len(flow.HandlerFlows))
			for i, handler := range flow.HandlerFlows {
				flows[i] = string(handler)
			}
			line += " (`" + strings.Join(flows, "`, `") + "`)"
		}
		if flow.FailurePropagates {
			line += "; failure propagates to `" + flow.PropagationTarget + "`"
		}
		lines = append(lines, line)
	}
	return lines
}
