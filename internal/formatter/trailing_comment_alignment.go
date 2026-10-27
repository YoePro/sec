package formatter

import (
	"strings"
	"unicode/utf8"
)

// alignDeclarationTrailingComments aligns local groups inside nominal
// declaration blocks. A blank line, standalone comment, multiline item, or
// nested block ends a group; a trailing comment does not. Trailing comments
// occupy the next structural column, which begins one standard space after
// the widest code cell of the group, the same spacing that separates
// `identifier:` from its type in the widest row. A group whose comment column
// would need more than maxPadding spaces on any commented row keeps single
// spaces instead; only enum explicit-value groups are exempt (MD-099/MD-100).
//
// Rules:
//   - rules/corrections/applied/missing-decisions-md010-md014-correction-20261003.md — §§ 5.12–5.19
//   - rules/tooling/formatter.md — § 9(2), § 9(10a), § 12(4)–(6a), § 15(4a)
func alignDeclarationTrailingComments(lines []string, indentationWidth int, maxPadding int) []string {
	for opener := 0; opener < len(lines); opener++ {
		if !isDeclarationAlignmentOpener(lines[opener]) {
			continue
		}
		baseIndent := leadingSpaces(lines[opener])
		end := declarationBlockEnd(lines, opener, baseIndent)
		if end < 0 {
			continue
		}
		alignDeclarationBodyGroups(lines, opener+1, end, baseIndent+indentationWidth, maxPadding, isEnumDeclarationOpener(lines[opener]))
	}
	return lines
}

// isDeclarationAlignmentOpener identifies nominal declaration tables for trailing-comment alignment.
// Rules: rules/tooling/formatter.md — § 9(8), (8b), § 12(4)–(6).
func isDeclarationAlignmentOpener(line string) bool {
	trimmed := strings.TrimSpace(line)
	if !strings.HasSuffix(codeBeforeTrailingComment(trimmed), "{") {
		return false
	}
	if strings.HasPrefix(trimmed, "enum ") || strings.HasPrefix(trimmed, "interface ") {
		return true
	}
	if !strings.HasPrefix(trimmed, "type ") {
		return false
	}
	return strings.Contains(trimmed, " struct {") ||
		strings.Contains(trimmed, " register[") ||
		strings.Contains(trimmed, " union {") ||
		strings.Contains(trimmed, " union error {") ||
		strings.Contains(trimmed, " enum {")
}

// isEnumDeclarationOpener identifies enum tables, whose explicit-value groups
// keep their comment column regardless of the padding cap.
// Rules: rules/tooling/formatter.md — § 15(4), (4a).
func isEnumDeclarationOpener(line string) bool {
	trimmed := strings.TrimSpace(line)
	return strings.HasPrefix(trimmed, "enum ") || strings.HasPrefix(trimmed, "type ") && strings.Contains(trimmed, " enum {")
}

// declarationBlockEnd finds the enclosing nominal table boundary.
// Rules: rules/tooling/formatter.md — § 9(8), (8b), § 12(4)–(6).
func declarationBlockEnd(lines []string, opener int, baseIndent int) int {
	for index := opener + 1; index < len(lines); index++ {
		trimmed := strings.TrimSpace(lines[index])
		if leadingSpaces(lines[index]) == baseIndent && strings.HasPrefix(trimmed, "}") {
			return index
		}
	}
	return -1
}

// alignDeclarationBodyGroups ends local alignment groups at trivia and structural boundaries.
// Rules: rules/tooling/formatter.md — § 9(8), (8b), § 12(4)–(6a), § 15(4a).
func alignDeclarationBodyGroups(lines []string, start int, end int, itemIndent int, maxPadding int, enum bool) {
	group := []int{}
	flush := func() {
		alignTrailingCommentGroup(lines, group, maxPadding, enum && enumExplicitValueGroup(lines, group))
		group = group[:0]
	}
	for index := start; index < end; index++ {
		line := lines[index]
		trimmed := strings.TrimSpace(line)
		code, _, _ := splitTrailingComment(trimmed)
		if trimmed == "" || leadingSpaces(line) != itemIndent || isStandaloneComment(trimmed) || delimiters(strings.TrimSpace(code)) > 0 {
			flush()
			continue
		}
		// Explicit and implicit enum runs are separate groups. Implicit
		// runs retain ordinary capped comment alignment, while each
		// explicit run keeps its mandatory exception (MD-100 §3(2)).
		if enum && len(group) > 0 {
			previous, _, _ := splitTrailingComment(lines[group[len(group)-1]])
			if strings.Contains(code, "=") != strings.Contains(previous, "=") {
				flush()
			}
		}
		group = append(group, index)
	}
	flush()
}

// enumExplicitValueGroup reports a group of enum members that all carry an
// explicit `=` value; an implicit member is not part of the exempt group.
// Rules: rules/tooling/formatter.md — § 15(4), (4a).
func enumExplicitValueGroup(lines []string, group []int) bool {
	for _, index := range group {
		code, _, _ := splitTrailingComment(lines[index])
		if !strings.Contains(code, "=") {
			return false
		}
	}
	return len(group) > 0
}

// structuralColumnSpacing is the formatter's standard spacing between
// structural columns: the single space after `identifier:` in the widest row
// of an aligned group (formatter.md § 9(3), § 9(5), § 22(3)).
const structuralColumnSpacing = 1

// alignTrailingCommentGroup places attached comments after the widest local
// code cell, or one space after their own code when any commented row would
// need more than maxPadding spaces and the group is not exempt.
// Rules: rules/tooling/formatter.md — § 9(8), (8b), § 12(4)–(6a).
func alignTrailingCommentGroup(lines []string, group []int, maxPadding int, exempt bool) {
	maxCodeWidth := 0
	commented := 0
	for _, index := range group {
		code, _, hasComment := splitTrailingComment(lines[index])
		if width := utf8.RuneCountInString(code); width > maxCodeWidth {
			maxCodeWidth = width
		}
		if hasComment {
			commented++
		}
	}
	if commented == 0 {
		return
	}
	drop := false
	for _, index := range group {
		code, _, hasComment := splitTrailingComment(lines[index])
		if hasComment && !exempt && maxCodeWidth-utf8.RuneCountInString(code)+structuralColumnSpacing > maxPadding {
			drop = true
		}
	}
	for _, index := range group {
		code, comment, hasComment := splitTrailingComment(lines[index])
		if !hasComment {
			continue
		}
		padding := structuralColumnSpacing
		if !drop {
			padding = maxCodeWidth - utf8.RuneCountInString(code) + structuralColumnSpacing
		}
		lines[index] = code + strings.Repeat(" ", padding) + comment
	}
}
