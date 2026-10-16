package sema

import (
	"sec/internal/ast"
	"sec/internal/diagnostics"
	"sec/internal/lexer"
)

// mutexGuardEnvironmentUnknown distinguishes a proven environment with no
// guard dependency from a callable whose value-specific captures are unknown.
// Nested callable captures use their canonical dependency summaries.
// Rules: rules/memory/transferability.md — §§17(3–5),39(4),40(5–6).
func (a *Analyzer) mutexGuardEnvironmentUnknown(environment AbstractClosureEnvironmentID, visiting map[AbstractClosureEnvironmentID]bool) bool {
	if environment == "" {
		return false
	}
	if visiting[environment] {
		return true
	}
	visiting[environment] = true
	defer delete(visiting, environment)
	for _, creation := range a.resolvedClosureCreations {
		if creation.Environment != environment {
			continue
		}
		if !creation.DependenciesKnown {
			return true
		}
		for _, capture := range creation.Captures {
			if _, contains := mutexGuardStorageType(capture.CapturedType, true, map[string]bool{}); !contains {
				continue
			}
			if capture.CapturedType.Kind != FunctionType || !capture.Dependencies.CallableDependenciesKnown ||
				a.mutexGuardEnvironmentUnknown(capture.Dependencies.CallableEnvironment, visiting) {
				return true
			}
		}
		return false
	}
	return true
}

// checkUnknownMutexGuardEnvironment withholds permission when an opaque
// callable or aggregate could hide execution-bound captures. This is Unproven,
// never a claim that an actual guard was found. Ordinary local calls/returns do
// not require this execution-boundary proof.
// Rules: rules/memory/transferability.md — §§17(4–5),39(4–5),40(5–6);
// rules/compiler/compiler_analysis.md — §7(4–8);
// rules/tooling/diagnostics.md — §§14–15.
func (a *Analyzer) checkUnknownMutexGuardEnvironment(value ast.Expression, typ Type, sink lexer.Token, boundary string, destination EscapeDestination) bool {
	if a.summaryPass || !a.callGraphPathReachable {
		return false
	}
	if _, contains := mutexGuardStorageType(typ, true, map[string]bool{}); !contains {
		return false
	}
	if typ.Kind == FunctionType {
		if identity, known := a.callableIdentityForExpression(explicitMoveSource(value)); known &&
			!a.mutexGuardEnvironmentUnknown(identity.Environment, map[AbstractClosureEnvironmentID]bool{}) {
			return false
		}
	}
	// A fresh aggregate exposes its actual callable values. A stored aggregate
	// needs canonical per-member callable facts; its shape alone is no proof.
	switch value := explicitMoveSource(value).(type) {
	case *ast.StructLiteral:
		unknown := false
		for _, field := range value.Fields {
			if field != nil && a.checkMutexGuardEscape(field.Value, a.expressionTypes[field.Value], sink, boundary, destination) {
				unknown = true
			}
		}
		return unknown
	case *ast.ArrayLiteral:
		unknown := false
		for _, element := range value.Elements {
			if a.checkMutexGuardEscape(element, a.expressionTypes[element], sink, boundary, destination) {
				unknown = true
			}
		}
		return unknown
	}
	before := len(a.errors)
	a.addErrorAtTokenWithMetadata(sink, diagnostics.RequiredProofUnavailable,
		"Keep the value in the current execution, or supply concrete value and capture dependencies that the compiler can prove safe at this boundary.",
		"cannot prove value dependencies are free of execution-bound mutex guards across %s", boundary)
	if len(a.errors) > before {
		a.errors[len(a.errors)-1].ProofState = diagnostics.ProofUnproven
	}
	if a.escapeAnalysis != nil {
		source := EscapeSource{Kind: EscapeSourceUnknown, ParameterIndex: -1, Token: expressionToken(value)}
		if source.Token.Line == 0 {
			source.Token = sink
		}
		if place, ok := a.resolvePlace(explicitMoveSource(value)); ok {
			source = a.mutexGuardEscapeSource(place, expressionToken(value))
		}
		fact := EscapeFact{Callable: a.currentCallable, Subject: escapeSubjectForType(typ), Mode: EscapeModeConcurrencyTransfer,
			Destination: destination, Sources: []EscapeSource{source}, Sink: sink, Unknown: true}
		a.escapeAnalysis.facts = append(a.escapeAnalysis.facts, cloneEscapeFact(fact))
		if source.ParameterIndex >= 0 && (destination == EscapeDestinationTask || destination == EscapeDestinationThread) {
			disposition := EscapeParameterTransferredToTask
			if destination == EscapeDestinationThread {
				disposition = EscapeParameterTransferredToThread
			}
			summary := a.escapeAnalysis.ensureSummary(a.currentFunctionMetadata, a.currentCallable)
			summary.Unknown = true
			a.escapeAnalysis.addParameterDisposition(summary, a.currentFunctionMetadata, source.ParameterIndex, disposition)
		}
	}
	return true
}
