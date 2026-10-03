package formatter

import (
	"sort"
	"strings"

	"sec/internal/ast"
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

// fixLegacyAssignedNamedType rewrites the legacy assigned named-type form
// `type Name = T` to the canonical `type Name T`, which has the same nominal
// meaning. The compact variant form `type Name = A B ...` is never rewritten
// because its enum or union replacement is not uniquely determined, and no
// rewrite happens when a comment sits next to the `=`.
//
// This is an opt-in Language Correction and must never run during ordinary
// formatting.
//
// Rules:
//   - rules/tooling/formatter.md — § 27(33)–(35)
//   - rules/corrections/applied/missing-decisions-md001-md009-correction-20261003.md — §§ 3.10–3.11, 4.4–4.5
func fixLegacyAssignedNamedType(text string) string {
	program := parser.New(lexer.New(text)).ParseProgram()
	if program == nil {
		return text
	}
	replacements := []formatterReplacement{}
	for _, statement := range program.Statements {
		declaration, ok := statement.(*ast.TypeDeclStatement)
		if !ok || declaration.AssignedType == nil || len(declaration.Variants) > 0 || declaration.AssignToken.Type != lexer.ASSIGN {
			continue
		}
		start, end := declaration.AssignToken.ByteStart, declaration.AssignToken.ByteEnd
		if start <= 0 || end > len(text) || text[start:end] != "=" {
			continue
		}
		for start > 0 && (text[start-1] == ' ' || text[start-1] == '\t') {
			start--
		}
		for end < len(text) && (text[end] == ' ' || text[end] == '\t') {
			end++
		}
		if end >= len(text) || text[end] == '\n' || text[end] == '\r' || strings.HasPrefix(text[end:], "/") {
			continue
		}
		replacements = append(replacements, formatterReplacement{start: start, end: end, text: " "})
	}
	sort.Slice(replacements, func(i, j int) bool { return replacements[i].start > replacements[j].start })
	for _, replacement := range replacements {
		text = text[:replacement.start] + replacement.text + text[replacement.end:]
	}
	return text
}

// fixMissingListSeparators inserts the comma that the parser proved missing
// between two items of a comma-separated list written on separate lines:
// struct fields, parameters, call arguments, and array literal elements. The
// parser records each such repair as an insert-missing-token recovery event
// whose After token is the previous item's last token, so the comma lands
// directly after the item and before any trailing comment. Same-line
// adjacency and lists whose grammar already accepts a line break (enum,
// union, register, struct literal) are never rewritten.
//
// This is an opt-in Language Correction and must never run during ordinary
// formatting.
//
// Rules:
//   - rules/tooling/formatter.md — § 27(37) missing list separators
//   - rules/compiler/parser_recovery.md — "Missing comma"
func fixMissingListSeparators(text string) string {
	result := parser.New(lexer.New(text)).Parse()
	offsets := []int{}
	seen := map[int]bool{}
	for _, event := range result.Recovery {
		if event.Kind != parser.RecoveryInsertMissingToken || event.After.Type == "" ||
			len(event.Expected) != 1 || event.Expected[0] != lexer.COMMA {
			continue
		}
		offset := event.After.ByteEnd
		if offset <= 0 || offset > len(text) || seen[offset] || text[offset-1:offset] == "," {
			continue
		}
		seen[offset] = true
		offsets = append(offsets, offset)
	}
	sort.Sort(sort.Reverse(sort.IntSlice(offsets)))
	for _, offset := range offsets {
		text = text[:offset] + "," + text[offset:]
	}
	return text
}
