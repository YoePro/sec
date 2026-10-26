package sema

import (
	"strings"

	"sec/internal/ast"
	"sec/internal/lexer"
)

// matchFieldBinding is one shallow named-field binding of a struct-like union
// payload: `Variant { field }`, `{ field: name }`, `{ field: ref name }` or
// `{ field: ref mut name }`.
type matchFieldBinding struct {
	Field      string
	FieldIndex int
	Name       string
	// Type is the binding's type: the field type for a copy, ref/ref mut of
	// it for a borrow.
	Type        Type
	Borrow      bool
	Mutable     bool
	Node        *ast.Identifier
	BorrowToken lexer.Token
	// Place is the payload field Place a borrow is taken from.
	Place Place
}

// ResolvedMatchFieldBinding is the resolved form of one shallow named-field
// binding. Its identifier keys ResolvedBindingOf and, for a borrow, the
// resolved reference plan.
type ResolvedMatchFieldBinding struct {
	Field      string
	FieldIndex int
	Identifier *ast.Identifier
	Type       Type
	Action     ResolvedMatchBindingAction
}

// analyzeMatchFieldPattern resolves `Variant { ... }` against a struct-like
// union variant. Destructuring is shallow and partial; a by-value field
// binding copies and a move-only field must be borrowed or reached through
// the whole payload, so the subject is never left partially moved.
//
// Rules: rules/control-flow/flowcontrol_match.md — §§ 10, 12–15;
// rules/foundations/grammar.md — StructLikeUnionPattern.
func (a *Analyzer) analyzeMatchFieldPattern(subject ast.Expression, pattern *ast.MatchPattern, subjectType Type) (matchPatternInfo, bool) {
	variantName, variantToken := pattern.Name, pattern.NameToken
	if variantToken.Line == 0 {
		variantToken = pattern.Token
	}
	if separator := strings.LastIndex(variantName, "."); separator >= 0 {
		if subjectType.Kind != UnionType || a.resolveTypeName(variantName[:separator]) != subjectType.Name {
			a.addErrorAtToken(pattern.Token, "match pattern must match %s", typeDisplayName(subjectType))
			return matchPatternInfo{}, false
		}
		variantName = variantName[separator+1:]
	}
	if subjectType.Kind != UnionType {
		a.addErrorAtToken(pattern.Token, "named-field match pattern requires a union subject with a struct-like variant, got %s", typeDisplayName(subjectType))
		return matchPatternInfo{}, false
	}
	variant, ok := lookupUnionVariant(subjectType, variantName)
	if !ok {
		a.addErrorAtToken(variantToken, "unknown union variant %s.%s", typeDisplayName(subjectType), variantName)
		return matchPatternInfo{}, false
	}
	a.bindDefinition(variantToken, variant.Token)
	if len(variant.PayloadFields) == 0 {
		a.addErrorAtTokenWithMetadata(variantToken, "",
			"Bind a single payload with `"+variant.Name+"(value)`, or match a payload-less variant by its name alone.",
			"union variant %s.%s has no named fields to destructure", typeDisplayName(subjectType), variant.Name)
		return matchPatternInfo{}, false
	}
	info := matchPatternInfo{Kind: "variant", Variant: variant.Name, VariantIndex: unionVariantIndex(subjectType, variant.Name)}
	payloadType := Type{Name: typeDisplayName(subjectType) + "." + variant.Name, Kind: StructType, Fields: variant.PayloadFields}
	subjectPlace, hasPlace := a.resolvePlace(subject)
	seenFields, seenNames := map[string]bool{}, map[string]bool{}
	valid := true
	for _, field := range pattern.Fields {
		if field == nil || field.Field == nil || field.Binding == nil || field.Binding.Name == nil {
			valid = false
			continue
		}
		index := -1
		for candidate, declared := range variant.PayloadFields {
			if declared.Name == field.Field.Value {
				index = candidate
			}
		}
		if index < 0 {
			a.addErrorAtToken(field.Field.Token, "union variant %s.%s has no field %s", typeDisplayName(subjectType), variant.Name, field.Field.Value)
			valid = false
			continue
		}
		declared := variant.PayloadFields[index]
		binding := field.Binding
		shorthand := binding.Name.Token == field.Field.Token
		if !shorthand {
			a.bindDefinition(field.Field.Token, declared.Token)
		}
		if seenFields[declared.Name] {
			a.addErrorAtToken(field.Field.Token, "field %s is bound more than once in this pattern", declared.Name)
			valid = false
			continue
		}
		seenFields[declared.Name] = true
		if binding.Name.Value == "_" {
			a.addErrorAtTokenWithMetadata(binding.Name.Token, "", "Omit the field instead; omitted fields are neither bound nor copied.",
				"field pattern %s: _ is not a binding", declared.Name)
			valid = false
			continue
		}
		if seenNames[binding.Name.Value] {
			a.addErrorAtToken(binding.Name.Token, "name %s is bound more than once in this pattern", binding.Name.Value)
			valid = false
			continue
		}
		seenNames[binding.Name.Value] = true
		entry := matchFieldBinding{Field: declared.Name, FieldIndex: index, Name: binding.Name.Value, Type: declared.Type, Node: binding.Name}
		switch binding.Mode {
		case ast.MatchBindingSharedRef, ast.MatchBindingMutableRef:
			if !hasPlace {
				a.addErrorAtToken(binding.Token, "cannot borrow union payload field from temporary match subject")
				valid = false
				continue
			}
			entry.Borrow, entry.Mutable, entry.BorrowToken = true, binding.Mode == ast.MatchBindingMutableRef, binding.Token
			entry.Place = appendPlaceProjection(unionPayloadPlace(subjectPlace, variant.Name, payloadType, variantToken),
				PlaceProjection{Kind: PlaceField, Name: declared.Name, Token: field.Field.Token})
			entry.Place.Type = declared.Type
			fieldType := declared.Type
			reference := Type{Name: referenceTypeName(fieldType, entry.Mutable), Kind: ReferenceType, Element: &fieldType, ReferenceMutable: entry.Mutable}
			entry.Type = a.referenceTypeWithOriginFromExpression(reference, subject)
			entry.Type.ReferenceOriginMatchScoped = true
			entry.Type.ReferenceOriginToken = binding.Name.Token
		default:
			if requiresOwnershipTransfer(declared.Type) {
				a.addErrorAtTokenWithMetadata(binding.Name.Token, "",
					"Borrow it with `"+declared.Name+": ref "+binding.Name.Value+"`, or bind the whole payload with `"+variant.Name+"(payload)` to take ownership.",
					"field %s of %s.%s is move-only; a named-field binding only copies", declared.Name, typeDisplayName(subjectType), variant.Name)
				valid = false
				continue
			}
		}
		info.FieldBindings = append(info.FieldBindings, entry)
	}
	return info, valid
}

// defineMatchFieldBindings declares an arm's named-field bindings and records
// the borrow, holder origin and reference plan of each borrowed field.
func (a *Analyzer) defineMatchFieldBindings(info matchPatternInfo) {
	for _, field := range info.FieldBindings {
		token := field.Node.Token
		if !a.defineSymbol(field.Name, field.Type, false, token) {
			continue
		}
		a.bindDefinition(token, token)
		a.assigned[field.Name] = true
		a.clearRootPlaceState(field.Name)
		delete(a.constInts, field.Name)
		if !field.Borrow || field.Place.Root == "" || field.Type.Kind == InvalidType {
			continue
		}
		if a.checkBorrowCreationPlace(field.Place, field.Mutable, field.BorrowToken) {
			continue
		}
		kind := sharedBorrow
		if field.Mutable {
			kind = mutableBorrow
		}
		a.borrows[field.Place.Root] = append(a.borrows[field.Place.Root], borrowRecord{
			Root: field.Place.Root, Place: field.Place, Holder: field.Name, Kind: kind, Token: field.BorrowToken,
		})
		a.localRefContainers[field.Name] = localReferenceOrigin{
			Name: field.Type.ReferenceOriginName, Token: field.Type.ReferenceOriginToken,
			Local: field.Type.ReferenceOriginLocal, MatchScoped: true,
			Place: clonePlace(field.Place), HasPlace: true,
		}
		a.recordReferencePlan(field.Node, field.Place, field.Mutable, false, field.BorrowToken)
	}
}

// resolvedMatchFieldBindings publishes the arm's named-field bindings.
func resolvedMatchFieldBindings(info matchPatternInfo) []ResolvedMatchFieldBinding {
	var resolved []ResolvedMatchFieldBinding
	for _, field := range info.FieldBindings {
		action := MatchBindingCopyTrivial
		switch {
		case field.Mutable:
			action = MatchBindingBorrowMutable
		case field.Borrow:
			action = MatchBindingBorrowShared
		default:
			switch CopyClassificationOf(field.Type) {
			case CopySemantic:
				action = MatchBindingCopySemantic
			case CopyConditional:
				action = MatchBindingConditional
			}
		}
		resolved = append(resolved, ResolvedMatchFieldBinding{Field: field.Field, FieldIndex: field.FieldIndex, Identifier: field.Node, Type: field.Type, Action: action})
	}
	return resolved
}

// matchBindingNames lists every name an arm binds.
func (info matchPatternInfo) matchBindingNames() []string {
	var names []string
	if info.BindingName != "" {
		names = append(names, info.BindingName)
	}
	for _, field := range info.FieldBindings {
		names = append(names, field.Name)
	}
	return names
}
