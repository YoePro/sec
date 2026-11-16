package sema

import (
	"encoding/json"

	"sec/internal/lexer"
)

// InstantiationIdentity identifies one semantic specialization independently of
// its physical symbol or signature cache. Owner arguments precede the callable's
// own arguments; both retain declaration order, including phantom parameters.
// Rules: rules/compiler/monomorphization.md — §§4.1–4.5,6.4,9.3.
type InstantiationIdentity struct {
	Key            string
	Declaration    CallableID
	Role           string
	OwnerArguments []string
	Arguments      []string
}

// genericCallableTemplate recovers the registered declaration without treating
// an owner's parameters as the independent method parameter list.
// Rules: rules/declarations/generics.md — §§8–9;
// rules/compiler/monomorphization.md — §§4.1,6.4.
func (a *Analyzer) genericCallableTemplate(function Function) (Function, bool) {
	for _, template := range a.functions[function.Name] {
		if !sameSourceToken(template.Token, function.Token) {
			continue
		}
		owner := a.types[template.ImplTarget]
		if len(template.GenericParameters) > 0 || len(owner.GenericParameters) > 0 {
			return template, true
		}
	}
	return Function{}, false
}

// InstantiationIdentityOf reads a complete resolved substitution without creating
// a cache entry or program demand. Symbolic arguments never acquire a concrete
// identity; signature equality cannot erase phantom owner or method arguments.
// Rules: rules/compiler/monomorphization.md — §§4,6.3–6.4,8.1,9.3.
func (a *Analyzer) InstantiationIdentityOf(function Function) (InstantiationIdentity, bool) {
	if a == nil || function.GenericConstraintFailure != nil {
		return InstantiationIdentity{}, false
	}
	template, ok := a.genericCallableTemplate(function)
	if !ok {
		return InstantiationIdentity{}, false
	}
	identity := InstantiationIdentity{Declaration: callableID(template), Role: "function"}
	if template.ImplTarget != "" {
		identity.Role = "instance-method"
		if template.Static {
			identity.Role = "static-method"
		}
	}
	arguments := func(names []string) ([]string, bool) {
		values := make([]string, len(names))
		for index, name := range names {
			typ, known := function.GenericSubstitution[name]
			if !known || typ.Kind == InvalidType || typ.Kind == "" || typeMentionsGenericParameter(typ) {
				return nil, false
			}
			values[index] = canonicalTypeIdentity(typ)
		}
		return values, true
	}
	identity.OwnerArguments, ok = arguments(a.types[template.ImplTarget].GenericParameters)
	if !ok {
		return InstantiationIdentity{}, false
	}
	identity.Arguments, ok = arguments(template.GenericParameters)
	if !ok {
		return InstantiationIdentity{}, false
	}
	key, _ := json.Marshal([]any{identity.Declaration, identity.Role, identity.OwnerArguments, identity.Arguments})
	identity.Key = graphIdentityDigest(key)
	return identity, true
}

// InstantiationCause records the selected source call and, for a nested demand,
// the enclosing concrete specialization. It is not an execution or link root.
// Rules: rules/compiler/monomorphization.md — §§7.2–7.4.
type InstantiationCause struct {
	Caller string
	Source lexer.Token
}

// InstantiationDemand groups repeated current-program causes of one canonical
// specialization; cache membership alone never adds a cause.
// Rules: rules/compiler/monomorphization.md — §§6.1–6.3,7.1–7.2.
type InstantiationDemand struct {
	Identity InstantiationIdentity
	Causes   []InstantiationCause
}
