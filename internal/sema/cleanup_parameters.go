package sema

import "sec/internal/lexer"

// Owned by-value parameters (functions.md § 6; destruction.md §§ 6.1, 9.2):
// the callee owns a non-trivially destructible by-value parameter for the
// duration of the call, so its destruction is registered in the function's
// root cleanup scope at entry and runs at every function exit unless the
// value was moved out first.

// CleanupAtFunctionEntry activates a parameter's registration on entry: the
// caller completed the transfer before the call.
const CleanupAtFunctionEntry CleanupActivation = "at-function-entry"

// CleanupRegisterParameter is an owned by-value parameter's registration.
const CleanupRegisterParameter CleanupRegistrationKind = "owned-parameter"

type ownedCleanupParameter struct {
	registration ResolvedOwnedCleanupRegistration
	source       lexer.Token
}

// recordOwnedCleanupParameters registers the current function's owned
// non-trivial by-value parameters when its root cleanup scope opens.
// Reference, variadic and trivially destructible parameters need none.
func (a *Analyzer) recordOwnedCleanupParameters(root CleanupScopeID) {
	if root == 0 || !a.inFunctionBody {
		return
	}
	for _, parameter := range a.currentFunctionMetadata.Parameters {
		if parameter.Ref || parameter.MutableRef || parameter.Variadic || parameter.Type.Kind == ReferenceType ||
			parameter.Type.Kind == InvalidType || TriviallyDestructible(parameter.Type) {
			continue
		}
		binding := a.bindingIDs[sourceTokenLocation(parameter.Token)]
		if binding == 0 {
			continue
		}
		place, ok := a.rootPlace(parameter.Name)
		if !ok {
			continue
		}
		registration := ResolvedOwnedCleanupRegistration{
			Action: a.cleanupIdentities.owned(binding, parameter.Token),
			Scope:  root, Invocation: a.currentCallable,
			Binding: binding, Place: cloneSlicePlanPlace(place),
			Activation:  CleanupAtFunctionEntry,
			Destruction: ResolveDestructionPlans(parameter.Type),
		}
		if a.ownedCleanupParameters == nil {
			a.ownedCleanupParameters = map[BindingID]ownedCleanupParameter{}
		}
		a.ownedCleanupParameters[binding] = ownedCleanupParameter{registration: registration, source: parameter.Token}
	}
}

// ResolvedOwnedCleanupParameterOf reads an owned parameter's entry
// registration, detached from analysis state.
func (a *Analyzer) ResolvedOwnedCleanupParameterOf(binding BindingID) (ResolvedOwnedCleanupRegistration, bool) {
	if a == nil {
		return ResolvedOwnedCleanupRegistration{}, false
	}
	parameter, ok := a.ownedCleanupParameters[binding]
	if !ok {
		return ResolvedOwnedCleanupRegistration{}, false
	}
	return cloneOwnedCleanupRegistration(parameter.registration), true
}
