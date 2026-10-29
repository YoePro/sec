package sema

import (
	"fmt"
	"strings"

	"sec/internal/ast"
	"sec/internal/diagnostics"
	"sec/internal/lexer"
)

// reportUnionVariantPayloadRequired reports a payload-carrying variant used
// as a value without its payload (rules/declarations/unions.md §§5.2,5.3).
// Canonical associated enums retain their source kind (atomics.md §6).
func (a *Analyzer) reportUnionVariantPayloadRequired(token lexer.Token, union Type, variant UnionVariant) {
	name := typeDisplayName(union) + "." + variant.Name
	var help string
	if len(variant.PayloadFields) > 0 {
		fields := make([]string, 0, len(variant.PayloadFields))
		for _, field := range variant.PayloadFields {
			fields = append(fields, field.Name+": ...")
		}
		help = fmt.Sprintf("`%s` carries named payload fields; construct it with `%s { %s }`.", name, name, strings.Join(fields, ", "))
	} else {
		payload := "its payload"
		if variant.Payload != nil {
			payload = "its `" + typeDisplayName(*variant.Payload) + "` payload"
		}
		help = fmt.Sprintf("`%s` carries a payload; construct it with %s, for example `%s(value)`.", name, payload, name)
	}
	a.addErrorAtTokenWithMetadata(token, diagnostics.UnionVariantPayloadRequired, help,
		"%s variant %s requires payload", sumSourceKind(union), name)
}

// reportOmittedPayloadPattern explains a legal bare variant pattern that
// selects every value of a payload-carrying variant without inspecting the
// payload (MD-029). It names the payload and the binding and explicit
// non-binding spellings; `Variant(_)` and `Variant {}` suppress it.
//
// Rules:
//   - rules/control-flow/flowcontrol_match.md — § 8 "Variant patterns"
//   - rules/declarations/unions.md — § 9.2a
//   - rules/tooling/diagnostics.md — § 24(4e)
func (a *Analyzer) reportOmittedPayloadPattern(token lexer.Token, union Type, variant UnionVariant) {
	name := typeDisplayName(union) + "." + variant.Name
	var carried, inspect, ignore string
	if len(variant.PayloadFields) > 0 {
		fields := make([]string, 0, len(variant.PayloadFields))
		names := make([]string, 0, len(variant.PayloadFields))
		for _, field := range variant.PayloadFields {
			fields = append(fields, field.Name+": "+typeDisplayName(field.Type))
			names = append(names, field.Name)
		}
		carried = "fields " + strings.Join(fields, ", ")
		inspect = fmt.Sprintf("%s { %s }", variant.Name, strings.Join(names, ", "))
		ignore = variant.Name + " {}"
	} else {
		carried = "a payload"
		if variant.Payload != nil {
			carried = "a `" + typeDisplayName(*variant.Payload) + "` payload"
		}
		inspect = variant.Name + "(payload)"
		ignore = variant.Name + "(_)"
	}
	a.addWarningAtTokenWithMetadata(token, diagnostics.OmittedPayloadPattern,
		fmt.Sprintf("To inspect it, write `%s`; to ignore it explicitly, write `%s`.", inspect, ignore),
		"%s carries %s that this pattern does not inspect", name, carried)
}

// reportOmittedTryHandlerPayload applies the MD-029 omitted-payload
// Information to a bare `Err(ErrorType.Variant)` try handler that selects a
// payload-carrying union error variant.
// Rules: rules/errors/errorhandling.md — § 15.3.
func (a *Analyzer) reportOmittedTryHandlerPayload(pattern *ast.MemberExpression, errorType Type) {
	if errorType.Kind != UnionType || pattern.Property == nil {
		return
	}
	variant, ok := lookupUnionVariant(errorType, pattern.Property.Value)
	if ok && (variant.Payload != nil || len(variant.PayloadFields) > 0) {
		a.reportOmittedPayloadPattern(pattern.Property.Token, errorType, variant)
	}
}
