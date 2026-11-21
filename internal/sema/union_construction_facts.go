package sema

import "sec/internal/ast"

type ResolvedUnionVariantKind string

const (
	ResolvedUnionVariantEmpty  ResolvedUnionVariantKind = "empty"
	ResolvedUnionVariantSingle ResolvedUnionVariantKind = "single"
	ResolvedUnionVariantFields ResolvedUnionVariantKind = "fields"
)

type ResolvedUnionConstruction struct {
	UnionType        Type
	VariantName      string
	VariantIndex     uint32
	Kind             ResolvedUnionVariantKind
	CanonicalFields  []string
	SourceFieldOrder []string
	// PayloadActions is a compatibility view of the frozen operand actions.
	// New consumers use PayloadPreparation as the single typed authority.
	PayloadActions []ResolvedStructFieldAction
	// PayloadPreparation retains operands in evaluation order, their exact
	// destination types and declaration positions, including synthesized defaults.
	PayloadPreparation []ResolvedUnionPayloadPreparation
}

// ResolvedUnionPayloadPreparation identifies one typed operand and the transfer
// validated by Sema. Source is compiler-owned AST identity; mutable fact data
// is detached at query boundaries. FieldIndex is the declared field position,
// or zero for an unnamed payload. Defaulted marks a synthesized omitted field.
// Rules: rules/declarations/unions.md §§5.2–5.4,7.8;
// rules/compiler/semantic_ir.md §§23,35(6).
type ResolvedUnionPayloadPreparation struct {
	Source     ast.Expression
	Type       Type
	FieldName  string
	FieldIndex int
	Defaulted  bool
	// Default is the validated semantic default, never reconstructed from Source.
	Default  *DefaultResolution
	Action   ResolvedStructFieldAction
	Transfer *ResolvedTransferPlan
}

// recordUnionConstruction records only the constructor selected by validation;
// later consumers never infer a constructor from a function/variable's name.
// Transfer decisions are frozen after terminal-return ownership has committed.
// Rules: rules/declarations/unions.md §§4–5,7.8;
// rules/compiler/semantic_ir.md §§23,35–36.
func (a *Analyzer) recordUnionConstruction(expr ast.Expression, typ Type, variant UnionVariant) {
	if len(typ.GenericParameters) != 0 {
		return
	}
	plan := ResolvedUnionConstruction{UnionType: typ, VariantName: variant.Name}
	for i, candidate := range typ.UnionVariants {
		if candidate.Name == variant.Name {
			plan.VariantIndex = uint32(i)
			break
		}
	}
	switch {
	case variant.Payload != nil:
		call, ok := expr.(*ast.CallExpression)
		if !ok || len(call.Arguments) != 1 {
			return
		}
		plan.Kind = ResolvedUnionVariantSingle
		plan.PayloadPreparation = []ResolvedUnionPayloadPreparation{{Source: call.Arguments[0], Type: *variant.Payload}}
	case len(variant.PayloadFields) != 0:
		literal, ok := expr.(*ast.StructLiteral)
		if !ok {
			return
		}
		plan.Kind = ResolvedUnionVariantFields
		for _, field := range variant.PayloadFields {
			plan.CanonicalFields = append(plan.CanonicalFields, field.Name)
		}
		for _, field := range literal.Fields {
			if field == nil || field.Name == nil || field.Spread {
				return
			}
			index := -1
			for j, declared := range variant.PayloadFields {
				if declared.Name == field.Name.Value {
					index = j
					break
				}
			}
			if index < 0 {
				return
			}
			plan.SourceFieldOrder = append(plan.SourceFieldOrder, field.Name.Value)
			operand := ResolvedUnionPayloadPreparation{
				Source: field.Value, Type: variant.PayloadFields[index].Type,
				FieldName: field.Name.Value, FieldIndex: index, Defaulted: field.SynthesizedDefault,
			}
			if operand.Defaulted {
				resolution := DefaultValueOf(operand.Type)
				operand.Default = &resolution
			}
			plan.PayloadPreparation = append(plan.PayloadPreparation, operand)
		}
	default:
		plan.Kind = ResolvedUnionVariantEmpty
	}
	if a.resolvedUnionConstructions == nil {
		a.resolvedUnionConstructions = map[ast.Expression]ResolvedUnionConstruction{}
	}
	a.resolvedUnionConstructions[expr] = cloneUnionConstruction(plan)
	a.recordUnionReferenceOrigins(expr, plan)
}

// freezeUnionConstructionPreparations consumes accepted ownership transfers
// after all construction/terminal-return commits. A rejected reusable move-only
// operand has no transfer and cannot publish a successful preparation plan.
// Rules: rules/declarations/unions.md §§5,7.8;
// rules/memory/ownership.md §17; rules/compiler/semantic_ir.md §§35–36.
func (a *Analyzer) freezeUnionConstructionPreparations() {
	for expr, plan := range a.resolvedUnionConstructions {
		valid := true
		plan.PayloadActions = nil
		for i := range plan.PayloadPreparation {
			operand := &plan.PayloadPreparation[i]
			source := operand.Source
			if move, explicit := explicitMoveArgument(source); explicit {
				source = move.Right
			}
			if transfer, ok := a.resolvedTransferPlans[source]; ok {
				operand.Transfer = &transfer
			} else if place, reusable := a.ResolvedPlaceOf(source); reusable && requiresOwnershipTransfer(place.Type) {
				valid = false
				break
			}
			operand.Action = a.constructionOperandAction(operand.Source, operand.Type)
			plan.PayloadActions = append(plan.PayloadActions, operand.Action)
		}
		if !valid {
			delete(a.resolvedUnionConstructions, expr)
			continue
		}
		a.resolvedUnionConstructions[expr] = cloneUnionConstruction(plan)
	}
}

// cloneUnionConstruction detaches all type/Place and slice data while retaining
// compiler-owned operand node identity, matching other resolved-plan queries.
// Rules: rules/compiler/compiler_analysis.md §55 "Analysis snapshot consistency".
func cloneUnionConstruction(plan ResolvedUnionConstruction) ResolvedUnionConstruction {
	plan.UnionType = unionPreparationTypeSnapshot(plan.UnionType, map[string]bool{})
	plan.CanonicalFields = append([]string(nil), plan.CanonicalFields...)
	plan.SourceFieldOrder = append([]string(nil), plan.SourceFieldOrder...)
	plan.PayloadActions = append([]ResolvedStructFieldAction(nil), plan.PayloadActions...)
	plan.PayloadPreparation = append([]ResolvedUnionPayloadPreparation(nil), plan.PayloadPreparation...)
	for i := range plan.PayloadPreparation {
		operand := &plan.PayloadPreparation[i]
		operand.Type = unionPreparationTypeSnapshot(operand.Type, map[string]bool{})
		if operand.Default != nil {
			resolution := cloneUnionPayloadDefault(*operand.Default)
			operand.Default = &resolution
		}
		if operand.Transfer != nil {
			transfer := *operand.Transfer
			transfer.Source = unionPreparationPlaceSnapshot(transfer.Source)
			if transfer.Destination.Place != nil {
				place := unionPreparationPlaceSnapshot(*transfer.Destination.Place)
				transfer.Destination.Place = &place
			}
			operand.Transfer = &transfer
		}
	}
	return plan
}

// ResolvedUnionConstructionOf returns the saved, validated constructor decision
// without revisiting AST shape, reclassifying ownership or mutating analysis.
// Rules: rules/compiler/semantic_ir.md §§23,35(6);
// rules/compiler/compiler_analysis.md §55 "Analysis snapshot consistency".
func (a *Analyzer) ResolvedUnionConstructionOf(expr ast.Expression) (ResolvedUnionConstruction, bool) {
	if a == nil || expr == nil {
		return ResolvedUnionConstruction{}, false
	}
	plan, ok := a.resolvedUnionConstructions[expr]
	if !ok {
		return ResolvedUnionConstruction{}, false
	}
	return cloneUnionConstruction(plan), true
}

// unionPreparationTypeSnapshot retains exact scalar domains and structural
// payload identity without sharing declaration slices. Recursive nominal edges
// retain their exact type surface rather than recursively copying forever.
// Rules: rules/compiler/semantic_ir.md §§23,35;
// rules/compiler/compiler_analysis.md §55 "Analysis snapshot consistency".
func unionPreparationTypeSnapshot(typ Type, visiting map[string]bool) Type {
	result := enumConversionTypeSnapshot(typ)
	key := canonicalTypeIdentity(typ)
	if visiting[key] {
		return result
	}
	visiting[key] = true
	defer delete(visiting, key)
	if typ.Element != nil {
		element := unionPreparationTypeSnapshot(*typ.Element, visiting)
		result.Element = &element
	}
	for i, argument := range typ.TypeArgs {
		result.TypeArgs[i] = unionPreparationTypeSnapshot(argument, visiting)
	}
	result.Fields = append([]StructField(nil), typ.Fields...)
	for i, field := range typ.Fields {
		result.Fields[i].Type = unionPreparationTypeSnapshot(field.Type, visiting)
		result.Fields[i].Tags = append([]StructTag(nil), field.Tags...)
	}
	result.RegisterWidth = typ.RegisterWidth
	result.RegisterAllocationOrder = typ.RegisterAllocationOrder
	result.RegisterByteOrder = typ.RegisterByteOrder
	result.RegisterShadow = typ.RegisterShadow
	result.RegisterFields = append([]RegisterField(nil), typ.RegisterFields...)
	for i, field := range typ.RegisterFields {
		result.RegisterFields[i].Type = unionPreparationTypeSnapshot(field.Type, visiting)
	}
	result.FunctionEnvironmentMoveOnly = typ.FunctionEnvironmentMoveOnly
	result.UnionDefault = typ.UnionDefault
	result.UnionVariants = append([]UnionVariant(nil), typ.UnionVariants...)
	for i, variant := range typ.UnionVariants {
		if variant.Payload != nil {
			payload := unionPreparationTypeSnapshot(*variant.Payload, visiting)
			result.UnionVariants[i].Payload = &payload
		}
		fields := append([]StructField(nil), variant.PayloadFields...)
		for j, field := range fields {
			fields[j].Type = unionPreparationTypeSnapshot(field.Type, visiting)
			fields[j].Tags = append([]StructTag(nil), field.Tags...)
		}
		result.UnionVariants[i].PayloadFields = fields
	}
	return result
}

// unionPreparationPlaceSnapshot detaches exact operand/destination Place types,
// indexes and alternatives without borrowing mutable storage from transfer facts.
// Rules: rules/compiler/semantic_ir.md §§10,35;
// rules/compiler/compiler_analysis.md §55 "Analysis snapshot consistency".
func unionPreparationPlaceSnapshot(place Place) Place {
	place.Type = unionPreparationTypeSnapshot(place.Type, map[string]bool{})
	place.Projections = clonePlaceProjections(place.Projections)
	for i := range place.Projections {
		place.Projections[i].IndexType = unionPreparationTypeSnapshot(place.Projections[i].IndexType, map[string]bool{})
	}
	place.AlternativeOrigins = append([]Place(nil), place.AlternativeOrigins...)
	for i, alternative := range place.AlternativeOrigins {
		place.AlternativeOrigins[i] = unionPreparationPlaceSnapshot(alternative)
	}
	return place
}

// cloneUnionPayloadDefault detaches exact default values and compact aggregate
// decisions before constructor facts cross the immutable query boundary.
// Rules: rules/declarations/unions.md §5.4;
// rules/compiler/compiler_analysis.md §55 "Analysis snapshot consistency".
func cloneUnionPayloadDefault(value DefaultResolution) DefaultResolution {
	value.Value = cloneStaticScalar(value.Value)
	value.Fields = append([]DefaultField(nil), value.Fields...)
	for i := range value.Fields {
		value.Fields[i].Value = cloneUnionPayloadDefault(value.Fields[i].Value)
	}
	value.Elements = append([]DefaultResolution(nil), value.Elements...)
	for i := range value.Elements {
		value.Elements[i] = cloneUnionPayloadDefault(value.Elements[i])
	}
	if value.ArrayElementDefault != nil {
		element := cloneUnionPayloadDefault(*value.ArrayElementDefault)
		value.ArrayElementDefault = &element
	}
	if value.Payload != nil {
		payload := cloneUnionPayloadDefault(*value.Payload)
		value.Payload = &payload
	}
	return value
}
