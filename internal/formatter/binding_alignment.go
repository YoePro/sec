package formatter

import (
	"strings"

	"sec/internal/cst"
	"sec/internal/lexer"
)

type bindingAlignmentRow struct {
	first, name, typeStart, initializer, last, comment int
	line, indent                                       int
	family                                             string
}

// bindingAlignmentReplacements extends the structural declaration table to
// let declarations, type-first declarations and named-value groups. Parser
// roles select the columns; real CST trivia and delimiter spans determine
// safe rows and boundaries. Fields/registers, parameters and named types use
// their existing structural table clients, rather than being guessed here.
//
// Rules:
//   - rules/tooling/formatter.md — § 9(1)–(10), § 12(4)–(6), § 29(2), (4), (7)
//   - rules/types/types.md — "Variable declarations", "Type-first declarations",
//     "Parenthesized immutable type-first groups"
func bindingAlignmentReplacements(document cst.Document, maxPadding int) []formatterReplacement {
	var replacements []formatterReplacement
	var group []bindingAlignmentRow
	flush := func() {
		replacements = append(replacements, alignBindingGroup(document, group, maxPadding)...)
		group = nil
	}
	for _, row := range bindingAlignmentRows(document) {
		if len(group) > 0 {
			previous := group[len(group)-1]
			if row.line != previous.line+1 || row.indent != previous.indent || row.family != previous.family {
				flush()
			}
		}
		group = append(group, row)
	}
	flush()
	return replacements
}

// bindingAlignmentRows accepts only complete single-line declarations with
// one binding per physical line. Inline comments and multiline constructs
// protect their row; trailing comments remain attached and do not split groups.
// Rules: rules/tooling/formatter.md — § 9(8), § 12(9a), § 25, § 29(4), (7).
func bindingAlignmentRows(document cst.Document) []bindingAlignmentRow {
	var rows []bindingAlignmentRow
	for index, element := range document.Elements {
		if !element.HasRole(cst.BindingName) || element.HasRole(cst.BindingMultiline) {
			continue
		}
		row := bindingAlignmentRow{first: index, name: index, typeStart: -1, initializer: -1, last: index, comment: -1, line: element.Token.Line}
		for row.first > 0 {
			previous := document.Elements[row.first-1]
			if previous.Kind == cst.Whitespace {
				if strings.ContainsAny(previous.Text, "\r\n") {
					break
				}
			} else if previous.Token.Line != row.line {
				break
			}
			row.first--
		}
		for row.first < index && document.Elements[row.first].Kind == cst.Whitespace {
			row.first++
		}
		valid := true
		bindings := 0
		for i := row.first; i < len(document.Elements); i++ {
			candidate := document.Elements[i]
			if candidate.Kind == cst.Whitespace {
				if strings.ContainsAny(candidate.Text, "\r\n") {
					break
				}
				continue
			}
			if candidate.Token.Line != row.line {
				break
			}
			endLine, _ := candidate.Token.EndPosition()
			if endLine != row.line {
				valid = false
			}
			if candidate.Kind == cst.Comment {
				if row.comment >= 0 {
					valid = false
				}
				row.comment = i
				continue
			}
			if row.comment >= 0 || candidate.Kind != cst.Token {
				valid = false
			}
			if candidate.HasRole(cst.BindingName) {
				bindings++
			}
			if candidate.HasRole(cst.BindingTypeStart) {
				row.typeStart = i
			}
			if candidate.HasRole(cst.BindingInitializer) {
				row.initializer = i
			}
			row.last = i
		}
		for _, group := range document.Groups {
			if group.Open >= row.first && group.Open <= row.last &&
				(group.Close < 0 || document.Elements[group.Close].Token.Line != row.line) {
				valid = false
			}
		}
		if !valid || bindings != 1 {
			continue
		}
		row.indent = document.Elements[row.first].Token.Column
		row.family = "type-first"
		if row.first == row.name {
			row.family = "named-value"
		} else {
			for i := row.first; i < row.name; i++ {
				if document.Elements[i].Token.Type == lexer.LET {
					row.family = "let"
					break
				}
			}
		}
		if row.typeStart >= 0 {
			row.family += "-typed"
		}
		rows = append(rows, row)
	}
	return rows
}

// alignBindingGroup lays out primary type and secondary initializer columns,
// dropping each over-budget candidate, the trailing-comment column included,
// for the whole group.
// Rules: rules/tooling/formatter.md — § 9(8a)–(10a), § 12(4)–(6a).
func alignBindingGroup(document cst.Document, group []bindingAlignmentRow, maxPadding int) []formatterReplacement {
	if len(group) < 2 {
		return nil
	}
	shifts := make([]int, len(group))
	var replacements []formatterReplacement
	align := func(anchors []int, capped bool) {
		lefts := make([]int, len(group))
		column, count := 0, 0
		for i, anchor := range anchors {
			lefts[i] = -1
			if anchor < 0 {
				continue
			}
			left := previousTokenIndex(document, anchor)
			if left < group[i].first {
				continue
			}
			lefts[i] = left
			column = max(column, elementEndColumn(document.Elements[left])+shifts[i]+1)
			count++
		}
		if count == 0 {
			return
		}
		drop := count < 2 && capped
		for i, left := range lefts {
			if left >= 0 && capped && column-elementEndColumn(document.Elements[left])-shifts[i] > maxPadding {
				drop = true
			}
		}
		for i, left := range lefts {
			if left < 0 {
				continue
			}
			padding := 1
			if !drop {
				padding = column - elementEndColumn(document.Elements[left]) - shifts[i]
			}
			anchor := document.Elements[anchors[i]]
			replacements = append(replacements, formatterReplacement{
				start: document.Elements[left].Span.End, end: anchor.Span.Start, text: strings.Repeat(" ", padding),
			})
			shifts[i] += elementEndColumn(document.Elements[left]) + padding - anchor.Token.Column
		}
	}
	anchors := make([]int, len(group))
	for i, row := range group {
		anchors[i] = row.typeStart
	}
	align(anchors, true)
	for i, row := range group {
		anchors[i] = row.initializer
	}
	align(anchors, true)
	// Include rows without comments in the width of the preceding code cell.
	// The comment column obeys the same cap as the code columns; when any
	// commented row would exceed it, every comment keeps one space (MD-099).
	commentColumn := 0
	for i, row := range group {
		commentColumn = max(commentColumn, elementEndColumn(document.Elements[row.last])+shifts[i]+1)
	}
	dropComments := false
	for i, row := range group {
		if row.comment >= 0 && commentColumn-elementEndColumn(document.Elements[row.last])-shifts[i] > maxPadding {
			dropComments = true
		}
	}
	for i, row := range group {
		if row.comment >= 0 {
			padding := 1
			if !dropComments {
				padding = commentColumn - elementEndColumn(document.Elements[row.last]) - shifts[i]
			}
			replacements = append(replacements, formatterReplacement{
				start: document.Elements[row.last].Span.End, end: document.Elements[row.comment].Span.Start,
				text: strings.Repeat(" ", padding),
			})
		}
	}
	return replacements
}
