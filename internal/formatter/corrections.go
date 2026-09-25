package formatter

import (
	"sort"

	"sec/internal/cst"
	"sec/internal/lexer"
	"sec/internal/parser"
)

// fixRedundantNestedParentheses removes the inner pair when one complete
// parenthesized CST group is the sole non-trivia content of another. Removing
// the inner pair preserves grouping and, for calls such as Call((value)),
// preserves the outer call delimiter. Comments between the two groups make
// the rewrite ineligible so their concrete attachment is not guessed.
//
// This is an opt-in Language Correction and must never run during ordinary
// formatting.
//
// Rules:
//   - rules/tooling/formatter.md — §17(13) parenthesis preservation
//   - rules/tooling/formatter.md — §26 "Language Corrections model"
//   - rules/tooling/formatter.md — §27(19) redundant expression parentheses
func fixRedundantNestedParentheses(text string) string {
	document := cst.Build(text, "")
	if len(document.Diagnostics) > 0 || len(document.UnmatchedClosers) > 0 {
		return text
	}

	removals := make([]formatterReplacement, 0)
	for _, inner := range document.Groups {
		if inner.Parent < 0 || inner.Close < 0 || inner.Parent >= len(document.Groups) {
			continue
		}
		outer := document.Groups[inner.Parent]
		if outer.Close < 0 ||
			document.Elements[outer.Open].Token.Type != lexer.LPAREN ||
			document.Elements[inner.Open].Token.Type != lexer.LPAREN {
			continue
		}
		if !onlyWhitespaceElements(document.Elements[outer.Open+1:inner.Open]) ||
			!onlyWhitespaceElements(document.Elements[inner.Close+1:outer.Close]) {
			continue
		}
		removals = append(removals,
			formatterReplacement{start: document.Elements[inner.Open].Span.Start, end: document.Elements[inner.Open].Span.End},
			formatterReplacement{start: document.Elements[inner.Close].Span.Start, end: document.Elements[inner.Close].Span.End},
		)
	}

	sort.Slice(removals, func(i, j int) bool { return removals[i].start > removals[j].start })
	for _, removal := range removals {
		text = text[:removal.start] + text[removal.end:]
	}
	return text
}

// fixRedundantControlConditionParentheses removes a complete outer grouping
// pair only when the parser proves that it owns the whole if/while condition or
// switch subject and that the following body parsed successfully.
//
// Rules:
//   - rules/tooling/formatter.md — §26 "Language Corrections model"
//   - rules/tooling/formatter.md — §27(17–18) control-condition parentheses
func fixRedundantControlConditionParentheses(text string) string {
	program := parser.New(lexer.New(text)).ParseProgram()
	document := cst.Build(text, "")
	document.ApplyProgramRoles(program)

	removals := make([]formatterReplacement, 0)
	for _, element := range document.Elements {
		if element.HasRole(cst.RedundantControlConditionDelimiter) {
			removals = append(removals, formatterReplacement{start: element.Span.Start, end: element.Span.End})
		}
	}
	sort.Slice(removals, func(i, j int) bool { return removals[i].start > removals[j].start })
	for _, removal := range removals {
		text = text[:removal.start] + text[removal.end:]
	}
	return text
}

func onlyWhitespaceElements(elements []cst.Element) bool {
	for _, element := range elements {
		if element.Kind != cst.Whitespace {
			return false
		}
	}
	return true
}
