package sema

import (
	"sort"

	"sec/internal/lexer"
)

// TryAssignmentFlow is a detached presentation of a successfully resolved
// fallible assignment's edges. It carries no AST or mutable Type storage.
// HandlerFlows retain source order; a produces-value handler also represents
// normal completion for the assignment's void success type.
// Rules: rules/errors/errorhandling.md — §§16,21,23,34,36.
type TryAssignmentFlow struct {
	Callable          CallableID
	Source            lexer.Token
	Kind              ResolvedTryAssignmentKind
	ErrorType         string
	SuccessContinues  bool
	FailureContinues  bool
	FailurePropagates bool
	PropagationTarget string
	Exhaustive        bool
	HandlerFlows      []ResolvedTryHandlerFlow
}

// TryAssignmentFlows projects canonical resolved plans, not handler syntax.
// Invalid statements and assignments without a resolved callable are omitted.
// Fresh slices and scalar values detach every publication from analyzer state.
// Rules: rules/errors/errorhandling.md — §§23,34,36;
// rules/analysis/call_graph.md — "One canonical graph, multiple analysis views".
func (a *Analyzer) TryAssignmentFlows() []TryAssignmentFlow {
	if a == nil || a.callGraph == nil {
		return nil
	}
	var result []TryAssignmentFlow
	for statement, fact := range a.resolvedTryAssignments {
		if _, exists := a.callGraph.Node(fact.Callable); !exists {
			continue
		}
		flow := TryAssignmentFlow{Callable: fact.Callable, Source: statement.Token,
			Kind: fact.Kind, ErrorType: typeDisplayName(fact.ErrorType), SuccessContinues: true}
		switch fact.Kind {
		case ResolvedTryAssignmentPropagation:
			flow.FailurePropagates = true
			flow.PropagationTarget = typeDisplayName(fact.EnclosingResultType)
			if fact.TestBoundary {
				flow.PropagationTarget = "test invocation"
			}
		case ResolvedTryAssignmentHandled:
			flow.Exhaustive = fact.HandlerPlan.Exhaustive
			flow.FailurePropagates = fact.HandlerPlan.ResidualPropagates
			if flow.FailurePropagates {
				flow.PropagationTarget = typeDisplayName(fact.HandlerPlan.EnclosingResultType)
				if fact.HandlerPlan.ResidualTestBoundary {
					flow.PropagationTarget = "test invocation"
				}
			}
			for _, handler := range fact.HandlerPlan.Handlers {
				flow.HandlerFlows = append(flow.HandlerFlows, handler.Flow)
				flow.FailureContinues = flow.FailureContinues || handler.Flow == TryHandlerProducesValue
			}
		default:
			continue
		}
		result = append(result, flow)
	}
	sort.Slice(result, func(i, j int) bool {
		a, b := result[i], result[j]
		if a.Source.File != b.Source.File {
			return a.Source.File < b.Source.File
		}
		if a.Source.Line != b.Source.Line {
			return a.Source.Line < b.Source.Line
		}
		if a.Source.Column != b.Source.Column {
			return a.Source.Column < b.Source.Column
		}
		return a.Callable < b.Callable
	})
	return result
}
