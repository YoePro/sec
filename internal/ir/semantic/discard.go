package semantic

import (
	"fmt"

	"sec/internal/ast"
	"sec/internal/lexer"
	"sec/internal/sema"
)

// buildDiscard lowers `discard expression` and `discard binding` to one
// explicit discard.value: the expression is evaluated, and the consumed value
// (or the binding's storage) is recorded with its trivial-destruction
// classification. A fresh value that needs destruction lowers to own.discard with
// its plan, a whole owned binding also ends its registration, and a stored
// field ends through its field Place.
//
// Rules:
//   - rules/control-flow/discard.md — §§ 3, 5, 9, 36
func (fb *functionBuilder) buildDiscard(stmt *ast.DiscardStatement) error {
	loc := location(stmt.Token)
	if stmt.Name != nil {
		fact, ok := fb.owner.analyzer.ResolvedBindingOf(stmt.Name)
		if !ok {
			return fb.unsupported("discard of an unresolved binding", stmt.Token)
		}
		if !sema.TriviallyDestructible(fact.Type) {
			return fb.discardOwnedBinding(stmt, fact, loc)
		}
		bind, ok := fb.bindings[fact.ID]
		if !ok {
			return fb.unsupported("discard of a binding without semantic storage", stmt.Token)
		}
		if bind.storage == 0 {
			if handled, err := fb.discardSliceHolder(builtValue{id: bind.value, typ: bind.typ}, stmt.Token); handled {
				return err
			}
		}
		op := Operation{Kind: OpDiscardValue, DiscardProvenance: DiscardExplicit, DiscardTrivial: true, Location: loc}
		if bind.storage != 0 {
			op.Storage = bind.storage
		} else {
			op.Operands = []ValueID{bind.value}
		}
		fb.emit(op)
		return nil
	}
	if stmt.Value == nil {
		return fb.unsupported("empty discard", stmt.Token)
	}
	if member, ok := stmt.Value.(*ast.MemberExpression); ok {
		if handled, err := fb.discardOwnedField(stmt, member, loc); handled {
			return err
		}
	}
	return fb.discardExpression(stmt.Value, DiscardExplicit, stmt.Token)
}

// discardOwnedField ends a stored field that needs destruction through its field
// Place: an available field moves out and is destroyed once by its own plan, a
// field owned on some paths is destroyed in place where owned, and an already
// unavailable one is a no-op. The owner stays registered and its cleanup skips
// the ended field, as after a field move (discard.md §§ 26, 35; destruction.md § 12).
func (fb *functionBuilder) discardOwnedField(stmt *ast.DiscardStatement, member *ast.MemberExpression, loc Location) (bool, error) {
	typ, typed := fb.owner.analyzer.ResolvedTypeOf(member)
	state, resolved := fb.owner.analyzer.ResolvedDiscardSourceStateOf(stmt)
	if !typed || !resolved || sema.TriviallyDestructible(typ) || fb.owner.maxPackage < ownershipLoweringPackage {
		return false, nil
	}
	root, chain := memberRoot(member)
	if root == nil {
		return true, fb.unsupported("P17 discard of a field of a computed owner", member.Token)
	}
	owned, stored := fb.ownedStorageOf(root)
	if !stored {
		return true, fb.unsupported("P17 discard of a field of an owner without storage", member.Token)
	}
	if state == sema.DiscardSourceUnavailable {
		return true, nil
	}
	planID, err := fb.owner.importDestructionPlans(sema.ResolveDestructionPlans(typ), typ)
	if err != nil {
		return true, err
	}
	place, err := fb.storedFieldPlace(owned, root, chain)
	if err != nil {
		return true, err
	}
	if state == sema.DiscardSourceConditional {
		fb.emit(Operation{Kind: OpOwnDestroyPlace, Operands: []ValueID{place.id}, Ownership: &OwnershipFacts{Cause: "cleanup", DestructionPlan: planID, IfOwned: true}, Location: loc})
		return true, nil
	}
	element, _ := fb.owner.module.Types.Lookup(place.typ)
	value := fb.result(Operation{Kind: OpOwnMoveFromPlace, Operands: []ValueID{place.id}, Ownership: &OwnershipFacts{}, Location: loc}, element.Element)
	fb.emit(Operation{Kind: OpOwnDiscard, Operands: []ValueID{value.id}, Ownership: &OwnershipFacts{DiscardOrigin: "explicit", DestructionPlan: planID}, Location: loc})
	return true, nil
}

// discardExpression evaluates expr and consumes its value.
func (fb *functionBuilder) discardExpression(expr ast.Expression, provenance DiscardProvenance, token lexer.Token) error {
	typ, ok := fb.owner.analyzer.ResolvedTypeOf(expr)
	if !ok {
		return fb.unsupported("discard of an untyped expression", token)
	}
	if !sema.TriviallyDestructible(typ) {
		return fb.discardFreshOwned(expr, typ, provenance, token)
	}
	value, err := fb.buildExpr(expr, 0)
	if err != nil {
		return err
	}
	if handled, err := fb.discardSliceHolder(value, token); handled {
		return err
	}
	fb.emit(Operation{Kind: OpDiscardValue, Operands: []ValueID{value.id}, DiscardProvenance: provenance, DiscardTrivial: true, Location: location(token)})
	return nil
}

// discardOwnedBinding ends a whole tracked owned binding: the value is destroyed once
// through its plan and the registration cancelled. An already unavailable binding is
// a legal no-op. A binding moved on only some paths needs path-exact state.
// Rules: rules/control-flow/discard.md — §§5,7; rules/memory/destruction.md — §§6.1,12.
func (fb *functionBuilder) discardOwnedBinding(stmt *ast.DiscardStatement, fact sema.ResolvedBinding, loc Location) error {
	if fb.owner.maxPackage < ownershipLoweringPackage {
		return fb.unsupported("discard of a binding that needs destruction", stmt.Token)
	}
	state, _ := fb.owner.analyzer.ResolvedDiscardSourceStateOf(stmt)
	switch state {
	case sema.DiscardSourceUnavailable:
		return nil
	case sema.DiscardSourceAvailable, sema.DiscardSourceConditional:
	default:
		return fb.unsupported("discard of an unresolved source state", stmt.Token)
	}
	conditional := state == sema.DiscardSourceConditional
	action, tracked := fb.trackedActions[fact.ID]
	if !tracked {
		return fb.unsupported("discard of an owned value without a tracked registration", stmt.Token)
	}
	planID, err := fb.owner.importDestructionPlans(sema.ResolveDestructionPlans(fact.Type), fact.Type)
	if err != nil {
		return err
	}
	var value builtValue
	if owned, stored := fb.ownedStorages[fact.ID]; stored && conditional {
		// Nothing can move out of a Place that may hold nothing, so it ends in place.
		place := fb.ownedStoragePlace(owned, stmt.Token)
		fb.emit(Operation{Kind: OpOwnDestroyPlace, Operands: []ValueID{place.id}, Ownership: &OwnershipFacts{Cause: "cleanup", DestructionPlan: planID, IfOwned: true}, Location: loc})
		fb.emitAll(fb.ownedCancels(fact.ID, action, "discarded", true, loc))
		return nil
	} else if stored {
		place := fb.ownedStoragePlace(owned, stmt.Token)
		value = fb.result(Operation{Kind: OpOwnMoveFromPlace, Operands: []ValueID{place.id}, Ownership: &OwnershipFacts{}, Location: loc}, owned.typ)
	} else if bound, known := fb.bindings[fact.ID]; known && bound.storage == 0 {
		value = builtValue{id: bound.value, typ: bound.typ}
	} else {
		return fb.unsupported("discard of a binding without semantic storage", stmt.Token)
	}
	fb.emit(Operation{Kind: OpOwnDiscard, Operands: []ValueID{value.id}, Ownership: &OwnershipFacts{DiscardOrigin: "explicit", DestructionPlan: planID, IfOwned: conditional}, Location: loc})
	fb.emitAll(fb.ownedCancels(fact.ID, action, "discarded", conditional, loc))
	return nil
}

// discardFreshOwned destroys a call or try result or a struct or array literal nothing
// else owns, once, through its type-level plan; the literal's operands are
// prepared, and moved sources cancelled, before it is destroyed
// (rules/control-flow/discard.md §§ 5, 9, aggregate temporary discard; dialect v13 § 11).
func (fb *functionBuilder) discardFreshOwned(expr ast.Expression, typ sema.Type, provenance DiscardProvenance, token lexer.Token) error {
	_, call := expr.(*ast.CallExpression)
	_, tried := expr.(*ast.TryExpression)
	_, literal := expr.(*ast.StructLiteral)
	_, array := expr.(*ast.ArrayLiteral)
	if !call && !tried && !literal && !array || fb.owner.maxPackage < ownershipLoweringPackage {
		return fb.unsupported(fmt.Sprintf("discard of %s, which needs destruction", typ.Name), token)
	}
	planID, err := fb.owner.importDestructionPlans(sema.ResolveDestructionPlans(typ), typ)
	if err != nil {
		return err
	}
	value, err := fb.buildExpr(expr, 0)
	if err != nil {
		return err
	}
	origin := "explicit"
	if provenance == DiscardImplicitCallResult {
		origin = "implicit-call-result"
	}
	fb.emit(Operation{Kind: OpOwnDiscard, Operands: []ValueID{value.id}, Ownership: &OwnershipFacts{DiscardOrigin: origin, DestructionPlan: planID}, Location: location(token)})
	return nil
}

// discardSliceHolder ends a constructed non-owning slice holder through P15's
// marker. It does not copy/destroy elements or discard the backing storage.
// Rules: rules/compiler/semantic_ir.md — §126.5,21;
// rules/mlir/packages/sec-mlir-dialect_package16.md — §§42,63.
func (fb *functionBuilder) discardSliceHolder(value builtValue, token lexer.Token) (bool, error) {
	typ, _ := fb.owner.module.Types.Lookup(value.typ)
	if !boundedView(typ.Kind) {
		return false, nil
	}
	facts := fb.references[value.id]
	if facts == nil {
		return true, fmt.Errorf("discarded slice holder has no reference facts")
	}
	fb.emit(Operation{Kind: OpReferenceEndBorrow, Operands: []ValueID{value.id}, Reference: cloneReferenceMetadata(facts), Location: location(token)})
	return true, nil
}

func verifyDiscardValue(op Operation, storages map[StorageID]Storage) error {
	if op.DiscardProvenance != DiscardExplicit && op.DiscardProvenance != DiscardImplicitCallResult {
		return fmt.Errorf("invalid discard provenance %q", op.DiscardProvenance)
	}
	if len(op.Results) != 0 || len(op.Successors) != 0 {
		return fmt.Errorf("discard.value has no results or successors")
	}
	if op.Storage != 0 {
		if len(op.Operands) != 0 || storages[op.Storage].ID == 0 {
			return fmt.Errorf("invalid discard.value of storage")
		}
		return nil
	}
	if len(op.Operands) != 1 {
		return fmt.Errorf("discard.value consumes exactly one value")
	}
	return nil
}
