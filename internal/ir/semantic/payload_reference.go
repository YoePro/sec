package semantic

import (
	"fmt"

	"sec/internal/ast"
	"sec/internal/lexer"
	"sec/internal/sema"
)

// lowerPayloadBorrow lowers a `ref` or `ref mut` payload binding of a match
// arm from Sema's resolved reference plan for the pattern's bound identifier.
// The payload Place is derived from the match subject's own Place, never from
// a copy of the union or of the payload: place.union-payload names the arm's
// variant test (union.is-variant, or result.is-err for Result) as its
// active-variant proof, and the borrow carries the plan's facts unchanged.
// The binding stays arm-scoped; Sema has already rejected escapes of a
// match-scoped payload origin.
//
// Payloads: a single payload, a struct-like payload as its canonical Package
// 13 synthetic payload struct, and Result Ok/Err payloads. Subjects: a local
// binding or by-value parameter and stored fields below it, a receiver field
// (`self.figure`, implicit `figure`), and — through subjectPlace, the Place the
// subject itself was read from — paths through array elements or a reference
// holder, so an index or dereference is evaluated exactly once.
//
// Rules:
//   - rules/mlir/packages/sec-mlir-dialect_package15.md — §§ 19, 44, 71, 99, 100, 121
//   - rules/memory/borrowing.md — § 12(5)
func (fb *functionBuilder) lowerPayloadBorrow(subjectExpr ast.Expression, subject builtValue, subjectPlace *matchSubjectPlace, arm *ast.MatchArm, armPlan sema.ResolvedMatchArm, guard ValueID) (builtValue, bool, error) {
	token := arm.Token
	reference := matchArmPayloadReference(arm.Pattern.Expression())
	if reference == nil {
		return builtValue{}, false, fb.unsupported("borrowed match payload without a ref pattern", token)
	}
	token = reference.Token
	variantIndex, ok := payloadVariantIndex(armPlan)
	if !ok {
		return builtValue{}, false, fb.unsupported("borrowed "+string(armPlan.PatternKind)+" payload", token)
	}
	plan, ok := fb.owner.analyzer.ResolvedReferencePlanOf(reference.Value)
	if !ok {
		return builtValue{}, false, fb.unsupported("borrowed match payload without a resolved reference plan", token)
	}
	if !plan.Facts.Checkable() {
		return builtValue{}, false, fb.unsupported("uncheckable borrowed match payload", token)
	}
	if guard == 0 {
		return builtValue{}, false, fb.unsupported("borrowed match payload without an active-variant test", token)
	}
	if subjectPlace != nil {
		return fb.emitPayloadBorrow(reference, plan, variantIndex, subject, subjectPlace.place, subjectPlace.mutable, guard, token)
	}
	if plans, steps, rooted, ok := fb.receiverFieldPath(subjectExpr); rooted && !plan.Implicit {
		// `match self.figure` / `match figure` inside a method: the union is
		// a stored field reached through the receiver reference.
		if !ok {
			return builtValue{}, false, fb.unsupported("borrowed match payload of the receiver itself", token)
		}
		unionPlace, mutable, err := fb.receiverPlace(plans, steps)
		if err != nil {
			return builtValue{}, false, err
		}
		return fb.emitPayloadBorrow(reference, plan, variantIndex, subject, unionPlace, mutable, guard, token)
	}
	if plan.Reborrow || plan.Implicit {
		return builtValue{}, false, fb.unsupported("borrowed match payload of a subject reached through a reference", token)
	}
	unionPlace, mutable, err := fb.localSubjectPlace(subjectExpr, reference, plan, true)
	if err != nil {
		return builtValue{}, false, err
	}
	return fb.emitPayloadBorrow(reference, plan, variantIndex, subject, unionPlace, mutable, guard, token)
}

// payloadVariantIndex is the variant a payload binding borrows: the union or
// Option variant, or Ok (0) / Err (1) of a Result.
func payloadVariantIndex(armPlan sema.ResolvedMatchArm) (UnionVariantIndex, bool) {
	switch armPlan.PatternKind {
	case sema.MatchPatternUnionVariant, sema.MatchPatternOptionSome:
		return UnionVariantIndex(armPlan.UnionVariantIndex), true
	case sema.MatchPatternResultOk:
		return 0, true
	case sema.MatchPatternResultErr:
		return 1, true
	}
	return 0, false
}

// localSubjectPlace projects the union Place of a subject rooted at a local
// binding or by-value parameter through the plan's resolved field (and, when
// elements are allowed, fixed-array element) steps.
func (fb *functionBuilder) localSubjectPlace(subjectExpr ast.Expression, reference *ast.RefExpression, plan sema.ResolvedReferencePlan, fieldsOnly bool) (builtValue, bool, error) {
	token := reference.Token
	projections := plan.Place.Projections
	if len(projections) == 0 || projections[len(projections)-1].Canonical() != sema.CanonicalProjectionUnionPayload {
		return builtValue{}, false, fb.unsupported("borrowed match payload without a resolved union payload Place", token)
	}
	var steps []ast.Expression
	rootExpr := subjectExpr
	for {
		switch step := rootExpr.(type) {
		case *ast.MemberExpression:
			steps = append([]ast.Expression{step}, steps...)
			rootExpr = step.Object
			continue
		case *ast.IndexExpression:
			if fieldsOnly {
				return builtValue{}, false, fb.unsupported("borrowed match payload of a subject reached through elements", token)
			}
			steps = append([]ast.Expression{step}, steps...)
			rootExpr = step.Left
			continue
		}
		break
	}
	prefix := projections[:len(projections)-1]
	if len(steps) != len(prefix) || !onlyFieldProjections(sema.Place{Projections: prefix}) {
		return builtValue{}, false, fb.unsupported("borrowed match payload whose subject path differs from its resolved Place", token)
	}
	root, err := fb.lowerRootPlace(reference, plan, rootExpr)
	if err != nil {
		return builtValue{}, false, err
	}
	rootPlaceType, ok := fb.owner.module.Types.Lookup(root.typ)
	if !ok {
		return builtValue{}, false, fmt.Errorf("payload root is not a Place")
	}
	unionPlace, err := fb.projectPlace(steps, prefix, root, rootPlaceType.Mutable, placeMetadataOf(plan.Place))
	return unionPlace, rootPlaceType.Mutable, err
}

// matchSubjectPlace is the Place a match subject was read from when an arm
// borrows its payload through elements or a reference holder.
type matchSubjectPlace struct {
	place   builtValue
	mutable bool
}

// buildBorrowedMatchSubject forms the subject's Place once and reads the
// subject from it when a payload is borrowed and the subject path goes through
// an array element or a (non-receiver) reference holder; the borrowed payload
// is then projected from that same Place, so no index or dereference is
// evaluated twice. It reports false when the ordinary subject evaluation
// applies.
func (fb *functionBuilder) buildBorrowedMatchSubject(expr *ast.MatchExpression, plan sema.ResolvedMatchPlan) (builtValue, *matchSubjectPlace, bool, error) {
	var borrowArm *ast.MatchArm
	for _, armPlan := range plan.Arms {
		if armPlan.BindingAction == sema.MatchBindingBorrowShared || armPlan.BindingAction == sema.MatchBindingBorrowMutable {
			if armPlan.SourceIndex >= 0 && armPlan.SourceIndex < len(expr.Arms) {
				borrowArm = expr.Arms[armPlan.SourceIndex]
				break
			}
		}
	}
	if borrowArm == nil {
		return builtValue{}, nil, false, nil
	}
	if _, _, rooted, _ := fb.receiverFieldPath(expr.Subject); rooted {
		return builtValue{}, nil, false, nil
	}
	holder, steps := referencePath(expr.Subject)
	if holder == nil {
		return builtValue{}, nil, false, nil
	}
	throughElement := false
	for _, step := range steps {
		if _, index := step.(*ast.IndexExpression); index {
			throughElement = true
		}
	}
	holderType, typed := fb.owner.analyzer.ResolvedTypeOf(holder)
	throughReference := typed && holderType.Kind == sema.ReferenceType
	if !throughElement && !throughReference {
		return builtValue{}, nil, false, nil
	}
	subjectType, typed := fb.owner.analyzer.ResolvedTypeOf(expr.Subject)
	if !typed || sema.CopyClassificationOf(subjectType) != sema.CopyTrivial {
		return builtValue{}, nil, true, fb.unsupported("borrowed payload of a non-copy-trivial subject reached through elements or references", expr.Token)
	}
	var place builtValue
	var mutable bool
	var err error
	if throughReference {
		use, ok := fb.owner.analyzer.ResolvedReferenceUseOf(expr.Subject)
		if !ok || len(steps) == 0 {
			return builtValue{}, nil, true, fb.unsupported("borrowed payload of a whole referent through a reference holder", expr.Token)
		}
		place, mutable, err = fb.derefUsePlace(expr.Subject, use)
	} else {
		reference := matchArmPayloadReference(borrowArm.Pattern.Expression())
		if reference == nil {
			return builtValue{}, nil, true, fb.unsupported("borrowed match payload without a ref pattern", expr.Token)
		}
		refPlan, ok := fb.owner.analyzer.ResolvedReferencePlanOf(reference.Value)
		if !ok {
			return builtValue{}, nil, true, fb.unsupported("borrowed match payload without a resolved reference plan", reference.Token)
		}
		place, mutable, err = fb.localSubjectPlace(expr.Subject, reference, refPlan, false)
	}
	if err != nil {
		return builtValue{}, nil, true, err
	}
	placeType, ok := fb.owner.module.Types.Lookup(place.typ)
	if !ok || placeType.Kind != TypePlace {
		return builtValue{}, nil, true, fmt.Errorf("match subject Place is not a Place")
	}
	subject := fb.result(Operation{Kind: OpPlaceRead, Operands: []ValueID{place.id}, Location: locationFromExpression(expr.Subject)}, placeType.Element)
	return subject, &matchSubjectPlace{place: place, mutable: mutable}, true, nil
}

// emitPayloadBorrow forms place.union-payload over the subject's union Place
// under the arm's variant test and borrows it with the plan's facts.
func (fb *functionBuilder) emitPayloadBorrow(reference *ast.RefExpression, plan sema.ResolvedReferencePlan, variantIndex UnionVariantIndex, subject builtValue, unionPlace builtValue, mutable bool, guard ValueID, token lexer.Token) (builtValue, bool, error) {
	placeType, ok := fb.owner.module.Types.Lookup(unionPlace.typ)
	if !ok || placeType.Kind != TypePlace || placeType.Element != subject.typ {
		return builtValue{}, false, fb.unsupported("borrowed match payload whose Place is not the match subject", token)
	}
	if _, isUnion := fb.owner.unionDefinition(subject.typ); !isUnion {
		if result, found := fb.owner.module.Types.Lookup(subject.typ); !found || result.Kind != TypeResult {
			return builtValue{}, false, fb.unsupported("borrowed payload of a non-union subject", token)
		}
	}
	payload, ok := variantPayloadType(fb.owner.module, subject.typ, variantIndex)
	if !ok {
		return builtValue{}, false, fb.unsupported("borrowed variant without a payload Place type", token)
	}
	if reference.Mutable && !mutable {
		return builtValue{}, false, fb.unsupported("mutable borrow of a read-only union payload", token)
	}
	facts := placeMetadataOf(plan.Place)
	facts.VariantProven = true
	payloadType := fb.owner.module.Types.Intern(Type{Kind: TypePlace, Element: payload, Mutable: mutable})
	payloadPlace := fb.placeResult(Operation{
		Kind: OpPlaceUnionPayload, Operands: []ValueID{unionPlace.id}, UnionVariant: variantIndex,
		VariantGuard: guard, Place: facts, Location: location(reference.Token),
	}, payloadType)
	borrowed, err := fb.emitBorrow(reference, plan, payloadPlace)
	if err != nil {
		return builtValue{}, false, err
	}
	return borrowed, true, nil
}

// endPayloadBorrow marks the end of an arm-scoped payload reference with
// ref.end-borrow on every edge that leaves the arm (armBlocks: the guard and
// body blocks): before a return or an unconditional branch out of the arm,
// and on a fresh edge block for a conditional exit such as the guard's false
// edge. Failure terminators end the function and need no marker. The binding
// cannot be used outside its arm, so the arm exit bounds Sema's resolved
// liveness; the marker carries the borrow's own BorrowID and LifetimeID.
//
// Rules:
//   - rules/mlir/packages/sec-mlir-dialect_package15.md — §§ 49, 50, 82
func (fb *functionBuilder) endPayloadBorrow(reference builtValue, armBlocks []*Block, loc Location) error {
	metadata := fb.references[reference.id]
	if metadata == nil {
		return fmt.Errorf("borrowed match payload %%%d has no reference facts", reference.id)
	}
	inside := map[BlockID]bool{}
	for _, block := range armBlocks {
		inside[block.ID] = true
	}
	end := func() Operation {
		return Operation{Kind: OpReferenceEndBorrow, Operands: []ValueID{reference.id}, Reference: cloneReferenceMetadata(metadata), Location: loc}
	}
	for _, block := range armBlocks {
		if len(block.Operations) == 0 {
			continue
		}
		last := len(block.Operations) - 1
		terminator := &block.Operations[last]
		switch {
		case terminator.Kind == OpReturn, terminator.Kind == OpBranch && !inside[terminator.Successors[0].Block]:
			block.Operations = append(block.Operations[:last], end(), block.Operations[last])
		case terminator.Kind == OpCondBranch:
			for index, successor := range terminator.Successors {
				if inside[successor.Block] {
					continue
				}
				edge := fb.newBlock()
				edge.Operations = []Operation{end(), {Kind: OpBranch, Successors: []BranchTarget{successor}, Synthesized: true, Location: loc}}
				terminator.Successors[index] = BranchTarget{Block: edge.ID}
			}
		}
	}
	return nil
}

// matchArmPayloadReference returns the ref or ref mut node binding a payload
// in a variant pattern.
func matchArmPayloadReference(pattern ast.Expression) *ast.RefExpression {
	var argument ast.Expression
	switch pattern := pattern.(type) {
	case *ast.CallExpression:
		if len(pattern.Arguments) == 1 {
			argument = pattern.Arguments[0]
		}
	case *ast.OkExpression:
		argument = pattern.Value
	case *ast.ErrExpression:
		argument = pattern.Value
	}
	reference, _ := argument.(*ast.RefExpression)
	return reference
}
