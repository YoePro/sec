package semantic

import (
	"sec/internal/ast"
	"sec/internal/sema"
)

// buildOptionTry lowers a Sema-resolved try over Option[T] with the canonical
// P11 union representation of Option: the operand is evaluated once, Some is
// tested with union.is-variant, the success edge unwraps the copy-trivial
// payload, and the absence edge either returns None through the compatible
// enclosing Option (bodyless propagation) or dispatches the source-ordered
// None handlers with guards, recovery-value merge and residual None
// propagation. Absence is never converted to a Result error.
//
// Rules:
//   - rules/errors/errorhandling.md — §10.2 "Option[T]", §12.2 "Option propagation", §§15-16, §19, §21, §35
//   - rules/mlir/packages/sec-mlir-dialect_package12.md — Option through the P11 union representation
//   - rules/compiler/semantic_ir.md — § 24 "Result and Option"
func (fb *functionBuilder) buildOptionTry(expr *ast.TryExpression, resolved sema.ResolvedTry) (builtValue, error) {
	var plan sema.ResolvedTryPlan
	if resolved.Kind == sema.ResolvedTryHandledOption {
		var ok bool
		plan, ok = fb.owner.analyzer.ResolvedTryPlanOf(expr)
		if !ok || !lowerableTryPlan(plan) {
			return builtValue{}, fb.unsupported("unresolved Option try handler plan", expr.Token)
		}
	}
	if sema.CopyClassificationOf(resolved.SuccessType) != sema.CopyTrivial {
		return builtValue{}, fb.unsupported("ownership-sensitive Option try payload", expr.Token)
	}
	option, err := fb.buildExpr(expr.Expression, 0)
	if err != nil {
		return builtValue{}, err
	}
	some, _, ok := fb.optionVariants(option.typ)
	if !ok {
		return builtValue{}, fb.unsupported("Option try operand without Option union definition", expr.Token)
	}
	definition, _ := fb.owner.unionDefinition(option.typ)
	payloadType := definition.Variants[some].Payload
	boolType, err := fb.owner.internType(sema.Type{Name: "bool", Kind: sema.BoolType})
	if err != nil {
		return builtValue{}, err
	}
	isSome := fb.result(Operation{Kind: OpUnionIsVariant, Operands: []ValueID{option.id}, UnionVariant: some, Location: location(expr.Token)}, boolType)
	successBlock, noneBlock := fb.newBlock(), fb.newBlock()
	fb.emit(Operation{Kind: OpCondBranch, Operands: []ValueID{isSome.id}, Successors: []BranchTarget{{Block: successBlock.ID}, {Block: noneBlock.ID}}, Location: location(expr.Token)})
	fb.current = successBlock
	payload := fb.result(Operation{Kind: OpUnionUnwrapPayload, Operands: []ValueID{option.id}, UnionVariant: some, PayloadActions: []UnionPayloadAction{UnionPayloadCopyTrivial}, Location: location(expr.Token)}, payloadType)

	if resolved.Kind == sema.ResolvedTryHandledOption {
		return fb.buildLocalTryHandlers(expr, plan, successBlock, payload, noneBlock, builtValue{typ: payloadType})
	}
	enclosing, err := fb.owner.internType(resolved.EnclosingOptionType)
	if err != nil {
		return builtValue{}, err
	}
	fb.current = noneBlock
	if err := fb.returnOptionNone(enclosing, Operation{Location: location(expr.Token)}); err != nil {
		return builtValue{}, err
	}
	fb.current = successBlock
	return payload, nil
}

// optionVariants returns the Some and None variant indexes of a compiler-known
// Option union.
func (fb *functionBuilder) optionVariants(typ TypeID) (some UnionVariantIndex, none UnionVariantIndex, ok bool) {
	definition, found := fb.owner.unionDefinition(typ)
	if !found || definition.Name != "Option" || len(definition.Variants) != 2 {
		return 0, 0, false
	}
	someFound, noneFound := false, false
	for _, variant := range definition.Variants {
		switch {
		case variant.Name == "Some" && variant.Kind == UnionVariantSingle:
			some, someFound = variant.Index, true
		case variant.Name == "None" && variant.Kind == UnionVariantEmpty:
			none, noneFound = variant.Index, true
		}
	}
	return some, none, someFound && noneFound
}

// returnOptionNone constructs None of the enclosing Option return type and
// returns it, carrying the given handler provenance on the return.
func (fb *functionBuilder) returnOptionNone(enclosing TypeID, metadata Operation) error {
	_, none, ok := fb.optionVariants(enclosing)
	if !ok {
		return &UnsupportedFeatureError{Feature: "None propagation without an enclosing Option return", Package: fb.owner.maxPackage, Location: metadata.Location}
	}
	absent := fb.result(Operation{Kind: OpUnionConstruct, UnionVariant: none, Location: metadata.Location}, enclosing)
	metadata.Kind = OpReturn
	metadata.Operands = []ValueID{absent.id}
	fb.emit(metadata)
	fb.current = nil
	return nil
}
