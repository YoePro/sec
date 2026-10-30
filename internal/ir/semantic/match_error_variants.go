package semantic

import (
	"fmt"
	"strings"

	"sec/internal/ast"
	"sec/internal/lexer"
	"sec/internal/sema"
)

// matchErrorArm is the unwrapped error value of an `Err(E.Variant...)` match
// arm and the variant its test selected; union records whether the variant
// indexes a union error (and so may carry a payload) rather than an enum.
type matchErrorArm struct {
	value   builtValue
	variant UnionVariantIndex
	union   bool
	// testBlock is the Err edge's block, created before the arm body so
	// arm provenance stays in source order.
	testBlock *Block
}

// emitErrorVariantPattern lowers the pattern test of an `Err(E.Variant...)`
// arm on a closed concrete error channel: the Result is tested for Err first,
// the error is unwrapped only on that edge, and the variant is tested on the
// unwrapped error before any payload is exposed. Both false edges continue
// with the next arm. The error is copied out of the Result, so only
// trivially copyable error types are lowered; others stay rejected until
// Semantic IR can project the Err payload as a Place.
//
// Rules:
//   - rules/errors/errorhandling.md — § 27.2b "Payload-bearing error patterns"
//   - rules/control-flow/flowcontrol_match.md — § 28 "Concrete error variants"
//   - rules/compiler/semantic_ir.md — § 71(6)
//   - rules/corrections/applied/md029-md042-md102-pattern-reachability-some-correction-20261028.md — §§ 6, 15(1)
func (fb *functionBuilder) emitErrorVariantPattern(subject builtValue, plan sema.ResolvedMatchPlan, arm sema.ResolvedMatchArm, matchedBlock, nextBlock *Block, boolType TypeID, meta Operation, token lexer.Token) error {
	result, ok := fb.owner.module.Types.Lookup(subject.typ)
	if !ok || result.Kind != TypeResult || len(plan.SubjectType.TypeArgs) != 2 {
		return fb.unsupported("concrete error variant match without a Result subject", token)
	}
	if !sema.TriviallyCopyable(plan.SubjectType.TypeArgs[1]) {
		return fb.unsupported("concrete error variant match on a non-trivially-copyable error type", token)
	}
	name := arm.UnionVariantName
	if dot := strings.LastIndex(name, "."); dot >= 0 {
		name = name[dot+1:]
	}

	isErr := meta
	isErr.Kind = OpResultIsErr
	isErr.Operands = []ValueID{subject.id}
	isErrValue := fb.result(isErr, boolType)
	errBlock := fb.errorArm.testBlock
	if errBlock == nil {
		return fmt.Errorf("concrete error variant arm %d has no test block", arm.SourceIndex)
	}
	branch := meta
	branch.Kind = OpCondBranch
	branch.Operands = []ValueID{isErrValue.id}
	branch.Successors = []BranchTarget{{Block: errBlock.ID}, {Block: nextBlock.ID}}
	fb.emit(branch)

	fb.current = errBlock
	unwrap := meta
	unwrap.Kind = OpResultUnwrapErr
	unwrap.Operands = []ValueID{subject.id}
	errorValue := fb.result(unwrap, result.Error)

	condition, selected, err := fb.errorVariantTest(errorValue, name, meta, boolType, token)
	if err != nil {
		return err
	}
	selected.testBlock = fb.errorArm.testBlock
	fb.errorArm = selected
	if selected.union {
		fb.variantGuard = condition.id
	}
	branch = meta
	branch.Kind = OpCondBranch
	branch.Operands = []ValueID{condition.id}
	branch.Successors = []BranchTarget{{Block: matchedBlock.ID}, {Block: nextBlock.ID}}
	fb.emit(branch)
	return nil
}

// errorVariantTest emits the test of a concrete error value for its variant
// name: an enum case comparison for an enum error, union.is-variant for a
// union error. The returned arm records the tested value and, for a union,
// the variant whose payload the true edge may project.
func (fb *functionBuilder) errorVariantTest(errorValue builtValue, name string, meta Operation, boolType TypeID, token lexer.Token) (builtValue, matchErrorArm, error) {
	test := meta
	if definition, ok := fb.owner.enumDefinition(errorValue.typ); ok {
		caseID, ok := enumCaseIDByName(definition, name)
		if !ok {
			return builtValue{}, matchErrorArm{}, fb.unsupported("unknown concrete error enum case", token)
		}
		constant := meta
		constant.Kind = OpEnumConstant
		constant.Operands = nil
		constant.EnumCase = caseID
		caseValue := fb.result(constant, errorValue.typ)
		test.Kind = OpEnumCompare
		test.Operands = []ValueID{errorValue.id, caseValue.id}
		test.IntegerCompare = IntegerCompareEQ
		return fb.result(test, boolType), matchErrorArm{value: errorValue}, nil
	}
	definition, ok := fb.owner.unionDefinition(errorValue.typ)
	if !ok {
		return builtValue{}, matchErrorArm{}, fb.unsupported("concrete error variant test without an enum or union error", token)
	}
	for index, variant := range definition.Variants {
		if variant.Name == name {
			test.Kind = OpUnionIsVariant
			test.Operands = []ValueID{errorValue.id}
			test.UnionVariant = UnionVariantIndex(index)
			return fb.result(test, boolType), matchErrorArm{value: errorValue, variant: UnionVariantIndex(index), union: true}, nil
		}
	}
	return builtValue{}, matchErrorArm{}, fb.unsupported("unknown concrete error union variant", token)
}

// matchArmVariantIndex is the variant an arm's record names: the selected
// error-union variant for an `Err(E.Variant...)` arm, otherwise the Sema
// union variant index.
func (fb *functionBuilder) matchArmVariantIndex(arm sema.ResolvedMatchArm) UnionVariantIndex {
	if arm.PatternKind == sema.MatchPatternResultErrVariant {
		return fb.errorArm.variant
	}
	return UnionVariantIndex(arm.UnionVariantIndex)
}

// unionVariantPayload copies the payload of the proven variant out of a
// union value: a single payload directly, a struct-like payload as its
// synthetic struct assembled from guarded field projections (package 13
// sections 66-68). meta carries the provenance of the projections, which must
// sit on the variant test's dedicated true edge.
func (fb *functionBuilder) unionVariantPayload(container builtValue, variantIndex UnionVariantIndex, bindingType sema.Type, meta Operation, token lexer.Token) (builtValue, error) {
	definition, ok := fb.owner.unionDefinition(container.typ)
	if !ok || int(variantIndex) >= len(definition.Variants) {
		return builtValue{}, fb.unsupported("unresolved union match payload", token)
	}
	variant := definition.Variants[variantIndex]
	meta.Operands = []ValueID{container.id}
	if variant.Kind == UnionVariantFields && variant.SyntheticPayloadStruct != 0 {
		construct := meta
		construct.Kind = OpStructConstruct
		construct.Operands = nil
		for _, field := range variant.PayloadFields {
			projection := meta
			projection.Kind = OpUnionUnwrapField
			projection.UnionVariant = variantIndex
			projection.UnionField = field.Name
			projection.PayloadActions = []UnionPayloadAction{UnionPayloadCopyTrivial}
			value := fb.result(projection, field.Type)
			construct.Operands = append(construct.Operands, value.id)
			construct.StructOrigins = append(construct.StructOrigins, StructOriginExplicit)
			construct.StructActions = append(construct.StructActions, StructActionCopyTrivial)
		}
		return fb.result(construct, variant.SyntheticPayloadStruct), nil
	}
	payloadType, err := fb.owner.internType(bindingType)
	if err != nil {
		return builtValue{}, err
	}
	if variant.Kind != UnionVariantSingle || variant.Payload != payloadType {
		return builtValue{}, fb.unsupported("mismatched union match payload", token)
	}
	meta.Kind = OpUnionUnwrapPayload
	meta.UnionVariant = variantIndex
	meta.PayloadActions = []UnionPayloadAction{UnionPayloadCopyTrivial}
	return fb.result(meta, payloadType), nil
}

// bindTryHandlerPayload projects the payload or shallow fields of an
// `Err(E.Variant(binding))` / `Err(E.Variant { fields })` try handler from the
// tested union error on the handler's guarded edge, before any `where` guard
// runs. A payload binding replaces input, the whole error, as the handler's
// bound value; field bindings are bound directly and restored by the returned
// function. Only trivially copyable payloads are lowered; ownership-sensitive
// transfers out of the protected error stay rejected.
//
// Rules:
//   - rules/errors/errorhandling.md — §§ 15.3, 20
//   - rules/compiler/semantic_ir.md — § 71(6)
func (fb *functionBuilder) bindTryHandlerPayload(handler *ast.TryHandler, plan sema.ResolvedTryHandler, input *builtValue, selected matchErrorArm) (func(), error) {
	restore := func() {}
	payloadBinding := plan.ErrorPayload && plan.BindingName != "" && plan.BindingName != "_"
	if !payloadBinding && !plan.FieldBindings {
		return restore, nil
	}
	if !selected.union {
		return nil, fb.unsupported("error payload handler without a tested union error", handler.Token)
	}
	meta := Operation{TryHandlerKind: TryHandlerErrVariant, TryHandlerIndex: plan.SourceIndex, Variant: plan.Variant, Location: location(handler.Token)}
	if payloadBinding {
		if !sema.TriviallyCopyable(plan.BindingType) {
			return nil, fb.unsupported("ownership-sensitive error payload handler binding", handler.Token)
		}
		payload, err := fb.unionVariantPayload(selected.value, selected.variant, plan.BindingType, meta, handler.Token)
		if err != nil {
			return nil, err
		}
		*input = payload
	}
	if !plan.FieldBindings {
		return restore, nil
	}
	definition, ok := fb.owner.unionDefinition(selected.value.typ)
	if !ok || int(selected.variant) >= len(definition.Variants) || definition.Variants[selected.variant].Kind != UnionVariantFields {
		return nil, fb.unsupported("error field handler without a struct-like union variant", handler.Token)
	}
	variant := definition.Variants[selected.variant]
	type saved struct {
		id       sema.BindingID
		previous binding
		existed  bool
	}
	var restores []saved
	restore = func() {
		for index := len(restores) - 1; index >= 0; index-- {
			if restores[index].existed {
				fb.bindings[restores[index].id] = restores[index].previous
			} else {
				delete(fb.bindings, restores[index].id)
			}
		}
	}
	for _, field := range plan.FieldBindingList {
		if field.Action != sema.MatchBindingCopyTrivial {
			restore()
			return nil, fb.unsupported("ownership-sensitive error field handler binding "+string(field.Action), handler.Token)
		}
		var fieldType TypeID
		for _, declared := range variant.PayloadFields {
			if declared.Name == field.Field {
				fieldType = declared.Type
			}
		}
		if fieldType == 0 || field.Identifier == nil {
			restore()
			return nil, fmt.Errorf("error field binding %s has no payload field", field.Field)
		}
		projection := meta
		projection.Kind = OpUnionUnwrapField
		projection.Operands = []ValueID{selected.value.id}
		projection.UnionVariant = selected.variant
		projection.UnionField = field.Field
		projection.PayloadActions = []UnionPayloadAction{UnionPayloadCopyTrivial}
		projection.Location = location(field.Identifier.Token)
		value := fb.result(projection, fieldType)
		fact, ok := fb.owner.analyzer.ResolvedBindingOf(field.Identifier)
		if !ok {
			restore()
			return nil, fmt.Errorf("error field binding %s has no resolved identity", field.Identifier.Value)
		}
		previous, existed := fb.bindings[fact.ID]
		restores = append(restores, saved{id: fact.ID, previous: previous, existed: existed})
		fb.bindings[fact.ID] = binding{value: value.id, typ: value.typ}
	}
	return restore, nil
}
