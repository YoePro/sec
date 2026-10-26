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
// `identifier:` from its type in the widest row.
//
// Rules:
//   - rules/corrections/applied/missing-decisions-md010-md014-correction-20261003.md — §§ 5.12–5.19
//   - rules/tooling/formatter.md — § 9(2), § 12(4)–(6)
func alignDeclarationTrailingComments(lines []string, indentationWidth int) []string {
	for opener := 0; opener < len(lines); opener++ {
		if !isDeclarationAlignmentOpener(lines[opener]) {
			continue
		}
		baseIndent := leadingSpaces(lines[opener])
		end := declarationBlockEnd(lines, opener, baseIndent)
		if end < 0 {
			continue
		}
		alignDeclarationBodyGroups(lines, opener+1, end, baseIndent+indentationWidth)
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
// Rules: rules/tooling/formatter.md — § 9(8), (8b), § 12(4)–(6).
func alignDeclarationBodyGroups(lines []string, start int, end int, itemIndent int) {
	group := []int{}
	flush := func() {
		alignTrailingCommentGroup(lines, group)
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
		group = append(group, index)
	}
	flush()
}

// structuralColumnSpacing is the formatter's standard spacing between
// structural columns: the single space after `identifier:` in the widest row
// of an aligned group (formatter.md § 9(3), § 9(5), § 22(3)).
const structuralColumnSpacing = 1

// alignTrailingCommentGroup places attached comments after the widest local code cell.
// Rules: rules/tooling/formatter.md — § 9(8), (8b), § 12(4)–(6).
func alignTrailingCommentGroup(lines []string, group []int) {
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
	for _, index := range group {
		code, comment, hasComment := splitTrailingComment(lines[index])
		if !hasComment {
			continue
		}
		padding := maxCodeWidth - utf8.RuneCountInString(code) + structuralColumnSpacing
		lines[index] = code + strings.Repeat(" ", padding) + comment
	}
}
