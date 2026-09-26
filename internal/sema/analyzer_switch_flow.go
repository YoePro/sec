package sema

import "sec/internal/ast"

// ResolvedSwitchClauseFlow is the source-ordered frontend flow summary for one
// switch clause. It preserves explicit fallthrough separately from ordinary
// continuation so later analyses do not infer the edge from statement text.
type ResolvedSwitchClauseFlow struct {
	SourceIndex  int
	Default      bool
	ItemCount    int
	FallsThrough bool
	Continues    bool
}

// ResolvedSwitchFlow is the compiler-owned frontend coverage and clause-flow
// fact for one analyzed switch. It is intentionally narrower than the future
// cross-layer ResolvedSwitchPlan: comma-alternative test edges are not yet
// split into their final canonical CFG nodes.
//
// Rules:
//   - rules/control-flow/flowcontrol_switch.md — §5 "Case evaluation order"
//   - rules/control-flow/flowcontrol_switch.md — §16 "Explicit fallthrough"
//   - rules/control-flow/flowcontrol_switch.md — §32 "Switch termination"
//   - rules/control-flow/flowcontrol_switch.md — §33 "Definite assignment"
//   - rules/corrections/applied/correction24-20260823.md — "Architectural correction"
type ResolvedSwitchFlow struct {
	HasSubject       bool
	SubjectType      Type
	Exhaustive       bool
	HasUnmatchedPath bool
	Clauses          []ResolvedSwitchClauseFlow
}

// ResolvedSwitchFlowOf returns a defensive snapshot recorded by completed
// semantic analysis. It performs no inference and never reconstructs enum or
// boolean coverage from AST spelling.
func (a *Analyzer) ResolvedSwitchFlowOf(stmt *ast.SwitchStatement) (ResolvedSwitchFlow, bool) {
	if a == nil || stmt == nil {
		return ResolvedSwitchFlow{}, false
	}
	flow, ok := a.resolvedSwitchFlows[stmt]
	if !ok {
		return ResolvedSwitchFlow{}, false
	}
	flow.Clauses = append([]ResolvedSwitchClauseFlow(nil), flow.Clauses...)
	return flow, true
}

// recordResolvedSwitchFlow publishes already resolved switch coverage and
// source-ordered clause edges for return checking and future analyses.
//
// Rules:
//   - rules/control-flow/flowcontrol_switch.md — §5 "Case evaluation order"
//   - rules/control-flow/flowcontrol_switch.md — §32 "Switch termination"
//   - rules/compiler/semantic_ir.md — "Control flow"
func (a *Analyzer) recordResolvedSwitchFlow(stmt *ast.SwitchStatement, subjectType Type, hasSubject bool, exhaustive bool, clauses []ResolvedSwitchClauseFlow) {
	if a == nil || stmt == nil {
		return
	}
	a.resolvedSwitchFlows[stmt] = ResolvedSwitchFlow{
		HasSubject:       hasSubject,
		SubjectType:      subjectType,
		Exhaustive:       exhaustive,
		HasUnmatchedPath: !exhaustive,
		Clauses:          append([]ResolvedSwitchClauseFlow(nil), clauses...),
	}
}
