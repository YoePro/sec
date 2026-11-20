package semantic

import (
	"fmt"
	"math/big"
	"sec/internal/ast"
	"sec/internal/sema"
)

// buildOrdinaryCallArgument preserves the ordinary argument borrow, slice and
// owned-temporary preparation paths for arguments beside a spread.
// Rules: rules/declarations/functions.md §§17–18; destruction.md §9.2;
// rules/mlir/packages/sec-mlir-dialect_package15.md §§59–60.
func (fb *functionBuilder) buildOrdinaryCallArgument(arg ast.Expression, parameter *sema.FunctionParameter) (builtValue, ArgumentAction, error) {
	expected := TypeID(0)
	if parameter != nil {
		if fb.owner.maxPackage >= sliceLoweringPackage && (sourceSliceType(parameter.Type) || parameter.Type.Kind == sema.StringViewType) {
			var sliceType TypeID
			mutable := false
			var err error
			if parameter.Type.Kind == sema.StringViewType {
				sliceType = fb.owner.module.Types.stringViewType()
			} else {
				sliceType, mutable, err = fb.owner.internSourceSliceType(parameter.Type)
			}
			if err != nil {
				return builtValue{}, "", err
			}
			v, err := fb.lowerSliceArgument(arg, sliceType)
			if err != nil {
				return builtValue{}, "", err
			}
			action := ArgumentSharedReference
			if mutable {
				action = ArgumentMutableReference
			}
			return v, action, nil
		}
		expected, _ = fb.owner.internType(parameter.Type)
	}
	if action, isReference := fb.referenceArgumentAction(expected); isReference {
		// package 15 §§ 59-60: the argument is the reference value its
		// resolved implicit borrow, explicit borrow or pass produces.
		v, err := fb.buildExpr(arg, expected)
		if err != nil {
			return builtValue{}, "", err
		}
		if v.typ != expected {
			return builtValue{}, "", fb.unsupported("reference argument whose type differs from its parameter", expressionToken(arg))
		}
		return v, action, nil
	}
	if _, isReference := arg.(*ast.RefExpression); isReference {
		return builtValue{}, "", fb.unsupported("reference argument call", expressionToken(arg))
	}
	v, err := fb.buildExpr(arg, expected)
	if err != nil {
		return builtValue{}, "", err
	}
	action := ArgumentCopyTrivial
	if !fb.storageAllowed(v.typ) || !trivialDestruction(fb.owner.module, v.typ) {
		if !fb.ownedArgumentLowerable(arg) {
			return builtValue{}, "", fb.unsupported("non-trivial call argument", expressionToken(arg))
		}
		action = ArgumentMove
		if fb.freshArgument(arg) && parameter != nil {
			fb.prepareOperand(v, parameter.Type, expressionToken(arg))
		}
	}
	return v, action, nil
}

// buildCallArguments consumes Sema's source table and prepared slots. A spread
// source is built once, then its resolved projections are emitted before the
// following source. Zero-element sources still execute. Ordinary arguments
// retain the existing reference/cleanup path. Unsupported runtime segments,
// reference projections and semantic copies remain explicit boundaries.
// Rules: rules/declarations/spread.md §§3,4.1–4.4;
// rules/corrections/applied/semantic-ir-spread-correction-20260813.md
// — Required semantic contract; functions.md §§17–18.
func (fb *functionBuilder) buildCallArguments(call *ast.CallExpression, plan *sema.ResolvedCallArgumentPlan, parameters []sema.FunctionParameter) ([]ValueID, []ArgumentAction, error) {
	values := []ValueID{}
	actions := []ArgumentAction{}
	if plan == nil {
		for index, arg := range call.Arguments {
			if _, spread := explicitMoveFree(arg).(*ast.SpreadExpression); spread {
				return nil, nil, fb.unsupported("spread call argument without a resolved preparation plan", expressionToken(arg))
			}
			var parameter *sema.FunctionParameter
			if index < len(parameters) {
				parameter = &parameters[index]
			}
			value, action, err := fb.buildOrdinaryCallArgument(arg, parameter)
			if err != nil {
				return nil, nil, err
			}
			values, actions = append(values, value.id), append(actions, action)
		}
		return values, actions, nil
	}
	if err := validateCallArgumentPlan(plan, parameters); err != nil {
		return nil, nil, fb.unsupported(err.Error(), call.Token)
	}
	indexType, err := fb.owner.internType(sema.Type{Name: "uint", Kind: sema.UintType})
	if err != nil {
		return nil, nil, err
	}
	argumentIndex := 0
	for sourceIndex, source := range plan.Sources {
		if !source.Spread {
			value, action, err := fb.buildOrdinaryCallArgument(source.Expression, &parameters[argumentIndex])
			if err != nil {
				return nil, nil, err
			}
			values, actions = append(values, value.id), append(actions, action)
			argumentIndex++
			continue
		}
		array, err := fb.buildExpr(source.Expression, 0)
		if err != nil {
			return nil, nil, err
		}
		arrayType, known := fb.owner.module.Types.Lookup(array.typ)
		if !known || arrayType.Kind != TypeArray {
			return nil, nil, fb.unsupported("spread call source without a fixed-array value", expressionToken(source.Expression))
		}
		if length, _ := sema.FixedArrayLength(source.Type); length.String() != arrayType.Length {
			return nil, nil, fmt.Errorf("spread source length disagrees with its resolved preparation")
		}
		for argumentIndex < len(plan.Expanded) && plan.Expanded[argumentIndex].Source == sourceIndex {
			slot := plan.Expanded[argumentIndex]
			preparedType := fb.owner.substituteTemplateType(slot.ValueType, fb.owner.substitution)
			if element, known := fb.owner.semaTypes[arrayType.Element]; !known || canonicalSemaType(element) != canonicalSemaType(preparedType) {
				return nil, nil, fmt.Errorf("spread prepared type disagrees with its array element")
			}
			expected, err := fb.owner.internType(parameters[argumentIndex].Type)
			if err != nil {
				return nil, nil, err
			}
			if expected != arrayType.Element {
				return nil, nil, fb.unsupported("spread prepared value whose type differs from its parameter", expressionToken(source.Expression))
			}
			loc := location(expressionToken(source.Expression))
			index := fb.result(Operation{Kind: OpConstInt, Integer: big.NewInt(slot.ElementIndex), Location: loc}, indexType)
			value := fb.result(Operation{
				Kind: OpArrayExtract, Operands: []ValueID{array.id, index.id},
				ArrayActions:   []ArrayTransferAction{ArrayActionCopyTrivial},
				ArrayCheckKind: ArrayIndexProvenSafe, ArrayProofKind: ArrayIndexProofConstant,
				Location: loc,
			}, arrayType.Element)
			values, actions = append(values, value.id), append(actions, ArgumentCopyTrivial)
			argumentIndex++
		}
	}
	return values, actions, nil
}

// validateCallArgumentPlan proves that normalization can consume the complete
// recorded expansion without rebuilding it from syntax. It admits no runtime
// pack erasure, inferred element move, missing projection or reordered source.
// Rules: rules/declarations/spread.md §§3,4; semantic-ir spread correction.
func validateCallArgumentPlan(plan *sema.ResolvedCallArgumentPlan, parameters []sema.FunctionParameter) error {
	for _, slot := range plan.Expanded {
		if slot.RuntimeSpread {
			return fmt.Errorf("runtime spread call argument")
		}
	}
	if len(plan.Expanded) != len(parameters) {
		return fmt.Errorf("spread call preparation does not match concrete parameter count")
	}
	next := 0
	for sourceIndex, source := range plan.Sources {
		if source.Expression == nil {
			return fmt.Errorf("spread call preparation has no source")
		}
		count := int64(1)
		if source.Spread {
			length, known := sema.FixedArrayLength(source.Type)
			if !known || !length.IsInt64() || source.Consuming {
				return fmt.Errorf("non-fixed or consuming spread call source")
			}
			count = length.Int64()
		}
		for index := int64(0); index < count; index++ {
			if next >= len(plan.Expanded) {
				return fmt.Errorf("spread call preparation has missing slots")
			}
			slot := plan.Expanded[next]
			if slot.Source != sourceIndex || slot.Prepared != source.Spread {
				return fmt.Errorf("spread call preparation has inconsistent source provenance")
			}
			if source.Spread && (slot.ElementIndex != index || slot.Operation != sema.ArgumentCopyTrivial || slot.Delivery != sema.ArgumentMove) {
				return fmt.Errorf("non-trivial or unordered spread call preparation")
			}
			if !source.Spread && slot.ElementIndex != -1 {
				return fmt.Errorf("ordinary call argument has a spread projection")
			}
			next++
		}
	}
	if next != len(plan.Expanded) {
		return fmt.Errorf("spread call preparation has excess slots")
	}
	return nil
}
