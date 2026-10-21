package sema

import (
	"sort"
)

// DestructionPlanID is the deterministic identity of one concrete type's
// destruction plan: derived from the canonical type identity, never from a
// source spelling, field offset or physical ABI (P17-11).
type DestructionPlanID string

// DestructionPlanKind classifies how ending an owned value's lifetime is
// discharged. Trivial plans emit no action; unsupported plans name a boundary
// owned by another package instead of pretending destruction is trivial.
type DestructionPlanKind string

const (
	DestructionTrivial     DestructionPlanKind = "trivial"
	DestructionStruct      DestructionPlanKind = "struct"
	DestructionFixedArray  DestructionPlanKind = "fixed-array"
	DestructionVariant     DestructionPlanKind = "active-variant"
	DestructionResource    DestructionPlanKind = "compiler-known-resource"
	DestructionUnsupported DestructionPlanKind = "unsupported"
)

// DestructionFieldStep destroys one remaining owned stored field. Steps are
// listed in execution order, i.e. reverse declaration order.
type DestructionFieldStep struct {
	Field string
	Index int
	Plan  DestructionPlanID
}

// DestructionPayloadStep destroys one owned payload part of a variant: the
// single payload (Field == "") or one named payload field.
type DestructionPayloadStep struct {
	Field string
	Index int
	Plan  DestructionPlanID
}

// DestructionVariantStep destroys the payload of one variant, only when it is
// the active variant and its payload is initialized and still owned.
type DestructionVariantStep struct {
	Variant string
	Index   int
	Payload []DestructionPayloadStep
}

// ResolvedDestructionPlan is the immutable destruction plan of one concrete
// type (P17-10, P17-16):
//
//   - struct: run the custom `free` body when CustomFree, then destroy the
//     remaining initialized owned fields in reverse declaration order;
//   - fixed array: destroy initialized still-owned elements in reverse index
//     order, compactly as one element plan and an exact count;
//   - active variant (union, Option, Result): destroy only the active,
//     initialized, unmoved payload;
//   - compiler-known resource: release through the resource's own contract;
//   - unsupported: a named boundary (owning dynamic arrays belong to P18,
//     generic templates have no concrete plan).
//
// Rules:
//   - rules/memory/destruction.md — §§ 3, 7.2, 7.3, 10, 15.4, 15.5
//   - rules/mlir/packages/sec-mlir-dialect_package17.md — §§ 9–14, 22
type ResolvedDestructionPlan struct {
	ID           DestructionPlanID
	TypeIdentity string
	Kind         DestructionPlanKind
	CustomFree   bool
	Fields       []DestructionFieldStep
	Element      DestructionPlanID
	ElementCount string
	// VariantFamily distinguishes the P17 § 20 active-union, active-result
	// and active-option kinds that share one representation here.
	VariantFamily string
	Variants      []DestructionVariantStep
	Resource      string
	Unsupported   string
}

// DestructionPlanSet is a detached, self-contained plan graph: Root and every
// plan it references by ID. Recursive types refer back to an ID already in
// the set instead of expanding infinitely.
type DestructionPlanSet struct {
	Root  DestructionPlanID
	Plans map[DestructionPlanID]ResolvedDestructionPlan
}

// Plan returns a detached copy of one plan in the set.
func (s DestructionPlanSet) Plan(id DestructionPlanID) (ResolvedDestructionPlan, bool) {
	plan, ok := s.Plans[id]
	if !ok {
		return ResolvedDestructionPlan{}, false
	}
	return plan.clone(), true
}

// IDs lists the set's plan identities in deterministic order.
func (s DestructionPlanSet) IDs() []DestructionPlanID {
	ids := make([]DestructionPlanID, 0, len(s.Plans))
	for id := range s.Plans {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	return ids
}

func (p ResolvedDestructionPlan) clone() ResolvedDestructionPlan {
	p.Fields = append([]DestructionFieldStep(nil), p.Fields...)
	variants := make([]DestructionVariantStep, len(p.Variants))
	for index, variant := range p.Variants {
		variant.Payload = append([]DestructionPayloadStep(nil), variant.Payload...)
		variants[index] = variant
	}
	if p.Variants == nil {
		variants = nil
	}
	p.Variants = variants
	return p
}

// destructionPlanIDOf derives the plan identity from the canonical type
// identity, so equal concrete types share one plan and the ID is stable
// across runs.
func destructionPlanIDOf(typ Type) DestructionPlanID {
	return DestructionPlanID("destroy:" + canonicalTypeIdentity(typ))
}

// ResolveDestructionPlans builds the detached destruction plan graph of a
// concrete type. It is pure: it neither consults nor mutates analysis state,
// so repeated queries return equal, independent sets (P17-14).
func ResolveDestructionPlans(typ Type) DestructionPlanSet {
	builder := destructionPlanBuilder{plans: map[DestructionPlanID]ResolvedDestructionPlan{}, inProgress: map[DestructionPlanID]bool{}}
	root := builder.plan(typ)
	return DestructionPlanSet{Root: root, Plans: builder.plans}
}

// ResolvedDestructionPlanOf is the Analyzer query for one concrete type's
// destruction plan graph.
func (a *Analyzer) ResolvedDestructionPlanOf(typ Type) DestructionPlanSet {
	return ResolveDestructionPlans(typ)
}

type destructionPlanBuilder struct {
	plans      map[DestructionPlanID]ResolvedDestructionPlan
	inProgress map[DestructionPlanID]bool
}

func (b *destructionPlanBuilder) plan(typ Type) DestructionPlanID {
	id := destructionPlanIDOf(typ)
	if _, done := b.plans[id]; done || b.inProgress[id] {
		return id
	}
	b.inProgress[id] = true
	defer delete(b.inProgress, id)
	plan := ResolvedDestructionPlan{ID: id, TypeIdentity: canonicalTypeIdentity(typ), CustomFree: typ.CustomFree}
	switch {
	case typ.Kind == GenericType:
		plan.Kind, plan.Unsupported = DestructionUnsupported, "generic template; only concrete instantiations have destruction plans"
	case typ.Kind == AnyType:
		plan.Kind, plan.Unsupported = DestructionUnsupported, "any has no concrete destruction plan"
	case typ.Kind == ArrayType && arrayShapeOf(typ) == ArrayShapeDynamic:
		plan.Kind, plan.Unsupported = DestructionUnsupported, "owning dynamic array destruction and backing release belong to P18"
	case typ.Kind == StructType && compilerKnownRequiresDestruction(typ):
		plan.Kind, plan.Resource = DestructionResource, typ.Name
	case typ.Kind == StructType && (typ.CustomFree || !triviallyDestructible(typ, map[string]bool{})):
		plan.Kind = DestructionStruct
		for index := len(typ.Fields) - 1; index >= 0; index-- {
			field := typ.Fields[index]
			if TriviallyDestructible(field.Type) {
				continue
			}
			plan.Fields = append(plan.Fields, DestructionFieldStep{Field: field.Name, Index: index, Plan: b.plan(field.Type)})
		}
	case typ.Kind == ArrayType && typ.Element != nil && !TriviallyDestructible(*typ.Element):
		plan.Kind = DestructionFixedArray
		plan.Element = b.plan(*typ.Element)
		if length, ok := exactFixedArrayLength(typ); ok {
			plan.ElementCount = length.String()
		}
	case typ.Kind == ResultType && !TriviallyDestructible(typ):
		plan.Kind, plan.VariantFamily = DestructionVariant, "result"
		for index, name := range []string{"Ok", "Err"} {
			if index >= len(typ.TypeArgs) || TriviallyDestructible(typ.TypeArgs[index]) {
				continue
			}
			plan.Variants = append(plan.Variants, DestructionVariantStep{Variant: name, Index: index,
				Payload: []DestructionPayloadStep{{Plan: b.plan(typ.TypeArgs[index])}}})
		}
	case typ.Kind == UnionType && (typ.CustomFree || !triviallyDestructible(typ, map[string]bool{})):
		plan.Kind, plan.VariantFamily = DestructionVariant, "union"
		if typ.Name == "Option" {
			plan.VariantFamily = "option"
		}
		for index, variant := range typ.UnionVariants {
			step := DestructionVariantStep{Variant: variant.Name, Index: index}
			if variant.Payload != nil && !TriviallyDestructible(*variant.Payload) {
				step.Payload = append(step.Payload, DestructionPayloadStep{Plan: b.plan(*variant.Payload)})
			}
			for fieldIndex := len(variant.PayloadFields) - 1; fieldIndex >= 0; fieldIndex-- {
				field := variant.PayloadFields[fieldIndex]
				if TriviallyDestructible(field.Type) {
					continue
				}
				step.Payload = append(step.Payload, DestructionPayloadStep{Field: field.Name, Index: fieldIndex, Plan: b.plan(field.Type)})
			}
			if len(step.Payload) > 0 {
				plan.Variants = append(plan.Variants, step)
			}
		}
	case TriviallyDestructible(typ):
		plan.Kind = DestructionTrivial
	default:
		plan.Kind, plan.Unsupported = DestructionUnsupported, "no destruction representation for type kind "+string(typ.Kind)
	}
	b.plans[id] = plan
	return id
}

// TransferAction is the ownership action of one value transfer (P17-08).
type TransferAction string

const (
	TransferCopyTrivial  TransferAction = "copy-trivial"
	TransferCopySemantic TransferAction = "copy-semantic"
	TransferMove         TransferAction = "move"
	TransferRejected     TransferAction = "rejected"
)

// ResolveTransferAction maps a concrete type's copy classification to the
// ownership action of a transfer. An implicit (copy-shaped) transfer copies
// only trivially or semantically copyable values; MoveOnly and @noCopy values
// need an explicit move, and a still-conditional classification is never
// treated as a trivial copy. An explicit move is legal for every concrete
// classification, since @noCopy forbids copying, not moving. The reason
// explains a rejection.
//
// Rules:
//   - rules/memory/copy_move.md — §§ 3.2–3.5
//   - rules/mlir/packages/sec-mlir-dialect_package17.md — §§ 5–7
func ResolveTransferAction(typ Type, explicitMove bool) (TransferAction, string) {
	classification := CopyClassificationOf(typ)
	if typ.Kind == InvalidType || typ.Kind == VoidType || typ.Kind == NeverType {
		return TransferRejected, "no value to transfer"
	}
	if explicitMove {
		if typ.Kind == GenericType {
			return TransferRejected, "generic template; resolve the concrete type first"
		}
		return TransferMove, ""
	}
	switch classification {
	case CopyTrivial:
		return TransferCopyTrivial, ""
	case CopySemantic:
		return TransferCopySemantic, ""
	case CopyMoveOnly:
		return TransferRejected, "move-only value requires an explicit move (<-)"
	case CopyNonCopyable:
		return TransferRejected, "@noCopy value requires an explicit move (<-)"
	default:
		return TransferRejected, "copyability is not resolved for this use; it is not a trivial copy"
	}
}
