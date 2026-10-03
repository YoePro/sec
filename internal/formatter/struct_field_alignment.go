package formatter

import (
	"strings"
	"unicode/utf8"

	"sec/internal/cst"
	"sec/internal/lexer"
)

const defaultMaximumAlignmentPadding = 16

type structFieldAlignmentAnchor struct {
	colon     int
	typeStart int
	tag       int
	tagPrefix int
	line      int
}

// structFieldAlignmentReplacements aligns the declared-type anchor of each
// contiguous parser-confirmed struct-field group. It consumes the same CST as
// the other role-based formatter passes and never infers fields from text.
//
// Rules:
//   - rules/tooling/formatter.md — §9(1–2) syntactic alignment anchors
//   - rules/tooling/formatter.md — §9(5–6) struct fields and tags
//   - rules/tooling/formatter.md — §9(8–10) group boundaries and padding limit
func structFieldAlignmentReplacements(document cst.Document, maxPadding int) []formatterReplacement {
	anchors := structFieldAlignmentAnchors(document)
	replacements := []formatterReplacement{}
	group := []structFieldAlignmentAnchor{}
	flush := func() {
		replacements = append(replacements, alignStructFieldGroup(document, group, maxPadding)...)
		group = group[:0]
	}

	for _, anchor := range anchors {
		if len(group) > 0 && !structFieldAnchorsAreContiguous(document, group[len(group)-1], anchor) {
			flush()
		}
		group = append(group, anchor)
	}
	flush()
	return replacements
}

func structFieldAlignmentAnchors(document cst.Document) []structFieldAlignmentAnchor {
	anchors := []structFieldAlignmentAnchor{}
	for typeIndex, element := range document.Elements {
		if !element.HasRole(cst.StructFieldTypeStart) {
			continue
		}
		colonIndex := typeIndex - 1
		for colonIndex >= 0 && document.Elements[colonIndex].Kind != cst.Token {
			colonIndex--
		}
		if colonIndex < 0 || !document.Elements[colonIndex].HasRole(cst.StructFieldColon) ||
			document.Elements[colonIndex].Token.Line != element.Token.Line {
			continue
		}
		tagIndex, tagPrefixIndex := -1, -1
		previousToken := typeIndex
		for index := typeIndex + 1; index < len(document.Elements); index++ {
			candidate := document.Elements[index]
			if candidate.Kind == cst.Token && candidate.Token.Line != element.Token.Line {
				break
			}
			if candidate.HasRole(cst.StructFieldTag) {
				tagIndex, tagPrefixIndex = index, previousToken
				break
			}
			if candidate.Kind == cst.Token {
				previousToken = index
			}
		}
		anchors = append(anchors, structFieldAlignmentAnchor{
			colon: colonIndex, typeStart: typeIndex, tag: tagIndex,
			tagPrefix: tagPrefixIndex, line: element.Token.Line,
		})
	}
	return anchors
}

func structFieldAnchorsAreContiguous(document cst.Document, previous, current structFieldAlignmentAnchor) bool {
	if current.line != previous.line+1 {
		return false
	}
	for index := previous.typeStart + 1; index < current.colon; index++ {
		element := document.Elements[index]
		if element.Kind == cst.Comment || (element.Kind == cst.Token &&
			(element.Token.Type == lexer.LBRACE || element.Token.Type == lexer.RBRACE)) {
			return false
		}
		if element.Kind == cst.Whitespace && strings.Count(strings.ReplaceAll(element.Text, "\r\n", "\n"), "\n") > 1 {
			return false
		}
	}
	return true
}

func alignStructFieldGroup(document cst.Document, group []structFieldAlignmentAnchor, maxPadding int) []formatterReplacement {
	if len(group) == 0 {
		return nil
	}
	maximumColonColumn := 0
	for _, anchor := range group {
		column := document.Elements[anchor.colon].Token.Column
		if column > maximumColonColumn {
			maximumColonColumn = column
		}
	}
	typeColumn := maximumColonColumn + 2
	for _, anchor := range group {
		colon := document.Elements[anchor.colon]
		padding := typeColumn - (colon.Token.Column + 1)
		if padding > maxPadding {
			return unalignedStructFieldTagReplacements(document, group)
		}
	}

	replacements := make([]formatterReplacement, 0, len(group))
	for _, anchor := range group {
		colon := document.Elements[anchor.colon]
		typeStart := document.Elements[anchor.typeStart]
		padding := typeColumn - (colon.Token.Column + 1)
		replacements = append(replacements, formatterReplacement{
			start: colon.Span.End,
			end:   typeStart.Span.Start,
			text:  strings.Repeat(" ", padding),
		})
	}
	replacements = append(replacements, alignStructFieldTags(document, group, typeColumn, maxPadding)...)
	return replacements
}

// alignStructFieldTags treats tags as the secondary §9 anchor. Failure to fit
// the secondary column never discards the already-proven primary type column.
func alignStructFieldTags(document cst.Document, group []structFieldAlignmentAnchor, typeColumn int, maxPadding int) []formatterReplacement {
	tagged := make([]structFieldAlignmentAnchor, 0, len(group))
	maximumPrefixEndColumn := 0
	for _, anchor := range group {
		if anchor.tag < 0 || anchor.tagPrefix < 0 {
			continue
		}
		tagged = append(tagged, anchor)
		endColumn := structFieldTagPrefixEndColumn(document, anchor, typeColumn)
		if endColumn > maximumPrefixEndColumn {
			maximumPrefixEndColumn = endColumn
		}
	}
	if len(tagged) < 2 {
		return unalignedStructFieldTagReplacements(document, tagged)
	}
	tagColumn := maximumPrefixEndColumn + 1
	for _, anchor := range tagged {
		prefixEndColumn := structFieldTagPrefixEndColumn(document, anchor, typeColumn)
		if tagColumn-prefixEndColumn > maxPadding {
			return unalignedStructFieldTagReplacements(document, tagged)
		}
	}

	replacements := make([]formatterReplacement, 0, len(tagged))
	for _, anchor := range tagged {
		prefix := document.Elements[anchor.tagPrefix]
		tag := document.Elements[anchor.tag]
		prefixEndColumn := structFieldTagPrefixEndColumn(document, anchor, typeColumn)
		replacements = append(replacements, formatterReplacement{
			start: prefix.Span.End,
			end:   tag.Span.Start,
			text:  strings.Repeat(" ", tagColumn-prefixEndColumn),
		})
	}
	return replacements
}

func unalignedStructFieldTagReplacements(document cst.Document, group []structFieldAlignmentAnchor) []formatterReplacement {
	replacements := []formatterReplacement{}
	for _, anchor := range group {
		if anchor.tag < 0 || anchor.tagPrefix < 0 {
			continue
		}
		prefix := document.Elements[anchor.tagPrefix]
		tag := document.Elements[anchor.tag]
		replacements = append(replacements, formatterReplacement{
			start: prefix.Span.End,
			end:   tag.Span.Start,
			text:  " ",
		})
	}
	return replacements
}

func structFieldTagPrefixEndColumn(document cst.Document, anchor structFieldAlignmentAnchor, typeColumn int) int {
	typeStart := document.Elements[anchor.typeStart]
	prefix := document.Elements[anchor.tagPrefix]
	typeShift := typeColumn - typeStart.Token.Column
	return prefix.Token.Column + utf8.RuneCountInString(prefix.Text) + typeShift
}
