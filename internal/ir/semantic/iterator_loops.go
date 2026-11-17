package semantic

import (
	"sec/internal/ast"
	"sec/internal/sema"
)

// LoopForIterator is a compiler-known Iterator[T] loop: repeated statically
// resolved Next calls with Some/None control flow.
// Rules: rules/control-flow/flowcontrol_for.md — § 37.
const LoopForIterator LoopKind = "for-iterator"

// buildIteratorFor lowers `for value in iterator` from Sema's
// ResolvedForIteration plan, never by rediscovering Next from member names.
// Each iteration borrows the reusable iterator storage as Sema planned,
// calls the resolved concrete Next directly, tests Some, binds the
// copy-trivial payload (or discards it) and runs the body; None leaves the
// loop. It reports handled=false for loops that are not Iterator[T] loops.
//
// Forms not lowered yet stop at a named boundary: a fresh temporary iterator
// (hidden loop-owned storage and its destruction at every exit), a borrowed
// ref mut Iterator[T] view (runtime Next dispatch, MD-057/MD-108), a generic
// or specialized Next, and an element that is not trivially copyable.
//
// Rules:
//   - rules/control-flow/flowcontrol_for.md — § 37 "Iterator protocol", § 37.1
//   - rules/compiler/semantic_ir.md — § 65(2) loop forms, § 66(1) loop CFG
//   - rules/compiler/compiler_analysis.md — lowering consumes resolved facts
func (fb *functionBuilder) buildIteratorFor(stmt *ast.ForStatement) (bool, error) {
	plan, ok := fb.owner.analyzer.ResolvedForIterationOf(stmt)
	if !ok || plan.Kind != sema.ForIterationCompilerKnownIterator {
		return false, nil
	}
	if fb.owner.maxPackage < ownershipLoweringPackage {
		return true, fb.unsupported("Iterator[T] for loop", stmt.Token)
	}
	switch plan.Source {
	case sema.ForIteratorBorrowedView:
		return true, fb.unsupported("for loop over a borrowed ref mut Iterator[T] view (runtime Next dispatch)", stmt.Token)
	case sema.ForIteratorFreshTemporary:
		return true, fb.unsupported("for loop over a fresh temporary iterator (loop-owned iterator storage)", stmt.Token)
	}
	next := plan.Next
	if plan.NextCallable == "" || isGenericInstance(next) || len(next.GenericParameters) != 0 || !fb.owner.instanceMethodLowerable(next) {
		return true, fb.unsupported("Iterator[T] for loop with a generic or unlowerable Next", stmt.Token)
	}
	if len(stmt.Bindings) != 1 || stmt.Bindings[0].Mode != ast.ForBindingValue {
		return true, fb.unsupported("Iterator[T] for binding form", stmt.Token)
	}
	if transfer, _ := sema.ResolveTransferAction(plan.ElementType, false); transfer != sema.TransferCopyTrivial {
		return true, fb.unsupported("Iterator[T] for loop over an element that is not trivially copyable", stmt.Token)
	}
	loc := location(stmt.Token)
	optionType, err := fb.owner.internSignatureType(next.ReturnType)
	if err != nil {
		return true, err
	}
	some, _, ok := fb.optionVariants(optionType)
	if !ok {
		return true, fb.unsupported("Iterator[T] Next without an Option result", stmt.Token)
	}
	definition, _ := fb.owner.unionDefinition(optionType)
	payloadType := definition.Variants[some].Payload
	element, err := fb.owner.internType(plan.SourceType)
	if err != nil {
		return true, err
	}
	referenceType := fb.owner.module.Types.Intern(Type{Kind: TypeReference, Element: element, Mutable: next.ReceiverMutable})
	action := ArgumentSharedReference
	if next.ReceiverMutable {
		action = ArgumentMutableReference
	}
	boolType, err := fb.boolType()
	if err != nil {
		return true, err
	}

	loop := &activeLoop{id: fb.nextLoop}
	fb.nextLoop++
	record := LoopRecord{ID: loop.id, Kind: LoopForIterator, ContinuesAfter: true, Location: loc}
	fb.fn.Loops = append(fb.fn.Loops, record)
	recordIndex := len(fb.fn.Loops) - 1
	loop.condition = fb.newBlock()
	record.ConditionBlock = loop.condition.ID
	fb.emit(Operation{Kind: OpBranch, Successors: []BranchTarget{{Block: loop.condition.ID}}, LoopID: loop.id, LoopEdge: LoopEdgeEnter, Location: loc})

	// Condition: borrow the iterator storage, advance it through the
	// resolved Next and test Some.
	fb.current = loop.condition
	receiver, err := fb.buildExpr(stmt.Iterable, referenceType)
	if err != nil {
		return true, err
	}
	if receiver.typ != referenceType {
		return true, fb.unsupported("Iterator[T] for loop without a resolved iterator borrow", expressionToken(stmt.Iterable))
	}
	option := fb.result(Operation{Kind: OpDirectCall, Callee: functionID(next, fb.owner.module.Types), Operands: []ValueID{receiver.id}, ArgumentActions: []ArgumentAction{action}, Location: loc}, optionType)
	isSome := fb.result(Operation{Kind: OpUnionIsVariant, Operands: []ValueID{option.id}, UnionVariant: some, Location: loc}, boolType)
	body := fb.newBlock()
	record.BodyBlock = body.ID
	exit := fb.loopExit(loop)
	fb.emit(Operation{Kind: OpCondBranch, Operands: []ValueID{isSome.id}, Successors: []BranchTarget{{Block: body.ID}, {Block: exit.ID}}, LoopID: loop.id, LoopEdge: LoopEdgeCondition, Location: loc})

	// Body: bind the yielded value, then run the source body.
	fb.current = body
	value := fb.result(Operation{Kind: OpUnionUnwrapPayload, Operands: []ValueID{option.id}, UnionVariant: some, PayloadActions: []UnionPayloadAction{UnionPayloadCopyTrivial}, Location: loc}, payloadType)
	if binding := stmt.Bindings[0]; !binding.Discard {
		fact, ok := fb.owner.analyzer.ResolvedBindingDeclaredAt(binding.Token.File, binding.Token.Line, binding.Token.Column)
		if !ok {
			return true, fb.unsupported("unresolved for binding "+binding.Name, binding.Token)
		}
		fb.bindings[fact.ID] = bindingOf(value)
	}
	fb.loops = append(fb.loops, loop)
	err = fb.buildScope(stmt.Body)
	fb.loops = fb.loops[:len(fb.loops)-1]
	if err != nil {
		return true, err
	}
	if fb.current != nil {
		fb.emit(Operation{Kind: OpBranch, Successors: []BranchTarget{{Block: loop.condition.ID}}, LoopID: loop.id, LoopEdge: LoopEdgeBack, Location: loc})
	}
	record.ExitBlock = loop.exit.ID
	fb.current = loop.exit
	fb.fn.Loops[recordIndex] = record
	return true, nil
}

func bindingOf(value builtValue) binding { return binding{value: value.id, typ: value.typ} }
