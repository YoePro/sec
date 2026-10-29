package sema

import (
	"strings"

	"sec/internal/lexer"
)

// reportUndeclaredStringEnumValue retains the existing conversion error and
// adds repairs from resolved declared value classes. A known invalid constant
// cannot be repaired merely by adding try.
// Rules: rules/declarations/enums.md — §§7,12.6,18;
// rules/tooling/diagnostics.md — mentor help.
func (a *Analyzer) reportUndeclaredStringEnumValue(token lexer.Token, target Type, value string) {
	names, values := []string{}, []string{}
	for _, class := range EnumValueClasses(target) {
		if class.StringValue == nil || len(class.Members) == 0 {
			continue
		}
		names = append(names, typeDisplayName(target)+"."+class.Members[0])
		values = append(values, lexer.QuoteDecodedStringLiteral(*class.StringValue))
	}
	help := "Use a declared member of `" + typeDisplayName(target) + "`."
	if len(names) > 0 {
		help += " Members: `" + strings.Join(names, "`, `") + "`. Declared string values: " + strings.Join(values, ", ") + "."
	}
	a.addErrorAtTokenWithMetadata(token, "", help,
		"string value %q is not a declared value of closed enum %s", value, target.Name)
}
