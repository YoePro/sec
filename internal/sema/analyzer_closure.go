package sema

import (
	"fmt"

	"sec/internal/ast"
	"sec/internal/lexer"
)

// FunctionValueID identifies the source semantic origin of a function value.
// Repeated runtime evaluation may produce multiple values with this origin.
//
// Rules:
//   - rules/analysis/closure_analysis.md — "Function value"
type FunctionValueID string

// ClosureCreationSiteID identifies the source evaluation site of a lambda.
//
// Rules:
//   - rules/analysis/closure_analysis.md — "Closure instance"
type ClosureCreationSiteID string

// AbstractClosureEnvironmentID joins environments created at one creation site
// for finite analysis; it is never a runtime object identity.
//
// Rules:
//   - rules/analysis/closure_analysis.md — "Abstract closure identity"
type AbstractClosureEnvironmentID string

type CallableBodyKind string

const (
	CallableBodyNamedFunction      CallableBodyKind = "named-function"
	CallableBodyNonCapturingLambda CallableBodyKind = "non-capturing-lambda"
	CallableBodyCapturingLambda    CallableBodyKind = "capturing-lambda"
)

// ResolvedCallableIdentity records the exact body selected by a function-value
// expression and, for lambdas, the creation-site and optional abstract
// environment identity. AbstractsRuntimeInstances explicitly prevents clients
// from interpreting Environment as one concrete closure object.
//
// Rules:
//   - rules/analysis/closure_analysis.md — "Core semantic entities"
//   - rules/analysis/closure_analysis.md — "Abstract closure identity"
type ResolvedCallableIdentity struct {
	Kind                      CallableBodyKind
	Body                      CallableBodyID
	Value                     FunctionValueID
	Targets                   CallableTargetSet
	NamedTarget               CallableID
	CreationSite              ClosureCreationSiteID
	Environment               AbstractClosureEnvironmentID
	HasEnvironment            bool
	AbstractsRuntimeInstances bool
	Source                    lexer.Token
}

// ClosureCreationSummary is the canonical creation-site result consumed by
// later callable, escape, lifetime, and lowering analyses. Capture transfer and
// dependency facts remain ordered exactly as in the source capture list. The
// summary describes semantics only and chooses no physical environment layout
// or allocation strategy.
//
// Rules:
//   - rules/analysis/closure_analysis.md — "Callable creation"
//   - rules/analysis/closure_analysis.md — "Returning newly created closures"
//   - rules/analysis/closure_analysis.md — "Semantic IR requirements"
type ClosureCreationSummary struct {
	Body                      CallableBodyID
	Value                     FunctionValueID
	Targets                   CallableTargetSet
	CreationSite              ClosureCreationSiteID
	Environment               AbstractClosureEnvironmentID
	HasEnvironment            bool
	AbstractsRuntimeInstances bool
	Captures                  []CaptureRecord
	DependenciesKnown         bool
	Source                    lexer.Token
}

// recordNamedCallableIdentity publishes the exact declaration selected for an
// unambiguous named-function value. Overload resolution must complete before
// this fact is recorded.
//
// Rules:
//   - rules/analysis/closure_analysis.md — "Named functions"
//   - rules/analysis/closure_analysis.md — "Exact target sets"
func (a *Analyzer) recordNamedCallableIdentity(expression ast.Expression, function Function) {
	if a == nil || expression == nil || a.summaryPass {
		return
	}
	source := expressionToken(expression)
	target := callableID(function)
	body := callableBodyID(function)
	a.resolvedCallableIdentities[expression] = ResolvedCallableIdentity{
		Kind:        CallableBodyNamedFunction,
		Body:        body,
		Value:       FunctionValueID(a.callableIdentityKey("function-value", source)),
		Targets:     exactCallableTargetSet(body),
		NamedTarget: target,
		Source:      source,
	}
}

// recordLambdaCallableIdentity assigns distinct typed identities to the lambda
// body, its source function-value origin, its creation site, and (when
// capturing) the finite abstract environment. Invalid capture lists publish no
// callable identity fact.
//
// Rules:
//   - rules/analysis/closure_analysis.md — "Callable body", "Closure instance", "Closure environment"
//   - rules/analysis/closure_analysis.md — "Abstract closure identity"
func (a *Analyzer) recordLambdaCallableIdentity(lambda *ast.LambdaExpression) {
	if a == nil || lambda == nil || a.summaryPass {
		return
	}
	captureCount, duplicate := explicitCaptureCount(lambda)
	captures := a.resolvedLambdaCaptures[lambda]
	if duplicate || captureCount != len(captures) {
		return
	}
	source := lambda.Token
	base := a.callableIdentityKey("lambda", source)
	identity := ResolvedCallableIdentity{
		Kind:         CallableBodyNonCapturingLambda,
		Body:         CallableBodyID("callable-body|" + base),
		Value:        FunctionValueID("function-value|" + base),
		CreationSite: ClosureCreationSiteID("closure-site|" + base),
		Source:       source,
	}
	identity.Targets = exactCallableTargetSet(identity.Body)
	if captureCount != 0 {
		identity.Kind = CallableBodyCapturingLambda
		identity.Environment = AbstractClosureEnvironmentID("abstract-environment|" + base)
		identity.HasEnvironment = true
		identity.AbstractsRuntimeInstances = true
	}
	a.resolvedCallableIdentities[lambda] = identity
	a.recordClosureCreationSummary(lambda, identity, captures)
}

// recordFunctionValueCall connects an indirect invocation to the canonical
// callable target set already proven for its callee value. Unknown values are
// deliberately omitted until a sound open callable contract exists.
//
// Rules:
//   - rules/analysis/call_graph.md — "Call-site record" and "Dispatch kinds"
//   - rules/analysis/closure_analysis.md — "Callable-flow analysis"
//   - rules/analysis/closure_analysis.md — "Soundness of target sets"
func (a *Analyzer) recordFunctionValueCall(call *ast.CallExpression) {
	if a == nil || call == nil || a.summaryPass || a.currentCallable == "" {
		return
	}
	identity, ok := a.callableIdentityForExpression(call.Callee)
	if !ok {
		return
	}
	dispatch := CallDispatchFunctionValue
	if identity.HasEnvironment {
		dispatch = CallDispatchClosure
	}
	a.callGraph.addTargetSetCall(a.currentCallable, identity.Targets, call.Token, dispatch, CallExecutionSynchronous)
}

// recordClosureCreationSummary joins already-validated identity, target, and
// capture facts into one immutable compiler result. It performs no new capture
// inference and therefore cannot turn an invalid capture into a summary.
//
// Rules:
//   - rules/analysis/closure_analysis.md — "Capturing lambda"
//   - rules/analysis/closure_analysis.md — "Capture record"
//   - rules/analysis/closure_analysis.md — "Returning newly created closures"
func (a *Analyzer) recordClosureCreationSummary(lambda *ast.LambdaExpression, identity ResolvedCallableIdentity, captures []CaptureRecord) {
	if a == nil || lambda == nil {
		return
	}
	a.resolvedClosureCreations[lambda] = ClosureCreationSummary{
		Body:                      identity.Body,
		Value:                     identity.Value,
		Targets:                   cloneCallableTargetSet(identity.Targets),
		CreationSite:              identity.CreationSite,
		Environment:               identity.Environment,
		HasEnvironment:            identity.HasEnvironment,
		AbstractsRuntimeInstances: identity.AbstractsRuntimeInstances,
		Captures:                  cloneCaptureRecords(captures),
		DependenciesKnown:         true,
		Source:                    identity.Source,
	}
}

// callableIdentityKey creates deterministic snapshot-local source identity
// without treating it as runtime object identity.
//
// Rules:
//   - rules/analysis/closure_analysis.md — "Callable body"
//   - rules/analysis/closure_analysis.md — "Closure creation in loops and recursion"
func (a *Analyzer) callableIdentityKey(kind string, source lexer.Token) string {
	return fmt.Sprintf("%s|%s|%s|%s:%d:%d", kind, a.currentModule, a.currentCallable, source.File, source.Line, source.Column)
}

// explicitCaptureCount counts unique syntactic captures while retaining a
// duplicate marker so invalid capture lists cannot acquire semantic identities.
//
// Rules:
//   - rules/declarations/lambda-functions.md — §21 "Capture name resolution"
func explicitCaptureCount(lambda *ast.LambdaExpression) (int, bool) {
	seen := map[string]bool{}
	count := 0
	duplicate := false
	for _, capture := range lambda.Captures {
		if capture.Name == nil {
			continue
		}
		if seen[capture.Name.Value] {
			duplicate = true
			continue
		}
		seen[capture.Name.Value] = true
		count++
	}
	return count, duplicate
}

// ResolvedCallableIdentityOf returns the exact immutable callable identity fact
// recorded for a named-function value or lambda expression. It performs no
// inference and never manufactures a runtime closure-instance identity.
//
// Rules:
//   - rules/analysis/closure_analysis.md — "Core semantic entities"
//   - rules/compiler/compiler_analysis.md — immutable analysis results
func (a *Analyzer) ResolvedCallableIdentityOf(expression ast.Expression) (ResolvedCallableIdentity, bool) {
	if a == nil || expression == nil {
		return ResolvedCallableIdentity{}, false
	}
	identity, ok := a.resolvedCallableIdentities[expression]
	return cloneResolvedCallableIdentity(identity), ok
}

// cloneResolvedCallableIdentity detaches the finite target set before a
// callable identity crosses a symbol or Analyzer snapshot boundary.
//
// Rules:
//   - rules/analysis/closure_analysis.md — "Callable target sets"
//   - rules/compiler/compiler_analysis.md — immutable analysis results
func cloneResolvedCallableIdentity(identity ResolvedCallableIdentity) ResolvedCallableIdentity {
	identity.Targets = cloneCallableTargetSet(identity.Targets)
	return identity
}

// cloneCallableTargetSet detaches the finite known-target list for immutable
// snapshot publication.
//
// Rules:
//   - rules/analysis/closure_analysis.md — "Callable target sets"
func cloneCallableTargetSet(targets CallableTargetSet) CallableTargetSet {
	targets.KnownTargets = append([]CallableBodyID(nil), targets.KnownTargets...)
	return targets
}

// ResolvedClosureCreationOf returns a defensive snapshot of one successfully
// analyzed lambda creation. Absence means no valid creation summary was
// recorded; it never implies a non-capturing or environment-free proof.
//
// Rules:
//   - rules/analysis/closure_analysis.md — "Callable creation"
//   - rules/compiler/compiler_analysis.md — immutable analysis results
func (a *Analyzer) ResolvedClosureCreationOf(lambda *ast.LambdaExpression) (ClosureCreationSummary, bool) {
	if a == nil || lambda == nil {
		return ClosureCreationSummary{}, false
	}
	summary, ok := a.resolvedClosureCreations[lambda]
	if !ok {
		return ClosureCreationSummary{}, false
	}
	summary.Targets = cloneCallableTargetSet(summary.Targets)
	summary.Captures = cloneCaptureRecords(summary.Captures)
	return summary, true
}
