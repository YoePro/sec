package semantic

import (
	"fmt"

	"sec/internal/ast"
	"sec/internal/sema"
)

// Exactness proof bases of P20 binary-float conversion plans beyond the
// integer bases (sema.ResolvedNumericConversionPlan): the same concrete type,
// and an integer argument whose proven value interval fits the target.
const (
	numericIdentityProof      = "identity"
	numericValueIntervalProof = "value-interval"
)

// buildNumericConversion lowers an explicit P20 conversion between binary
// floats, built-in integers and bool from Sema's resolved plan. Float-to-bool
// is the total float.to-bool; a conversion the plan proved exact (widening,
// same format, constant or proven integer interval) is one
// numeric.convert-exact with the plan's proof, and needs no failure edge even
// under try. A conversion that needs a runtime check is the checked form.
//
// Rules:
//   - rules/mlir/packages/sec-mlir-dialect_package20.md — §§ 42–61, 63, 70–72
//   - rules/types/types.md — "Explicit conversions"
func (fb *functionBuilder) buildNumericConversion(call *ast.CallExpression, plan sema.ResolvedNumericConversionPlan) (builtValue, error) {
	point, protected := fb.failurePoint(call)
	if plan.RuntimeCheck && !plan.ProvenExact {
		if !protected {
			return builtValue{}, fb.unsupported("checked numeric conversion "+plan.SourceType.Name+" to "+plan.TargetType.Name+" (needs try)", call.Token)
		}
		return fb.buildCheckedNumericConversion(call, plan, point)
	}
	if protected {
		// The plan proved the value representable: the point has no
		// failure edge (package20.md § 44).
		fb.failures.routed[call] = true
	}
	sourceType, err := fb.owner.internType(plan.SourceType)
	if err != nil {
		return builtValue{}, err
	}
	targetType, err := fb.owner.internType(plan.TargetType)
	if err != nil {
		return builtValue{}, err
	}
	value, err := fb.buildExpr(call.Arguments[0], sourceType)
	if err != nil {
		return builtValue{}, err
	}
	loc := location(call.Token)
	if plan.Kind == sema.NumericConvertFloatToBool {
		return fb.result(Operation{Kind: OpFloatToBool, Operands: []ValueID{value.id}, Location: loc}, targetType), nil
	}
	if value.typ == targetType {
		return value, nil
	}
	proof := &ConversionProof{Basis: plan.Proof.Basis, PlanPointerWidthBits: plan.PlanWidthBits}
	return fb.result(Operation{Kind: OpNumericConvertExact, Operands: []ValueID{value.id}, ConversionProof: proof, Location: loc}, targetType), nil
}

// buildCheckedNumericConversion emits numeric.convert-checked with the
// plan's ordered failures: a candidate, a failed flag and one exclusive
// reason flag per failure. The failed edge dispatches on the reason flags in
// priority order, constructs the matching payload-less ConversionError
// variant and routes it to the active try; only the success edge consumes the
// candidate (package20.md §§ 41, 47–59, 71–72).
func (fb *functionBuilder) buildCheckedNumericConversion(call *ast.CallExpression, plan sema.ResolvedNumericConversionPlan, point sema.TryFailurePoint) (builtValue, error) {
	if len(plan.Failures) == 0 {
		return builtValue{}, fb.unsupported("checked numeric conversion without failure causes", call.Token)
	}
	sourceType, err := fb.owner.internType(plan.SourceType)
	if err != nil {
		return builtValue{}, err
	}
	targetType, err := fb.owner.internType(plan.TargetType)
	if err != nil {
		return builtValue{}, err
	}
	errorType, err := fb.owner.internType(point.ErrorType)
	if err != nil {
		return builtValue{}, err
	}
	definition, ok := fb.owner.unionDefinition(errorType)
	if !ok {
		return builtValue{}, fb.unsupported("ConversionError union definition", call.Token)
	}
	variants := make([]int, len(plan.Failures))
	failures := make([]string, len(plan.Failures))
	for position, failure := range plan.Failures {
		failures[position] = string(failure)
		variants[position] = -1
		for index, variant := range definition.Variants {
			if variant.Name == string(failure) && variant.Kind == UnionVariantEmpty {
				variants[position] = index
			}
		}
		if variants[position] < 0 {
			return builtValue{}, fb.unsupported("ConversionError."+string(failure), call.Token)
		}
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
	results := []Value{candidate, failed}
	for range failures {
		results = append(results, fb.newValue(boolType, OwnershipImmediate, loc))
	}
	fb.emit(Operation{Kind: OpNumericConvertChecked, Operands: []ValueID{value.id}, Results: results, ConversionFailures: failures, Location: loc})
	errorBlock, successBlock := fb.newBlock(), fb.newBlock()
	fb.emit(Operation{Kind: OpCondBranch, Operands: []ValueID{failed.ID}, Successors: []BranchTarget{{Block: errorBlock.ID}, {Block: successBlock.ID}}, Location: loc})
	fb.current = errorBlock
	for position := range failures {
		if position < len(failures)-1 {
			reasonBlock, nextBlock := fb.newBlock(), fb.newBlock()
			fb.emit(Operation{Kind: OpCondBranch, Operands: []ValueID{results[2+position].ID}, Successors: []BranchTarget{{Block: reasonBlock.ID}, {Block: nextBlock.ID}}, Location: loc})
			fb.current = reasonBlock
			errorValue := fb.result(Operation{Kind: OpUnionConstruct, UnionVariant: UnionVariantIndex(variants[position]), Location: loc}, errorType)
			fb.routeFailure(call, errorValue, loc)
			fb.current = nextBlock
			continue
		}
		// The last remaining reason is the one that holds.
		errorValue := fb.result(Operation{Kind: OpUnionConstruct, UnionVariant: UnionVariantIndex(variants[position]), Location: loc}, errorType)
		fb.routeFailure(call, errorValue, loc)
	}
	fb.current = successBlock
	return builtValue{id: candidate.ID, typ: targetType}, nil
}

// checkedNumericFailure admits the ConversionError variants of a checked
// float conversion.
func checkedNumericFailure(name string) bool {
	switch sema.ConversionFailureKind(name) {
	case sema.ConversionFailureNonFinite, sema.ConversionFailureOutOfRange, sema.ConversionFailurePrecisionLoss:
		return true
	}
	return false
}

// checkedNumericFailurePriority is the P20 error priority (§ 41).
var checkedNumericFailurePriority = map[string]int{
	string(sema.ConversionFailureNonFinite):     0,
	string(sema.ConversionFailureOutOfRange):    1,
	string(sema.ConversionFailurePrecisionLoss): 2,
}

// verifyCheckedNumericConversionOperation checks the float form of
// numeric.convert-checked: a numeric operand and candidate of which at least
// one is a binary float, a bool failed flag and one bool reason flag per
// distinct failure, listed in priority order.
func verifyCheckedNumericConversionOperation(module *Module, op Operation, values map[ValueID]Value) error {
	if op.ConversionFailure != "" || len(op.Operands) != 1 || len(op.Results) != 2+len(op.ConversionFailures) {
		return fmt.Errorf("invalid numeric.convert-checked")
	}
	source, candidate := values[op.Operands[0]].Type, op.Results[0].Type
	if !isNumericConversionType(module.Types, source) || !isNumericConversionType(module.Types, candidate) ||
		!typeHasKind(module.Types, source, TypeFloat) && !typeHasKind(module.Types, candidate, TypeFloat) {
		return fmt.Errorf("invalid numeric.convert-checked operand or candidate type")
	}
	previous := -1
	for _, failure := range op.ConversionFailures {
		priority, known := checkedNumericFailurePriority[failure]
		if !known || !checkedNumericFailure(failure) || priority <= previous {
			return fmt.Errorf("numeric.convert-checked failures must be distinct NonFinite, OutOfRange, PrecisionLoss in priority order")
		}
		previous = priority
	}
	for _, flag := range op.Results[1:] {
		if !typeHasKind(module.Types, flag.Type, TypeBool) {
			return fmt.Errorf("numeric.convert-checked flags must be bool")
		}
	}
	return nil
}
