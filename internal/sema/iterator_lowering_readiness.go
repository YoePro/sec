package sema

import (
	"fmt"
	"sort"
	"strings"

	"sec/internal/ast"
	"sec/internal/diagnostics"
	"sec/internal/lexer"
)

// IteratorLoweringIssue distinguishes invalid source from missing positive
// iteration proof. It is an internal pipeline result, not a new Sec diagnostic.
// Rules: rules/compiler/compiler_pipeline.md — §§2(5–7), 32–34.
type IteratorLoweringIssue struct {
	Source         lexer.Token
	Classification diagnostics.ProofState // Invalid or Unproven
	Reason         string
}

// IteratorLoweringReadinessError prevents publication of partial Semantic IR
// when mandatory iterator resolution is missing or invalid.
// Rules: rules/compiler/compiler_pipeline.md — §§23(3–4), 33(1,3), 34(2–3).
type IteratorLoweringReadinessError struct{ Issues []IteratorLoweringIssue }

// Error presents deterministic source-specific pipeline evidence.
// Rules: rules/compiler/compiler_pipeline.md — §33(3).
func (e *IteratorLoweringReadinessError) Error() string {
	var messages []string
	for _, issue := range e.Issues {
		where := ""
		if issue.Source.Line > 0 {
			where = fmt.Sprintf(" at %s:%d:%d", issue.Source.File, issue.Source.Line, issue.Source.Column)
		}
		messages = append(messages, fmt.Sprintf("iterator lowering readiness %s%s: %s", issue.Classification, where, issue.Reason))
	}
	return strings.Join(messages, "; ")
}

type iteratorLoweringRequirement struct {
	module                                         string
	caller                                         CallableID
	source                                         lexer.Token
	iterable                                       ast.Expression
	step                                           ast.Expression
	bindings                                       []ast.ForBinding
	conformance                                    string
	next                                           CallableID
	reachable, required, invalid, conformanceValid bool
}

// beginIteratorLoweringRequirement records a final-body obligation before
// iterable checking, so a missing plan cannot erase the requirement itself.
// Summary and speculative backedge probes never replace final-body evidence.
// Rules: rules/compiler/compiler_pipeline.md — §§23(3), 33(1,3);
// rules/compiler/compiler_analysis.md — §18; rules/control-flow/flowcontrol_for.md — §37.
func (a *Analyzer) beginIteratorLoweringRequirement(stmt *ast.ForStatement) func() {
	if a.summaryPass || a.iterationDependencyProbe {
		return func() {}
	}
	requirement := &iteratorLoweringRequirement{module: a.currentModule, caller: a.currentCallable,
		source: stmt.Token, iterable: stmt.Iterable, step: stmt.Step,
		bindings: append([]ast.ForBinding(nil), stmt.Bindings...), reachable: a.callGraphPathReachable}
	if a.iteratorLoweringRequirements == nil {
		a.iteratorLoweringRequirements = map[*ast.ForStatement]*iteratorLoweringRequirement{}
	}
	a.iteratorLoweringRequirements[stmt] = requirement
	errorsAtEntry := len(a.errors)
	return func() { requirement.invalid = len(a.errors) > errorsAtEntry }
}

// ValidateIteratorLoweringReadiness consumes Sema-owned obligations and plans
// for reachable protocol loops of the requested module. It performs no name,
// method or conformance discovery and proves no backend capability. Built-in
// categories do not require a protocol plan. This is the iterator-specific
// prerequisite, not the complete CompilationPlan-qualified completion gate.
// Rules: rules/compiler/compiler_pipeline.md — §§23(3–4), 32(1–5), 33(1–3), 34(2–3);
// rules/compiler/semantic_ir.md — §§64(2–5), 65(3,5–6).
func (a *Analyzer) ValidateIteratorLoweringReadiness(program *ast.Program, module string) error {
	issues := []IteratorLoweringIssue{}
	if a == nil || program == nil || a.iteratorLoweringProgram != program {
		return &IteratorLoweringReadinessError{Issues: []IteratorLoweringIssue{{Classification: "Unproven", Reason: "no completed iterator facts for this program snapshot"}}}
	}
	for stmt, requirement := range a.iteratorLoweringRequirements {
		if requirement.module != module || !requirement.required || !requirement.reachable {
			continue
		}
		issue := IteratorLoweringIssue{Source: requirement.source, Classification: "Unproven"}
		if requirement.invalid || !requirement.conformanceValid {
			issue.Classification = "Invalid"
			issue.Reason = "iterator conformance, source, binding or body failed semantic validation"
		} else {
			issue.Reason = a.iteratorPlanReadinessReason(stmt, requirement)
		}
		if issue.Reason != "" {
			issues = append(issues, issue)
		}
	}
	if len(issues) == 0 {
		return nil
	}
	sort.Slice(issues, func(i, j int) bool {
		left, right := issues[i].Source, issues[j].Source
		if left.File != right.File {
			return left.File < right.File
		}
		if left.Line != right.Line {
			return left.Line < right.Line
		}
		if left.Column != right.Column {
			return left.Column < right.Column
		}
		return issues[i].Reason < issues[j].Reason
	})
	return &IteratorLoweringReadinessError{Issues: issues}
}

// iteratorPlanReadinessReason checks completeness and agreement of existing
// canonical resolution, storage/lifetime and invocation facts. It does not
// repair absent evidence or re-run interface resolution in a lower stage.
// Rules: rules/compiler/semantic_ir.md — §§64(2–5), 65(3,5–6);
// rules/compiler/compiler_pipeline.md — §§33(1,3), 34(2–3).
func (a *Analyzer) iteratorPlanReadinessReason(stmt *ast.ForStatement, requirement *iteratorLoweringRequirement) string {
	plan, ok := a.resolvedForIterations[stmt]
	if !ok {
		return "missing ResolvedForIteration plan"
	}
	if stmt.Iterable != requirement.iterable || stmt.Step != requirement.step || !sameSourceToken(stmt.Token, requirement.source) || len(stmt.Bindings) != len(requirement.bindings) {
		return "iterator syntax no longer matches the analyzed snapshot"
	}
	for index, binding := range stmt.Bindings {
		if binding != requirement.bindings[index] {
			return "iterator binding no longer matches the analyzed snapshot"
		}
	}
	if plan.Kind != ForIterationCompilerKnownIterator || plan.ElementType.Kind == InvalidType ||
		!plan.Conformance.Intrinsic || plan.Conformance.Kind != InterfaceType || plan.Conformance.Name != "Iterator" ||
		graphTypeIdentity(plan.Conformance) != requirement.conformance ||
		len(plan.Conformance.TypeArgs) != 1 || graphTypeIdentity(plan.Conformance.TypeArgs[0]) != graphTypeIdentity(plan.ElementType) {
		return "incomplete resolved Iterator[T] conformance or yield type"
	}
	if typ, known := a.expressionTypes[requirement.iterable]; !known || graphTypeIdentity(typ) != graphTypeIdentity(plan.SourceType) {
		return "iterator source type disagrees with resolved expression facts"
	}
	if plan.NextCallable == "" || plan.NextCallable != requirement.next || plan.NextCallable != callableID(plan.Next) || plan.Next.Static ||
		len(explicitInterfaceComparableParameters(plan.Next.Parameters)) != 0 || plan.Next.ReturnType.Name != "Option" ||
		len(plan.Next.ReturnType.TypeArgs) != 1 || graphTypeIdentity(plan.Next.ReturnType.TypeArgs[0]) != graphTypeIdentity(plan.ElementType) {
		return "missing or inconsistent concrete Next() Option[T] target"
	}
	if a.callGraph == nil {
		return "missing Next call-graph facts"
	}
	if _, known := a.callGraph.nodes[plan.NextCallable]; !known {
		return "resolved Next callable is absent from the call graph"
	}
	matched := false
	for _, site := range a.callGraph.sites {
		if site.Caller == requirement.caller && sameSourceToken(site.Source, requirement.source) &&
			site.Dispatch == CallDispatchStaticMethod && site.Execution == CallExecutionSynchronous &&
			site.TargetSet.IsClosed && len(site.Targets) == 1 && site.Targets[0] == plan.NextCallable {
			matched = true
			break
		}
	}
	if !matched {
		return "missing canonical static Next invocation"
	}
	if len(stmt.Bindings) != 1 || stmt.Bindings[0].Mode != ast.ForBindingValue ||
		(plan.Binding != ForIteratorOwnedValueBinding && plan.Binding != ForIteratorDiscardBinding) ||
		(stmt.Bindings[0].Discard != (plan.Binding == ForIteratorDiscardBinding)) {
		return "missing or inconsistent owned-value/discard binding plan"
	}
	dependencies, known := a.iterationDependencies[stmt]
	if !known || !dependencies.BodyValid || dependencies.Callable != requirement.caller ||
		graphTypeIdentity(dependencies.SourceType) != graphTypeIdentity(plan.SourceType) || dependencies.Source != plan.Source ||
		dependencies.TemporaryDestroyedAtExit != plan.DestroysTemporary {
		return "missing or inconsistent iterator storage/lifetime facts"
	}
	if !plan.RequiresMutableReceiver {
		return "missing iterator advancement authority"
	}
	switch plan.Source {
	case ForIteratorReusableStorage:
		if plan.SourcePlace.Root == "" || !plan.SourcePlace.Addressable || !plan.SourcePlace.Mutable || plan.DestroysTemporary ||
			Relationship(plan.SourcePlace, dependencies.SourcePlace) != PlaceSame ||
			sourceTokenLocation(plan.SourcePlace.RootToken) != sourceTokenLocation(dependencies.SourcePlace.RootToken) {
			return "incomplete reusable iterator storage plan"
		}
	case ForIteratorFreshTemporary:
		if plan.SourcePlace.Root != "" {
			return "fresh iterator temporary unexpectedly names reusable storage"
		}
	default:
		return "missing iterator source ownership mode"
	}
	return ""
}

// SemanticDiagnostics exports each readiness failure as an independent,
// source-local mandatory occurrence. Missing required evidence stays Unproven;
// an unknown producer classification cannot be promoted to a proven violation.
// Returned diagnostics are detached from the stored proof issues.
// Rules: rules/compiler/compiler_analysis.md — §7(4–8), §58(3);
// rules/compiler/compiler_pipeline.md — §33(3);
// rules/tooling/diagnostics.md — §§5, 8–10, 14.
func (e *IteratorLoweringReadinessError) SemanticDiagnostics() []Error {
	result := make([]Error, 0, len(e.Issues))
	for _, issue := range e.Issues {
		state, id, help := diagnostics.ProofUnproven, diagnostics.RequiredProofUnavailable,
			"Lowering requires positive analysis evidence for the current snapshot. Complete or refresh the missing analysis; this failure does not prove a source rule violation."
		if issue.Classification == diagnostics.ProofInvalid {
			state, id, help = diagnostics.ProofInvalid, diagnostics.RequiredAnalysisInvalid,
				"Correct the source violation reported by the owning semantic analysis before requesting lowering."
		}
		endLine, endColumn := issue.Source.EndPosition()
		result = append(result, Error{ID: id, Severity: diagnostics.SeverityError, ProofState: state,
			Message: "iterator lowering readiness: " + issue.Reason, Help: help,
			File: issue.Source.File, Line: issue.Source.Line, Column: issue.Source.Column,
			EndLine: endLine, EndColumn: endColumn})
	}
	return result
}
