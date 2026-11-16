package sema

import (
	"sec/internal/ast"
	"sec/internal/lexer"
)

// recordPropertyReceiver records how an instance property operation produces
// its accessor's receiver, exactly as for an instance-method call (a getter's
// receiver plan comes from validatePropertyReadReceiver): an owned
// receiver Place is implicitly borrowed with the accessor's capability, a
// reference holder is passed, and the enclosing method's own receiver (`self.P`
// or an implicit member `P`) is copied or reborrowed. A compound update
// evaluates its receiver once with the setter's (mutable) capability.
// Rules: rules/declarations/properties.md — §§ 4–9, 15;
// rules/compiler/semantic_ir.md — § 26(2)–(3); rules/declarations/impl.md — § 8.
func (a *Analyzer) recordPropertyReceiver(expr ast.Expression, property Property, kind ResolvedPropertyAccessKind) {
	if a.summaryPass || property.Static || !a.inFunctionBody {
		return
	}
	callable := property.GetterCallable
	if kind != PropertyRead {
		callable = property.SetterCallable
	}
	if callable == nil {
		return
	}
	if kind == PropertyCompoundUpdate && property.GetterCallable != nil {
		a.recordCompoundGetterReborrow(expr, *property.GetterCallable)
	}
	switch access := expr.(type) {
	case *ast.MemberExpression:
		if receiver, ok := access.Object.(*ast.Identifier); ok {
			if use, isReceiver := a.implicitReceiverUses[receiver]; isReceiver && use.Self {
				a.recordSelfReborrow(expr, access.Token, *callable)
				return
			}
		}
		// validatePropertyReadReceiver already planned a getter's receiver.
		if kind != PropertyRead {
			a.recordReceiverReferencePlan(*callable, access.Object)
		}
	case *ast.Identifier:
		a.recordSelfReborrow(expr, access.Token, *callable)
	}
}

// recordSelfReborrow publishes the reborrow that passes the enclosing method's
// own mutable receiver to an accessor or method (recordReceiverReborrow).
func (a *Analyzer) recordSelfReborrow(key ast.Expression, token lexer.Token, callee Function) {
	if !functionUsesReceiver(callee) || !functionUsesReceiver(a.currentFunctionMetadata) || !a.currentFunctionMetadata.ReceiverMutable {
		return
	}
	place, ok := a.resolvePlace(&ast.Identifier{Token: token, Value: "self"})
	if !ok {
		return
	}
	place.RootKind = PlaceRootReceiver
	if a.receiverReborrows == nil {
		a.receiverReborrows = map[ast.Expression]ReferenceFacts{}
	}
	a.receiverReborrows[key] = a.referenceFactsForPlace(place, callee.ReceiverMutable, token)
}

// ResolvedPropertyReceiverReborrowOf returns the reborrow facts that pass the
// enclosing method's mutable receiver to the accessor of a property operation.
func (a *Analyzer) ResolvedPropertyReceiverReborrowOf(expr ast.Expression) (ReferenceFacts, bool) {
	if a == nil {
		return ReferenceFacts{}, false
	}
	facts, ok := a.receiverReborrows[expr]
	return cloneReferenceFacts(facts), ok
}

// PropertyAccessorsOf returns the getter and setter callables of a declared
// property of owner (the generic template's for a generic owner instance).
// Rules: rules/declarations/properties.md — §§ 4–6, 14.
func (a *Analyzer) PropertyAccessorsOf(owner Type, name string) (getter, setter *Function, ok bool) {
	if a == nil {
		return nil, nil, false
	}
	owner = dereferenceType(owner)
	if declared, exists := a.types[owner.Name]; exists {
		owner = declared
	}
	property, found := lookupProperty(owner, name)
	if !found {
		return nil, nil, false
	}
	clone := func(function *Function) *Function {
		if function == nil {
			return nil
		}
		copied := *function
		copied.Parameters = append([]FunctionParameter(nil), function.Parameters...)
		return &copied
	}
	return clone(property.GetterCallable), clone(property.SetterCallable), true
}

// propertyReadValue reports a property read, whose getter produces a fresh
// value rather than naming a reusable source Place (properties.md § 4).
func (a *Analyzer) propertyReadValue(expr ast.Expression) bool {
	access, ok := a.resolvedPropertyAccesses[expr]
	return ok && access.Kind == PropertyRead
}

// recordCompoundGetterReborrow publishes the reborrow through which a
// compound property update's getter reads its receiver: the update evaluates
// the receiver once, with the setter's mutable capability, and the getter
// receives a fresh shared (or mutable) reborrow of it at the property name.
// Rules: rules/compiler/semantic_ir.md — § 26(2)–(3);
// rules/declarations/properties.md — § 7.
func (a *Analyzer) recordCompoundGetterReborrow(expr ast.Expression, getter Function) {
	var receiver ast.Expression
	token := expressionToken(expr)
	switch access := expr.(type) {
	case *ast.MemberExpression:
		receiver = access.Object
		if access.Property != nil {
			token = access.Property.Token
		}
	case *ast.Identifier:
		receiver = &ast.Identifier{Token: access.Token, Value: "self"}
	default:
		return
	}
	place, ok := a.resolvePlace(receiver)
	if !ok {
		return
	}
	if identifier, named := receiver.(*ast.Identifier); named {
		if use, isReceiver := a.implicitReceiverUses[identifier]; identifier.Value == "self" || isReceiver && use.Self {
			place.RootKind = PlaceRootReceiver
		}
	}
	if place.Type.Kind == ReferenceType && place.Type.Element != nil {
		place = a.canonicalDereferencePlace(receiver, place, place.Type)
	}
	if a.compoundGetterReborrows == nil {
		a.compoundGetterReborrows = map[ast.Expression]ReferenceFacts{}
	}
	// The setter's receiver takes the site's identity; the getter's reborrow
	// is a distinct borrow of it at the same site.
	facts := a.referenceFactsWithoutIdentity(place, getter.ReceiverMutable, token)
	key := "compound-get:" + referenceSiteKey(token)
	facts.Borrow = a.placeIdentities.borrow("borrow:" + key)
	facts.Lifetime = a.placeIdentities.lifetime("lifetime:" + key)
	a.compoundGetterReborrows[expr] = facts
}

// ResolvedCompoundGetterReborrowOf returns the getter reborrow facts of a
// compound property update.
func (a *Analyzer) ResolvedCompoundGetterReborrowOf(expr ast.Expression) (ReferenceFacts, bool) {
	if a == nil {
		return ReferenceFacts{}, false
	}
	facts, ok := a.compoundGetterReborrows[expr]
	return cloneReferenceFacts(facts), ok
}
