package sema

import (
	"fmt"
	"strings"

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

// reportUnionPatternPayloadBindingRequired reports a match pattern naming a
// payload-carrying variant without binding the payload
// (rules/declarations/unions.md §§8.1,9.2–9.4).
// Rules: rules/concurrency/atomics.md — §6 (associated-value core enum).
func (a *Analyzer) reportUnionPatternPayloadBindingRequired(token lexer.Token, union Type, variant UnionVariant) {
	name := typeDisplayName(union) + "." + variant.Name
	help := fmt.Sprintf("Bind the payload in the pattern, for example `%s(payload) => ...`", variant.Name)
	if len(variant.PayloadFields) > 0 {
		help += fmt.Sprintf(" or `%s { %s } => ...`", variant.Name, variant.PayloadFields[0].Name)
	}
	help += fmt.Sprintf(". To test the active variant without the payload, use `value is %s`.", name)
	a.addErrorAtTokenWithMetadata(token, diagnostics.UnionPatternPayloadBindingRequired, help,
		"%s variant %s requires payload binding", sumSourceKind(union), name)
}
