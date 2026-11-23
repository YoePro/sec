package semantic

import (
	"sec/internal/ast"
	"sec/internal/sema"
	"strings"
)

// buildCall consumes the selected Sema target and rejects unavailable method
// dispatch before attempting any direct-call lowering.
// Rules: rules/compiler/semantic_ir.md — resolved call facts;
// rules/declarations/interfaces.md — §6; compiler_pipeline.md — lowering prerequisites.
func (fb *functionBuilder) buildCall(call *ast.CallExpression) (builtValue, error) {
	if boundary, ok := fb.dynamicArrayMemberBoundary(call); ok {
		return builtValue{}, boundary
	}
	if value, handled, err := fb.buildArenaCall(call); handled {
		return value, err
	}
	if plan, materialized := fb.owner.analyzer.ResolvedRuneArrayMaterializationOf(call); materialized {
		if member, ok := call.Callee.(*ast.MemberExpression); ok {
			return fb.buildRuneArrayMaterialization(call, member, plan)
		}
	}
	if boundary, ok := fb.coreArrayMaterializationBoundary(call); ok {
		return builtValue{}, boundary
	}
	if _, rounded := fb.owner.analyzer.RoundedConversionOf(call); rounded {
		// MD-047: Semantic IR has no rounding-conversion operation yet.
		return builtValue{}, fb.unsupported("explicit rounded conversion (RoundingMode)", call.Token)
	}
	if fb.owner.analyzer.IsTypePackForward(call) {
		return fb.buildPackForward(call)
	}
	if snapshot, current := fb.owner.analyzer.BuildInfoCurrentOf(call); current {
		return fb.buildBuildInfoCurrent(call, snapshot)
	}
	if value, handled, err := fb.buildAtomicObjectCall(call); handled {
		return value, err
	}
	// MD-049: a proven constant conversion into a named decimal type is a
	// const.decimal on the target's grid.
	if pair, constant := fb.owner.analyzer.ResolvedDecimalConstantConversionOf(call); constant {
		if target, typed := fb.owner.analyzer.ResolvedTypeOf(call); typed {
			typeID, err := fb.owner.internType(target)
			if err != nil {
				return builtValue{}, err
			}
			decimal, err := fb.decimalFromSemaConstant(sema.DefaultConstant{Decimal: &pair, Lexeme: call.String()}, typeID)
			if err != nil {
				return builtValue{}, err
			}
			return fb.result(Operation{Kind: OpConstDecimal, Decimal: &decimal, Location: location(call.Token)}, typeID), nil
		}
	}
	if fb.owner.analyzer.IsDynamicArrayLengthCall(call) && fb.owner.maxPackage >= dynamicArrayLoweringPackage && len(call.Arguments) == 1 {
		return fb.buildDynamicArrayLength(call.Arguments[0], call.Token)
	}
	if plan, allocating := fb.owner.analyzer.ResolvedAllocationPlanOf(call); allocating && fb.owner.maxPackage >= dynamicArrayLoweringPackage {
		if member, ok := call.Callee.(*ast.MemberExpression); ok && member.Property != nil {
			return fb.buildDynamicArrayAppend(call, member, plan)
		}
	}
	resolved, ok := fb.owner.analyzer.ResolvedCallTarget(call)
	if !ok {
		// Public lookup is implemented, but checked materialization needs
		// P23 and its P18/P19 allocation/string-storage prerequisites.
		// Rules: rules/corrections/applied/md109-core-string-materialization-correction-20261121.md
		// — §§5.2,9.3,10.3; rules/mlir/packages/sec-mlir-dialect_package23.md — §§35–37.
		if member, ok := call.Callee.(*ast.MemberExpression); ok && member.Property != nil {
			site := member.Property.Token
			if known, ok := fb.owner.analyzer.CompilerKnownMemberAt(site.File, site.Line, site.Column); ok && (known.ID == "CKM-RESULT-OK" || known.ID == "CKM-RESULT-ERR") {
				return fb.buildResultProjection(call, member, known.ID == "CKM-RESULT-OK")
			}
			if known, ok := fb.owner.analyzer.CompilerKnownMemberAt(site.File, site.Line, site.Column); ok && (known.ID == "CKM-STRING-FROMBYTEARRAY" || known.ID == "CKM-STRING-FROMRUNEARRAY") {
				return builtValue{}, fb.unsupported("string."+known.Name+" materialization requires P23 allocation/string lowering", site)
			} else if ok && strings.HasPrefix(known.ID, "CKM-TOSTRING") {
				return builtValue{}, fb.unsupported("ToString() needs P23 allocation/string lowering and per-specialization selection of a user ToString", site)
			}
		}
		if operation, register := fb.owner.analyzer.ResolvedRegisterOperationOf(call); register {
			return fb.buildHardwareRegisterOperation(call, operation)
		}
		if result, typed := fb.owner.analyzer.TypeOf(call); typed && result.Kind == sema.RegisterType {
			// MD-110 conversions are built before buildCall; reaching here
			// means Sema recorded no conversion for the operand.
			return builtValue{}, fb.unsupported("integer-to-register conversion "+result.Name+"(...) without a resolved MD-110 conversion", call.Token)
		}
		return builtValue{}, fb.unsupported("unresolved function call", call.Token)
	}
	if resolved.Kind == sema.ResolvedInterfaceMethodCall {
		return builtValue{}, fb.unsupported("interface method dispatch", call.Token)
	}
	callee := functionID(resolved.Function, fb.owner.module.Types)
	parameters := resolved.Function.Parameters
	if !isGenericInstance(resolved.Function) {
		if err := fb.owner.declareImportedCallee(resolved.Function, call.Token); err != nil {
			return builtValue{}, err
		}
	}
	if isGenericInstance(resolved.Function) && resolved.Kind != sema.ResolvedInterfaceMethodCall {
		spec, err := fb.owner.demandSpecialization(call, resolved.Function)
		if err != nil {
			return builtValue{}, err
		}
		callee, parameters = spec.id, spec.function.Parameters
	}
	instanceMethod := !resolved.Function.Static && resolved.Function.ImplTarget != "" && resolved.Kind != sema.ResolvedInterfaceMethodCall
	if instanceMethod && !fb.owner.instanceMethodLowerable(resolved.Function) {
		return builtValue{}, fb.unsupported("instance method call", call.Token)
	}
	if resolved.Kind == sema.ResolvedStaticMethodCall {
		if resolved.Function.Static && !staticMethodLowerable(resolved.Function) {
			return builtValue{}, fb.unsupported("static method of a generic implementation", call.Token)
		}
	}
	args := make([]ValueID, 0, len(call.Arguments)+1)
	actions := make([]ArgumentAction, 0, len(call.Arguments)+1)
	if instance, hardware := fb.hardwareReceiverArgument(call); instanceMethod && hardware {
		// MD-077 § 12(2): a member called on a hardware instance runs its
		// hardware variant with the lent instance authority.
		variant, err := fb.owner.demandHardwareVariant(resolved.Function, call.Token)
		if err != nil {
			return builtValue{}, err
		}
		callee = variant
		args = append(args, instance.id)
		actions = append(actions, ArgumentHardwareInstance)
	} else if instanceMethod {
		receiver, action, err := fb.buildReceiverArgument(call, resolved.Function)
		if err != nil {
			return builtValue{}, err
		}
		args = append(args, receiver.id)
		actions = append(actions, action)
	}
	// Argument moves cancel their source cleanup only when the call commits,
	// after every argument was prepared (destruction.md § 9.2(3)).
	pendingCancels := len(fb.pendingArgumentCancels)
	// Fresh owned arguments stay prepared until the call takes them (P17-16).
	preparedDepth := len(fb.prepared)
	defer fb.takePrepared(preparedDepth)
	argumentValues, argumentActions, err := fb.buildCallArguments(call, resolved.Arguments, parameters)
	if err != nil {
		return builtValue{}, err
	}
	args = append(args, argumentValues...)
	actions = append(actions, argumentActions...)
	ret, err := fb.owner.internSignatureType(resolved.Function.ReturnType)
	if err != nil {
		return builtValue{}, err
	}
	kind := OpDirectCall
	if resolved.Kind == sema.ResolvedForeignCall {
		kind = OpForeignCall
	}
	for _, cancel := range fb.pendingArgumentCancels[pendingCancels:] {
		fb.emit(cancel)
	}
	fb.pendingArgumentCancels = fb.pendingArgumentCancels[:pendingCancels]
	op := Operation{Kind: kind, Callee: callee, Operands: args, ArgumentActions: actions, Location: location(call.Token)}
	if err := fb.forwardAllocationContext(&op, resolved, call); err != nil {
		return builtValue{}, err
	}
	if call.HasUnsafePrefix() {
		marker := location(call.Unsafe)
		op.UnsafeAuthorization = &marker
	}
	if resolved.Function.ReturnType.Kind == sema.VoidType {
		fb.emit(op)
		return builtValue{typ: ret}, nil
	}
	if resolved.Function.ReturnType.Kind == sema.ReferenceType || resolved.Function.ReturnType.Kind == sema.StringViewType && fb.owner.maxPackage >= sliceLoweringPackage {
		// The returned reference carries the caller-side facts Sema
		// instantiated from the callee's returned-origin summary
		// (package15.md §§ 61-62).
		facts, ok := fb.owner.analyzer.ResolvedCallReferenceOf(call)
		if !ok || !facts.Checkable() {
			return builtValue{}, fb.unsupported("call returning a reference without an instantiated origin", call.Token)
		}
		ownership := OwnershipSharedReference
		if resolved.Function.ReturnType.ReferenceMutable {
			ownership = OwnershipMutableReference
		}
		metadata := referenceMetadataOf(facts)
		value := fb.newValue(ret, ownership, op.Location)
		op.Results = []Value{value}
		op.Reference = metadata
		fb.emit(op)
		fb.recordReference(value.ID, metadata)
		return builtValue{id: value.ID, typ: ret}, nil
	}
	return fb.result(op, ret), nil
}
