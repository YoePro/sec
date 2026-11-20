package semantic

import (
	"sec/internal/ast"
	"sec/internal/sema"
)

// unionPayloadAction lowers the payload transfer Sema resolved for a union
// construction operand (P17-35). A moved owner or fresh value is taken
// directly once its producer lowers; a specialization may make a generic
// operand trivially copyable, as for struct fields.
// Rules: rules/compiler/semantic_ir.md §§23,35(6);
// rules/mlir/packages/sec-mlir-dialect_package17.md §§14,65.
func (fb *functionBuilder) unionPayloadAction(resolved sema.ResolvedStructFieldAction, operand ast.Expression, payload sema.Type) (UnionPayloadAction, error) {
	switch resolved {
	case sema.StructFieldCopyTrivial:
		return UnionPayloadCopyTrivial, nil
	case sema.StructFieldMove, sema.StructFieldConstructDirect:
		if len(fb.owner.substitution) != 0 && fb.owner.concreteTriviallyCopyable(payload) {
			return UnionPayloadCopyTrivial, nil
		}
		if !fb.ownedArgumentLowerable(operand) {
			return "", fb.unsupported("non-trivial union payload transfer", expressionToken(operand))
		}
		return UnionPayloadConstructDirect, nil
	}
	return "", fb.unsupported("union payload transfer "+string(resolved), expressionToken(operand))
}

// buildUnionConstruction consumes Sema's saved typed operands in evaluation
// order, then assembles the selected variant in declaration order. Constructor
// AST shape and legacy parallel action/field slices do not select operands or
// decide ownership. Unsupported actions remain explicit capability boundaries.
// Rules: rules/declarations/unions.md §§5,7.8;
// rules/compiler/semantic_ir.md §§23,35–36;
// rules/mlir/packages/sec-mlir-dialect_package17.md §§14,65.
func (fb *functionBuilder) buildUnionConstruction(expr ast.Expression, plan sema.ResolvedUnionConstruction, resultType TypeID) (builtValue, error) {
	variant, err := fb.validateUnionPreparation(expr, plan, resultType)
	if err != nil {
		return builtValue{}, err
	}
	depth := len(fb.prepared)
	defer fb.takePrepared(depth)
	op := Operation{Kind: OpUnionConstruct, UnionVariant: UnionVariantIndex(plan.VariantIndex), Location: locationFromExpression(expr)}
	count := len(plan.PayloadPreparation)
	op.Operands = make([]ValueID, count)
	op.PayloadActions = make([]UnionPayloadAction, count)
	if plan.Kind == sema.ResolvedUnionVariantFields {
		for _, field := range variant.PayloadFields {
			op.UnionFields = append(op.UnionFields, field.Name)
		}
	}
	for _, operand := range plan.PayloadPreparation {
		expected := variant.Payload
		if plan.Kind == sema.ResolvedUnionVariantFields {
			expected = variant.PayloadFields[operand.FieldIndex].Type
		}
		action, err := fb.unionPayloadAction(operand.Action, operand.Source, operand.Type)
		if err != nil {
			return builtValue{}, err
		}
		var value builtValue
		if operand.Defaulted {
			value, err = fb.buildResolvedDefault(operand.Type, *operand.Default, locationFromExpression(operand.Source))
		} else {
			value, err = fb.buildExpr(operand.Source, expected)
		}
		if err != nil {
			return builtValue{}, err
		}
		if value.typ != expected {
			return builtValue{}, fb.unsupported("union payload preparation produced a different type", expressionToken(operand.Source))
		}
		if plan.Kind == sema.ResolvedUnionVariantFields {
			fb.prepareOperand(value, operand.Type, expressionToken(operand.Source))
		}
		op.Operands[operand.FieldIndex] = value.id
		op.PayloadActions[operand.FieldIndex] = action
	}
	return fb.result(op, resultType), nil
}

// validateUnionPreparation admits complete typed plans before evaluating any
// payload. It checks identities, shape and destination positions; it never
// repairs missing facts by reinterpreting constructor syntax or copyability.
// Rules: rules/compiler/semantic_ir.md §§2,23,35–36;
// rules/compiler/compiler_pipeline.md §32; rules/declarations/unions.md §5.
func (fb *functionBuilder) validateUnionPreparation(expr ast.Expression, plan sema.ResolvedUnionConstruction, resultType TypeID) (UnionVariantDefinition, error) {
	reject := func(reason string) (UnionVariantDefinition, error) {
		return UnionVariantDefinition{}, fb.unsupported("union payload preparation "+reason, expressionToken(expr))
	}
	definition, ok := fb.owner.unionDefinition(resultType)
	if !ok || uint64(plan.VariantIndex) >= uint64(len(definition.Variants)) {
		return reject("without selected union definition")
	}
	selected, err := fb.owner.internType(plan.UnionType)
	if err != nil {
		return UnionVariantDefinition{}, err
	}
	if selected != resultType {
		return reject("has a different union type")
	}
	variant := definition.Variants[plan.VariantIndex]
	if variant.Name != plan.VariantName {
		return reject("has a different variant identity")
	}
	count := 0
	switch plan.Kind {
	case sema.ResolvedUnionVariantEmpty:
		if variant.Kind != UnionVariantEmpty {
			return reject("has a different empty shape")
		}
	case sema.ResolvedUnionVariantSingle:
		if variant.Kind != UnionVariantSingle {
			return reject("has a different single-payload shape")
		}
		count = 1
	case sema.ResolvedUnionVariantFields:
		if variant.Kind != UnionVariantFields {
			return reject("has a different field-payload shape")
		}
		count = len(variant.PayloadFields)
	default:
		return reject("has an unknown variant kind")
	}
	if len(plan.PayloadPreparation) != count {
		return reject("is missing typed operands")
	}
	seen := make([]bool, count)
	for _, operand := range plan.PayloadPreparation {
		if operand.Source == nil || operand.FieldIndex < 0 || operand.FieldIndex >= count || seen[operand.FieldIndex] {
			return reject("has a missing or duplicate operand position")
		}
		seen[operand.FieldIndex] = true
		expected := variant.Payload
		if plan.Kind == sema.ResolvedUnionVariantFields {
			field := variant.PayloadFields[operand.FieldIndex]
			if field.Name != operand.FieldName {
				return reject("has a different declared field")
			}
			expected = field.Type
		} else if operand.FieldName != "" || operand.Defaulted {
			return reject("has field metadata on an unnamed payload")
		}
		if operand.Defaulted && (operand.Default == nil || operand.Default.Kind == sema.NoDefault || operand.Default.Kind == "") || !operand.Defaulted && operand.Default != nil {
			return reject("has inconsistent semantic default facts")
		}
		actual, err := fb.owner.internType(operand.Type)
		if err != nil {
			return UnionVariantDefinition{}, err
		}
		if actual != expected {
			return reject("has a different operand type")
		}
		switch operand.Action {
		case sema.StructFieldCopyTrivial, sema.StructFieldMove, sema.StructFieldConstructDirect:
		default:
			return reject("needs unsupported transfer " + string(operand.Action))
		}
	}
	return variant, nil
}
