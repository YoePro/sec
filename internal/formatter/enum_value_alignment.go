package formatter

import (
	"strings"

	"sec/internal/cst"
	"sec/internal/lexer"
)

type enumValueAlignmentAnchor struct {
	assignment int
	line       int
}

// enumValueAlignmentReplacements aligns parser-confirmed explicit enum-value
// assignments in contiguous homogeneous groups. Standalone comments, blank
// lines, implicit members and declaration boundaries split a group; no
// padding limit disables it. Member order and initializer spelling never
// change;
// a trailing comment belongs to its member and does not split the group.
//
// Rules:
//   - rules/tooling/formatter.md — §9(1–2) syntactic alignment anchors
//   - rules/tooling/formatter.md — §9(8) group boundaries
//   - rules/tooling/formatter.md — §15(4) enum explicit-value assignments, always aligned (MD-100)
func enumValueAlignmentReplacements(document cst.Document, maxPadding int) []formatterReplacement {
	anchors := enumValueAlignmentAnchors(document)
	replacements := []formatterReplacement{}
	group := []enumValueAlignmentAnchor{}
	flush := func() {
		replacements = append(replacements, alignEnumValueGroup(document, group, maxPadding)...)
		group = group[:0]
	}
	for _, anchor := range anchors {
		if len(group) > 0 && !enumValueAnchorsAreContiguous(document, group[len(group)-1], anchor) {
			flush()
		}
		group = append(group, anchor)
	}
	flush()
	return replacements
}

func enumValueAlignmentAnchors(document cst.Document) []enumValueAlignmentAnchor {
	anchors := []enumValueAlignmentAnchor{}
	for index, element := range document.Elements {
		if element.HasRole(cst.EnumValueAssignment) {
			anchors = append(anchors, enumValueAlignmentAnchor{assignment: index, line: element.Token.Line})
		}
	}
	return anchors
}

func enumValueAnchorsAreContiguous(document cst.Document, previous, current enumValueAlignmentAnchor) bool {
	if current.line != previous.line+1 {
		return false
	}
	lineBreakSeen := false
	for index := previous.assignment + 1; index < current.assignment; index++ {
		element := document.Elements[index]
		if element.Kind == cst.Token && (element.Token.Type == lexer.LBRACE || element.Token.Type == lexer.RBRACE) {
			return false
		}
		// A trailing comment belongs to its member and keeps the group; a
		// comment after the line break is standalone and ends it (§ 9(8)).
		if element.Kind == cst.Comment && lineBreakSeen {
			return false
		}
		if element.Kind == cst.Whitespace {
			breaks := strings.Count(strings.ReplaceAll(element.Text, "\r\n", "\n"), "\n")
			if breaks > 1 {
				return false
			}
			if breaks > 0 {
				lineBreakSeen = true
			}
		}
	}
	return true
}

func alignEnumValueGroup(document cst.Document, group []enumValueAlignmentAnchor, maxPadding int) []formatterReplacement {
	if len(group) == 0 {
		return nil
	}
	targetColumn := 0
	for _, anchor := range group {
		previous := previousEnumValueToken(document, anchor)
		if previous < 0 {
			return nil
		}
		_, endColumn := document.Elements[previous].Token.EndPosition()
		if endColumn+1 > targetColumn {
			targetColumn = endColumn + 1
		}
	}
	// Enum value tables always keep one `=` column: the § 9(10) padding
	// limit does not apply to them (MD-100, user decision 2026-10-17).
	_ = maxPadding

	replacements := []formatterReplacement{}
	for _, anchor := range group {
		assignment := document.Elements[anchor.assignment]
		previous := previousEnumValueToken(document, anchor)
		if previous < 0 {
			continue
		}
		previousElement := document.Elements[previous]
		_, endColumn := previousElement.Token.EndPosition()
		padding := 1
		if targetColumn > 0 {
			padding = targetColumn - endColumn
			if padding < 1 {
				padding = 1
			}
		}
		replacements = append(replacements, formatterReplacement{
			start: previousElement.Span.End,
			end:   assignment.Span.Start,
			text:  strings.Repeat(" ", padding),
		})
		if next := nextEnumValueToken(document, anchor); next >= 0 {
			nextElement := document.Elements[next]
			if nextElement.Token.Line == assignment.Token.Line {
				replacements = append(replacements, formatterReplacement{
					start: assignment.Span.End,
					end:   nextElement.Span.Start,
					text:  " ",
				})
			}
		}
	}
	return replacements
}

func previousEnumValueToken(document cst.Document, anchor enumValueAlignmentAnchor) int {
	for index := anchor.assignment - 1; index >= 0; index-- {
		if document.Elements[index].Kind != cst.Token {
			continue
		}
		if document.Elements[index].Token.Line != anchor.line {
			return -1
		}
		return index
	}
	return -1
}

func nextEnumValueToken(document cst.Document, anchor enumValueAlignmentAnchor) int {
	for index := anchor.assignment + 1; index < len(document.Elements); index++ {
		if document.Elements[index].Kind == cst.Token {
			return index
		}
	}
	return -1
}
