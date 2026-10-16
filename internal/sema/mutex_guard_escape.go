package sema

import (
	"sec/internal/ast"
	"sec/internal/lexer"
)

// mutexGuardInType finds execution-bound guard storage, including references
// and aggregate payloads. Callable signatures and channel/execution handles do
// not themselves contain their parameter/message/result values.
// Rules: rules/concurrency/mutex.md — §§6(3),39(1–5),40(1);
// rules/memory/transferability.md — §§1(6),15,17;
// rules/analysis/escape_analysis.md — "ConcurrencyTransfer".
func mutexGuardInType(typ Type, visiting map[string]bool) (Type, bool) {
	return mutexGuardStorageType(typ, false, visiting)
}

// mutexGuardStorageType walks the canonical stored value graph for guard or
// callable-environment dependencies; signatures and phantom type arguments are
// not stored values.
// Rules: rules/memory/transferability.md — §§5(1–9),15,17.
func mutexGuardStorageType(typ Type, callable bool, visiting map[string]bool) (Type, bool) {
	if isMutexGuardType(typ) {
		return typ, !callable
	}
	if typ.Kind == FunctionType {
		return typ, callable
	}
	if typ.Kind == GenericType {
		// An unresolved substitution is not evidence that guard/callable
		// capabilities cannot be present (transferability.md §38).
		return typ, callable
	}
	switch typ.Name {
	case "Sender", "Receiver", "Channel", "Task", "Thread", "Event", "Subscription":
		return Type{}, false
	}
	key := typeDisplayName(typ)
	if visiting[key] {
		return Type{}, false
	}
	visiting[key] = true
	defer delete(visiting, key)
	if typ.NamedBase != nil {
		if guard, ok := mutexGuardStorageType(*typ.NamedBase, callable, visiting); ok {
			return guard, true
		}
	}
	if typ.Element != nil {
		if guard, ok := mutexGuardStorageType(*typ.Element, callable, visiting); ok {
			return guard, true
		}
	}
	for _, field := range typ.Fields {
		if guard, ok := mutexGuardStorageType(field.Type, callable, visiting); ok {
			return guard, true
		}
	}
	// Only compiler-owned carriers use type arguments as hidden storage.
	// An ordinary Phantom[T] with no T field does not contain a T value.
	if typ.Intrinsic || typ.Kind == ResultType {
		for _, argument := range typ.TypeArgs {
			if guard, ok := mutexGuardStorageType(argument, callable, visiting); ok {
				return guard, true
			}
		}
	}
	for _, variant := range typ.UnionVariants {
		if variant.Payload != nil {
			if guard, ok := mutexGuardStorageType(*variant.Payload, callable, visiting); ok {
				return guard, true
			}
		}
	}
	return Type{}, false
}

// mutexGuardInClosure consumes canonical environment/capture facts, including
// a captured callable's own environment. It never treats a callable's declared
// parameter or return types as evidence of a captured guard.
// Rules: rules/concurrency/mutex.md — §39(5),§40;
// rules/analysis/closure_analysis.md — "Closure environment as dependency carrier".
func (a *Analyzer) mutexGuardInClosure(environment AbstractClosureEnvironmentID, visiting map[AbstractClosureEnvironmentID]bool) (Type, lexer.Token, bool) {
	if environment == "" || visiting[environment] {
		return Type{}, lexer.Token{}, false
	}
	visiting[environment] = true
	defer delete(visiting, environment)
	for _, creation := range a.resolvedClosureCreations {
		if creation.Environment != environment {
			continue
		}
		for _, capture := range creation.Captures {
			if guard, ok := mutexGuardInType(capture.CapturedType, map[string]bool{}); ok {
				source := capture.SourcePlace.RootToken
				if source.Line == 0 {
					source = capture.Source
				}
				if symbol, ok := a.symbols[capture.SourcePlace.Root]; ok && symbol.Token == capture.SourcePlace.RootToken && symbol.mutexAcquisition != nil {
					source = symbol.mutexAcquisition.call.Token
				}
				return guard, source, true
			}
			if guard, source, ok := a.mutexGuardInClosure(capture.Dependencies.CallableEnvironment, visiting); ok {
				return guard, source, true
			}
		}
	}
	return Type{}, lexer.Token{}, false
}

// checkMutexGuardEscape rejects moving, borrowing or capturing a guard into a
// different execution/storage domain, retaining historical canonical escape
// facts and a navigable origin even when the transfer is invalid.
// Rules: rules/concurrency/mutex.md — §§39(1–5),40(1–2),63;
// rules/analysis/escape_analysis.md — "Canonical sinks", "ConcurrencyTransfer".
func (a *Analyzer) checkMutexGuardEscape(value ast.Expression, typ Type, sink lexer.Token, boundary string, destination EscapeDestination) bool {
	if a.summaryPass || !a.callGraphPathReachable {
		return false
	}
	guard, found := mutexGuardInType(typ, map[string]bool{})
	source := expressionToken(value)
	closureDependency := false
	if identity, ok := a.callableIdentityForExpression(explicitMoveSource(value)); ok && !found {
		guard, source, found = a.mutexGuardInClosure(identity.Environment, map[AbstractClosureEnvironmentID]bool{})
		closureDependency = found
	}
	if !found {
		return a.checkUnknownMutexGuardEnvironment(value, typ, sink, boundary, destination)
	}
	origin := EscapeSource{Kind: EscapeSourceTemporary, ParameterIndex: -1, Token: source}
	placeValue := explicitMoveSource(value)
	if ref, ok := placeValue.(*ast.RefExpression); ok {
		placeValue = ref.Value
	}
	if place, ok := a.resolvePlace(placeValue); ok {
		origin = a.mutexGuardEscapeSource(place, source)
		if symbol, ok := a.symbols[place.Root]; ok && !closureDependency {
			source = symbol.Token
			if symbol.mutexAcquisition != nil {
				source = symbol.mutexAcquisition.call.Token
			}
		}
	}
	if acquisition := a.mutexAcquisitionFromExpression(value); acquisition != nil && !acquisition.invalidated {
		source = acquisition.call.Token
		if resource := acquisition.resource.place.Type; isMutexType(resource) {
			guard = mutexGuardType(resource.TypeArgs[0])
		}
	}
	if a.escapeAnalysis != nil {
		subject := escapeSubjectForType(typ)
		if isMutexGuardType(typ) {
			subject = EscapeSubjectHandle
		}
		fact := EscapeFact{Callable: a.currentCallable, Subject: subject, Mode: EscapeModeConcurrencyTransfer,
			Destination: destination, Sources: []EscapeSource{origin}, Sink: sink}
		a.escapeAnalysis.facts = append(a.escapeAnalysis.facts, cloneEscapeFact(fact))
		if destination == EscapeDestinationTask || destination == EscapeDestinationThread {
			disposition := EscapeParameterTransferredToTask
			if destination == EscapeDestinationThread {
				disposition = EscapeParameterTransferredToThread
			}
			if origin.ParameterIndex >= 0 {
				summary := a.escapeAnalysis.ensureSummary(a.currentFunctionMetadata, a.currentCallable)
				a.escapeAnalysis.addParameterDisposition(summary, a.currentFunctionMetadata, origin.ParameterIndex, disposition)
			}
		}
	}
	before := len(a.errors)
	a.addErrorAtTokenWithPrevious(sink, source, "%s is bound to the current execution entity and cannot cross %s", typeDisplayName(guard), boundary)
	if len(a.errors) > before {
		a.errors[len(a.errors)-1].RelatedLabel = "execution-bound guard origin"
	}
	return true
}

// checkMutexGuardSpawn checks only transported arguments, receiver, callable
// environment and child result. A guard retained solely by the parent does
// not cross spawn; suspension restrictions are checked separately.
// Rules: rules/concurrency/mutex.md — §§28(3),39–40.
func (a *Analyzer) checkMutexGuardSpawn(spawn *ast.SpawnExpression, result Type) bool {
	if a.summaryPass || !a.callGraphPathReachable {
		return false
	}
	destination := EscapeDestinationTask
	if spawn.Kind == "thread" {
		destination = EscapeDestinationThread
	}
	invalid := false
	if _, contains := mutexGuardInType(result, map[string]bool{}); contains {
		invalid = a.checkMutexGuardEscape(spawn.Value, result, spawn.Token, "spawn result", destination)
	} else if _, unknown := mutexGuardStorageType(result, true, map[string]bool{}); unknown {
		// The spawned callable's environment is not the returned value's
		// environment. A child result needs its own value-specific facts.
		invalid = a.checkUnknownMutexGuardEnvironment(nil, result, spawn.Token, "spawn result", destination)
	}
	check := func(value ast.Expression) {
		if spread, ok := value.(*ast.SpreadExpression); ok {
			value = spread.Value
		}
		if a.checkMutexGuardEscape(value, a.expressionTypes[value], spawn.Token, "spawn", destination) {
			invalid = true
		}
	}
	switch value := spawn.Value.(type) {
	case *ast.CallExpression:
		for _, argument := range value.Arguments {
			check(argument)
		}
		check(value.Callee)
		if member, ok := value.Callee.(*ast.MemberExpression); ok {
			check(member.Object)
		}
	case *ast.LambdaExpression:
		check(value)
	}
	return invalid
}

// mutexGuardEscapeSource preserves canonical Place provenance without
// attributing a lambda parameter or a shadowing local to an outer parameter.
// Rules: rules/analysis/escape_analysis.md — "Parameter summary facts";
// rules/analysis/closure_analysis.md — "Abstract closure identity".
func (a *Analyzer) mutexGuardEscapeSource(place Place, fallback lexer.Token) EscapeSource {
	function := a.currentFunctionMetadata
	if a.inLambda || place.RootKind != PlaceRootParameter {
		function = Function{}
	}
	source := escapeSourceFromPlace(place, function, "", fallback)
	if a.inLambda && place.RootKind == PlaceRootParameter {
		source.Kind, source.ParameterIndex = EscapeSourceParameter, -1
	}
	return source
}
