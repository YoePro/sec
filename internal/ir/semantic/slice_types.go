package semantic

import (
	"fmt"
	"math/big"
)

// Slice kinds preserve shared-copy versus mutable-move semantics independently
// of direct element references and owning arrays. Runtime extent is value data.
// Rules: rules/mlir/packages/sec-mlir-dialect_package16.md — §§31–32,39–41,75–76;
// rules/mlir/semantic-ir/sec_semantic_ir_slice_v1.md — §§1–4.
const (
	TypeSharedSlice  TypeKind = "slice"
	TypeMutableSlice TypeKind = "slice-mut"
)

// SliceRangeID identifies one symbolic range evaluation; zero is absence.
// It carries no address, layout, stride or machine-width assumption.
// Rules: rules/mlir/packages/sec-mlir-dialect_package16.md — §§25–27.
type SliceRangeID uint32

// StaticSliceRange is the exact half-open range relative to a slice's source.
// Empty ranges retain their source reference facts and borrow obligations.
// Rules: rules/mlir/packages/sec-mlir-dialect_package16.md — §§14–16,25,30.
type StaticSliceRange struct{ Start, EndExclusive *big.Int }

// SliceFacts reuses P15 reference validity, borrow/lifetime, storage, address
// space and epoch metadata. OriginPlaces names semantic Place SSA values.
// RuntimeLength names a uint SSA value; it never participates in type identity.
// Guard dominance, SSA validity and source lowering are subsequent P16 work.
// Rules: rules/mlir/packages/sec-mlir-dialect_package16.md — §§31–32,44,47,76;
// rules/mlir/semantic-ir/sec_semantic_ir_slice_v1.md — §§2–4.
type SliceFacts struct {
	Element       TypeID
	Mutable       bool
	Reference     ReferenceMetadata
	OriginPlaces  []ValueID
	RuntimeLength ValueID
	StaticRange   *StaticSliceRange
	DynamicRange  SliceRangeID
}

// Clone returns detached semantic facts without creating a new borrow or
// changing any origin, validity proof or symbolic range identity.
// Rules: rules/mlir/packages/sec-mlir-dialect_package16.md — §§25,31,37;
// rules/mlir/semantic-ir/sec_semantic_ir_slice_v1.md — §2.
func (facts SliceFacts) Clone() SliceFacts {
	facts.OriginPlaces = append([]ValueID(nil), facts.OriginPlaces...)
	facts.Reference.OriginStorages = append([]uint32(nil), facts.Reference.OriginStorages...)
	if facts.StaticRange != nil {
		copied := &StaticSliceRange{}
		if facts.StaticRange.Start != nil {
			copied.Start = new(big.Int).Set(facts.StaticRange.Start)
		}
		if facts.StaticRange.EndExclusive != nil {
			copied.EndExclusive = new(big.Int).Set(facts.StaticRange.EndExclusive)
		}
		facts.StaticRange = copied
	}
	return facts
}

// InternSlice interns exactly (element, authority), never a runtime length or
// physical descriptor. Mutable slices have a distinct move-only type kind.
// Source builders must obtain element/authority from resolved Sema slice plans.
// Rules: rules/mlir/packages/sec-mlir-dialect_package16.md — §§31–32,39,76;
// rules/mlir/semantic-ir/sec_semantic_ir_slice_v1.md — §§1–3.
func (table *TypeTable) InternSlice(element TypeID, mutable bool) (TypeID, error) {
	kind := TypeSharedSlice
	if mutable {
		kind = TypeMutableSlice
	}
	typ := Type{Kind: kind, Element: element}
	if err := verifySliceType(table, 0, typ); err != nil {
		return 0, err
	}
	return table.Intern(typ), nil
}

// verifySliceType validates the canonical semantic type only. Spatial/temporal
// guards and ownership flow belong to later operation verification, not layout.
// Rules: rules/mlir/packages/sec-mlir-dialect_package16.md — §§31–32,75–76;
// rules/compiler/semantic_ir.md — §20 "Types".
func verifySliceType(table *TypeTable, id TypeID, typ Type) error {
	if typ.Kind != TypeSharedSlice && typ.Kind != TypeMutableSlice {
		return nil
	}
	element, ok := table.Lookup(typ.Element)
	if !ok || element.Kind == TypePlace || element.Kind == TypeVoid || element.Kind == TypeNever {
		return fmt.Errorf("slice !%d has invalid element type !%d", id, typ.Element)
	}
	if typeKey(typ) != typeKey(Type{Kind: typ.Kind, Element: typ.Element}) {
		return fmt.Errorf("slice !%d must be identified only by element type and authority", id)
	}
	return nil
}
