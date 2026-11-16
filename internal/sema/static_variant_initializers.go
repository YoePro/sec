package sema

import "math/big"

// ResolvedStaticVariant retains the canonical owning type, active member and
// payload type identity. A named-field variant has PayloadFields in declaration
// order and an aggregate Payload; no synthetic nominal payload type is invented.
// Payload-free members (None and Ok(void) included) have no Payload.
// Rules: rules/compiler/compile_time_evaluation.md — §32(1–4);
// rules/declarations/unions.md — §5; rules/declarations/static.md — §§15–16.
type ResolvedStaticVariant struct {
	Owner         Type
	Name          string
	PayloadType   *Type
	PayloadFields []StructField
	Payload       *ResolvedStaticInitializer
}

// staticVariantInitializerOf consumes a materialized CTE value and its concrete
// declaration, preserving only the active payload. Unknown members and missing
// or unsupported payloads publish no initializer; no runtime startup is added.
// Rules: rules/compiler/compile_time_evaluation.md — §§32,36,47;
// rules/declarations/static.md — §§15–16.
func staticVariantInitializerOf(typ Type, value DefaultConstant) (ResolvedStaticInitializer, bool) {
	name := value.Nominal.Member
	if value.Aggregate != nil {
		if value.Aggregate.Type.Kind != "" && canonicalTypeIdentity(value.Aggregate.Type) != canonicalTypeIdentity(typ) {
			return ResolvedStaticInitializer{}, false
		}
		name = value.Aggregate.Variant
	} else if value.Nominal.Owner != nominalConstantOwner(typ) {
		return ResolvedStaticInitializer{}, false
	}
	payloadType, fields, known := cteMaterializedVariant(typ, name)
	if !known {
		return ResolvedStaticInitializer{}, false
	}
	variant := &ResolvedStaticVariant{Owner: snapshotStaticVariantType(typ), Name: name}
	if payloadType != nil {
		payload := snapshotStaticVariantType(*payloadType)
		variant.PayloadType = &payload
	}
	if fields != nil || payloadType != nil && payloadType.Kind != VoidType {
		if value.Aggregate == nil || value.Aggregate.Payload == nil {
			return ResolvedStaticInitializer{}, false
		}
		var payload ResolvedStaticInitializer
		var ok bool
		if fields != nil {
			carrier := Type{Kind: StructType, Fields: fields}
			variant.PayloadFields = snapshotStaticVariantType(carrier).Fields
			payload, ok = staticInitializerOf(carrier, *value.Aggregate.Payload)
		} else {
			payload, ok = staticInitializerOf(*payloadType, *value.Aggregate.Payload)
		}
		if !ok {
			return ResolvedStaticInitializer{}, false
		}
		variant.Payload = &payload
	} else if value.Aggregate != nil && value.Aggregate.Payload != nil && value.Aggregate.Payload.Kind != VoidType {
		return ResolvedStaticInitializer{}, false
	}
	return ResolvedStaticInitializer{Variant: variant}, true
}

// cloneStaticVariant detaches recursive payload facts and canonical type data.
// Rules: rules/compiler/compile_time_evaluation.md — §32(3);
// rules/compiler/compiler_pipeline.md — immutable resolved facts.
func cloneStaticVariant(variant *ResolvedStaticVariant) *ResolvedStaticVariant {
	if variant == nil {
		return nil
	}
	copy := *variant
	copy.Owner = snapshotStaticVariantType(copy.Owner)
	if copy.PayloadType != nil {
		payloadType := snapshotStaticVariantType(*copy.PayloadType)
		copy.PayloadType = &payloadType
	}
	copy.PayloadFields = snapshotStaticVariantType(Type{Kind: StructType, Fields: copy.PayloadFields}).Fields
	if copy.Payload != nil {
		payload := cloneStaticInitializer(*copy.Payload)
		copy.Payload = &payload
	}
	return &copy
}

// cloneStaticScalar detaches exact scalar values; aggregates are represented by
// initializer subtrees instead. Nominal enum identities and text are immutable.
// Rules: rules/compiler/compile_time_evaluation.md — §§31–32,36.
func cloneStaticScalar(value DefaultConstant) DefaultConstant {
	value.Integer = clonePlaceConstantIndex(value.Integer)
	if value.Exact != nil {
		value.Exact = new(big.Rat).Set(value.Exact)
	}
	return value
}

// snapshotStaticVariantType copies the finite semantic type snapshot, including
// exact enum constants reachable through nested payload and field types.
// Rules: rules/compiler/compile_time_evaluation.md — §32(3);
// rules/compiler/compiler_pipeline.md — immutable resolved facts.
func snapshotStaticVariantType(typ Type) Type {
	copy := semanticSnapshotType(typ)
	detachStaticVariantEnumConstants(&copy)
	return copy
}

// detachStaticVariantEnumConstants finishes detachment of mutable enum values
// in an already finite structural snapshot; it performs no resolution.
// Rules: rules/compiler/compile_time_evaluation.md — §32(3).
func detachStaticVariantEnumConstants(typ *Type) {
	for name, value := range typ.EnumConsts {
		value.Value = clonePlaceConstantIndex(value.Value)
		if value.StringValue != nil {
			text := *value.StringValue
			value.StringValue = &text
		}
		typ.EnumConsts[name] = value
	}
	if typ.Element != nil {
		detachStaticVariantEnumConstants(typ.Element)
	}
	for i := range typ.TypeArgs {
		detachStaticVariantEnumConstants(&typ.TypeArgs[i])
	}
	for i := range typ.Fields {
		detachStaticVariantEnumConstants(&typ.Fields[i].Type)
	}
	for i := range typ.UnionVariants {
		variant := &typ.UnionVariants[i]
		if variant.Payload != nil {
			detachStaticVariantEnumConstants(variant.Payload)
		}
		for j := range variant.PayloadFields {
			detachStaticVariantEnumConstants(&variant.PayloadFields[j].Type)
		}
	}
}
