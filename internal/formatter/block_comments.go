package formatter

import (
	"sort"
	"strings"

	"sec/internal/cst"
)

// formatCSTBlockComments canonicalizes standalone multiline block comments
// from lossless CST comment tokens. Inline and nested comments remain outside
// this first slice because their indentation and internal delimiter ownership
// require the broader grammar-aware comment model.
//
// Rules:
//   - rules/tooling/formatter.md — §12(8) multiline aligned-star form
//   - rules/tooling/formatter.md — §12(10) documentation comment form
//   - rules/tooling/formatter.md — §12(11) paragraph separation
//   - rules/tooling/formatter.md — §12(12) surrounding indentation
//   - rules/tooling/formatter.md — §12(14) preformatted preservation
func formatCSTBlockComments(text string) string {
	document := cst.Build(text, "")
	replacements := []formatterReplacement{}
	for _, element := range document.Elements {
		if element.Kind != cst.Comment || !strings.Contains(element.Text, "\n") ||
			!strings.HasPrefix(element.Text, "/*") || !strings.HasSuffix(element.Text, "*/") {
			continue
		}
		indent, standalone := standaloneCommentIndent(text, element.Span)
		if !standalone || hasNestedBlockComment(element.Text) {
			continue
		}
		formatted, ok := alignedBlockComment(element.Text, indent)
		if !ok || formatted == element.Text {
			continue
		}
		replacements = append(replacements, formatterReplacement{
			start: element.Span.Start,
			end:   element.Span.End,
			text:  formatted,
		})
	}

	sort.Slice(replacements, func(i, j int) bool { return replacements[i].start > replacements[j].start })
	for _, replacement := range replacements {
		text = text[:replacement.start] + replacement.text + text[replacement.end:]
	}
	return text
}

// standaloneBlockCommentLines returns the one-based physical lines wholly
// owned by standalone block-comment tokens. The legacy indentation pass must
// treat delimiters on these lines as comment text rather than source syntax.
func standaloneBlockCommentLines(text string) map[int]bool {
	document := cst.Build(text, "")
	lines := map[int]bool{}
	for _, element := range document.Elements {
		if element.Kind != cst.Comment || !strings.HasPrefix(element.Text, "/*") {
			continue
		}
		if _, standalone := standaloneCommentIndent(text, element.Span); !standalone {
			continue
		}
		first := element.Token.Line
		for line := first; line <= first+strings.Count(element.Text, "\n"); line++ {
			lines[line] = true
		}
	}
	return lines
}

func standaloneCommentIndent(text string, span cst.Span) (string, bool) {
	lineStart := strings.LastIndex(text[:span.Start], "\n") + 1
	prefix := text[lineStart:span.Start]
	if strings.Trim(prefix, " \t") != "" {
		return "", false
	}
	lineEnd := len(text)
	if offset := strings.IndexByte(text[span.End:], '\n'); offset >= 0 {
		lineEnd = span.End + offset
	}
	if strings.Trim(text[span.End:lineEnd], " \t\r") != "" {
		return "", false
	}
	return prefix, true
}

func hasNestedBlockComment(comment string) bool {
	return strings.Contains(comment[2:len(comment)-2], "/*")
}

func alignedBlockComment(comment, indent string) (string, bool) {
	documentation := strings.HasPrefix(comment, "/**")
	openerLength := 2
	opener := "/*"
	if documentation {
		openerLength = 3
		opener = "/**"
	}
	inner := comment[openerLength : len(comment)-2]
	rawLines := strings.Split(inner, "\n")
	for len(rawLines) > 0 && strings.TrimSpace(rawLines[0]) == "" {
		rawLines = rawLines[1:]
	}
	for len(rawLines) > 0 && strings.TrimSpace(rawLines[len(rawLines)-1]) == "" {
		rawLines = rawLines[:len(rawLines)-1]
	}

	lines := make([]string, 0, len(rawLines))
	lastBlank := false
	for _, raw := range rawLines {
		content := blockCommentLineContent(raw, indent)
		blank := content == ""
		if blank && lastBlank {
			continue
		}
		lines = append(lines, content)
		lastBlank = blank
	}

	var result strings.Builder
	result.WriteString(opener)
	for _, line := range lines {
		result.WriteByte('\n')
		result.WriteString(indent)
		result.WriteString(" *")
		if line != "" {
			result.WriteByte(' ')
			result.WriteString(line)
		}
	}
	result.WriteByte('\n')
	result.WriteString(indent)
	result.WriteString(" */")
	return result.String(), true
}

func blockCommentLineContent(raw, indent string) string {
	if indent != "" && strings.HasPrefix(raw, indent) {
		raw = raw[len(indent):]
	}
	trimmedLeft := strings.TrimLeft(raw, " \t")
	if strings.HasPrefix(trimmedLeft, "*") {
		trimmedLeft = strings.TrimPrefix(trimmedLeft, "*")
		trimmedLeft = strings.TrimPrefix(trimmedLeft, " ")
		if len(trimmedLeft)-len(strings.TrimLeft(trimmedLeft, " \t")) >= 4 {
			return strings.TrimRight(trimmedLeft, "\r")
		}
	}
	if len(raw)-len(strings.TrimLeft(raw, " \t")) >= 4 {
		return strings.TrimRight(raw, "\r")
	}
	return strings.TrimSpace(trimmedLeft)
}
