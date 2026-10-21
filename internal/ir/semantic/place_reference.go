package semantic

import (
	"fmt"
	"strings"
)

// Semantic IR Places and direct safe references (SEC-MLIR-P15).
//
// A Place is an addressable semantic path, never a numeric address or a
// language value; a direct reference is a non-null, non-owning `ref T` or
// `ref mut T` value. Value, storage, Place, safe reference and RawPtr stay
// distinct categories. No operation here chooses a physical representation,
// computes offsets/strides/GEPs, copies an enclosing aggregate to form a
// subplace, or requires a runtime borrow counter.
//
// Rules:
//   - rules/compiler/semantic_ir.md — §§ 10(5)-(11), 45(5)-(13), 46(6)-(9), 48(6)-(7), 96(5), 108(5)
//   - rules/mlir/packages/sec-mlir-dialect_package15.md — §§ 10-62
//   - rules/mlir/semantic-ir/sec_semantic_ir_place_reference_v1.md

const (
	// TypePlace is `place<T, ro|rw>`; Type.Mutable selects rw.
	TypePlace TypeKind = "place"
	// TypeReference is `ref T` or, with Type.Mutable, `ref mut T`.
	TypeReference TypeKind = "reference"
)

const (
	OpPlaceValueRoot               OpKind = "place.value-root"
	OpPlaceStorageRoot             OpKind = "place.storage-root"
	OpPlaceField                   OpKind = "place.field"
	OpPlaceArrayIndexInBounds      OpKind = "place.array-index-in-bounds"
	OpPlaceArrayElement            OpKind = "place.array-element"
	OpPlaceUnionPayload            OpKind = "place.union-payload"
	OpReferenceDerefPlace          OpKind = "ref.deref-place"
	OpPlaceRead                    OpKind = "place.read"
	OpPlaceWrite                   OpKind = "place.write"
	OpReferenceBorrowShared        OpKind = "ref.borrow-shared"
	OpReferenceBorrowMutable       OpKind = "ref.borrow-mut"
	OpReferenceCopyShared          OpKind = "ref.copy-shared"
	OpReferenceMove                OpKind = "ref.move"
	OpReferenceReborrowShared      OpKind = "ref.reborrow-shared"
	OpReferenceReborrowMutable     OpKind = "ref.reborrow-mut"
	OpReferenceIsValid             OpKind = "ref.is-valid"
	OpReferenceGenerationFailure   OpKind = "fail.reference-generation"
	OpReferenceEndBorrow           OpKind = "ref.end-borrow"
	OpReferenceCompare             OpKind = "ref.compare"
	invalidReferenceGenerationID          = 8
	invalidReferenceGenerationName        = "InvalidReferenceGeneration"
)

// PlaceRootKind is the binding role of a Place root.
type PlaceRootKind string

const (
	PlaceRootLocal     PlaceRootKind = "local"
	PlaceRootParameter PlaceRootKind = "parameter"
	PlaceRootReceiver  PlaceRootKind = "receiver"
	PlaceRootStatic    PlaceRootKind = "static"
	PlaceRootDeref     PlaceRootKind = "deref"
)

// AddressSpace names a storage.md § 16 memory space.
type AddressSpace string

const (
	AddressSpaceOrdinary AddressSpace = "ordinary"
	AddressSpaceMMIO     AddressSpace = "mmio"
)

// PlaceMetadata carries the canonical facts of a Place-producing operation:
// the stable root identity and kind, the storage-domain identity, the memory
// space, and the dynamic index identity of an element projection.
type PlaceMetadata struct {
	Root           uint32
	RootKind       PlaceRootKind
	Storage        uint32
	AddressSpace   AddressSpace
	DynamicIndexID uint32
	// VariantProven marks a union payload Place formed under the
	// active-variant proof of the same union storage.
	VariantProven bool
}

// ReferenceValidity is the validity policy of a direct reference.
type ReferenceValidity string

const (
	ReferenceProven       ReferenceValidity = "proven"
	ReferenceDynamicEpoch ReferenceValidity = "dynamic-epoch"
)

// ReferenceMetadata carries the complete facts of a reference-producing or
// reference-consuming operation (package15.md § 31).
type ReferenceMetadata struct {
	Borrow             uint32
	Lifetime           uint32
	SourceBorrow       uint32
	Validity           ReferenceValidity
	ProofProvenance    string
	InvalidationDomain uint32
	EpochDependency    uint32
	AddressSpace       AddressSpace
	AddressStability   string
	Pinned             bool
	Provenance         string
	OriginStorages     []uint32
}

func placeType(types *TypeTable, id TypeID) (Type, bool) {
	typ, ok := types.Lookup(id)
	return typ, ok && typ.Kind == TypePlace
}

func referenceType(types *TypeTable, id TypeID) (Type, bool) {
	typ, ok := types.Lookup(id)
	return typ, ok && typ.Kind == TypeReference
}

func isPlaceReferenceOperation(kind OpKind) bool {
	switch kind {
	case OpPlaceValueRoot, OpPlaceStorageRoot, OpPlaceStaticRoot, OpPlaceField, OpPlaceArrayIndexInBounds, OpPlaceArrayElement,
		OpPlaceUnionPayload, OpReferenceDerefPlace, OpPlaceRead, OpPlaceWrite, OpReferenceBorrowShared,
		OpReferenceBorrowMutable, OpReferenceCopyShared, OpReferenceMove, OpReferenceReborrowShared,
		OpReferenceReborrowMutable, OpReferenceIsValid, OpReferenceGenerationFailure, OpReferenceEndBorrow,
		OpReferenceCompare:
		return true
	}
	return false
}

// placeConsumer reports the operations that may take a Place operand: only
// Place projections, reads, writes, bounds predicates and borrows. Every other
// use would make a Place escape as a language value.
// Rules: rules/compiler/semantic_ir.md — §96(5), §126.7;
// rules/mlir/packages/sec-mlir-dialect_package16.md — §42.
func placeConsumer(kind OpKind) bool {
	switch kind {
	case OpSliceBorrowShared, OpSliceBorrowMutable, OpSliceElementPlace, OpPlaceField, OpPlaceArrayIndexInBounds, OpPlaceArrayElement, OpPlaceUnionPayload, OpPlaceRead,
		OpPlaceWrite, OpReferenceBorrowShared, OpReferenceBorrowMutable,
		// A reborrow may derive from a dereferenced sub-Place (§ 78);
		// verifySubplaceReborrowSource checks where that Place comes from.
		OpReferenceReborrowShared, OpReferenceReborrowMutable:
		return true
	}
	return false
}

func formatPlaceReferenceType(t Type) string {
	switch t.Kind {
	case TypePlace:
		if t.Mutable {
			return fmt.Sprintf("place<!%d, rw>", t.Element)
		}
		return fmt.Sprintf("place<!%d, ro>", t.Element)
	case TypeReference:
		if t.Mutable {
			return fmt.Sprintf("ref_mut<!%d>", t.Element)
		}
		return fmt.Sprintf("ref<!%d>", t.Element)
	}
	return ""
}

// formatPlaceReferenceOperation prints the deterministic operand and fact
// suffix of a P15 operation (semantic_ir.md § 108(5)): stable root/storage
// identities, structured projections, exact constant indexes, reference kind,
// borrow/lifetime identities, validity, epoch dependency and stability.
func formatPlaceReferenceOperation(out *strings.Builder, op Operation) {
	if len(op.Operands) > 0 {
		out.WriteByte(' ')
		for index, operand := range op.Operands {
			if index > 0 {
				out.WriteString(", ")
			}
			fmt.Fprintf(out, "%%%d", operand)
		}
	}
	switch op.Kind {
	case OpPlaceStorageRoot:
		fmt.Fprintf(out, " $%d", op.Storage)
	case OpPlaceStaticRoot:
		fmt.Fprintf(out, " @%d", op.Static)
	case OpPlaceField:
		fmt.Fprintf(out, " field=#%d", op.StructField)
	case OpPlaceArrayElement:
		fmt.Fprintf(out, " bounds=%s proof=%s", op.ArrayCheckKind, op.ArrayProofKind)
		if op.Integer != nil {
			fmt.Fprintf(out, " index=%s", op.Integer.String())
		}
		if op.ArrayGuard != 0 {
			fmt.Fprintf(out, " guard=%%%d", op.ArrayGuard)
		}
	case OpPlaceUnionPayload:
		fmt.Fprintf(out, " variant=#%d", op.UnionVariant)
		if op.VariantGuard != 0 {
			fmt.Fprintf(out, " guard=%%%d", op.VariantGuard)
		}
	case OpReferenceCompare:
		fmt.Fprintf(out, " %s", op.Operator)
	case OpReferenceGenerationFailure:
		fmt.Fprintf(out, " reason=%s#%d", op.PanicReason, op.PanicReasonID)
	}
	if op.ReferenceGuard != 0 {
		fmt.Fprintf(out, " valid=%%%d", op.ReferenceGuard)
	}
	if place := op.Place; place != nil {
		fmt.Fprintf(out, " {root=#%d kind=%s storage=#%d space=%s", place.Root, place.RootKind, place.Storage, place.AddressSpace)
		if place.DynamicIndexID != 0 {
			fmt.Fprintf(out, " index-id=#%d", place.DynamicIndexID)
		}
		if place.VariantProven {
			out.WriteString(" variant-proven")
		}
		out.WriteByte('}')
	}
	formatReferenceMetadata(out, op.Reference)
}

// formatReferenceMetadata prints the deterministic reference fact suffix
// shared by reference operations and reference parameters.
func formatReferenceMetadata(out *strings.Builder, ref *ReferenceMetadata) {
	if ref != nil {
		fmt.Fprintf(out, " {borrow=#%d lifetime=#%d", ref.Borrow, ref.Lifetime)
		if ref.SourceBorrow != 0 {
			fmt.Fprintf(out, " from=#%d", ref.SourceBorrow)
		}
		fmt.Fprintf(out, " validity=%s", ref.Validity)
		if ref.ProofProvenance != "" {
			fmt.Fprintf(out, " proof=%s", ref.ProofProvenance)
		}
		if ref.EpochDependency != 0 {
			fmt.Fprintf(out, " domain=#%d epoch=#%d", ref.InvalidationDomain, ref.EpochDependency)
		}
		fmt.Fprintf(out, " space=%s stability=%s provenance=%s", ref.AddressSpace, ref.AddressStability, ref.Provenance)
		if ref.Pinned {
			out.WriteString(" pinned")
		}
		if len(ref.OriginStorages) > 0 {
			parts := make([]string, len(ref.OriginStorages))
			for index, storage := range ref.OriginStorages {
				parts[index] = fmt.Sprintf("#%d", storage)
			}
			fmt.Fprintf(out, " origins=[%s]", strings.Join(parts, ","))
		}
		out.WriteByte('}')
	}
}

func verifyPlaceReferenceTypes(table *TypeTable, id TypeID, t Type) error {
	switch t.Kind {
	case TypePlace, TypeReference:
		element, ok := table.Lookup(t.Element)
		if !ok {
			return fmt.Errorf("%s !%d has invalid element type !%d", t.Kind, id, t.Element)
		}
		if element.Kind == TypePlace || element.Kind == TypeVoid || element.Kind == TypeNever {
			return fmt.Errorf("%s !%d has unsupported element type", t.Kind, id)
		}
	}
	return nil
}

func validRootKind(kind PlaceRootKind) bool {
	switch kind {
	case PlaceRootLocal, PlaceRootParameter, PlaceRootReceiver, PlaceRootStatic, PlaceRootDeref:
		return true
	}
	return false
}

func validAddressSpace(space AddressSpace) bool {
	return space == AddressSpaceOrdinary || space == AddressSpaceMMIO || strings.HasPrefix(string(space), "target:")
}

// verifyReferenceMetadata requires complete reference facts: identities,
// a proven policy with its proof or a dynamic policy with its domain/epoch
// dependency, and an address space.
func verifyReferenceMetadata(op Operation) error {
	ref := op.Reference
	if ref == nil {
		return fmt.Errorf("%s requires reference metadata", op.Kind)
	}
	if ref.Borrow == 0 || ref.Lifetime == 0 || !validAddressSpace(ref.AddressSpace) || ref.AddressStability == "" || ref.Provenance == "" {
		return fmt.Errorf("%s has incomplete reference metadata", op.Kind)
	}
	switch ref.Validity {
	case ReferenceProven:
		if ref.ProofProvenance == "" || ref.EpochDependency != 0 {
			return fmt.Errorf("%s proven reference requires closed proof provenance and no epoch dependency", op.Kind)
		}
	case ReferenceDynamicEpoch:
		if ref.InvalidationDomain == 0 || ref.EpochDependency == 0 {
			return fmt.Errorf("%s dynamic-epoch reference requires an invalidation domain and epoch dependency", op.Kind)
		}
	default:
		return fmt.Errorf("%s reference is neither proven nor dynamically checkable", op.Kind)
	}
	return nil
}

// verifyPlaceReferenceOperation checks one P15 operation (semantic_ir.md
// § 96(5)): root/projection types, authority that never increases, exact
// array index/bounds facts, union variant proof, copy-trivial reads and
// trivially replaceable writes, complete reference facts, borrow derivation
// and no `ref` -> `ref mut` path.
func verifyPlaceReferenceOperation(module *Module, op Operation, values map[ValueID]Value, storages map[StorageID]Storage) error {
	types := module.Types
	operandType := func(index int) TypeID { return values[op.Operands[index]].Type }
	result := func() (Type, bool) {
		if len(op.Results) != 1 {
			return Type{}, false
		}
		return types.Lookup(op.Results[0].Type)
	}
	requirePlaceMetadata := func() error {
		if op.Place == nil || op.Place.Root == 0 || !validRootKind(op.Place.RootKind) || !validAddressSpace(op.Place.AddressSpace) {
			return fmt.Errorf("%s requires complete Place root metadata", op.Kind)
		}
		return nil
	}
	switch op.Kind {
	case OpPlaceValueRoot:
		place, ok := result()
		if len(op.Operands) != 1 || !ok || place.Kind != TypePlace || place.Mutable || place.Element != operandType(0) {
			return fmt.Errorf("invalid place.value-root: an immutable SSA root is a read-only Place of its value type")
		}
		return requirePlaceMetadata()
	case OpPlaceStorageRoot:
		place, ok := result()
		storage, declared := storages[op.Storage]
		if len(op.Operands) != 0 || !ok || place.Kind != TypePlace || !declared || place.Element != storage.Type {
			return fmt.Errorf("invalid place.storage-root")
		}
		if place.Mutable && !storage.Mutable {
			return fmt.Errorf("place.storage-root authority exceeds storage mutability")
		}
		if !storage.AddressTaken {
			return fmt.Errorf("place.storage-root addresses storage $%d that is not marked address-taken", op.Storage)
		}
		return requirePlaceMetadata()
	case OpPlaceField:
		base, baseOK := placeType(types, firstOperandType(op, values))
		place, ok := result()
		if len(op.Operands) != 1 || !baseOK || !ok || place.Kind != TypePlace || place.Mutable != base.Mutable {
			return fmt.Errorf("invalid place.field: authority must be preserved")
		}
		definition, found := structDefinition(module, base.Element)
		if !found || int(op.StructField) >= len(definition.Fields) || definition.Fields[op.StructField].Type != place.Element {
			return fmt.Errorf("place.field projection does not match the struct field")
		}
	case OpPlaceArrayIndexInBounds:
		base, baseOK := placeType(types, firstOperandType(op, values))
		if len(op.Operands) != 2 || !baseOK || len(op.Results) != 1 || !typeHasKind(types, op.Results[0].Type, TypeBool) || !typeHasKind(types, base.Element, TypeArray) || !isBuiltinIntegerType(types, operandType(1)) {
			return fmt.Errorf("invalid place.array-index-in-bounds")
		}
	case OpPlaceArrayElement:
		base, baseOK := placeType(types, firstOperandType(op, values))
		place, ok := result()
		if len(op.Operands) != 2 || !baseOK || !ok || place.Kind != TypePlace || place.Mutable != base.Mutable || !isBuiltinIntegerType(types, operandType(1)) {
			return fmt.Errorf("invalid place.array-element: authority must be preserved")
		}
		array, arrayOK := types.Lookup(base.Element)
		if !arrayOK || array.Kind != TypeArray || array.Element != place.Element {
			return fmt.Errorf("place.array-element projection does not match the array element")
		}
		if err := verifyArrayBoundsMetadata(op); err != nil {
			return err
		}
		if op.Integer != nil {
			length, lengthOK := parseCanonicalArrayLength(array.Length)
			if !lengthOK || op.Integer.Sign() < 0 || op.Integer.Cmp(length) >= 0 {
				return fmt.Errorf("place.array-element constant index %s is outside the exact length %s", op.Integer, array.Length)
			}
		}
		if op.ArrayCheckKind == ArrayIndexRuntimeCheck && op.ArrayGuard == 0 {
			return fmt.Errorf("place.array-element runtime bounds check requires a guard")
		}
	case OpPlaceUnionPayload:
		base, baseOK := placeType(types, firstOperandType(op, values))
		place, ok := result()
		if len(op.Operands) != 1 || !baseOK || !ok || place.Kind != TypePlace || place.Mutable != base.Mutable {
			return fmt.Errorf("invalid place.union-payload: authority must be preserved")
		}
		if op.Place == nil || !op.Place.VariantProven {
			return fmt.Errorf("place.union-payload requires the active-variant proof of the same union storage")
		}
		if payload, found := variantPayloadType(module, base.Element, op.UnionVariant); !found || payload != place.Element {
			return fmt.Errorf("place.union-payload projection does not match the variant payload")
		}
	case OpReferenceDerefPlace:
		ref, refOK := referenceType(types, firstOperandType(op, values))
		place, ok := result()
		if len(op.Operands) != 1 || !refOK || !ok || place.Kind != TypePlace || place.Element != ref.Element || place.Mutable != ref.Mutable {
			return fmt.Errorf("invalid ref.deref-place: shared references give read-only and mutable references read/write Places")
		}
		return verifyReferenceMetadata(op)
	case OpPlaceRead:
		place, placeOK := placeType(types, firstOperandType(op, values))
		if len(op.Operands) != 1 || !placeOK || len(op.Results) != 1 || op.Results[0].Type != place.Element {
			return fmt.Errorf("invalid place.read")
		}
		if !package14TrivialValue(module, place.Element) {
			return fmt.Errorf("place.read supports copy-trivial referents only; move-only and semantic-copy reads are unsupported")
		}
	case OpPlaceWrite:
		place, placeOK := placeType(types, firstOperandType(op, values))
		if len(op.Operands) != 2 || !placeOK || len(op.Results) != 0 || operandType(1) != place.Element {
			return fmt.Errorf("invalid place.write")
		}
		if !place.Mutable {
			return fmt.Errorf("place.write requires a read/write Place")
		}
		if !package14TrivialValue(module, place.Element) {
			return fmt.Errorf("place.write supports trivially replaceable referents only; non-trivial replacement is unsupported")
		}
	case OpReferenceBorrowShared, OpReferenceBorrowMutable:
		place, placeOK := placeType(types, firstOperandType(op, values))
		ref, ok := result()
		mutable := op.Kind == OpReferenceBorrowMutable
		if len(op.Operands) != 1 || !placeOK || !ok || ref.Kind != TypeReference || ref.Element != place.Element || ref.Mutable != mutable {
			return fmt.Errorf("invalid %s", op.Kind)
		}
		if mutable && !place.Mutable {
			return fmt.Errorf("ref.borrow-mut requires a read/write Place")
		}
		if err := verifyReferenceMetadata(op); err != nil {
			return err
		}
		if op.Reference.SourceBorrow != 0 {
			return fmt.Errorf("%s creates a new borrow and has no source borrow", op.Kind)
		}
	case OpReferenceCopyShared, OpReferenceMove:
		source, sourceOK := referenceType(types, firstOperandType(op, values))
		ref, ok := result()
		if len(op.Operands) != 1 || !sourceOK || !ok || op.Results[0].Type != firstOperandType(op, values) || ref.Mutable != source.Mutable {
			return fmt.Errorf("invalid %s", op.Kind)
		}
		if op.Kind == OpReferenceCopyShared && source.Mutable {
			return fmt.Errorf("ref.copy-shared cannot copy a mutable reference; ref mut is move-only")
		}
		if op.Kind == OpReferenceMove && !source.Mutable {
			return fmt.Errorf("ref.move transfers a mutable reference holder; shared references are copied")
		}
		if err := verifyReferenceMetadata(op); err != nil {
			return err
		}
		if op.Reference.SourceBorrow != op.Reference.Borrow {
			return fmt.Errorf("%s keeps the borrow of its source", op.Kind)
		}
	case OpReferenceReborrowShared, OpReferenceReborrowMutable:
		// The source is the existing reference, or a sub-Place derived from
		// its dereference (§ 78); the function-level check ties such a Place
		// to the reference named by SourceBorrow.
		ref, ok := result()
		mutable := op.Kind == OpReferenceReborrowMutable
		if len(op.Operands) != 1 || !ok || ref.Kind != TypeReference || ref.Mutable != mutable {
			return fmt.Errorf("invalid %s", op.Kind)
		}
		if source, isReference := referenceType(types, firstOperandType(op, values)); isReference {
			if ref.Element != source.Element {
				return fmt.Errorf("invalid %s", op.Kind)
			}
			if mutable && !source.Mutable {
				return fmt.Errorf("ref.reborrow-mut cannot derive ref mut from ref")
			}
		} else if place, isPlace := placeType(types, firstOperandType(op, values)); isPlace {
			if ref.Element != place.Element {
				return fmt.Errorf("invalid %s", op.Kind)
			}
			if mutable && !place.Mutable {
				return fmt.Errorf("ref.reborrow-mut cannot derive ref mut from a read-only Place")
			}
		} else {
			return fmt.Errorf("invalid %s", op.Kind)
		}
		if err := verifyReferenceMetadata(op); err != nil {
			return err
		}
		if op.Reference.SourceBorrow == 0 || op.Reference.SourceBorrow == op.Reference.Borrow {
			return fmt.Errorf("%s derives a new borrow from its source borrow", op.Kind)
		}
	case OpReferenceIsValid:
		if len(op.Operands) != 1 || !directSafeReferenceType(types, firstOperandType(op, values)) || len(op.Results) != 1 || !typeHasKind(types, op.Results[0].Type, TypeBool) {
			return fmt.Errorf("invalid ref.is-valid")
		}
		if op.Reference == nil || op.Reference.Validity != ReferenceDynamicEpoch {
			return fmt.Errorf("ref.is-valid applies only to dynamic-epoch references")
		}
		return verifyReferenceMetadata(op)
	case OpReferenceGenerationFailure:
		if len(op.Operands) != 0 || len(op.Results) != 0 || len(op.Successors) != 0 || op.PanicReasonID != invalidReferenceGenerationID || op.PanicReason != invalidReferenceGenerationName {
			return fmt.Errorf("invalid fail.reference-generation: it terminates with panic.invalid-reference-generation")
		}
	case OpReferenceEndBorrow:
		if len(op.Operands) != 1 || !directSafeReferenceType(types, firstOperandType(op, values)) || len(op.Results) != 0 {
			return fmt.Errorf("invalid ref.end-borrow")
		}
		return verifyReferenceMetadata(op)
	case OpReferenceCompare:
		left, leftOK := referenceType(types, firstOperandType(op, values))
		if len(op.Operands) != 2 || !leftOK || len(op.Results) != 1 || !typeHasKind(types, op.Results[0].Type, TypeBool) || (op.Operator != "eq" && op.Operator != "ne") {
			return fmt.Errorf("invalid ref.compare")
		}
		right, rightOK := referenceType(types, operandType(1))
		if !rightOK || right.Element != left.Element {
			return fmt.Errorf("ref.compare requires compatible direct references")
		}
	}
	return nil
}

func firstOperandType(op Operation, values map[ValueID]Value) TypeID {
	if len(op.Operands) == 0 {
		return 0
	}
	return values[op.Operands[0]].Type
}

// verifyPlaceReferenceFunction checks the function-level P15 rules: no Place
// escapes as a source value, call argument, stored value, aggregate element,
// union payload, branch argument or return; every use of a dynamic-epoch
// reference is guarded by its validity predicate whose false edge ends in
// fail.reference-generation; and no use of a reference is reachable after
// its resolved borrow end.
// Slice length/index/reborrow observations share this temporal gate. Slice
// construction also retains its exact source extent and reference facts.
// Rules: rules/mlir/packages/sec-mlir-dialect_package15.md — §§59–63;
// rules/mlir/packages/sec-mlir-dialect_package16.md — §§42–58,78–84.
func verifyPlaceReferenceFunction(module *Module, fn *Function, blocks map[BlockID]*Block, values map[ValueID]Value, dom map[BlockID]map[BlockID]bool) error {
	types := module.Types
	sliceFn, sliceBlocks, aliases := sliceVerificationView(types, fn, blocks)
	isPlace := func(id ValueID) bool {
		_, ok := placeType(types, values[id].Type)
		return ok
	}
	dynamic := map[ValueID]bool{}
	validity := map[ValueID]ValueID{}
	for _, parameter := range fn.Parameters {
		if parameter.Reference != nil && parameter.Reference.Validity == ReferenceDynamicEpoch {
			dynamic[parameter.Value.ID] = true
		}
	}
	definitions := map[ValueID]Operation{}
	definitionBlocks := map[ValueID]BlockID{}
	sources := map[ValueID]*ReferenceMetadata{}
	sliceDefinitions := map[ValueID]Operation{}
	for _, parameter := range fn.Parameters {
		sources[parameter.Value.ID] = parameter.Reference
	}
	for _, b := range fn.Blocks {
		for _, op := range b.Operations {
			for _, result := range op.Results {
				definitions[result.ID] = op
				definitionBlocks[result.ID] = b.ID
				sources[result.ID] = op.Reference
			}
		}
	}
	for _, block := range sliceFn.Blocks {
		for _, op := range block.Operations {
			for _, result := range op.Results {
				sliceDefinitions[result.ID] = op
			}
		}
	}
	resolveForwardedSliceFacts(types, fn, blocks, sources)
	for id, facts := range sources {
		if facts != nil && facts.Validity == ReferenceDynamicEpoch && directSafeReferenceType(types, values[id].Type) {
			dynamic[id] = true
		}
	}
	for _, b := range fn.Blocks {
		for index, op := range b.Operations {
			if err := verifySliceElementReferenceFacts(op, definitions); err != nil {
				return err
			}
			if construction, _, _ := sliceConstructionKind(op.Kind); construction {
				if err := verifySliceBorrowSource(types, sliceBlocks[b.ID].Operations[index], values, sliceDefinitions, sources); err != nil {
					return err
				}
			}
			sliceSource, _ := types.Lookup(firstOperandType(op, values))
			if op.Kind == OpSliceElementPlace || op.Kind == OpSliceCopyShared || op.Kind == OpSliceMoveMutable || op.Kind == OpReferenceEndBorrow && (sliceSource.Kind == TypeSharedSlice || sliceSource.Kind == TypeMutableSlice) {
				if err := verifySliceHolderFacts(op, sources[op.Operands[0]]); err != nil {
					return err
				}
			}
			if (op.Kind == OpReferenceReborrowShared || op.Kind == OpReferenceReborrowMutable) && len(op.Operands) == 1 && isPlace(op.Operands[0]) {
				if err := verifySubplaceReborrowSource(op, definitions, sources); err != nil {
					return err
				}
			} else if !placeConsumer(op.Kind) {
				for _, operand := range op.Operands {
					if isPlace(operand) {
						return fmt.Errorf("Place %%%d escapes through %s", operand, op.Kind)
					}
				}
			}
			for _, successor := range op.Successors {
				for _, argument := range successor.Arguments {
					if isPlace(argument) {
						return fmt.Errorf("Place %%%d escapes as a block argument", argument)
					}
				}
			}
			if op.Reference != nil && op.Reference.Validity == ReferenceDynamicEpoch && len(op.Results) == 1 {
				if directSafeReferenceType(types, op.Results[0].Type) {
					dynamic[op.Results[0].ID] = true
				}
			}
			if op.Kind == OpReferenceIsValid {
				validity[op.Results[0].ID] = op.Operands[0]
			}
		}
	}
	for _, b := range fn.Blocks {
		for index, op := range b.Operations {
			_, reborrow, _ := sliceConstructionKind(op.Kind)
			if op.Kind != OpReferenceDerefPlace && op.Kind != OpReferenceCompare && op.Kind != OpSliceLength && op.Kind != OpSliceIndexInBounds && op.Kind != OpSliceElementPlace && !reborrow {
				continue
			}
			sliceObservation := op.Kind == OpSliceLength || op.Kind == OpSliceIndexInBounds || op.Kind == OpSliceElementPlace || reborrow
			if sliceObservation {
				op = sliceBlocks[b.ID].Operations[index]
			}
			for _, operand := range op.Operands {
				if !dynamic[operand] {
					continue
				}
				guarded := validity[op.ReferenceGuard]
				if sliceObservation {
					guarded = forwardedValue(guarded, aliases)
				}
				if op.ReferenceGuard == 0 || guarded != operand {
					return fmt.Errorf("%s of dynamic-epoch reference %%%d requires its ref.is-valid guard", op.Kind, operand)
				}
				guardFn, guardBlocks := fn, blocks
				if sliceObservation {
					guardFn, guardBlocks = sliceFn, sliceBlocks
				}
				if !guardFailsToGenerationEndpoint(guardFn, guardBlocks, op.ReferenceGuard) {
					return fmt.Errorf("the false edge of validity guard %%%d must terminate with fail.reference-generation", op.ReferenceGuard)
				}
			}
			if sliceObservation {
				if err := verifySliceObservationValidity(op, b.ID, sliceBlocks, dom, sliceDefinitions, definitionBlocks, sources); err != nil {
					return err
				}
			}
		}
	}
	return verifyNoUseAfterEndBorrow(fn, blocks, values, types)
}

// verifySubplaceReborrowSource requires the Place a reborrow derives from to
// be reached from ref.deref-place of the reference its SourceBorrow names,
// through Place projections only; a Place over owned storage is a borrow,
// not a reborrow.
func verifySubplaceReborrowSource(op Operation, definitions map[ValueID]Operation, sources map[ValueID]*ReferenceMetadata) error {
	id := op.Operands[0]
	for {
		definition, ok := definitions[id]
		if !ok {
			return fmt.Errorf("%s of Place %%%d is not derived from a dereferenced reference", op.Kind, op.Operands[0])
		}
		switch definition.Kind {
		case OpPlaceField, OpPlaceArrayElement, OpPlaceUnionPayload:
			id = definition.Operands[0]
			continue
		case OpReferenceDerefPlace:
			source := sources[definition.Operands[0]]
			if source == nil || op.Reference == nil || source.Borrow != op.Reference.SourceBorrow {
				return fmt.Errorf("%s of Place %%%d does not derive from the reference its source borrow names", op.Kind, op.Operands[0])
			}
			return nil
		}
		return fmt.Errorf("%s of Place %%%d is not derived from a dereferenced reference", op.Kind, op.Operands[0])
	}
}

// guardFailsToGenerationEndpoint requires a conditional branch on guard whose
// false successor terminates with the invalid-reference-generation endpoint.
func guardFailsToGenerationEndpoint(fn *Function, blocks map[BlockID]*Block, guard ValueID) bool {
	for _, b := range fn.Blocks {
		for _, op := range b.Operations {
			if op.Kind != OpCondBranch || len(op.Operands) != 1 || op.Operands[0] != guard || len(op.Successors) != 2 {
				continue
			}
			failure := blocks[op.Successors[1].Block]
			if failure == nil || len(failure.Operations) == 0 {
				return false
			}
			return failure.Operations[len(failure.Operations)-1].Kind == OpReferenceGenerationFailure
		}
	}
	return false
}

func clonePlaceMetadata(place *PlaceMetadata) *PlaceMetadata {
	if place == nil {
		return nil
	}
	copy := *place
	return &copy
}

func cloneReferenceMetadata(ref *ReferenceMetadata) *ReferenceMetadata {
	if ref == nil {
		return nil
	}
	copy := *ref
	copy.OriginStorages = append([]uint32(nil), ref.OriginStorages...)
	return &copy
}

// variantPayloadType is the Place element of a variant payload: a single
// payload, the canonical Package 13 synthetic payload struct of a struct-like
// variant, or the Ok (0) / Err (1) payload of a Result (sec-mlir-dialect
// package 15 §§ 44, 71, 121).
func variantPayloadType(module *Module, container TypeID, index UnionVariantIndex) (TypeID, bool) {
	if typ, ok := module.Types.Lookup(container); ok && typ.Kind == TypeResult {
		switch index {
		case 0:
			return typ.Success, typ.Success != 0
		case 1:
			return typ.Error, typ.Error != 0
		}
		return 0, false
	}
	_, variant, found := unionVariant(module, container, index)
	if !found {
		return 0, false
	}
	switch variant.Kind {
	case UnionVariantSingle:
		return variant.Payload, variant.Payload != 0
	case UnionVariantFields:
		return variant.SyntheticPayloadStruct, variant.SyntheticPayloadStruct != 0
	}
	return 0, false
}
