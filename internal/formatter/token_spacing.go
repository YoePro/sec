package formatter

import (
	"sort"
	"strings"

	"sec/internal/cst"
	"sec/internal/lexer"
	"sec/internal/parser"
)

// formatCSTTokenSpacing normalizes the horizontal gap between two adjacent
// real tokens on the same line when the parser has assigned either token a
// spacing role. A gap that crosses a line break or holds a comment is never
// touched, so line structure, indentation, and comment placement are left to
// the later passes. Tokens without a spacing role keep their source spacing.
//
// Rules:
//   - rules/tooling/formatter.md — §6(4) declaration colon, §6(5) binary operators, §6(6) unary operators
//   - rules/tooling/formatter.md — §6(7) assignment and initialization operators
//   - rules/tooling/formatter.md — §8(1)–(3) same-line opening braces and "} else {"
//   - rules/tooling/formatter.md — §18(1) control-flow braces, §18(12) canonical loop forms
//   - rules/tooling/formatter.md — §21(5) consuming call-site marker attachment
//   - rules/tooling/formatter.md — §25 "Malformed and incomplete source"
func formatCSTTokenSpacing(text string) string {
	document := cst.Build(text, "")
	if hasUncertainConcreteSyntax(document) {
		return text
	}
	document.ApplyProgramRoles(parser.New(lexer.New(text)).ParseProgram())

	replacements := []formatterReplacement{}
	previous := -1
	for index, element := range document.Elements {
		switch element.Kind {
		case cst.Whitespace:
			if strings.ContainsAny(element.Text, "\n\r") {
				previous = -1
			}
			continue
		case cst.Token:
		default:
			previous = -1
			continue
		}
		if previous >= 0 {
			left := document.Elements[previous]
			if desired, ok := canonicalTokenGap(left, element); ok {
				start, end := left.Span.End, element.Span.Start
				if text[start:end] != desired {
					replacements = append(replacements, formatterReplacement{start: start, end: end, text: desired})
				}
			}
		}
		previous = index
	}

	sort.Slice(replacements, func(i, j int) bool { return replacements[i].start > replacements[j].start })
	for _, replacement := range replacements {
		text = text[:replacement.start] + replacement.text + text[replacement.end:]
	}
	return text
}

// canonicalTokenGap returns the canonical same-line gap between two adjacent
// tokens, or false when neither token's role determines it.
func canonicalTokenGap(left, right cst.Element) (string, bool) {
	switch {
	case left.HasRole(cst.PrefixOperator):
		return "", true
	case right.HasRole(cst.DeclarationColon):
		return "", true
	case left.HasRole(cst.BinaryOperator) || right.HasRole(cst.BinaryOperator) ||
		left.HasRole(cst.AssignmentOperator) || right.HasRole(cst.AssignmentOperator):
		return " ", true
	case left.HasRole(cst.DeclarationColon) || left.HasRole(cst.SpacedKeyword):
		return " ", true
	case right.HasRole(cst.ExecutableBlockOpen) || right.HasRole(cst.ControlBodyOpen) || right.HasRole(cst.AvailabilityBlockOpen):
		return " ", true
	}
	return "", false
}
