package formatter

import (
	"reflect"

	"sec/internal/ast"
	"sec/internal/cst"
	"sec/internal/lexer"
	"sec/internal/parser"
)

// RangeEdit formats selected complete syntax units in full document context,
// then returns a minimal edit confined to those units. The normal formatter
// alone decides layout. Complete top-level units retain nested indentation,
// declaration alignment, comments and import-group ordering. Unselected bytes
// are untouched, including their line endings and blank lines. Malformed or
// incomplete syntax receives no speculative edit. Byte offsets must be scalar
// and CRLF boundaries. An empty range selects the unit containing the cursor.
// Rules: rules/tooling/formatter.md §§5,9,12,13,25,28,29;
// rules/tooling/lsp.md Appendix A.8.
func RangeEdit(source Source, options Options, start, end int) (Edit, bool) {
	text := source.Text
	if start < 0 || end < start || end > len(text) || !editBoundary(text, start) || !editBoundary(text, end) {
		return Edit{}, false
	}
	before, regions := formattingRegions(text)
	if before == nil {
		return Edit{}, false
	}
	formatted := Format(source, options)
	if formatted.Malformed {
		return Edit{}, false
	}
	if start == 0 && end == len(text) {
		return MinimalEdit(text, formatted.Text)
	}
	after, newRegions := formattingRegions(formatted.Text)
	if after == nil || len(regions) != len(newRegions) {
		return Edit{}, false
	}
	for i, stmt := range before.Statements {
		if reflect.TypeOf(stmt) != reflect.TypeOf(after.Statements[i]) {
			return Edit{}, false
		}
	}
	first, last := -1, -1
	for i, region := range regions {
		intersects := region.Start < end && region.End > start
		if start == end {
			intersects = region.Start <= start && start < region.End
		}
		if intersects {
			if first < 0 {
				first = i
			}
			last = i
		}
	}
	if first < 0 {
		return Edit{}, false
	}
	// Imports are one canonical sortable region, never individual destinations.
	if _, ok := before.Statements[first].(*ast.ImportStatement); ok {
		for first > 0 {
			if _, ok := before.Statements[first-1].(*ast.ImportStatement); !ok {
				break
			}
			first--
		}
	}
	if _, ok := before.Statements[last].(*ast.ImportStatement); ok {
		for last+1 < len(regions) {
			if _, ok := before.Statements[last+1].(*ast.ImportStatement); !ok {
				break
			}
			last++
		}
	}
	oldStart, oldEnd := regions[first].Start, regions[last].End
	newStart, newEnd := newRegions[first].Start, newRegions[last].End
	edit, changed := MinimalEdit(text[oldStart:oldEnd], formatted.Text[newStart:newEnd])
	edit.Start += oldStart
	edit.End += oldStart
	return edit, changed
}

// OnTypeEdit accepts a real closing block delimiter or a whitespace newline
// immediately before the post-typing cursor. Comment/literal contents never
// act as triggers. The selected unit uses the same canonical range formatter.
// Rules: rules/tooling/formatter.md §§5,12,25,28,29;
// rules/tooling/lsp.md Appendix A.8.
func OnTypeEdit(source Source, options Options, cursor int, trigger string) (Edit, bool) {
	text := source.Text
	if cursor <= 0 || cursor > len(text) || (trigger != "}" && trigger != "\n") || text[cursor-1:cursor] != trigger {
		return Edit{}, false
	}
	for _, element := range cst.Build(text, "").Elements {
		valid := trigger == "}" && element.Kind == cst.Token && element.Token.Type == lexer.RBRACE && element.Span.End == cursor
		valid = valid || (trigger == "\n" && element.Kind == cst.Whitespace && element.Span.Start < cursor && element.Span.End >= cursor)
		if !valid {
			continue
		}
		at := cursor - 1
		if text[at] == '\n' && at > 0 && text[at-1] == '\r' {
			at--
		}
		return RangeEdit(source, options, at, at)
	}
	return Edit{}, false
}

// formattingRegions admits only complete parser/CST snapshots for local edits.
// Rules: rules/tooling/formatter.md §§5,25; rules/tooling/lsp.md Appendix A.8.
func formattingRegions(text string) (*ast.Program, []cst.Span) {
	document := cst.Build(text, "")
	if hasUncertainConcreteSyntax(document) {
		return nil, nil
	}
	p := parser.New(lexer.New(text))
	program := p.ParseProgram()
	if len(p.Errors()) != 0 {
		return nil, nil
	}
	regions := document.FormattingRegions(program)
	if len(regions) != len(program.Statements) {
		return nil, nil
	}
	return program, regions
}
