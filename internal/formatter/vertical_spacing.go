package formatter

import (
	"sort"
	"strings"

	"sec/internal/cst"
	"sec/internal/lexer"
	"sec/internal/parser"
)

// formatVerticalSpacing renders parser-owned sibling boundaries. Only whitespace
// outside comments/literals is changed; malformed syntax is left untouched.
// Optional control-flow and outlier separation is not added automatically.
// Rules: rules/tooling/formatter.md — §7(3–13),§12,§25,§29.
func formatVerticalSpacing(text string, compact bool) string {
	document := cst.Build(text, "")
	if hasUncertainConcreteSyntax(document) {
		return text
	}
	p := parser.New(lexer.New(text))
	program := p.ParseProgram()
	if len(p.Errors()) != 0 {
		return text
	}
	var replacements []formatterReplacement
	seen := map[int]bool{}
	for _, boundary := range document.VerticalBoundaries(program) {
		if seen[boundary.Whitespace] {
			continue
		}
		seen[boundary.Whitespace] = true
		element := document.Elements[boundary.Whitespace]
		if !strings.Contains(element.Text, "\n") {
			continue // Horizontal layout belongs to the declaration/block passes.
		}
		count := 0
		if boundary.Required {
			count = 2
		} else if compact && boundary.Compact {
			// Compact avoids adding optional separation; an author's existing
			// blank line between declarations is never optional to remove.
			// Rules: rules/tooling/formatter.md — §7(1),§7(4).
			count = 1
			if strings.Count(element.Text, "\n") >= 2 {
				count = 2
			}
		} else if boundary.Together {
			count = 1
		} else if !compact && boundary.Structured {
			count = 2
		}
		if count == 0 {
			continue
		}
		last := strings.LastIndex(element.Text, "\n")
		want := strings.Repeat("\n", count) + element.Text[last+1:]
		if want != element.Text {
			replacements = append(replacements, formatterReplacement{start: element.Span.Start, end: element.Span.End, text: want})
		}
	}
	sort.Slice(replacements, func(i, j int) bool { return replacements[i].start > replacements[j].start })
	for _, replacement := range replacements {
		text = text[:replacement.start] + replacement.text + text[replacement.end:]
	}
	return text
}
