package semantic

import (
	"fmt"

	"sec/internal/ast"
	"sec/internal/sema"
)

// Instance-method receivers (P15-47). From package 15 an instance method with
// a shared or mutable receiver lowers with an explicit first parameter `self`
// of type ref Target or ref mut Target carrying Sema's receiver facts
// (semantic_ir.md § 60(3)). Field reads and writes through `self.f` or an
// implicit field name dereference that parameter and project the field
// Place; a call passes the receiver argument Sema resolved: an implicit borrow
// of an owned receiver Place, a reference holder passed shared or reborrowed,
// or the caller's own `self`. Consuming receivers and generic implementations
// stay unsupported.
//
// Rules:
//   - rules/declarations/impl.md — § 8 "self", § 11 "Methods"
//   - rules/compiler/semantic_ir.md — §§ 60(3), 61(4)
//   - rules/mlir/packages/sec-mlir-dialect_package15.md — §§ 59-60

// instanceMethodLowerable reports an instance method Semantic IR builds:
// package 15, a borrowed (not consuming) receiver and no generic parameters.
func (b *builder) instanceMethodLowerable(function sema.Function) bool {
	return b.maxPackage >= referenceLoweringPackage && !function.Static && function.ImplTarget != "" &&
		!function.ReceiverConsuming && len(function.GenericParameters) == 0 && len(function.GenericSubstitution) == 0
}

// addReceiverParameter adds the explicit `self` reference parameter.
func (fb *functionBuilder) addReceiverParameter(function sema.Function, decl *ast.FunctionDeclaration) error {
	facts, ok := fb.owner.analyzer.ReceiverReferenceFactsOf(function)
	if !ok || !facts.Checkable() {
		return fb.unsupported("instance method receiver without resolved reference facts", decl.Token)
	}
	target, err := fb.owner.internType(facts.Referent)
	if err != nil {
		return locateUnsupportedType(err, decl.Token)
	}
	ownership := OwnershipSharedReference
	if function.ReceiverMutable {
		ownership = OwnershipMutableReference
	}
	referenceType := fb.owner.module.Types.Intern(Type{Kind: TypeReference, Element: target, Mutable: function.ReceiverMutable})
	metadata := referenceMetadataOf(facts)
	value := fb.newValue(referenceType, ownership, location(decl.Token))
	fb.fn.Parameters = append(fb.fn.Parameters, Parameter{Name: "self", Value: value, Location: location(decl.Token), Reference: metadata})
	fb.recordReference(value.ID, metadata)
	fb.receiver = builtValue{id: value.ID, typ: referenceType}
	return nil
}

// receiverFieldPath resolves expr as a path of stored fields beneath the
// receiver: an implicit field name, or `self` followed by member steps.
func (fb *functionBuilder) receiverFieldPath(expr ast.Expression) ([]sema.ResolvedStructMemberPlan, []ast.Expression, bool, bool) {
	if fb.receiver.typ == 0 {
		return nil, nil, false, false
	}
	var steps []*ast.MemberExpression
	current := expr
	for {
		member, ok := current.(*ast.MemberExpression)
		if !ok {
			break
		}
		steps = append([]*ast.MemberExpression{member}, steps...)
		current = member.Object
	}
	identifier, ok := current.(*ast.Identifier)
	if !ok {
		return nil, nil, false, false
	}
	use, ok := fb.owner.analyzer.ResolvedImplicitReceiverOf(identifier)
	if !ok {
		return nil, nil, false, false
	}
	var plans []sema.ResolvedStructMemberPlan
	var tokens []ast.Expression
	if !use.Self {
		plans = append(plans, use.Field)
		tokens = append(tokens, identifier)
	}
	for _, step := range steps {
		plan, ok := fb.owner.analyzer.ResolvedStructMemberOf(step)
		if !ok || plan.Kind != sema.MemberStoredField {
			return nil, nil, true, false
		}
		plans = append(plans, plan)
		tokens = append(tokens, step)
	}
	return plans, tokens, true, len(plans) > 0
}

// receiverPlace dereferences the receiver once under its validity policy and
// projects the stored-field path.
// isReceiverSelf reports a bare `self` naming the enclosing method's receiver.
func (fb *functionBuilder) isReceiverSelf(expr ast.Expression) bool {
	identifier, ok := expr.(*ast.Identifier)
	if !ok || fb.receiver.typ == 0 {
		return false
	}
	use, isReceiver := fb.owner.analyzer.ResolvedImplicitReceiverOf(identifier)
	return isReceiver && use.Self
}

// receiverRootPlace dereferences the receiver reference once (validating a
// dynamic-epoch receiver) and returns the receiver's whole Place.
func (fb *functionBuilder) receiverRootPlace(loc Location) (builtValue, bool, error) {
	if fb.receiver.typ == 0 {
		return builtValue{}, false, fmt.Errorf("self outside an instance method")
	}
	metadata := fb.references[fb.receiver.id]
	refType, _ := fb.owner.module.Types.Lookup(fb.receiver.typ)
	guard, err := fb.validateReference(fb.receiver, metadata, loc)
	if err != nil {
		return builtValue{}, false, err
	}
	placeType := fb.owner.module.Types.Intern(Type{Kind: TypePlace, Element: refType.Element, Mutable: refType.Mutable})
	return fb.placeResult(Operation{Kind: OpReferenceDerefPlace, Operands: []ValueID{fb.receiver.id}, Reference: metadata, ReferenceGuard: guard, Location: loc}, placeType), refType.Mutable, nil
}

func (fb *functionBuilder) receiverPlace(plans []sema.ResolvedStructMemberPlan, steps []ast.Expression) (builtValue, bool, error) {
	metadata := fb.references[fb.receiver.id]
	refType, _ := fb.owner.module.Types.Lookup(fb.receiver.typ)
	loc := locationFromExpression(steps[0])
	guard, err := fb.validateReference(fb.receiver, metadata, loc)
	if err != nil {
		return builtValue{}, false, err
	}
	placeType := fb.owner.module.Types.Intern(Type{Kind: TypePlace, Element: refType.Element, Mutable: refType.Mutable})
	place := fb.placeResult(Operation{Kind: OpReferenceDerefPlace, Operands: []ValueID{fb.receiver.id}, Reference: metadata, ReferenceGuard: guard, Location: loc}, placeType)
	for index, plan := range plans {
		fieldType, err := fb.owner.internType(plan.MemberType)
		if err != nil {
			return builtValue{}, false, err
		}
		fieldPlace := fb.owner.module.Types.Intern(Type{Kind: TypePlace, Element: fieldType, Mutable: refType.Mutable})
		place = fb.placeResult(Operation{Kind: OpPlaceField, Operands: []ValueID{place.id}, StructField: StructFieldID(plan.FieldID), Location: locationFromExpression(steps[index])}, fieldPlace)
	}
	return place, refType.Mutable, nil
}

// lowerReceiverRead reads a stored field of the receiver; a bare `self` value
// is not lowered.
func (fb *functionBuilder) lowerReceiverRead(expr ast.Expression) (builtValue, error) {
	plans, steps, rooted, ok := fb.receiverFieldPath(expr)
	if !rooted {
		return builtValue{}, errNotReference
	}
	if _, borrowed := fb.owner.analyzer.ResolvedReferencePlanOf(expr); borrowed {
		// An implicit call-site borrow of the receiver or a receiver field
		// is lowered from its reference plan, not read as a value.
		return builtValue{}, errNotReference
	}
	if !ok {
		return builtValue{}, fb.unsupported("receiver used as a value or through a non-field member", expressionToken(expr))
	}
	if sema.CopyClassificationOf(plans[len(plans)-1].MemberType) != sema.CopyTrivial {
		return builtValue{}, fb.unsupported("non-copy-trivial read of a receiver field", expressionToken(expr))
	}
	place, _, err := fb.receiverPlace(plans, steps)
	if err != nil {
		return builtValue{}, err
	}
	placeType, _ := fb.owner.module.Types.Lookup(place.typ)
	return fb.result(Operation{Kind: OpPlaceRead, Operands: []ValueID{place.id}, Location: locationFromExpression(expr)}, placeType.Element), nil
}

// lowerReceiverWrite stores to a stored field of a mutable receiver: the
// destination Place first, then the source (operators.md "Assignment
// evaluation order").
func (fb *functionBuilder) lowerReceiverWrite(stmt *ast.AssignmentStatement) (bool, error) {
	plans, steps, rooted, ok := fb.receiverFieldPath(stmt.Target)
	if !rooted {
		return false, nil
	}
	if !ok || stmt.Operator != "=" {
		return true, fb.unsupported("receiver write other than a plain field assignment", stmt.Token)
	}
	if !sema.TriviallyDestructible(plans[len(plans)-1].MemberType) {
		return true, fb.unsupported("non-trivial replacement of a receiver field", stmt.Token)
	}
	place, mutable, err := fb.receiverPlace(plans, steps)
	if err != nil {
		return true, err
	}
	if !mutable {
		return true, fb.unsupported("write through a shared receiver", stmt.Token)
	}
	placeType, _ := fb.owner.module.Types.Lookup(place.typ)
	value, err := fb.buildExpr(stmt.Value, placeType.Element)
	if err != nil {
		return true, err
	}
	fb.emit(Operation{Kind: OpPlaceWrite, Operands: []ValueID{place.id, value.id}, Location: location(stmt.Token)})
	return true, nil
}

// buildReceiverArgument produces the receiver argument of an instance-method
// call from Sema's resolved receiver plan or pass, or from the caller's own
// `self`.
func (fb *functionBuilder) buildReceiverArgument(call *ast.CallExpression, callee sema.Function) (builtValue, ArgumentAction, error) {
	target, ok := fb.owner.analyzer.ReceiverReferenceFactsOf(callee)
	if !ok {
		return builtValue{}, "", fb.unsupported("instance method call without receiver facts", call.Token)
	}
	element, err := fb.owner.internType(target.Referent)
	if err != nil {
		return builtValue{}, "", err
	}
	referenceType := fb.owner.module.Types.Intern(Type{Kind: TypeReference, Element: element, Mutable: callee.ReceiverMutable})
	action := ArgumentSharedReference
	if callee.ReceiverMutable {
		action = ArgumentMutableReference
	}
	member, ok := call.Callee.(*ast.MemberExpression)
	if !ok {
		// A bare method name inside a method calls it on the implicit self.
		value, err := fb.passReceiver(call, callee.ReceiverMutable, location(call.Token))
		return value, action, err
	}
	if identifier, isIdentifier := member.Object.(*ast.Identifier); isIdentifier {
		if use, isReceiver := fb.owner.analyzer.ResolvedImplicitReceiverOf(identifier); isReceiver && use.Self {
			value, err := fb.passReceiver(call, callee.ReceiverMutable, location(identifier.Token))
			return value, action, err
		}
	}
	value, err := fb.buildExpr(member.Object, referenceType)
	if err != nil {
		return builtValue{}, "", err
	}
	if value.typ != referenceType {
		return builtValue{}, "", fb.unsupported("receiver argument without a resolved borrow or reference pass", expressionToken(member.Object))
	}
	return value, action, nil
}

// passReceiver passes the enclosing method's own receiver to another
// instance method: copied when shared, reborrowed when the callee needs a
// mutable receiver or the caller's receiver is mutable.
func (fb *functionBuilder) passReceiver(call *ast.CallExpression, mutable bool, loc Location) (builtValue, error) {
	if fb.receiver.typ == 0 {
		return builtValue{}, fmt.Errorf("self outside an instance method")
	}
	metadata := fb.references[fb.receiver.id]
	sourceType, _ := fb.owner.module.Types.Lookup(fb.receiver.typ)
	if mutable && !sourceType.Mutable {
		return builtValue{}, &UnsupportedFeatureError{Feature: "mutable method call on a shared receiver", Package: fb.owner.maxPackage, Location: loc}
	}
	if !sourceType.Mutable {
		copied := *metadata
		copied.SourceBorrow = copied.Borrow
		value := fb.newValue(fb.receiver.typ, OwnershipSharedReference, loc)
		fb.emit(Operation{Kind: OpReferenceCopyShared, Operands: []ValueID{fb.receiver.id}, Results: []Value{value}, Reference: &copied, Location: loc})
		fb.recordReference(value.ID, &copied)
		return builtValue{id: value.ID, typ: fb.receiver.typ}, nil
	}
	// A mutable receiver is never copied: it is reborrowed for the call with
	// the fresh identity Sema resolved, mutable or shared as the callee needs
	// (package15.md §§ 47-48, 60).
	facts, ok := fb.owner.analyzer.ResolvedReceiverReborrowOf(call)
	if !ok || !facts.Checkable() {
		return builtValue{}, &UnsupportedFeatureError{Feature: "reborrow of a mutable receiver without resolved reborrow facts", Package: fb.owner.maxPackage, Location: loc}
	}
	derived := referenceMetadataOf(facts)
	derived.SourceBorrow = metadata.Borrow
	if len(derived.OriginStorages) == 0 {
		derived.OriginStorages = append([]uint32(nil), metadata.OriginStorages...)
	}
	if derived.Borrow == derived.SourceBorrow || derived.Validity != metadata.Validity {
		return builtValue{}, &UnsupportedFeatureError{Feature: "receiver reborrow without a distinct borrow identity or with a different validity policy", Package: fb.owner.maxPackage, Location: loc}
	}
	kind, ownership := OpReferenceReborrowShared, OwnershipSharedReference
	if mutable {
		kind, ownership = OpReferenceReborrowMutable, OwnershipMutableReference
	}
	referenceType := fb.owner.module.Types.Intern(Type{Kind: TypeReference, Element: sourceType.Element, Mutable: mutable})
	value := fb.newValue(referenceType, ownership, loc)
	fb.emit(Operation{Kind: kind, Operands: []ValueID{fb.receiver.id}, Results: []Value{value}, Reference: derived, Location: loc})
	fb.recordReference(value.ID, derived)
	return builtValue{id: value.ID, typ: referenceType}, nil
}
