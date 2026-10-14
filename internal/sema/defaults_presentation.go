package sema

import (
	"math/big"
	"strings"
)

// DefaultValueDisplay renders the default selected by the canonical resolver.
// Rules: rules/types/default_values.md — "LSP and tooling", A.11.
func DefaultValueDisplay(typ Type) (string, DefaultKind, bool) {
	resolution := DefaultValueOf(typ)
	if resolution.Kind == NoDefault {
		return "", NoDefault, false
	}
	return defaultResolutionDisplay(typ, resolution), resolution.Kind, true
}

// DefaultValuePreview returns a bounded presentation of a semantic default.
// It avoids materializing every element solely to render large fixed arrays in
// interactive tooling. DefaultValueOf remains the exact construction model.
func DefaultValuePreview(typ Type, maxArrayElements int) (string, DefaultKind, bool) {
	if typ.Kind != ArrayType {
		return DefaultValueDisplay(typ)
	}
	if typ.Element == nil {
		return "", NoDefault, false
	}
	if arrayShapeOf(typ) == ArrayShapeDynamic {
		if !collectionDefaultSatisfiesContracts(typ, new(big.Int)) {
			return "", NoDefault, false
		}
		return "[]", ArrayDefault, true
	}
	length, ok := exactFixedArrayLength(typ)
	if !ok {
		return "", NoDefault, false
	}
	if !collectionDefaultSatisfiesContracts(typ, length) {
		return "", NoDefault, false
	}
	if length.Sign() == 0 {
		return "[]", ArrayDefault, true
	}

	element, _, ok := DefaultValuePreview(*typ.Element, maxArrayElements)
	if !ok {
		return "", NoDefault, false
	}
	if maxArrayElements < 1 {
		maxArrayElements = 1
	}
	if length.Cmp(big.NewInt(int64(maxArrayElements))) > 0 {
		return "[" + element + ", ...]", ArrayDefault, true
	}

	elements := make([]string, int(length.Int64()))
	for index := range elements {
		elements[index] = element
	}
	return "[" + strings.Join(elements, ", ") + "]", ArrayDefault, true
}

// defaultResolutionDisplay renders a resolved default without choosing a new one.
// Rules: rules/types/default_values.md — "LSP and tooling", A.7, A.11.
func defaultResolutionDisplay(typ Type, resolution DefaultResolution) string {
	switch resolution.Kind {
	case CollectionDefault:
		return typeDisplayName(typ) + " {}"
	case PrimitiveDefault, NamedDefault, RangeDefault, MembershipDefault, ExplicitTypeDefault:
		if resolution.Value.Kind == EnumType || resolution.Value.Kind == UnionType {
			owner := typeDisplayName(typ)
			if typ.ConstantBaseName != "" && len(typ.GenericParameters) == 0 {
				owner = typ.Name
			}
			return owner + "." + resolution.Value.String
		}
		return resolution.Value.Lexeme
	case EnumDefault:
		return typ.Name + "." + resolution.Value.Lexeme
	case UnionDefault:
		if resolution.Payload != nil {
			return typ.Name + "." + resolution.Variant + "(" + defaultResolutionDisplay(unionDefaultPayloadType(typ, resolution.Variant), *resolution.Payload) + ")"
		}
		if len(resolution.Fields) > 0 {
			variant, _ := unionVariantByName(typ, resolution.Variant)
			parts := make([]string, 0, len(resolution.Fields))
			for _, field := range resolution.Fields {
				fieldType, ok := unionPayloadFieldType(variant, field.Name)
				if ok {
					parts = append(parts, field.Name+": "+defaultResolutionDisplay(fieldType, field.Value))
				}
			}
			return typ.Name + "." + resolution.Variant + " { " + strings.Join(parts, ", ") + " }"
		}
		return typ.Name + "." + resolution.Variant
	case StructDefault:
		parts := make([]string, 0, len(resolution.Fields))
		for _, field := range resolution.Fields {
			fieldType, ok := lookupStructField(typ, field.Name)
			if !ok {
				continue
			}
			parts = append(parts, field.Name+": "+defaultResolutionDisplay(fieldType, field.Value))
		}
		return typ.Name + " { " + strings.Join(parts, ", ") + " }"
	case ArrayDefault:
		if len(resolution.Elements) == 0 && resolution.ArrayElementDefault != nil {
			return "[" + defaultResolutionDisplay(*typ.Element, *resolution.ArrayElementDefault) + ", ...]"
		}
		parts := make([]string, 0, len(resolution.Elements))
		if typ.Element != nil {
			for _, element := range resolution.Elements {
				parts = append(parts, defaultResolutionDisplay(*typ.Element, element))
			}
		}
		return "[" + strings.Join(parts, ", ") + "]"
	default:
		return ""
	}
}

// ResolvedDefaultValueDisplay renders a previously selected DefaultValueOf fact.
// This is presentation only: it never chooses a default or constructs a value.
// Rules: rules/types/default_values.md — "LSP and tooling", A.7, A.11;
// rules/declarations/struct.md — §6 omitted fields.
func ResolvedDefaultValueDisplay(typ Type, value DefaultResolution) (string, bool) {
	if value.Kind == NoDefault || value.Kind == "" {
		return "", false
	}
	text := defaultResolutionDisplay(typ, value)
	return text, text != ""
}
