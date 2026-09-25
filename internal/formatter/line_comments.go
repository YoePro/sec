package formatter

import (
	"sort"
	"strings"

	"sec/internal/cst"
)

// formatCSTLineComments normalizes only the margin immediately following a
// real lexer-owned // delimiter. Comment text is otherwise retained and is
// never discovered by scanning string or block-comment contents.
//
// Rules:
//   - rules/tooling/formatter.md — §12(1) non-empty line-comment margin
//   - rules/tooling/formatter.md — §12(2) empty line-comment form
//   - rules/tooling/formatter.md — §12(7) no aggressive prose reflow
//   - rules/foundations/lexical_structure.md — §5.4 /// is an ordinary comment
func formatCSTLineComments(text string) string {
	document := cst.Build(text, "")
	replacements := []formatterReplacement{}
	for _, element := range document.Elements {
		if element.Kind != cst.Comment || !strings.HasPrefix(element.Text, "//") {
			continue
		}
		content := strings.TrimLeft(element.Text[2:], " \t")
		formatted := "//"
		if content != "" {
			formatted += " " + content
		}
		if formatted == element.Text {
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
