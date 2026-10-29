package main

import (
	"strings"

	"sec/internal/lexer"
	"sec/internal/sema"
)

// stringEnumHoverDetails presents the closed string domain and aliases from
// Sema's canonical value classes, together with the language conversion contract.
// It does not resolve members, evaluate initializers or choose a runtime plan.
// Rules: rules/declarations/enums.md — §§7–8,12.1,12.6,18;
// rules/tooling/lsp.md — "Hover".
func stringEnumHoverDetails(typ sema.Type) string {
	if typ.Underlying != "string" {
		return ""
	}
	owner := sema.TypeDisplayName(typ)
	lines := []string{"Underlying type: `string`", "Domain: closed (declared string values only)."}
	if typ.EnumDefault != "" {
		lines = append(lines, "Default: `"+owner+"."+typ.EnumDefault+"`")
	}
	for _, class := range sema.EnumValueClasses(typ) {
		if class.StringValue == nil || len(class.Members) < 2 {
			continue
		}
		for _, alias := range class.Members[1:] {
			lines = append(lines, "Alias: `"+owner+"."+alias+"` = `"+owner+"."+class.Members[0]+"` ("+lexer.QuoteDecodedStringLiteral(*class.StringValue)+")")
		}
	}
	lines = append(lines,
		"Runtime string conversion: `try "+owner+"(value)`; result `Result["+owner+", EnumValueError]`, failure `EnumValueError.UndeclaredValue`.",
		"Conversion to `string` is infallible.")
	return "\n\n" + strings.Join(lines, "\n\n")
}

// stringEnumDeclarationMembers renders resolved values in source order while
// leaving default selection in the separate semantic-default description.
// Rules: rules/declarations/enums.md — §§4–5,8,18.
func stringEnumDeclarationMembers(typ sema.Type) []string {
	rows := make([]string, 0, len(typ.EnumValues))
	for _, name := range typ.EnumValues {
		row := name
		if value := typ.EnumConsts[name]; value.StringValue != nil {
			row += " = " + lexer.QuoteDecodedStringLiteral(*value.StringValue)
		}
		rows = append(rows, row)
	}
	return rows
}
