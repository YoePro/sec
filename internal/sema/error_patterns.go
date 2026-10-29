package sema

import (
	"sec/internal/ast"
	"sec/internal/lexer"
)

// MD-029 error patterns shared by Result match arms and try handlers:
// `Err(E.Variant(binding))`, `Err(E.Variant { fields })`, `Err(E.Variant {})`
// and the contextual `Err(.Variant...)` forms. Selection resolves the
// concrete error identity first, then the variant, and only then exposes the
// payload. Destructuring is one level deep; a by-value binding never silently
// moves a move-only payload out of a reusable Result.
//
// Rules:
//   - rules/errors/errorhandling.md — §§ 15.3, 20, 27.2b
//   - rules/foundations/grammar.md — "Try expression" (ErrPattern)
//   - rules/control-flow/flowcontrol_match.md — § 28 "Concrete error variants"
//   - rules/corrections/md029-md042-md102-pattern-reachability-some-correction-20261028.md — §§ 2–7

// errorSelection is the resolved variant of an MD-029 error pattern.
type errorSelection struct {
	// errorType is the concrete error type the variant belongs to.
	errorType Type
	variant   UnionVariant
	// enumCase is set for an enum error variant, which carries no payload.
	enumCase EnumValue
	isEnum   bool
	// open records narrowing from the open root `error` channel.
	open  bool
	name  string
	token lexer.Token
}

// isStructuredErrPattern reports an Err pattern that the MD-029 analysis
// owns: a contextual selection or one with a payload pattern.
func isStructuredErrPattern(pattern *ast.MatchPattern) bool {
	return pattern != nil && pattern.Name == "Err" && (pattern.ContextualErrorVariant != nil || pattern.ErrorPayload != nil)
}

// resolveErrorSelection resolves the variant an Err pattern selects from the
// failure channel. The contextual form needs one closed concrete error type
// supplied without looking up the variant name; it is never available for
// the open root `error` or a heterogeneous failure set.
func (a *Analyzer) resolveErrorSelection(pattern *ast.MatchPattern, channel Type, heterogeneous bool) (errorSelection, bool) {
	selection := errorSelection{}
	if contextual := pattern.ContextualErrorVariant; contextual != nil {
		selection.name, selection.token = contextual.Value, contextual.Token
		if channel.Kind == ErrorRootType || heterogeneous {
			a.addErrorAtTokenWithMetadata(contextual.Token, "",
				"Name the concrete error type, for example `Err(FileError."+contextual.Value+")`.",
				"Err(.%s) needs one closed concrete error type, but this failure channel is %s", contextual.Value, tryFailureChannelDisplay(channel, heterogeneous))
			return selection, false
		}
		selection.errorType = channel
	} else {
		member := pattern.ErrorVariant
		selection.name, selection.token = member.Property.Value, member.Property.Token
		owner, ok := typePathFromExpression(member.Object)
		if !ok {
			a.addErrorAtToken(member.Token, "Err(%s) must name ErrorType.Variant", member.String())
			return selection, false
		}
		ownerType, ok := a.types[a.resolveTypeName(owner)]
		if !ok {
			a.addErrorAtToken(member.Token, "unknown error type %s", owner)
			return selection, false
		}
		if definition, exists := a.typeDefinitionTokens[a.resolveTypeName(owner)]; exists {
			a.bindDefinition(expressionToken(member.Object), definition)
		}
		if channel.Kind == ErrorRootType {
			if !ownerType.ErrorAssignable {
				a.addErrorAtToken(member.Token, "Err(%s) must name a concrete error variant, got %s", member.String(), typeDisplayName(ownerType))
				return selection, false
			}
			selection.open = true
		} else if !sameConcreteType(ownerType, channel) {
			a.addErrorAtToken(member.Token, "Err(%s) must name a variant of %s", member.String(), typeDisplayName(channel))
			return selection, false
		}
		selection.errorType = ownerType
	}
	switch selection.errorType.Kind {
	case UnionType:
		variant, ok := lookupUnionVariant(selection.errorType, selection.name)
		if !ok {
			a.addErrorAtToken(selection.token, "%s has no variant %s", typeDisplayName(selection.errorType), selection.name)
			return selection, false
		}
		a.bindDefinition(selection.token, variant.Token)
		selection.variant = variant
	case EnumType:
		enumCase, ok := selection.errorType.EnumConsts[selection.name]
		if !ok {
			a.addErrorAtToken(selection.token, "%s has no variant %s", typeDisplayName(selection.errorType), selection.name)
			return selection, false
		}
		selection.enumCase, selection.isEnum = enumCase, true
	default:
		a.addErrorAtToken(selection.token, "Err(...%s) requires a closed concrete error type; %s has no variants to select", selection.name, typeDisplayName(selection.errorType))
		return selection, false
	}
	return selection, true
}

// tryFailureChannelDisplay describes a failure channel without one concrete
// error type for the contextual-selection diagnostic.
func tryFailureChannelDisplay(channel Type, heterogeneous bool) string {
	if heterogeneous {
		return "a heterogeneous failure set"
	}
	return "the open " + typeDisplayName(channel) + " channel"
}

// variantKey is the coverage key of the selection, shared with the existing
// `Err(E.Variant)` match and try paths.
func (selection errorSelection) variantKey() string {
	if selection.open {
		return openErrorNarrowingPrefix + typeDisplayName(selection.errorType) + "." + selection.name
	}
	if selection.isEnum {
		if key, ok := enumValueClassKey(selection.enumCase); ok {
			return concreteErrorVariantPrefix + key
		}
	}
	return concreteErrorVariantPrefix + selection.name
}

// errorPayloadPlace projects a Result Place to the payload of the selected
// error variant.
func errorPayloadPlace(resultPlace Place, selection errorSelection, token lexer.Token) Place {
	return unionPayloadPlace(resultPlace, "Err", selection.errorType, token)
}

// analyzeErrorPayloadPattern binds the payload pattern of an error selection
// against the error value stored at errorPlace. It returns the binding info
// (single payload) or the field bindings (shallow fields), applying the
// omitted-payload Information for a bare payload-carrying variant.
func (a *Analyzer) analyzeErrorPayloadPattern(subject ast.Expression, errorPlace Place, hasPlace bool, pattern *ast.MatchPattern, selection errorSelection) (matchPatternInfo, bool) {
	info := matchPatternInfo{Variant: selection.variantKey(), EnumCaseName: typeDisplayName(selection.errorType) + "." + selection.name}
	payload := pattern.ErrorPayload
	if selection.isEnum {
		if payload != nil {
			a.addErrorAtToken(payload.Token, "enum error variant %s.%s carries no payload to bind", typeDisplayName(selection.errorType), selection.name)
			return info, false
		}
		return info, true
	}
	variant := selection.variant
	carriesPayload := variant.Payload != nil || len(variant.PayloadFields) > 0
	if payload == nil {
		if carriesPayload {
			a.reportOmittedPayloadPattern(selection.token, selection.errorType, variant)
		}
		return info, true
	}
	if !carriesPayload {
		a.addErrorAtToken(payload.Token, "error variant %s.%s carries no payload to bind", typeDisplayName(selection.errorType), variant.Name)
		return info, false
	}
	if payload.Kind == ast.MatchPatternFields {
		fields, ok := a.analyzeUnionFieldPatternAt(subject, errorPlace, hasPlace, payload, selection.errorType)
		info.FieldBindings = fields.FieldBindings
		return info, ok
	}
	if payload.Binding == nil {
		return info, false
	}
	single, ok := a.analyzeUnionPayloadPattern(variant.Name, []ast.Expression{payload.Binding.Expression()}, payload.Token, selection.errorType)
	if !ok {
		return info, false
	}
	if single.PayloadDiscard {
		return info, true
	}
	info.BindingName, info.BindingType = single.BindingName, single.BindingType
	info.PayloadVariant, info.PayloadToken = variant.Name, single.PayloadToken
	info.PayloadBorrow, info.PayloadBorrowMutable = single.PayloadBorrow, single.PayloadBorrowMutable
	info.PayloadNode, info.PayloadBorrowToken = single.PayloadNode, single.PayloadBorrowToken
	payloadType := single.BindingType
	if single.PayloadBorrow && payloadType.Element != nil {
		payloadType = *payloadType.Element
	}
	info.PayloadPlaceResolved = true
	switch {
	case hasPlace:
		info.PayloadPlace = unionPayloadPlace(errorPlace, variant.Name, payloadType, info.PayloadToken)
		if single.PayloadBorrow {
			info.BindingType = a.referenceTypeWithOriginFromExpression(info.BindingType, subject)
			info.BindingType.ReferenceOriginMatchScoped = true
			info.BindingType.ReferenceOriginToken = info.PayloadToken
		} else if requiresOwnershipTransfer(payloadType) {
			// rules/errors/errorhandling.md § 15.3: no hidden partial move out
			// of a reusable Result; a fresh temporary subject may transfer.
			a.addErrorAtTokenWithMetadata(info.PayloadToken, "",
				"Borrow it with `"+variant.Name+"(ref "+info.BindingName+")`, or bind the whole error with `Err(name)` to take ownership.",
				"the %s payload of %s.%s is move-only; binding it by value would partially move the Result", typeDisplayName(payloadType), typeDisplayName(selection.errorType), variant.Name)
			return info, false
		}
	case single.PayloadBorrow:
		a.addErrorAtToken(info.PayloadToken, "cannot borrow an error payload from a temporary Result")
		return info, false
	}
	return info, true
}

// analyzeMatchErrPattern resolves an MD-029 Err arm of a Result match. The
// arm takes part in closed coverage like `Err(E.Variant)`; on an open
// `error` channel it narrows and never covers the open domain.
// Rules: rules/errors/errorhandling.md — § 27.2b.
func (a *Analyzer) analyzeMatchErrPattern(subject ast.Expression, pattern *ast.MatchPattern, subjectType Type) (matchPatternInfo, bool) {
	if subjectType.Kind != ResultType || len(subjectType.TypeArgs) != 2 {
		a.addErrorAtToken(pattern.Token, "Err pattern requires Result subject")
		return matchPatternInfo{}, false
	}
	selection, ok := a.resolveErrorSelection(pattern, subjectType.TypeArgs[1], false)
	if !ok {
		return matchPatternInfo{}, false
	}
	resultPlace, hasPlace := a.resolvePlace(subject)
	return a.analyzeErrorPayloadPattern(subject, errorPayloadPlace(resultPlace, selection, pattern.Token), hasPlace, pattern, selection)
}

// analyzeTryStructuredErrPattern resolves an MD-029 try handler. The protected
// Result is consumed by try, so its error payload is a temporary: a by-value
// binding takes ownership and a borrow has no Place to borrow from. A
// heterogeneous failure set supplies no single concrete type for `.Variant`,
// and an open-channel selection must name a member of that set.
// Rules: rules/errors/errorhandling.md — §§ 15.3, 20.
func (a *Analyzer) analyzeTryStructuredErrPattern(handler *ast.TryHandler, errorType Type) (kind string, bindingName string, variantName string, bindingType Type, ok bool) {
	pattern := handler.ErrPattern
	heterogeneous := a.tryFailureSet != nil && a.tryFailureSet.commonChannel == nil
	selection, ok := a.resolveErrorSelection(pattern, errorType, heterogeneous && pattern.ContextualErrorVariant != nil)
	if !ok {
		return "", "", "", Type{}, false
	}
	if selection.open && a.tryFailureSet != nil && a.tryFailureSet.commonChannel == nil {
		if _, member := a.tryFailureSet.member(selection.errorType); !member {
			a.addErrorAtToken(selection.token, "this try cannot produce %s; its failures are %s", typeDisplayName(selection.errorType), tryFailureSetDisplay(a.tryFailureSet.members))
			return "", "", "", Type{}, false
		}
	}
	info, ok := a.analyzeErrorPayloadPattern(nil, Place{}, false, pattern, selection)
	if !ok {
		return "", "", "", Type{}, false
	}
	variantName = selection.name
	if selection.open {
		variantName = typeDisplayName(selection.errorType) + "." + selection.name
	}
	if len(info.FieldBindings) > 0 || pattern.ErrorPayload != nil && pattern.ErrorPayload.Kind == ast.MatchPatternFields {
		if a.tryHandlerFields == nil {
			a.tryHandlerFields = map[*ast.TryHandler][]matchFieldBinding{}
		}
		a.tryHandlerFields[handler] = append([]matchFieldBinding{}, info.FieldBindings...)
	}
	if info.BindingName != "" {
		return "Err", info.BindingName, variantName, info.BindingType, true
	}
	return "Err", "", variantName, errorType, true
}
