package semantic

import (
	"fmt"
	"math/big"

	"sec/internal/ast"
	"sec/internal/sema"
)

// StringView is a shared, non-owning bounded view over valid UTF-8 string
// backing in rune coordinates (MD-126). It has no element type: it is never
// represented as `ref rune[]` or as contiguous stored runes. Holder transport
// (copy), whole/nested reborrow and length observation reuse the shared-slice
// operation kinds over this distinct type; only construction from a string
// Place and the runtime rune count of a string Place are string operations.
// Rules: rules/types/types.md — StringView; rules/collections/collections.md — §7.10;
// rules/compiler/semantic_ir.md — §17(9)–(10);
// rules/corrections/applied/MD-126-borrowed-string-ranges-rune-iteration-applied-20261105.md — §§2–4.
const TypeStringView TypeKind = "string-view"

const (
	// OpStringRuneLength observes the Unicode-scalar count of a string Place
	// without reading it as an owned value or allocating.
	OpStringRuneLength OpKind = "string.rune-len"
	// OpStringViewBorrow forms a shared StringView over a string Place from a
	// normalized rune (start, length) tuple.
	OpStringViewBorrow OpKind = "string.view-borrow"
)

// sharedBoundedView reports the shared bounded-reference value kinds whose
// holders are copied with slice.copy-shared and ended with ref.end-borrow.
// Rules: rules/memory/references.md — §3.3(1)–(5).
func sharedBoundedView(kind TypeKind) bool {
	return kind == TypeSharedSlice || kind == TypeStringView
}

// boundedView reports every bounded non-owning reference value kind.
// Rules: rules/memory/references.md — §3.3.
func boundedView(kind TypeKind) bool {
	return kind == TypeSharedSlice || kind == TypeMutableSlice || kind == TypeStringView
}

// isStringViewType reports whether id is the StringView type.
func isStringViewType(types *TypeTable, id TypeID) bool {
	typ, ok := types.Lookup(id)
	return ok && typ.Kind == TypeStringView
}

// stringViewType interns the singleton StringView type.
func (table *TypeTable) stringViewType() TypeID {
	return table.Intern(Type{Kind: TypeStringView})
}

// lowerStringRange consumes Sema's resolved string-range plan: the source
// (string Place or StringView holder) precedes each present endpoint in the
// recorded order, the rune extent is observed, the range is normalized or
// checked with RangeError precedence, and the view is published only on the
// success edge. No text is read, copied or allocated.
// Rules: rules/collections/collections.md — §7.10; rules/memory/borrowing.md — §5.5;
// rules/errors/runtime_checks.md — Slice checks; rules/compiler/semantic_ir.md — §17(9).
func (fb *functionBuilder) lowerStringRange(expr *ast.SliceExpression, resolved sema.ResolvedStringRangePlan) (builtValue, error) {
	plan := resolved.Range
	if plan.Mode != sema.SliceShared || plan.ReferenceFacts.Facts.MutableAuthority {
		return builtValue{}, fmt.Errorf("string range plan must be a shared borrow")
	}
	if !plan.HasValidEvaluationOrder() || !plan.ReferenceFacts.Facts.Checkable() {
		return builtValue{}, fb.unsupported("string range without checkable reference facts", expr.Token)
	}
	reborrow := plan.SourceKind == sema.SliceSourceStringView
	if !reborrow && plan.SourceKind != sema.SliceSourceString {
		return builtValue{}, fmt.Errorf("string range plan has source kind %q", plan.SourceKind)
	}
	loc := location(expr.Token)
	var place builtValue
	var parent *ReferenceMetadata
	var err error
	if reborrow {
		place, parent, err = fb.stringViewSourceValue(expr.Left)
	} else {
		sourcePlan := plan.ReferenceFacts
		sourcePlan.Place = plan.SourcePlace
		sourceExpr := &ast.RefExpression{Token: expr.Token, Value: expr.Left}
		place, parent, err = fb.fixedSliceSourcePlace(sourceExpr, sourcePlan)
	}
	if err != nil {
		return builtValue{}, err
	}
	uintType := fb.owner.module.Types.Intern(Type{Kind: TypeUint, Name: "uint", TargetSize: true})
	operands := []ValueID{0}
	if !reborrow && plan.SourceLength != nil {
		operands[0] = fb.result(Operation{Kind: OpConstInt, Integer: cloneBigInt(plan.SourceLength), Location: loc}, uintType).id
	}
	metadata := &SliceRangeMetadata{StartPresent: plan.Start.Present, EndPresent: plan.End.Present, EndExclusive: plan.EndExclusive}
	for _, operand := range plan.EvaluationOrder[1:] {
		endpoint, source := plan.Start, expr.Start
		if operand == sema.SliceOperandEnd {
			endpoint, source = plan.End, expr.End
		}
		typ, err := fb.owner.internType(endpoint.Type)
		if err != nil {
			return builtValue{}, err
		}
		value, err := fb.buildExpr(source, typ)
		if err != nil {
			return builtValue{}, err
		}
		operands = append(operands, value.id)
		if operand == sema.SliceOperandStart {
			metadata.StartType, metadata.StartSigned = value.typ, endpoint.Signed
		} else {
			metadata.EndType, metadata.EndSigned = value.typ, endpoint.Signed
		}
	}
	guard := ValueID(0)
	if reborrow {
		guard, err = fb.validateReference(place, parent, loc)
		if err != nil {
			return builtValue{}, err
		}
		operands[0] = fb.result(Operation{Kind: OpSliceLength, Operands: []ValueID{place.id}, ReferenceGuard: guard, Location: loc}, uintType).id
	} else if operands[0] == 0 {
		operands[0] = fb.result(Operation{Kind: OpStringRuneLength, Operands: []ValueID{place.id}, Location: loc}, uintType).id
	}
	start, length := fb.newValue(uintType, OwnershipImmediate, loc), fb.newValue(uintType, OwnershipImmediate, loc)
	normalize := Operation{Kind: OpSliceRangeNormalize, Operands: operands, Results: []Value{start, length}, SliceRange: metadata, Location: loc}
	switch plan.CheckKind {
	case sema.SliceRangeProvenSafe:
		proof, valid := semanticArrayIndexProof(sema.ArrayIndexProofKind(plan.ProofKind))
		if plan.ProofKind == "source-open-bound" {
			proof, valid = SliceRangeProofOpenBound, true
		}
		if !valid {
			return builtValue{}, fmt.Errorf("unsupported resolved string range proof %q", plan.ProofKind)
		}
		metadata.ProofKind = proof
		fb.emit(normalize)
	case sema.SliceRangeRuntimeCheck:
		errorType, err := fb.owner.internType(plan.ErrorType)
		if err != nil {
			return builtValue{}, err
		}
		boolean := fb.owner.module.Types.Intern(Type{Kind: TypeBool, Name: "bool"})
		failed, reason := fb.newValue(boolean, OwnershipImmediate, loc), fb.newValue(errorType, OwnershipImmediate, loc)
		normalize.Kind = OpSliceRangeCheck
		normalize.Results = append(normalize.Results, failed, reason)
		fb.emit(normalize)
		failure, success := fb.newBlock(), fb.newBlock()
		fb.emit(Operation{Kind: OpCondBranch, Operands: []ValueID{failed.ID}, Successors: []BranchTarget{{Block: failure.ID}, {Block: success.ID}}, Location: loc})
		fb.current = failure
		if _, protected := fb.failurePoint(expr); protected {
			fb.routeFailure(expr, builtValue{id: reason.ID, typ: errorType}, loc)
		} else {
			fb.emit(Operation{Kind: OpBoundsFailure, ArrayOperation: "slice-range", Location: loc})
		}
		fb.current = success
	default:
		return builtValue{}, fmt.Errorf("unknown resolved string range check %q", plan.CheckKind)
	}
	typ := fb.owner.module.Types.stringViewType()
	kind := OpStringViewBorrow
	if reborrow {
		kind = OpSliceReborrowShared
	}
	value := fb.newValue(typ, OwnershipSharedReference, loc)
	facts := referenceMetadataOf(plan.ReferenceFacts.Facts)
	if parent != nil {
		facts.SourceBorrow = parent.Borrow
	}
	origin := SliceFacts{Reference: *facts, RuntimeLength: length.ID, DynamicRange: SliceRangeID(plan.DynamicRange)}
	if !reborrow {
		origin.OriginPlaces = []ValueID{place.id}
	}
	if plan.StaticRange != nil {
		origin.StaticRange = &StaticSliceRange{Start: cloneBigInt(plan.StaticRange.Start), EndExclusive: cloneBigInt(plan.StaticRange.EndExclusive)}
	}
	detached := origin.Clone()
	fb.emit(Operation{Kind: kind, Operands: []ValueID{place.id, start.ID, length.ID}, Results: []Value{value}, Reference: facts, Slice: &detached, ReferenceGuard: guard, Location: loc})
	fb.recordReference(value.ID, facts)
	return builtValue{id: value.ID, typ: typ}, nil
}

// stringViewSourceValue evaluates a StringView range source once without
// copying its holder: a named holder, a nested range or a call result keeps
// its recorded reference facts.
// Rules: rules/collections/collections.md — §7.10; rules/memory/borrowing.md — §5.5(3).
func (fb *functionBuilder) stringViewSourceValue(expr ast.Expression) (builtValue, *ReferenceMetadata, error) {
	var value builtValue
	var err error
	switch source := expr.(type) {
	case *ast.Identifier:
		var facts *ReferenceMetadata
		value, facts, err = fb.referenceHolder(source)
		if err != nil {
			return builtValue{}, nil, err
		}
		if !isStringViewType(fb.owner.module.Types, value.typ) {
			return builtValue{}, nil, fmt.Errorf("string range source holder is not a StringView")
		}
		return value, facts, nil
	case *ast.SliceExpression, *ast.CallExpression:
		value, err = fb.buildExpr(source, fb.owner.module.Types.stringViewType())
	default:
		return builtValue{}, nil, fb.unsupported("StringView range source expression", expressionToken(expr))
	}
	if err != nil {
		return builtValue{}, nil, err
	}
	if !isStringViewType(fb.owner.module.Types, value.typ) || fb.references[value.id] == nil {
		return builtValue{}, nil, fb.unsupported("StringView range source without reference facts", expressionToken(expr))
	}
	return value, fb.references[value.id], nil
}

// verifyStringRuneLength checks the observation of a string Place.
// Rules: rules/compiler/semantic_ir.md — §17(9).
func verifyStringRuneLength(types *TypeTable, op Operation, values map[ValueID]Value) error {
	if len(op.Operands) != 1 || len(op.Results) != 1 || len(op.Successors) != 0 {
		return fmt.Errorf("string.rune-len requires one string Place and one uint result")
	}
	place, ok := types.Lookup(values[op.Operands[0]].Type)
	referent, found := types.Lookup(place.Element)
	if !ok || place.Kind != TypePlace || !found || referent.Kind != TypeString {
		return fmt.Errorf("string.rune-len requires a string Place")
	}
	result, ok := types.Lookup(op.Results[0].Type)
	if !ok || result.Kind != TypeUint || result.Name != "uint" || !result.TargetSize {
		return fmt.Errorf("string.rune-len must return uint")
	}
	return nil
}

// verifyStringViewBorrow checks construction from a string Place: normalized
// uint start/length, a shared StringView result and reference metadata.
// Rules: rules/collections/collections.md — §7.10; rules/compiler/semantic_ir.md — §17(9).
func verifyStringViewBorrow(types *TypeTable, op Operation, values map[ValueID]Value) error {
	if len(op.Operands) != 3 || len(op.Results) != 1 || len(op.Successors) != 0 || op.SliceRange != nil {
		return fmt.Errorf("string.view-borrow requires a string Place, normalized start/length and one view")
	}
	for _, operand := range op.Operands[1:] {
		typ, ok := types.Lookup(values[operand].Type)
		if !ok || typ.Kind != TypeUint || typ.Name != "uint" || !typ.TargetSize {
			return fmt.Errorf("string.view-borrow normalized start/length must be uint")
		}
	}
	place, ok := types.Lookup(values[op.Operands[0]].Type)
	referent, found := types.Lookup(place.Element)
	if !ok || place.Kind != TypePlace || !found || referent.Kind != TypeString {
		return fmt.Errorf("string.view-borrow requires a string Place")
	}
	if !isStringViewType(types, op.Results[0].Type) || op.Results[0].Ownership != OwnershipSharedReference {
		return fmt.Errorf("string.view-borrow must produce a shared StringView")
	}
	return verifyReferenceMetadata(op)
}

// verifyStringViewSource binds construction to the same source extent: the
// static rune count or string.rune-len of the same Place.
// Rules: rules/collections/collections.md — §7.10.
func verifyStringViewSource(op Operation, definitions map[ValueID]Operation) error {
	normalize, ok := definitions[op.Operands[1]]
	if !ok || normalize.Kind != OpSliceRangeNormalize && normalize.Kind != OpSliceRangeCheck ||
		len(normalize.Results) < 2 || normalize.Results[0].ID != op.Operands[1] || normalize.Results[1].ID != op.Operands[2] {
		return fmt.Errorf("string.view-borrow requires the same normalized range start/length tuple")
	}
	extent := definitions[normalize.Operands[0]]
	switch extent.Kind {
	case OpConstInt:
		if extent.Integer == nil || extent.Integer.Cmp(big.NewInt(0)) < 0 {
			return fmt.Errorf("string.view-borrow static rune extent is invalid")
		}
	case OpStringRuneLength:
		if len(extent.Operands) != 1 || extent.Operands[0] != op.Operands[0] {
			return fmt.Errorf("string.view-borrow range must use the same Place's rune length")
		}
	default:
		return fmt.Errorf("string.view-borrow range extent must be a static or observed rune count")
	}
	return nil
}

// OpStringViewByteLength observes the UTF-8 byte count within a view.
// Rules: rules/library/core-library.md — §5.8 (SizeOf).
const OpStringViewByteLength OpKind = "string-view.byte-len"

// buildStringViewLength observes the view's rune count (`view.Len`) through
// the shared bounded-view length operation, or its UTF-8 byte count
// (`view.SizeOf`), after validating the holder.
// Rules: rules/library/core-library.md — §5.8.
func (fb *functionBuilder) buildStringViewLength(expr *ast.MemberExpression, bytes bool) (builtValue, error) {
	view, facts, err := fb.stringViewSourceValue(expr.Object)
	if err != nil {
		return builtValue{}, err
	}
	loc := location(expr.Token)
	guard, err := fb.validateReference(view, facts, loc)
	if err != nil {
		return builtValue{}, err
	}
	uintType := fb.owner.module.Types.Intern(Type{Kind: TypeUint, Name: "uint", TargetSize: true})
	kind := OpSliceLength
	if bytes {
		kind = OpStringViewByteLength
	}
	return fb.result(Operation{Kind: kind, Operands: []ValueID{view.id}, ReferenceGuard: guard, Location: loc}, uintType), nil
}

// buildStringViewMethod lowers the represented read-only members. IsEmpty is
// `Len == 0`; the rune/byte readers, StartsWith and ToString remain an
// explicit boundary until their decoding and materialization operations exist.
// Rules: rules/library/core-library.md — §5.8.
func (fb *functionBuilder) buildStringViewMethod(call *ast.CallExpression, member *ast.MemberExpression, known sema.CompilerKnownMember) (builtValue, error) {
	if known.ID != "CKM-STRINGVIEW-ISEMPTY" {
		return builtValue{}, fb.unsupported("StringView."+known.Name, call.Token)
	}
	length, err := fb.buildStringViewLength(&ast.MemberExpression{Token: member.Token, Object: member.Object, Property: member.Property}, false)
	if err != nil {
		return builtValue{}, err
	}
	loc := location(call.Token)
	boolType, err := fb.boolType()
	if err != nil {
		return builtValue{}, err
	}
	zero := fb.result(Operation{Kind: OpConstInt, Integer: big.NewInt(0), Location: loc}, length.typ)
	return fb.forCompare(IntegerCompareEQ, length, zero, boolType, loc), nil
}

// OpStringViewDecodeNext decodes the Unicode scalar at a hidden UTF-8 cursor
// of a view: results (has, rune, next cursor). The cursor is a view-relative
// position owned by the traversal; it is never a source-visible index.
// Rules: rules/control-flow/flowcontrol_for.md — §19; rules/compiler/semantic_ir.md — §17(10).
const OpStringViewDecodeNext OpKind = "string-view.decode-next"

// LoopForStringView is rune-by-value iteration over a StringView.
// Rules: rules/control-flow/flowcontrol_for.md — §19 (MD-126).
const LoopForStringView LoopKind = "for-string-view"

// buildStringViewFor lowers `for ch in view` and `for i, ch in view`: the
// iterable is evaluated once, each step validates the view and decodes one
// rune by value; the optional index is the view-relative rune position. No
// text is copied and no per-rune object exists. A view formed by the
// iterable expression itself ends at loop exit.
// Rules: rules/control-flow/flowcontrol_for.md — §19; rules/memory/borrowing.md — §5.5(2);
// rules/compiler/semantic_ir.md — §17(10).
func (fb *functionBuilder) buildStringViewFor(stmt *ast.ForStatement) error {
	if stmt.Step != nil || len(stmt.Bindings) == 0 || len(stmt.Bindings) > 2 {
		return fb.unsupported("StringView for form", stmt.Token)
	}
	for _, binding := range stmt.Bindings {
		if binding.Mode != ast.ForBindingValue {
			return fb.unsupported("StringView for binding mode", binding.Token)
		}
	}
	loc := location(stmt.Token)
	viewType := fb.owner.module.Types.stringViewType()
	var view builtValue
	var facts *ReferenceMetadata
	var err error
	_, holder := stmt.Iterable.(*ast.Identifier)
	if holder {
		view, facts, err = fb.stringViewSourceValue(stmt.Iterable)
	} else {
		view, err = fb.buildExpr(stmt.Iterable, viewType)
		if err == nil {
			facts = fb.references[view.id]
		}
	}
	if err != nil {
		return err
	}
	if view.typ != viewType || facts == nil {
		return fb.unsupported("StringView iterable without reference facts", stmt.Token)
	}
	uintType := fb.owner.module.Types.Intern(Type{Kind: TypeUint, Name: "uint", TargetSize: true})
	boolType, err := fb.boolType()
	if err != nil {
		return err
	}
	runeType, err := fb.owner.internType(sema.Type{Kind: sema.RuneType, Name: "rune"})
	if err != nil {
		return err
	}
	zero := fb.result(Operation{Kind: OpConstInt, Integer: big.NewInt(0), Location: loc}, uintType)
	valueBinding := stmt.Bindings[len(stmt.Bindings)-1]
	cursor := fb.forStorage(valueBinding.Name+".cursor", uintType, zero, loc)
	// The index has the binding's resolved integer type (flowcontrol_for.md §19).
	var index StorageID
	indexType := uintType
	if len(stmt.Bindings) == 2 {
		first := stmt.Bindings[0]
		if fact, ok := fb.owner.analyzer.ResolvedBindingDeclaredAt(first.Token.File, first.Token.Line, first.Token.Column); ok && !first.Discard {
			if indexType, err = fb.owner.internType(fact.Type); err != nil {
				return err
			}
		}
		start := fb.result(Operation{Kind: OpConstInt, Integer: big.NewInt(0), Location: loc}, indexType)
		index = fb.forStorage(first.Name+".index", indexType, start, loc)
	}

	loop := &activeLoop{id: fb.nextLoop}
	fb.nextLoop++
	record := LoopRecord{ID: loop.id, Kind: LoopForStringView, ContinuesAfter: true, Location: loc}
	fb.fn.Loops = append(fb.fn.Loops, record)
	recordIndex := len(fb.fn.Loops) - 1
	loop.condition = fb.newBlock()
	record.ConditionBlock = loop.condition.ID
	fb.emit(Operation{Kind: OpBranch, Successors: []BranchTarget{{Block: loop.condition.ID}}, LoopID: loop.id, LoopEdge: LoopEdgeEnter, Location: loc})
	fb.current = loop.condition
	position := fb.result(Operation{Kind: OpStorageLoad, Storage: cursor, Location: loc}, uintType)
	guard, err := fb.validateReference(view, facts, loc)
	if err != nil {
		return err
	}
	has, decoded, next := fb.newValue(boolType, OwnershipImmediate, loc), fb.newValue(runeType, OwnershipImmediate, loc), fb.newValue(uintType, OwnershipImmediate, loc)
	fb.emit(Operation{Kind: OpStringViewDecodeNext, Operands: []ValueID{view.id, position.id}, Results: []Value{has, decoded, next}, ReferenceGuard: guard, Location: loc})
	body := fb.newBlock()
	record.BodyBlock = body.ID
	exit := fb.loopExit(loop)
	fb.emit(Operation{Kind: OpCondBranch, Operands: []ValueID{has.ID}, Successors: []BranchTarget{{Block: body.ID}, {Block: exit.ID}}, LoopID: loop.id, LoopEdge: LoopEdgeCondition, Location: loc})

	fb.current = body
	fb.emit(Operation{Kind: OpStorageStore, Storage: cursor, Operands: []ValueID{next.ID}, Location: loc})
	bind := func(source ast.ForBinding, value builtValue) error {
		if source.Discard {
			return nil
		}
		fact, ok := fb.owner.analyzer.ResolvedBindingDeclaredAt(source.Token.File, source.Token.Line, source.Token.Column)
		if !ok {
			return fb.unsupported("unresolved for binding "+source.Name, source.Token)
		}
		fb.bindings[fact.ID] = binding{value: value.id, typ: value.typ}
		return nil
	}
	if len(stmt.Bindings) == 2 {
		current := fb.result(Operation{Kind: OpStorageLoad, Storage: index, Location: loc}, indexType)
		if err := bind(stmt.Bindings[0], current); err != nil {
			return err
		}
		one := fb.result(Operation{Kind: OpConstInt, Integer: big.NewInt(1), Location: loc}, indexType)
		advanced, err := fb.forAdd(current, one, indexType, loc)
		if err != nil {
			return err
		}
		fb.emit(Operation{Kind: OpStorageStore, Storage: index, Operands: []ValueID{advanced.id}, Location: loc})
	}
	if err := bind(valueBinding, builtValue{id: decoded.ID, typ: runeType}); err != nil {
		return err
	}
	fb.loops = append(fb.loops, loop)
	err = fb.buildScope(stmt.Body)
	fb.loops = fb.loops[:len(fb.loops)-1]
	if err != nil {
		return err
	}
	if fb.current != nil {
		fb.emit(Operation{Kind: OpBranch, Successors: []BranchTarget{{Block: loop.condition.ID}}, LoopID: loop.id, LoopEdge: LoopEdgeBack, Location: loc})
	}
	record.ExitBlock = loop.exit.ID
	fb.current = loop.exit
	if !holder {
		fb.emit(Operation{Kind: OpReferenceEndBorrow, Operands: []ValueID{view.id}, Reference: cloneReferenceMetadata(facts), Location: loc, Synthesized: true})
	}
	fb.fn.Loops[recordIndex] = record
	return nil
}

// verifyStringViewDecodeNext checks the decoding step's operand and result
// types: (view, uint cursor) -> (bool, rune, uint cursor).
// Rules: rules/control-flow/flowcontrol_for.md — §19.
func verifyStringViewDecodeNext(types *TypeTable, op Operation, values map[ValueID]Value) error {
	if len(op.Operands) != 2 || len(op.Results) != 3 || len(op.Successors) != 0 || op.Reference != nil {
		return fmt.Errorf("string-view.decode-next requires (view, cursor) and (has, rune, next)")
	}
	if !isStringViewType(types, values[op.Operands[0]].Type) {
		return fmt.Errorf("string-view.decode-next requires a StringView")
	}
	isUint := func(id TypeID) bool {
		typ, ok := types.Lookup(id)
		return ok && typ.Kind == TypeUint && typ.Name == "uint" && typ.TargetSize
	}
	if !isUint(values[op.Operands[1]].Type) || !isUint(op.Results[2].Type) ||
		!typeHasKind(types, op.Results[0].Type, TypeBool) || !typeHasKind(types, op.Results[1].Type, TypeRune) {
		return fmt.Errorf("string-view.decode-next has invalid cursor or result types")
	}
	return nil
}
