package formatter

import (
	"strings"

	"sec/internal/cst"
)

// multilineTokenLine describes how the line-oriented pass must treat one
// source line that intersects a string token spanning several lines.
type multilineTokenLine struct {
	// start marks the line on which the token begins; Masked replaces the token
	// text so delimiter counting ignores characters inside the literal.
	start  bool
	masked string
	// continuation marks a later line of the token. Such a line is emitted
	// byte-for-byte; tail holds the source after the token's final byte on
	// the token's last line, which still participates in delimiter counting.
	continuation bool
	tail         string
}

// multilineTokenLines locates real string tokens whose source spans line
// breaks. Their bytes are literal program data, so indentation, trimming,
// blank-line normalization, and delimiter counting must not treat them as
// code.
//
// Rules:
//   - rules/tooling/formatter.md — §5 "Required syntax representation", §6(8) preserved source-sensitive constructs
//   - rules/tooling/formatter.md — §29 "Formatter invariants"
//   - rules/foundations/lexical_structure.md — string and raw string literals
func multilineTokenLines(text string) map[int]multilineTokenLine {
	result := map[int]multilineTokenLine{}
	document := cst.Build(text, "")
	lineStarts := []int{0}
	for index := 0; index < len(text); index++ {
		if text[index] == '\n' {
			lineStarts = append(lineStarts, index+1)
		}
	}
	lineOf := func(offset int) int {
		low, high := 0, len(lineStarts)-1
		for low < high {
			middle := (low + high + 1) / 2
			if lineStarts[middle] <= offset {
				low = middle
			} else {
				high = middle - 1
			}
		}
		return low
	}
	lineEnd := func(line int) int {
		if line+1 < len(lineStarts) {
			return lineStarts[line+1] - 1
		}
		return len(text)
	}
	for _, element := range document.Elements {
		if element.Kind != cst.Token || !strings.Contains(element.Text, "\n") {
			continue
		}
		first, last := lineOf(element.Span.Start), lineOf(element.Span.End-1)
		startLine := text[lineStarts[first]:lineEnd(first)]
		column := element.Span.Start - lineStarts[first]
		result[first] = multilineTokenLine{start: true, masked: startLine[:column] + `""`}
		for line := first + 1; line <= last; line++ {
			entry := multilineTokenLine{continuation: true}
			if line == last {
				entry.tail = text[element.Span.End:lineEnd(line)]
			}
			result[line] = entry
		}
	}
	return result
}
