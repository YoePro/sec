package semantic

import (
	"fmt"
	"slices"

	"sec/internal/ast"
	"sec/internal/sema"
)

// OpSliceLength observes the slice's element count without reading elements
// or transferring the slice holder. Both .Len and len(...) denote this uint
// observation; there is no signed-length operation or hidden conversion.
// Rules: rules/mlir/packages/sec-mlir-dialect_package16.md — §§47–49,110–111;
// rules/corrections/applied/md051-md074-md088-length-numeric-string-correction-20261017.md — §1.
const OpSliceLength OpKind = "slice.length"

// directSafeReferenceType identifies safe reference values for validity
// checks without treating a slice as a direct reference to one element.
// Rules: rules/mlir/packages/sec-mlir-dialect_package16.md — §§31–32,44,84.
func directSafeReferenceType(types *TypeTable, id TypeID) bool {
	typ, ok := types.Lookup(id)
	return ok && (typ.Kind == TypeReference || boundedView(typ.Kind))
}

// verifySliceLengthOperation requires one bounded slice and its exact uint
// count. Raw pointers, direct element references, arrays and signed lengths
// are different semantic operations.
// Rules: rules/mlir/packages/sec-mlir-dialect_package16.md — §§47–49,110–111.
func verifySliceLengthOperation(types *TypeTable, op Operation, values map[ValueID]Value) error {
	if len(op.Operands) != 1 || len(op.Results) != 1 || len(op.Successors) != 0 {
		return fmt.Errorf("slice.length requires one slice operand and one uint result")
	}
	if source, ok := types.Lookup(values[op.Operands[0]].Type); op.Kind == OpStringViewByteLength && (!ok || source.Kind != TypeStringView) {
		return fmt.Errorf("string-view.byte-len requires a StringView")
	}
	source, ok := types.Lookup(values[op.Operands[0]].Type)
	result, resultOK := types.Lookup(op.Results[0].Type)
	if !ok || !boundedView(source.Kind) ||
		!resultOK || result.Kind != TypeUint || result.Name != "uint" || !result.TargetSize {
		return fmt.Errorf("slice.length requires a shared/mutable slice and target-sized uint result")
	}
	if op.Reference != nil {
		return fmt.Errorf("slice.length observes its operand; it does not create a new reference")
	}
	return nil
}

// verifySliceObservationValidity consumes the canonical P15 source/guard facts.
// Length and slice indexing share this temporal gate. A dynamic guard must validate the same borrow and epoch, and its successful
// branch must dominate the observation with no failure-path bypass. The stale
// endpoint stays InvalidReferenceGeneration independently of spatial try flow.
// Rules: rules/mlir/packages/sec-mlir-dialect_package16.md — §§44–49,50–58,81–84,110–111;
// rules/memory/reference_model.md — §30.
func verifySliceObservationValidity(op Operation, block BlockID, blocks map[BlockID]*Block,
	dom map[BlockID]map[BlockID]bool, definitions map[ValueID]Operation,
	definitionBlocks map[ValueID]BlockID, sources map[ValueID]*ReferenceMetadata) error {
	source := sources[op.Operands[0]]
	if source == nil {
		return fmt.Errorf("%s source has no canonical reference facts", op.Kind)
	}
	if err := verifyReferenceMetadata(Operation{Kind: op.Kind, Reference: source}); err != nil {
		return err
	}
	if source.Validity != ReferenceDynamicEpoch {
		return nil
	}
	guard, ok := definitions[op.ReferenceGuard]
	if !ok || guard.Kind != OpReferenceIsValid || len(guard.Operands) != 1 || guard.Operands[0] != op.Operands[0] ||
		!sameSliceValidityFacts(source, guard.Reference) {
		return fmt.Errorf("%s requires a validity guard for the same source borrow and epoch", op.Kind)
	}
	guardBlock := blocks[definitionBlocks[op.ReferenceGuard]]
	if guardBlock == nil || len(guardBlock.Operations) == 0 {
		return fmt.Errorf("%s validity guard has no block", op.Kind)
	}
	branch := guardBlock.Operations[len(guardBlock.Operations)-1]
	if branch.Kind != OpCondBranch || len(branch.Operands) != 1 || branch.Operands[0] != op.ReferenceGuard || len(branch.Successors) != 2 {
		return fmt.Errorf("%s validity guard must select success and stale-reference failure", op.Kind)
	}
	graph := map[BlockID][]BlockID{}
	for id, b := range blocks {
		for _, operation := range b.Operations {
			for _, successor := range operation.Successors {
				graph[id] = append(graph[id], successor.Block)
			}
		}
	}
	success, failure := branch.Successors[0].Block, branch.Successors[1].Block
	if !dom[block][guardBlock.ID] || !dom[block][success] || reachableAvoiding(graph, failure, block, guardBlock.ID) {
		return fmt.Errorf("%s is not dominated by its source validity guard's success edge", op.Kind)
	}
	return nil
}

// sameSliceValidityFacts ties a check to the complete existing P15 identity;
// matching only the slice SSA operand cannot justify another epoch or borrow.
// Rules: rules/mlir/packages/sec-mlir-dialect_package16.md — §§44–46,84.
func sameSliceValidityFacts(source, guard *ReferenceMetadata) bool {
	return source != nil && guard != nil && source.Borrow == guard.Borrow && source.Lifetime == guard.Lifetime &&
		source.SourceBorrow == guard.SourceBorrow && source.Validity == guard.Validity &&
		source.InvalidationDomain == guard.InvalidationDomain && source.EpochDependency == guard.EpochDependency &&
		source.AddressSpace == guard.AddressSpace && source.AddressStability == guard.AddressStability &&
		source.Pinned == guard.Pinned && source.Provenance == guard.Provenance &&
		source.ProofProvenance == guard.ProofProvenance && slices.Equal(source.OriginStorages, guard.OriginStorages)
}

// OpSliceFill replaces every element of a mutable slice, in increasing index
// order, with a trivial copy of its value operand (collections.md § 7.9). It
// shares the slice observation validity gate, changes neither Len nor
// storage identity and never allocates.
// Rules: rules/collections/collections.md — § 7.9.
const OpSliceFill OpKind = "slice.fill"

// verifySliceFillOperation requires a mutable slice, a value of its element
// type and a trivially copyable and destructible element.
// Rules: rules/collections/collections.md — § 7.9.
func verifySliceFillOperation(module *Module, op Operation, values map[ValueID]Value) error {
	if len(op.Operands) != 2 || len(op.Results) != 0 || len(op.Successors) != 0 {
		return fmt.Errorf("slice.fill takes a mutable slice and a value and has no result")
	}
	slice, ok := module.Types.Lookup(values[op.Operands[0]].Type)
	if !ok || slice.Kind != TypeMutableSlice || values[op.Operands[1]].Type != slice.Element {
		return fmt.Errorf("slice.fill requires a mutable slice and a value of its element type")
	}
	if ownedCopyClass(module, slice.Element) != "trivial" || !trivialDestruction(module, slice.Element) {
		return fmt.Errorf("slice.fill requires a trivially copyable and destructible element")
	}
	return nil
}

// buildSliceFill lowers `values.Fill(value)` on a mutable slice: the slice
// holder is read, the value evaluated exactly once, the holder validated,
// and one slice.fill replaces every element. Sema admits only infallibly
// copyable elements and Sec 0.1 has no semantic copy, so a replaced element
// needs no destruction.
// Rules: rules/collections/collections.md — § 7.9.
func (fb *functionBuilder) buildSliceFill(call *ast.CallExpression, member *ast.MemberExpression) (builtValue, error) {
	token := member.Property.Token
	if len(call.Arguments) != 1 {
		return builtValue{}, fmt.Errorf("Fill takes one value")
	}
	source, facts, err := fb.sliceSourceValue(member.Object)
	if err != nil {
		return builtValue{}, err
	}
	slice, ok := fb.owner.module.Types.Lookup(source.typ)
	if !ok || slice.Kind != TypeMutableSlice {
		return builtValue{}, fb.unsupported("Fill on a receiver that is not a mutable slice", token)
	}
	if ownedCopyClass(fb.owner.module, slice.Element) != "trivial" || !trivialDestruction(fb.owner.module, slice.Element) {
		return builtValue{}, fb.unsupported("Fill of a non-trivially copyable element", token)
	}
	value, err := fb.buildExpr(call.Arguments[0], slice.Element)
	if err != nil {
		return builtValue{}, err
	}
	loc := location(token)
	guard, err := fb.validateReference(source, facts, loc)
	if err != nil {
		return builtValue{}, err
	}
	fb.emit(Operation{Kind: OpSliceFill, Operands: []ValueID{source.id, value.id}, ReferenceGuard: guard, Location: loc})
	if _, temporary := member.Object.(*ast.RefExpression); temporary {
		if _, err := fb.discardSliceHolder(source, expressionToken(member.Object)); err != nil {
			return builtValue{}, err
		}
	}
	void, err := fb.owner.internType(sema.Type{Name: "void", Kind: sema.VoidType})
	if err != nil {
		return builtValue{}, err
	}
	return builtValue{typ: void}, nil
}

// sliceFillCall reports a call of the compiler-known mutable-slice Fill.
// Rules: rules/collections/collections.md — § 7.9.
func (fb *functionBuilder) sliceFillCall(call *ast.CallExpression) (*ast.MemberExpression, bool) {
	member, ok := call.Callee.(*ast.MemberExpression)
	if !ok || member.Property == nil {
		return nil, false
	}
	site := member.Property.Token
	known, ok := fb.owner.analyzer.CompilerKnownMemberAt(site.File, site.Line, site.Column)
	return member, ok && known.ID == "CKM-MUTABLE-SLICE-FILL"
}
