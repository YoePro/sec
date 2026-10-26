package semantic

import (
	"fmt"

	"sec/internal/ast"
	"sec/internal/sema"
)

// buildIntegerConversion lowers an explicit built-in integer conversion that
// Sema proved exact to one numeric.convert-exact operation; the argument is
// built in its own source type, so no implicit shaping hides a narrowing.
// A conversion that can fail is the checked form under a failure-set try:
// numeric.convert-checked yields a candidate and a failed flag, the failure
// edge routes ConversionError.OutOfRange and only the success edge consumes
// the candidate (package20.md §§ 48–54, 71).
//
// Rules:
//   - rules/types/types.md — "Explicit conversions"
//   - rules/mlir/packages/sec-mlir-dialect_package20.md — §§ 44, 60–63
func (fb *functionBuilder) buildIntegerConversion(call *ast.CallExpression, conversion sema.ResolvedIntegerConversion) (builtValue, error) {
	exact := conversion.Exact && conversion.Proof.Proven()
	point, protected := fb.failurePoint(call)
	if !exact && !protected {
		return builtValue{}, fb.unsupported("checked integer conversion "+conversion.SourceType.Name+" to "+conversion.TargetType.Name+" (needs try)", call.Token)
	}
	if exact && protected {
		// The relation can fail but this target's proof shows the value
		// fits: the point has no failure edge.
		fb.failures.routed[call] = true
	}
	if !exact {
		return fb.buildCheckedIntegerConversion(call, conversion, point)
	}
	sourceType, err := fb.owner.internType(conversion.SourceType)
	if err != nil {
		return builtValue{}, err
	}
	targetType, err := fb.owner.internType(conversion.TargetType)
	if err != nil {
		return builtValue{}, err
	}
	value, err := fb.buildExpr(call.Arguments[0], sourceType)
	if err != nil {
		return builtValue{}, err
	}
	if value.typ == targetType {
		return value, nil
	}
	return fb.result(Operation{Kind: OpNumericConvertExact, Operands: []ValueID{value.id}, ConversionProof: conversionProofOf(conversion.Proof), Location: location(call.Token)}, targetType), nil
}

// conversionProofOf carries Sema's proof into Semantic IR unchanged.
func conversionProofOf(proof sema.IntegerConversionProof) *ConversionProof {
	return &ConversionProof{Basis: string(proof.Basis), PlanPointerWidthBits: proof.PlanPointerWidthBits}
}

// compilerWideningProof is the proof of the builder's own lossless widening
// into a type wider than every source domain on every plan (for-loop range
// distances into int128/int256).
var compilerWideningProof = ConversionProof{Basis: "compiler-widening"}

// validConversionProof checks a proof's basis and plan compatibility.
func validConversionProof(module *Module, proof *ConversionProof) error {
	if proof == nil {
		return fmt.Errorf("missing exactness proof")
	}
	switch proof.Basis {
	case string(sema.IntegerConversionConstant), string(sema.IntegerConversionSourceDomain),
		string(sema.IntegerConversionClosedEnumDomain), string(sema.IntegerConversionOpenEnumDomain), compilerWideningProof.Basis,
		registerFieldIntervalProof.Basis:
	default:
		return fmt.Errorf("unknown exactness proof basis %q", proof.Basis)
	}
	if proof.PlanPointerWidthBits != 0 && proof.PlanPointerWidthBits != module.PointerWidthBits {
		return fmt.Errorf("exactness proof made for a %d-bit plan, module plan is %d-bit", proof.PlanPointerWidthBits, module.PointerWidthBits)
	}
	return nil
}

// OpNumericConvertChecked is P20's NumericConvertCheckedOp for built-in
// integer targets: one operand, a target-typed candidate and a bool failed
// flag; ConversionFailure is the static reason the failure edge maps to
// ConversionError (package20.md §§ 71–72).
const OpNumericConvertChecked OpKind = "numeric.convert-checked"

// buildCheckedIntegerConversion emits the checked conversion, branches on
// its failed flag and routes the ConversionError.OutOfRange variant to the active
// failure-set try; the success edge continues with the candidate.
func (fb *functionBuilder) buildCheckedIntegerConversion(call *ast.CallExpression, conversion sema.ResolvedIntegerConversion, point sema.TryFailurePoint) (builtValue, error) {
	failure, checked := fb.owner.analyzer.CheckedConversionOf(call)
	if !checked {
		failure = sema.ConversionFailureOutOfRange
	}
	if failure != sema.ConversionFailureOutOfRange {
		return builtValue{}, fb.unsupported("checked conversion with "+string(failure)+" failure", call.Token)
	}
	sourceType, err := fb.owner.internType(conversion.SourceType)
	if err != nil {
		return builtValue{}, err
	}
	targetType, err := fb.owner.internType(conversion.TargetType)
	if err != nil {
		return builtValue{}, err
	}
	if !checkedConversionInteger(fb.owner.module, sourceType) || !checkedConversionInteger(fb.owner.module, targetType) {
		return builtValue{}, fb.unsupported("checked conversion between "+conversion.SourceType.Name+" and "+conversion.TargetType.Name, call.Token)
	}
	errorType, err := fb.owner.internType(point.ErrorType)
	if err != nil {
		return builtValue{}, err
	}
	// ConversionError is a closed union error (sec/core/error.sec); the
	// intrinsic failures are its payload-less variants.
	definition, ok := fb.owner.unionDefinition(errorType)
	if !ok {
		return builtValue{}, fb.unsupported("ConversionError union definition", call.Token)
	}
	variantIndex := -1
	for index, variant := range definition.Variants {
		if variant.Name == string(failure) && variant.Kind == UnionVariantEmpty {
			variantIndex = index
		}
	}
	if variantIndex < 0 {
		return builtValue{}, fb.unsupported("ConversionError."+string(failure), call.Token)
	}
	boolType, err := fb.owner.internType(sema.Type{Name: "bool", Kind: sema.BoolType})
	if err != nil {
		return builtValue{}, err
	}
	value, err := fb.buildExpr(call.Arguments[0], sourceType)
	if err != nil {
		return builtValue{}, err
	}
	loc := location(call.Token)
	candidate := fb.newValue(targetType, OwnershipImmediate, loc)
	failed := fb.newValue(boolType, OwnershipImmediate, loc)
	fb.emit(Operation{Kind: OpNumericConvertChecked, Operands: []ValueID{value.id}, Results: []Value{candidate, failed}, ConversionFailure: string(failure), Location: loc})
	errorBlock, successBlock := fb.newBlock(), fb.newBlock()
	fb.emit(Operation{Kind: OpCondBranch, Operands: []ValueID{failed.ID}, Successors: []BranchTarget{{Block: errorBlock.ID}, {Block: successBlock.ID}}, Location: loc})
	fb.current = errorBlock
	errorValue := fb.result(Operation{Kind: OpUnionConstruct, UnionVariant: UnionVariantIndex(variantIndex), Location: loc}, errorType)
	fb.routeFailure(call, errorValue, loc)
	fb.current = successBlock
	return builtValue{id: candidate.ID, typ: targetType}, nil
}

// checkedConversionInteger reports whether id is a built-in signed or
// unsigned integer (or byte) a checked conversion supports.
func checkedConversionInteger(module *Module, id TypeID) bool {
	typ, ok := module.Types.Lookup(id)
	return ok && (typ.Kind == TypeInt || typ.Kind == TypeUint || typ.Kind == TypeByte)
}

func verifyCheckedConversionOperation(module *Module, op Operation, values map[ValueID]Value) error {
	if len(op.Operands) != 1 || len(op.Results) != 2 || op.ConversionFailure != string(sema.ConversionFailureOutOfRange) ||
		!checkedConversionInteger(module, values[op.Operands[0]].Type) || !checkedConversionInteger(module, op.Results[0].Type) ||
		!typeHasKind(module.Types, op.Results[1].Type, TypeBool) {
		return fmt.Errorf("invalid numeric.convert-checked")
	}
	return nil
}

// verifyCheckedConversionCandidates requires every use of a checked
// conversion candidate (and of a float.clamp-checked value) to sit where the conversion is known to have
// succeeded: in a block entered only by the false edge of a conditional
// branch on its own failed flag, or dominated by such a block
// (package20.md § 71: the candidate is consumed only on success).
func verifyCheckedConversionCandidates(fn *Function, dom map[BlockID]map[BlockID]bool) error {
	candidates := map[ValueID]ValueID{}
	predecessors := map[BlockID]int{}
	for _, block := range fn.Blocks {
		for _, op := range block.Operations {
			if (op.Kind == OpNumericConvertChecked || op.Kind == OpFloatClampChecked) && len(op.Results) == 2 {
				candidates[op.Results[0].ID] = op.Results[1].ID
			}
			for _, successor := range op.Successors {
				predecessors[successor.Block]++
			}
		}
	}
	if len(candidates) == 0 {
		return nil
	}
	successEntries := map[ValueID][]BlockID{}
	for _, block := range fn.Blocks {
		for _, op := range block.Operations {
			if op.Kind == OpCondBranch && len(op.Operands) == 1 && len(op.Successors) == 2 && predecessors[op.Successors[1].Block] == 1 {
				successEntries[op.Operands[0]] = append(successEntries[op.Operands[0]], op.Successors[1].Block)
			}
		}
	}
	for _, block := range fn.Blocks {
		for _, op := range block.Operations {
			for _, operand := range operationOperands(op) {
				failed, candidate := candidates[operand]
				if !candidate {
					continue
				}
				guarded := false
				for _, entry := range successEntries[failed] {
					if entry == block.ID || dom[block.ID][entry] {
						guarded = true
					}
				}
				if !guarded {
					return fmt.Errorf("numeric.convert-checked candidate %%%d used where the conversion may have failed", operand)
				}
			}
		}
	}
	return nil
}
