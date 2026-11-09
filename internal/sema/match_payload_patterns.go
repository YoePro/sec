package sema

import (
	"sec/internal/ast"
	"sec/internal/diagnostics"
	"sec/internal/lexer"
)

// checkMatchPayloadDiscard rejects non-binding patterns that would ignore an
// unresolved lifecycle obligation, using the same check for Result and unions.
// Rules: rules/control-flow/flowcontrol_match.md §§7(4),8(5);
// rules/control-flow/discard.md §18(1)-(3).
func (a *Analyzer) checkMatchPayloadDiscard(pattern string, payload Type, token lexer.Token) bool {
	if a.isDiscardable(payload) {
		return true
	}
	help := "Bind the payload and resolve its task or thread obligation instead of ignoring it."
	if pattern == "Err(_)" {
		help = "Bind the error and resolve its task or thread obligation instead of using Err(_)."
	}
	a.addErrorAtTokenWithMetadata(token, diagnostics.NonDiscardableValue, help,
		"%s cannot ignore %s because it may contain an unresolved lifecycle handle", pattern, a.nonDiscardableSubject(payload))
	return false
}

// checkMatchVariantDiscard resolves the selected variant's payload alone;
// obligations in other inactive variants do not prevent a payload-less pattern.
// Rules: rules/control-flow/flowcontrol_match.md §8(4)-(5);
// rules/control-flow/discard.md §18(2).
func (a *Analyzer) checkMatchVariantDiscard(subject Type, variant UnionVariant, pattern string, token lexer.Token) bool {
	if variant.Payload != nil {
		return a.checkMatchPayloadDiscard(pattern, *variant.Payload, token)
	}
	if len(variant.PayloadFields) > 0 {
		return a.checkMatchPayloadDiscard(pattern, FieldPayloadType(subject, variant), token)
	}
	return true
}

// analyzeUnionPayloadPattern resolves a whole-payload binding, borrow or
// explicit ignore without waiving lifecycle obligations.
// Rules: rules/control-flow/flowcontrol_match.md §§8(5),9,13-15;
// rules/control-flow/discard.md §18.
func (a *Analyzer) analyzeUnionPayloadPattern(variantName string, arguments []ast.Expression, token lexer.Token, subjectType Type) (matchPatternInfo, bool) {
	variant, ok := lookupUnionVariant(subjectType, variantName)
	if !ok {
		a.addErrorAtToken(token, "unknown union variant %s.%s", typeDisplayName(subjectType), variantName)
		return matchPatternInfo{}, false
	}
	if variant.Payload == nil && len(variant.PayloadFields) == 0 {
		a.addErrorAtToken(token, "payload-less union variant %s.%s must not bind a payload", typeDisplayName(subjectType), variant.Name)
		return matchPatternInfo{}, false
	}
	if len(arguments) != 1 {
		a.addErrorAtToken(token, "union variant %s.%s requires 1 payload binding, got %d", typeDisplayName(subjectType), variant.Name, len(arguments))
		return matchPatternInfo{}, false
	}
	bindingExpression := arguments[0]
	binding, borrowed, mutable, ok := unionPayloadPatternBinding(bindingExpression)
	if !ok {
		a.addErrorAtToken(expressionToken(bindingExpression), "union payload pattern must bind an identifier, ref identifier, or ref mut identifier")
		return matchPatternInfo{}, false
	}

	var payloadType Type
	if variant.Payload != nil {
		payloadType = *variant.Payload
	} else {
		payloadType = Type{
			Name:   typeDisplayName(subjectType) + "." + variant.Name,
			Kind:   StructType,
			Fields: variant.PayloadFields,
		}
	}
	info := matchPatternInfo{Kind: "variant", Variant: variant.Name, VariantIndex: unionVariantIndex(subjectType, variant.Name), PayloadVariant: variant.Name}
	if binding.Value == "_" {
		if !a.checkMatchPayloadDiscard(variantName+"(_)", payloadType, binding.Token) {
			return matchPatternInfo{}, false
		}
		info.PayloadDiscard = true
		return info, true
	}
	info.BindingName = binding.Value
	info.PayloadToken = binding.Token
	info.PayloadBorrow = borrowed
	info.PayloadBorrowMutable = mutable
	info.PayloadNode = binding
	if borrowed {
		info.PayloadBorrowToken = expressionToken(bindingExpression)
	}

	info.BindingType = payloadType
	if borrowed {
		info.BindingType = Type{
			Name:             referenceTypeName(payloadType, mutable),
			Kind:             ReferenceType,
			Element:          &payloadType,
			ReferenceMutable: mutable,
		}
	}
	return info, true
}

// analyzeResultPayloadPattern resolves Result payload bindings and validates
// deliberate omission against the payload's recursive discardability.
// Rules: rules/control-flow/flowcontrol_match.md §§7(4),8(5),9,13;
// rules/control-flow/discard.md §18; rules/errors/errorhandling.md §17.
func (a *Analyzer) analyzeResultPayloadPattern(kind string, expr ast.Expression, token lexer.Token, payloadType Type) (matchPatternInfo, bool) {
	info := matchPatternInfo{Kind: kind, PayloadVariant: kind}
	if expr == nil {
		return info, a.checkMatchPayloadDiscard(kind, payloadType, token)
	}
	binding, borrowed, mutable, ok := unionPayloadPatternBinding(expr)
	if !ok {
		a.addErrorAtToken(expressionToken(expr), "%s payload pattern must bind an identifier, ref identifier, or ref mut identifier", kind)
		return matchPatternInfo{}, false
	}
	if binding.Value == "_" {
		if !a.checkMatchPayloadDiscard(kind+"(_)", payloadType, binding.Token) {
			return matchPatternInfo{}, false
		}
		info.PayloadDiscard = true
		return info, true
	}
	info.BindingName = binding.Value
	info.BindingType = payloadType
	info.PayloadToken = binding.Token
	info.PayloadBorrow = borrowed
	info.PayloadBorrowMutable = mutable
	info.PayloadNode = binding
	if borrowed {
		info.PayloadBorrowToken = expressionToken(expr)
		info.BindingType = Type{
			Name: referenceTypeName(payloadType, mutable), Kind: ReferenceType,
			Element: &payloadType, ReferenceMutable: mutable,
		}
	}
	return info, true
}

// unionPayloadPatternBinding reads the parser-owned whole-payload binding mode.
// Rules: rules/control-flow/flowcontrol_match.md §§9,13-14.
func unionPayloadPatternBinding(expr ast.Expression) (binding *ast.Identifier, borrowed bool, mutable bool, ok bool) {
	switch expr := expr.(type) {
	case *ast.Identifier:
		return expr, false, false, true
	case *ast.RefExpression:
		binding, ok := expr.Value.(*ast.Identifier)
		return binding, true, expr.Mutable, ok
	default:
		return nil, false, false, false
	}
}
