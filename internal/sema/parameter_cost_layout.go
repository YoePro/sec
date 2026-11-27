package sema

import "sec/internal/layout"

// parameterCostModel answers by-value copy size from the canonical
// ResolvedLayout of the analyzer's plan. Size is cost evidence only: it never
// changes BorrowSufficient, ConsumptionRequired, ExactExtent or Retained, and
// it never comes from host widths.
// Rules: rules/analysis/parameter_usage_analysis.md — §§ 64, 87(2)-(3), 92;
// rules/memory/layout.md — § 47.5.
type parameterCostModel struct {
	resolver *layout.Resolver
}

// parameterCostModel binds one resolver to the analyzer's plan, created once
// per analysis; it is nil when no plan with verified scalar alignments exists.
// Rules: rules/analysis/parameter_usage_analysis.md — § 64(1).
func (a *Analyzer) parameterCostModel() *parameterCostModel {
	if a.costModel != nil {
		return a.costModel
	}
	plan := a.targetCapabilityPlan
	model := &parameterCostModel{}
	if plan.PointerWidthBits != 0 && plan.Alignments != nil {
		if resolver, err := layout.NewResolver(plan, plan.Alignments); err == nil {
			model.resolver = resolver
		}
	}
	a.costModel = model
	return model
}

// Copy-cost size sources.
const (
	ParameterSizeFromLayout   = "layout"
	ParameterSizeFromEstimate = "estimate"
)

// sizeOf is the copy cost of typ in bytes and its source: the resolved
// layout size under the plan when complete, otherwise the conservative
// structural estimate, which remains only for descriptor types (string,
// references, slices, raw pointers, function values) whose canonical layout
// P24 has not defined yet, and for analysis without a selected plan.
// Rules: rules/analysis/parameter_usage_analysis.md — § 64(1)-(4).
func (m *parameterCostModel) sizeOf(typ Type) (int64, bool) {
	size, known, _ := m.sizeWithSource(typ)
	return size, known
}

// sizeWithSource also names the size source, "layout" or "estimate".
// Rules: rules/analysis/parameter_usage_analysis.md — §§ 64(1), 87(2).
func (m *parameterCostModel) sizeWithSource(typ Type) (int64, bool, string) {
	if m != nil && m.resolver != nil {
		resolved := m.resolver.Resolve(LayoutRequirementOf(typ))
		if resolved.Complete() && resolved.SizeBytes != nil && resolved.SizeBytes.IsInt64() {
			return resolved.SizeBytes.Int64(), true, ParameterSizeFromLayout
		}
	}
	size, known := estimatedTypeSizeBytes(typ, map[string]bool{})
	return size, known, ParameterSizeFromEstimate
}

// estimatedTypeSizeBytes is the conservative structural fallback of sizeOf.
func estimatedTypeSizeBytes(typ Type, visiting map[string]bool) (int64, bool) {
	switch typ.Kind {
	case BoolType, CharType:
		return 1, true
	case RuneType:
		return 4, true
	case IntType, UintType, FloatType:
		return numericTypeSizeBytes(typ), true
	case DecimalType:
		return 16, true
	case EnumType, RegisterType:
		if typ.BitWidth > 0 {
			return maxParameterSize(1, (typ.BitWidth+7)/8), true
		}
		if typ.RegisterWidth > 0 {
			return maxParameterSize(1, (typ.RegisterWidth+7)/8), true
		}
		return 4, true
	case StringType, ReferenceType, RawPtrType, SliceType, FunctionType:
		return 16, true
	case ArrayType:
		length, ok := legacyArrayLength(typ)
		if typ.Element == nil || !ok || length < 0 {
			return 0, false
		}
		elementSize, ok := estimatedTypeSizeBytes(*typ.Element, visiting)
		if !ok || length != 0 && elementSize > int64(^uint64(0)>>1)/length {
			return 0, false
		}
		return elementSize * length, true
	case StructType:
		key := typeDisplayName(typ)
		if visiting[key] {
			return 0, false
		}
		visiting[key] = true
		var total int64
		for _, field := range typ.Fields {
			fieldSize, ok := estimatedTypeSizeBytes(field.Type, visiting)
			if !ok {
				delete(visiting, key)
				return 0, false
			}
			total += fieldSize
		}
		delete(visiting, key)
		return total, true
	case ResultType, UnionType:
		var maxPayload int64
		for _, argument := range typ.TypeArgs {
			if size, ok := estimatedTypeSizeBytes(argument, visiting); ok && size > maxPayload {
				maxPayload = size
			}
		}
		for _, variant := range typ.UnionVariants {
			if variant.Payload != nil {
				if size, ok := estimatedTypeSizeBytes(*variant.Payload, visiting); ok && size > maxPayload {
					maxPayload = size
				}
			}
			var fields int64
			fieldsOK := len(variant.PayloadFields) > 0
			for _, field := range variant.PayloadFields {
				size, ok := estimatedTypeSizeBytes(field.Type, visiting)
				if !ok {
					fieldsOK = false
					break
				}
				fields += size
			}
			if fieldsOK && fields > maxPayload {
				maxPayload = fields
			}
		}
		return 8 + maxPayload, true
	default:
		return 0, false
	}
}
