package sema

import "sec/internal/ast"

// checkEventHandlerArgument validates the handler of Event[T].Subscribe under
// the owner-approved borrowed contract (MD-134, 2026-11-13):
//
//   - the callable type is fn(ref T) void; the payload is borrowed, never
//     owned (events.md §§ 10(1), 11(2));
//   - the handler is reusable: a consuming callable is rejected (§ 10(2));
//   - a stateless callable (a named function or a capture-free lambda) is
//     passed as a function value, because no environment is retained
//     (§§ 6(7), 10(4));
//   - a capturing callable stays in externally owned storage and is passed as
//     `ref handler`, or as `ref mut handler` when its inferred capability
//     mutates its environment; the event never copies, moves or allocates it
//     (§§ 6(4), 6(7), 10(2)).
//
// The retained-borrow lifetime and owner-pinning obligations of a live
// Subscription are separate, still-open analyses (concurrency.events-v2).
func (a *Analyzer) checkEventHandlerArgument(argument ast.Expression, payload Type) bool {
	expected := eventHandlerType(payload)
	if reference, retained := argument.(*ast.RefExpression); retained {
		return a.checkRetainedEventHandler(reference, expected)
	}
	handlerType, _ := a.inferExpression(argument)
	if handlerType.Kind == InvalidType {
		return false
	}
	if !a.eventHandlerSignature(argument, handlerType, expected) {
		return false
	}
	if normalizedCallableCapability(handlerType.FunctionCapability) == CallableConsuming {
		a.reportConsumingEventHandler(argument, handlerType)
		return false
	}
	identity, known := a.callableIdentityForExpression(argument)
	if !known || identity.HasEnvironment || normalizedCallableCapability(handlerType.FunctionCapability) != CallableShared {
		token := expressionToken(argument)
		a.addErrorAtTokenWithMetadata(token, "",
			"A capturing or environment-mutating handler is retained through its external storage: bind it with let and pass `ref handler`, or `ref mut handler` when it mutates its environment. A named function or capture-free lambda may be passed directly.",
			"Event.Subscribe cannot retain this handler by value; a capturing callable must stay in its own storage and be subscribed as ref or ref mut")
		return false
	}
	return true
}

// checkRetainedEventHandler validates `Subscribe(ref handler)` and
// `Subscribe(ref mut handler)`: the referent is a named, addressable callable
// of type fn(ref T) void whose capability the reference authority covers.
func (a *Analyzer) checkRetainedEventHandler(reference *ast.RefExpression, expected Type) bool {
	if _, named := reference.Value.(*ast.Identifier); !named {
		a.addErrorAtTokenWithMetadata(reference.Token, "",
			"Bind the callable with let first; the event retains a reference to that storage for the life of the subscription.",
			"Event.Subscribe retains a reference to externally owned callable storage; ref must name a callable binding")
		return false
	}
	referenceType, _ := a.inferExpression(reference)
	if referenceType.Kind == InvalidType {
		return false
	}
	if referenceType.Kind != ReferenceType || referenceType.Element == nil {
		return false
	}
	handlerType := *referenceType.Element
	if !a.eventHandlerSignature(reference.Value, handlerType, expected) {
		return false
	}
	switch normalizedCallableCapability(handlerType.FunctionCapability) {
	case CallableConsuming:
		a.reportConsumingEventHandler(reference.Value, handlerType)
		return false
	case CallableMutable:
		if !reference.Mutable {
			a.addErrorAtTokenWithMetadata(reference.Token, "",
				"This handler mutates its captured environment, so the event needs exclusive access to it: pass `ref mut handler`.",
				"Event.Subscribe handler of capability %s requires ref mut, not ref", typeDisplayName(handlerType))
			return false
		}
	}
	return true
}

// eventHandlerSignature compares the parameter and result of a handler with
// fn(ref T) void; the callable capability is checked separately.
func (a *Analyzer) eventHandlerSignature(argument ast.Expression, handlerType Type, expected Type) bool {
	comparable := handlerType
	if comparable.Kind == FunctionType {
		comparable.FunctionCapability = expected.FunctionCapability
		comparable.Named, comparable.Name = false, expected.Name
	}
	if sameFunctionType(comparable, expected) {
		return true
	}
	a.addErrorAtTokenWithMetadata(expressionToken(argument), "",
		"An event handler borrows the published payload: declare it as fn(data: ref T) void. Events never pass the payload by value (events.md § 10).",
		"Event.Subscribe handler must be %s, got %s", typeDisplayName(expected), typeDisplayName(handlerType))
	return false
}

func (a *Analyzer) reportConsumingEventHandler(argument ast.Expression, handlerType Type) {
	a.addErrorAtTokenWithMetadata(expressionToken(argument), "",
		"An event may publish repeatedly, so its handler must be reusable; a consuming callable can run only once.",
		"Event.Subscribe handler %s is consuming; event handlers must be reusable", typeDisplayName(handlerType))
}
