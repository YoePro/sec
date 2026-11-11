package cst

import (
	"sec/internal/ast"
	"sec/internal/ast/walk"
)

// FormattingRegions returns complete physical-line envelopes of parser-owned
// top-level statements. Selecting a nested member therefore includes its impl
// or type, preserving indentation, alignment groups and comment ownership.
// Regions may overlap when statements share a physical line.
// Rules: rules/tooling/formatter.md §§5,9,12,25,28;
// rules/tooling/lsp.md Appendix A.8.
func (d Document) FormattingRegions(program *ast.Program) []Span {
	text := d.Text()
	regions := make([]Span, 0, len(program.Statements))
	for _, stmt := range program.Statements {
		start, end, ok := walk.Span(stmt)
		if !ok || start < 0 || end > len(text) {
			return nil
		}
		// Some AST declarations retain their member tokens without the final
		// delimiter. The lossless CST supplies the matching real endpoint.
		for _, group := range d.Groups {
			if group.Span.Start >= start && group.Span.Start < end && group.Span.End > end {
				end = group.Span.End
			}
		}
		for _, comment := range program.Comments {
			if comment.Target != stmt {
				continue
			}
			for _, token := range comment.Comments {
				start = min(start, token.ByteStart)
			}
		}
		for start > 0 && text[start-1] != '\n' && text[start-1] != '\r' {
			start--
		}
		for end < len(text) && text[end] != '\n' && text[end] != '\r' {
			end++
		}
		if end < len(text) && text[end] == '\r' {
			end++
		}
		if end < len(text) && text[end] == '\n' {
			end++
		}
		regions = append(regions, Span{Start: start, End: end})
	}
	return regions
}
