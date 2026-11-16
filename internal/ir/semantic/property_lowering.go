package semantic

import (
	"fmt"
	"strings"

	"sec/internal/ast"
	"sec/internal/lexer"
	"sec/internal/sema"
)

// User-defined properties lower to their accessor functions. Each getter,
// setter and fallible setter body is built once per concrete owner as an
// ordinary function (`Owner.P.get`, `Owner.P.set`, `Owner.P.try-set`), on
// demand, and every property operation is a call.direct of that accessor
// carrying its PropertyFacts, so Semantic IR never presents a property as a
// field access. The receiver is produced once, exactly as for an
// instance-method call.
//
// Rules:
//   - rules/compiler/semantic_ir.md — § 26
//   - rules/declarations/properties.md — §§ 4–9, 14, 15
//   - rules/compiler/monomorphization.md — § 9.3 (accessors of a generic owner)

// accessorDeclaration presents one accessor body as a function declaration
// for the shared function builder: the getter has no value parameter, a
// setter its one value parameter.
func accessorDeclaration(property *ast.PropertyDeclaration, getter bool) *ast.FunctionDeclaration {
	if getter {
		return &ast.FunctionDeclaration{Token: property.Getter.Token, EndToken: property.Getter.EndToken, Attributes: property.GetterAttributes, Name: property.Name, Body: property.Getter, Static: property.Static}
	}
	setter := property.Setter
	decl := &ast.FunctionDeclaration{Token: setter.Body.Token, EndToken: setter.Body.EndToken, Attributes: setter.Attributes, Name: property.Name, Body: setter.Body, Static: property.Static}
	if setter.Parameter != nil {
		decl.Parameters = []*ast.Parameter{{Token: setter.Parameter.Token, Name: setter.Parameter, Type: property.Type}}
	}
	return decl
}

// registerPropertyAccessors registers the accessor bodies of one property
// declaration; a property operation demands the accessor it calls.
func (b *builder) registerPropertyAccessors(owner string, property *ast.PropertyDeclaration) {
	if property == nil || property.Name == nil {
		return
	}
	getter, setter, ok := b.analyzer.PropertyAccessorsOf(sema.Type{Name: owner}, property.Name.Value)
	if !ok {
		return
	}
	if getter != nil && property.Getter != nil {
		b.registerGenericTemplate(accessorDeclaration(property, true), *getter)
	}
	if setter != nil && property.Setter != nil && !property.Setter.Invalid && property.Setter.Body != nil {
		b.registerGenericTemplate(accessorDeclaration(property, false), *setter)
	}
}

// demandAccessor returns the function a property operation on owner calls,
// demanding its body: the accessor itself for a non-generic owner, or its
// specialization for the owner's concrete type arguments.
func (fb *functionBuilder) demandAccessor(accessor sema.Function, owner sema.Type, token lexer.Token) (FunctionID, sema.Function, error) {
	b := fb.owner
	key := genericTemplateKey(accessor)
	if b.specializations.templates[key] == nil {
		return "", accessor, fb.unsupported("property accessor without a source body", token)
	}
	owner = b.concreteType(owner)
	if owner.Kind == sema.ReferenceType && owner.Element != nil {
		owner = *owner.Element
	}
	parameters := b.analyzer.GenericTypeParameters(owner.Name)
	if len(parameters) == 0 {
		id := FunctionID(accessor.Module + "::" + qualifiedCallableName(accessor) + "(" + accessorSignature(accessor) + ")")
		if _, seen := b.specializations.demanded[id]; !seen {
			spec := &specialization{id: id, function: accessor, token: token}
			if b.specializations.demanded == nil {
				b.specializations.demanded = map[FunctionID]*specialization{}
			}
			b.specializations.demanded[id] = spec
			b.specializations.pending = append(b.specializations.pending, spec)
		}
		return id, accessor, nil
	}
	if len(owner.TypeArgs) != len(parameters) {
		return "", accessor, fb.unsupported("property of a generic owner without concrete type arguments", token)
	}
	substitution := make(map[string]sema.Type, len(parameters))
	for index, name := range parameters {
		substitution[name] = owner.TypeArgs[index]
	}
	target := accessor
	target.GenericSubstitution = substitution
	target.GenericParameters = nil
	target.Parameters = append([]sema.FunctionParameter(nil), accessor.Parameters...)
	for index := range target.Parameters {
		target.Parameters[index].Type = sema.SubstituteGenericType(target.Parameters[index].Type, substitution)
	}
	target.ReturnType = sema.SubstituteGenericType(accessor.ReturnType, substitution)
	spec, err := b.demandSpecialization(&ast.CallExpression{Token: token}, target)
	if err != nil {
		return "", accessor, err
	}
	return spec.id, spec.function, nil
}

func accessorSignature(accessor sema.Function) string {
	signature := ""
	for index, parameter := range accessor.Parameters {
		if index > 0 {
			signature += ","
		}
		signature += canonicalSemaType(parameter.Type)
	}
	return signature
}

// propertyReceiver produces the receiver reference of an instance accessor
// call: an implicit borrow or reference pass of the owner Place Sema planned,
// or the enclosing method's own receiver, copied or reborrowed.
func (fb *functionBuilder) propertyReceiver(expr ast.Expression, accessor sema.Function, owner sema.Type) (builtValue, ArgumentAction, error) {
	owner = fb.owner.concreteType(owner)
	if owner.Kind == sema.ReferenceType && owner.Element != nil {
		owner = *owner.Element
	}
	element, err := fb.owner.internType(owner)
	if err != nil {
		return builtValue{}, "", err
	}
	referenceType := fb.owner.module.Types.Intern(Type{Kind: TypeReference, Element: element, Mutable: accessor.ReceiverMutable})
	action := ArgumentSharedReference
	if accessor.ReceiverMutable {
		action = ArgumentMutableReference
	}
	token := expressionToken(expr)
	switch access := expr.(type) {
	case *ast.Identifier:
		value, err := fb.passReceiver(expr, accessor.ReceiverMutable, location(token))
		return value, action, err
	case *ast.MemberExpression:
		if identifier, ok := access.Object.(*ast.Identifier); ok {
			if use, isReceiver := fb.owner.analyzer.ResolvedImplicitReceiverOf(identifier); isReceiver && use.Self {
				value, err := fb.passReceiver(expr, accessor.ReceiverMutable, location(identifier.Token))
				return value, action, err
			}
		}
		value, err := fb.buildExpr(access.Object, referenceType)
		if err != nil {
			return builtValue{}, "", err
		}
		if value.typ != referenceType {
			return builtValue{}, "", fb.unsupported("property receiver without a resolved borrow or reference pass", expressionToken(access.Object))
		}
		return value, action, nil
	}
	return builtValue{}, "", fb.unsupported("property receiver of an unsupported form", token)
}

// propertyAccess is the Sema property operation of expr when it calls a
// source accessor that Semantic IR builds.
func (fb *functionBuilder) propertyAccess(expr ast.Expression) (sema.ResolvedPropertyAccess, *sema.Function, *sema.Function, bool) {
	if fb.owner.maxPackage < ownershipLoweringPackage {
		return sema.ResolvedPropertyAccess{}, nil, nil, false
	}
	access, ok := fb.owner.analyzer.ResolvedPropertyAccessOf(expr)
	if !ok {
		return access, nil, nil, false
	}
	getter, setter, declared := fb.owner.analyzer.PropertyAccessorsOf(access.OwnerType, access.Name)
	if !declared {
		return access, nil, nil, false
	}
	built := func(accessor *sema.Function) bool {
		return accessor != nil && fb.owner.specializations.templates[genericTemplateKey(*accessor)] != nil
	}
	if !built(getter) && !built(setter) {
		// A compiler-known or register property keeps its own producer.
		return access, nil, nil, false
	}
	return access, getter, setter, true
}

// callAccessor emits one accessor call with the receiver (for an instance
// property) and the setter value already prepared.
func (fb *functionBuilder) callAccessor(kind PropertyAccessKind, access sema.ResolvedPropertyAccess, callee FunctionID, accessor sema.Function, operands []ValueID, actions []ArgumentAction, token lexer.Token) (builtValue, error) {
	owner := fb.owner.concreteType(access.OwnerType)
	if owner.Kind == sema.ReferenceType && owner.Element != nil {
		owner = *owner.Element
	}
	op := Operation{Kind: OpDirectCall, Callee: callee, Operands: operands, ArgumentActions: actions, Property: &PropertyFacts{Kind: kind, Owner: canonicalSemaType(owner), Name: access.Name, Static: access.Static}, Location: location(token)}
	ret, err := fb.owner.internSignatureType(accessor.ReturnType)
	if err != nil {
		return builtValue{}, err
	}
	if accessor.ReturnType.Kind == sema.VoidType {
		fb.emit(op)
		return builtValue{typ: ret}, nil
	}
	if accessor.ReturnType.Kind == sema.ReferenceType || accessor.ReturnType.Kind == sema.StringViewType {
		return builtValue{}, fb.unsupported("property getter returning a reference", token)
	}
	return fb.result(op, ret), nil
}

// buildPropertyRead lowers a property read to its getter call.
func (fb *functionBuilder) buildPropertyRead(expr ast.Expression) (builtValue, bool, error) {
	access, getter, _, ok := fb.propertyAccess(expr)
	if !ok || access.Kind != sema.PropertyRead {
		return builtValue{}, false, nil
	}
	token := expressionToken(expr)
	if getter == nil {
		return builtValue{}, true, fb.unsupported("read of a property without a getter", token)
	}
	callee, target, err := fb.demandAccessor(*getter, access.OwnerType, token)
	if err != nil {
		return builtValue{}, true, err
	}
	var operands []ValueID
	var actions []ArgumentAction
	if !access.Static {
		receiver, action, err := fb.propertyReceiver(expr, target, access.OwnerType)
		if err != nil {
			return builtValue{}, true, err
		}
		operands, actions = []ValueID{receiver.id}, []ArgumentAction{action}
	}
	value, err := fb.callAccessor(PropertyAccessRead, access, callee, target, operands, actions, token)
	return value, true, err
}

// lowerPropertyAssignment lowers `p.P = value` to the setter call: the
// receiver is produced first, then the value, which the setter takes by value
// (a fresh owned value moves in). A compound update reads through the getter
// and writes through the setter with one receiver evaluation.
func (fb *functionBuilder) lowerPropertyAssignment(stmt *ast.AssignmentStatement) (bool, error) {
	access, getter, setter, ok := fb.propertyAccess(stmt.Target)
	if !ok {
		return false, nil
	}
	switch access.Kind {
	case sema.PropertyWrite:
	case sema.PropertyCompoundUpdate:
		if access.Fallible {
			return true, fb.unsupported("fallible compound property update outside a try assignment", stmt.Token)
		}
		_, err := fb.lowerPropertyCompound(stmt, access, getter, setter)
		return true, err
	default:
		return true, fb.unsupported(fmt.Sprintf("property %s outside a try assignment", access.Kind), stmt.Token)
	}
	if setter == nil {
		return true, fb.unsupported("write of a property without a setter", stmt.Token)
	}
	callee, target, err := fb.demandAccessor(*setter, access.OwnerType, stmt.Token)
	if err != nil {
		return true, err
	}
	var operands []ValueID
	var actions []ArgumentAction
	if !access.Static {
		receiver, action, err := fb.propertyReceiver(stmt.Target, target, access.OwnerType)
		if err != nil {
			return true, err
		}
		operands, actions = []ValueID{receiver.id}, []ArgumentAction{action}
	}
	value, action, err := fb.accessorValue(stmt.Value, target)
	if err != nil {
		return true, err
	}
	operands, actions = append(operands, value.id), append(actions, action)
	_, err = fb.callAccessor(PropertyAccessWrite, access, callee, target, operands, actions, stmt.Token)
	return true, err
}

// accessorValue builds the value a setter takes by value: a trivial copy, or
// a move of a fresh or explicitly moved owned value.
func (fb *functionBuilder) accessorValue(expr ast.Expression, setter sema.Function) (builtValue, ArgumentAction, error) {
	if len(setter.Parameters) == 0 {
		return builtValue{}, "", fb.unsupported("property setter without a value parameter", expressionToken(expr))
	}
	parameter := setter.Parameters[len(setter.Parameters)-1]
	expected, err := fb.owner.internType(parameter.Type)
	if err != nil {
		return builtValue{}, "", err
	}
	value, err := fb.buildExpr(expr, expected)
	if err != nil {
		return builtValue{}, "", err
	}
	if fb.storageAllowed(value.typ) && trivialDestruction(fb.owner.module, value.typ) {
		return value, ArgumentCopyTrivial, nil
	}
	if !fb.ownedArgumentLowerable(expr) {
		return builtValue{}, "", fb.unsupported("non-trivial property setter value", expressionToken(expr))
	}
	return value, ArgumentMove, nil
}

// lowerPropertyCompound lowers `p.P op= value`: the receiver is produced once
// with the setter's capability, the getter reads through a fresh reborrow of
// it, the operator applies with its checks and the setter writes the result
// (semantic_ir.md § 26(2)–(3); properties.md § 7).
func (fb *functionBuilder) lowerPropertyCompound(stmt *ast.AssignmentStatement, access sema.ResolvedPropertyAccess, getter, setter *sema.Function) (builtValue, error) {
	resolved, ok := fb.owner.analyzer.ResolvedCompoundAssignmentOf(stmt)
	if !ok || getter == nil || setter == nil || resolved.RightType == nil {
		return builtValue{}, fb.unsupported("compound property update "+stmt.Operator+" without a getter, a setter and a resolved operator", stmt.Token)
	}
	if _, ok := stmt.Value.(*ast.TryExpression); ok {
		return builtValue{}, fb.unsupported("try arithmetic", expressionToken(stmt.Value))
	}
	setID, setTarget, err := fb.demandAccessor(*setter, access.OwnerType, stmt.Token)
	if err != nil {
		return builtValue{}, err
	}
	getID, getTarget, err := fb.demandAccessor(*getter, access.OwnerType, stmt.Token)
	if err != nil {
		return builtValue{}, err
	}
	loc := location(stmt.Token)
	var setOperands, getOperands []ValueID
	var setActions, getActions []ArgumentAction
	if !access.Static {
		receiver, action, err := fb.propertyReceiver(stmt.Target, setTarget, access.OwnerType)
		if err != nil {
			return builtValue{}, err
		}
		facts, known := fb.owner.analyzer.ResolvedCompoundGetterReborrowOf(stmt.Target)
		if !known || !facts.Checkable() {
			return builtValue{}, fb.unsupported("compound property update without its getter reborrow", stmt.Token)
		}
		read, readAction, err := fb.reborrowReceiver(receiver, facts, getTarget.ReceiverMutable, loc)
		if err != nil {
			return builtValue{}, err
		}
		setOperands, setActions = []ValueID{receiver.id}, []ArgumentAction{action}
		getOperands, getActions = []ValueID{read.id}, []ArgumentAction{readAction}
	}
	current, err := fb.callAccessor(PropertyAccessCompoundRead, access, getID, getTarget, getOperands, getActions, stmt.Token)
	if err != nil {
		return builtValue{}, err
	}
	rightType, err := fb.owner.internType(*resolved.RightType)
	if err != nil {
		return builtValue{}, err
	}
	resultType, err := fb.owner.internType(resolved.ResultType)
	if err != nil {
		return builtValue{}, err
	}
	if resultType != current.typ {
		return builtValue{}, fmt.Errorf("compound property update type does not match its getter")
	}
	value, err := fb.buildExpr(stmt.Value, rightType)
	if err != nil {
		return builtValue{}, err
	}
	op := Operation{Operands: []ValueID{current.id, value.id}, Location: loc, Operator: strings.TrimSuffix(stmt.Operator, "=")}
	if !semanticOperatorKind(resolved, &op) {
		return builtValue{}, fb.unsupported("compound property update "+stmt.Operator, stmt.Token)
	}
	updated := builtValue{}
	if resolved.RuntimeCheck || resolved.ShiftCountProven {
		if updated, err = fb.emitCheckedOperator(op, resultType, nil, nil, nil); err != nil {
			return builtValue{}, err
		}
	} else {
		updated = fb.result(op, resultType)
	}
	if !fb.storageAllowed(updated.typ) || !trivialDestruction(fb.owner.module, updated.typ) {
		return builtValue{}, fb.unsupported("compound property update of a non-trivial value", stmt.Token)
	}
	setOperands, setActions = append(setOperands, updated.id), append(setActions, ArgumentCopyTrivial)
	// A fallible compound update's setter yields its Result (properties.md § 7).
	return fb.callAccessor(PropertyAccessCompoundWrite, access, setID, setTarget, setOperands, setActions, stmt.Token)
}

// reborrowReceiver derives a fresh shared or mutable reborrow of a receiver
// reference with Sema's identity for it.
func (fb *functionBuilder) reborrowReceiver(source builtValue, facts sema.ReferenceFacts, mutable bool, loc Location) (builtValue, ArgumentAction, error) {
	metadata := fb.references[source.id]
	sourceType, ok := fb.owner.module.Types.Lookup(source.typ)
	if metadata == nil || !ok || sourceType.Kind != TypeReference || mutable && !sourceType.Mutable {
		return builtValue{}, "", &UnsupportedFeatureError{Feature: "receiver reborrow without a reference source of sufficient authority", Package: fb.owner.maxPackage, Location: loc}
	}
	derived := referenceMetadataOf(facts)
	derived.SourceBorrow = metadata.Borrow
	if len(derived.OriginStorages) == 0 {
		derived.OriginStorages = append([]uint32(nil), metadata.OriginStorages...)
	}
	if derived.Borrow == derived.SourceBorrow {
		return builtValue{}, "", &UnsupportedFeatureError{Feature: "receiver reborrow without a distinct borrow identity", Package: fb.owner.maxPackage, Location: loc}
	}
	kind, ownership, action := OpReferenceReborrowShared, OwnershipSharedReference, ArgumentSharedReference
	if mutable {
		kind, ownership, action = OpReferenceReborrowMutable, OwnershipMutableReference, ArgumentMutableReference
	}
	referenceType := fb.owner.module.Types.Intern(Type{Kind: TypeReference, Element: sourceType.Element, Mutable: mutable})
	value := fb.newValue(referenceType, ownership, loc)
	fb.emit(Operation{Kind: kind, Operands: []ValueID{source.id}, Results: []Value{value}, Reference: derived, Location: loc})
	fb.recordReference(value.ID, derived)
	return builtValue{id: value.ID, typ: referenceType}, action, nil
}

// lowerFallibleProperty lowers `try p.P = value` through a `try set` setter:
// the setter call yields Result[void, E]; its success edge continues and its
// failure edge propagates through the enclosing Result or enters the local
// handlers Sema resolved for the statement (errorhandling.md §§ 23, 35;
// semantic_ir.md § 26(3)).
func (fb *functionBuilder) lowerFallibleProperty(stmt *ast.TryAssignmentStatement, fact sema.ResolvedTryAssignment) (bool, error) {
	access, getter, setter, ok := fb.propertyAccess(stmt.Assignment.Target)
	if !ok || !access.Fallible {
		return false, nil
	}
	var result builtValue
	if access.Kind == sema.PropertyCompoundUpdate {
		var err error
		if result, err = fb.lowerPropertyCompound(stmt.Assignment, access, getter, setter); err != nil {
			return true, err
		}
	} else {
		if setter == nil {
			return true, fb.unsupported("fallible write of a property without a setter", stmt.Token)
		}
		callee, target, err := fb.demandAccessor(*setter, access.OwnerType, stmt.Token)
		if err != nil {
			return true, err
		}
		var operands []ValueID
		var actions []ArgumentAction
		if !access.Static {
			receiver, action, err := fb.propertyReceiver(stmt.Assignment.Target, target, access.OwnerType)
			if err != nil {
				return true, err
			}
			operands, actions = []ValueID{receiver.id}, []ArgumentAction{action}
		}
		value, action, err := fb.accessorValue(stmt.Assignment.Value, target)
		if err != nil {
			return true, err
		}
		operands, actions = append(operands, value.id), append(actions, action)
		if result, err = fb.callAccessor(PropertyAccessFallibleWrite, access, callee, target, operands, actions, stmt.Token); err != nil {
			return true, err
		}
	}
	resultType, ok := fb.owner.module.Types.Lookup(result.typ)
	if !ok || resultType.Kind != TypeResult {
		return true, fb.unsupported("fallible setter without a Result", stmt.Token)
	}
	boolType, err := fb.owner.internType(sema.Type{Name: "bool", Kind: sema.BoolType})
	if err != nil {
		return true, err
	}
	loc := location(stmt.Token)
	isErr := fb.result(Operation{Kind: OpResultIsErr, Operands: []ValueID{result.id}, Location: loc}, boolType)
	errorBlock, successBlock := fb.newBlock(), fb.newBlock()
	fb.emit(Operation{Kind: OpCondBranch, Operands: []ValueID{isErr.id}, Successors: []BranchTarget{{Block: errorBlock.ID}, {Block: successBlock.ID}}, Location: loc})
	fb.current = errorBlock
	errorValue := fb.result(Operation{Kind: OpResultUnwrapErr, Operands: []ValueID{result.id}, Location: loc}, resultType.Error)
	if fact.Kind == sema.ResolvedTryAssignmentHandled {
		if !lowerableTryPlan(fact.HandlerPlan) {
			return true, fb.unsupported("non-exhaustive fallible property handler plan", stmt.Token)
		}
		handlers := &ast.TryExpression{Token: stmt.Token, Handlers: stmt.Handlers}
		_, err := fb.buildLocalTryHandlers(handlers, fact.HandlerPlan, successBlock, builtValue{typ: resultType.Success}, errorBlock, errorValue)
		return true, err
	}
	enclosing, err := fb.owner.internType(fact.EnclosingResultType)
	if err != nil {
		return true, err
	}
	enclosingType, ok := fb.owner.module.Types.Lookup(enclosing)
	if !ok || enclosingType.Kind != TypeResult || enclosingType.Error != errorValue.typ {
		return true, fb.unsupported("fallible property propagation with error widening", stmt.Token)
	}
	propagated := fb.result(Operation{Kind: OpResultErr, Operands: []ValueID{errorValue.id}, Location: loc}, enclosing)
	fb.emit(Operation{Kind: OpReturn, Operands: []ValueID{propagated.id}, Location: loc})
	fb.current = successBlock
	return true, nil
}

// completeFallibleSetter ends a `try set` body that completes normally with
// its success: the setter returns Ok() (properties.md § 6; errorhandling.md § 24).
func (fb *functionBuilder) completeFallibleSetter(decl *ast.FunctionDeclaration, resolved sema.Function) error {
	if fb.current == nil || resolved.ImplTarget == "" || resolved.ReturnType.Kind != sema.ResultType || !strings.HasSuffix(resolved.Name, ".try-set") {
		return nil
	}
	resultType, err := fb.owner.internSignatureType(resolved.ReturnType)
	if err != nil {
		return err
	}
	loc := location(decl.Body.EndToken)
	if decl.Body.EndToken.Line == 0 {
		loc = location(decl.Token)
	}
	ok, err := fb.buildResultConstructor(nil, resultType, true, loc)
	if err != nil {
		return err
	}
	fb.emit(Operation{Kind: OpReturn, Operands: []ValueID{ok.id}, Location: loc})
	fb.current = nil
	return nil
}
