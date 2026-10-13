package sema

import (
	"fmt"
	"strings"

	"sec/internal/diagnostics"
	"sec/internal/lexer"
)

// reportUnionVariantPayloadRequired reports a payload-carrying variant used
// as a value without its payload (rules/declarations/unions.md §§5.2,5.3).
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
		"union variant %s requires payload", name)
}

// reportUnionPatternPayloadBindingRequired reports a match pattern naming a
// payload-carrying variant without binding the payload
// (rules/declarations/unions.md §§8.1,9.2–9.4).
func (a *Analyzer) reportUnionPatternPayloadBindingRequired(token lexer.Token, union Type, variant UnionVariant) {
	name := typeDisplayName(union) + "." + variant.Name
	help := fmt.Sprintf("Bind the payload in the pattern, for example `%s(payload) => ...`", variant.Name)
	if len(variant.PayloadFields) > 0 {
		help += fmt.Sprintf(" or `%s { %s } => ...`", variant.Name, variant.PayloadFields[0].Name)
	}
	help += fmt.Sprintf(". To test the active variant without the payload, use `value is %s`.", name)
	a.addErrorAtTokenWithMetadata(token, diagnostics.UnionPatternPayloadBindingRequired, help,
		"union variant %s requires payload binding", name)
}
