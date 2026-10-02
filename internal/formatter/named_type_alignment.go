package formatter

import (
	"strings"
	"unicode/utf8"

	"sec/internal/cst"
)

type namedTypeAlignmentAnchor struct {
	nameEnd        int
	baseStart      int
	contractPrefix int
	contractStart  int
	line           int
}

// namedTypeAlignmentReplacements aligns the base-type and first contract or
// default anchors of contiguous simple named-type declarations, the compact
// homogeneous declaration block permitted by the formatter rulebook. Anchors
// come from parser-confirmed CST roles, never from text scanning.
//
// Rules:
//   - rules/tooling/formatter.md — §7(6) adjacent simple named-type declarations
//   - rules/tooling/formatter.md — §9(1–2) syntactic alignment anchors
//   - rules/tooling/formatter.md — §9(8–10) group boundaries and padding limit
func namedTypeAlignmentReplacements(document cst.Document) []formatterReplacement {
	replacements := []formatterReplacement{}
	group := []namedTypeAlignmentAnchor{}
	flush := func() {
		replacements = append(replacements, alignNamedTypeGroup(document, group)...)
		group = group[:0]
	}
	for _, anchor := range namedTypeAlignmentAnchors(document) {
		if len(group) > 0 && !namedTypeAnchorsAreContiguous(document, group[len(group)-1], anchor) {
			flush()
		}
		group = append(group, anchor)
	}
	flush()
	return replacements
}

func namedTypeAlignmentAnchors(document cst.Document) []namedTypeAlignmentAnchor {
	anchors := []namedTypeAlignmentAnchor{}
	for baseIndex, element := range document.Elements {
		if !element.HasRole(cst.NamedTypeBaseStart) {
			continue
		}
		nameEnd := previousTokenIndex(document, baseIndex)
		if nameEnd < 0 || document.Elements[nameEnd].Token.Line != element.Token.Line {
			continue
		}
		anchor := namedTypeAlignmentAnchor{nameEnd: nameEnd, baseStart: baseIndex, contractPrefix: -1, contractStart: -1, line: element.Token.Line}
		for index := baseIndex + 1; index < len(document.Elements); index++ {
			candidate := document.Elements[index]
			if candidate.Kind == cst.Comment || candidate.Kind == cst.Token && candidate.Token.Line != element.Token.Line {
				break
			}
			if candidate.HasRole(cst.NamedTypeContractStart) {
				anchor.contractStart = index
				anchor.contractPrefix = previousTokenIndex(document, index)
				break
			}
		}
		anchors = append(anchors, anchor)
	}
	return anchors
}

func previousTokenIndex(document cst.Document, index int) int {
	for previous := index - 1; previous >= 0; previous-- {
		if document.Elements[previous].Kind == cst.Token {
			return previous
		}
	}
	return -1
}

func namedTypeAnchorsAreContiguous(document cst.Document, previous, current namedTypeAlignmentAnchor) bool {
	if current.line != previous.line+1 {
		return false
	}
	for index := previous.baseStart + 1; index < current.nameEnd; index++ {
		element := document.Elements[index]
		if element.Kind == cst.Comment {
			return false
		}
		if element.Kind == cst.Whitespace && strings.Count(element.Text, "\n") > 1 {
			return false
		}
	}
	return true
}

func elementEndColumn(element cst.Element) int {
	return element.Token.Column + utf8.RuneCountInString(element.Text)
}

func alignNamedTypeGroup(document cst.Document, group []namedTypeAlignmentAnchor) []formatterReplacement {
	if len(group) < 2 {
		return nil
	}
	baseColumn := 0
	for _, anchor := range group {
		if column := elementEndColumn(document.Elements[anchor.nameEnd]) + 1; column > baseColumn {
			baseColumn = column
		}
	}
	for _, anchor := range group {
		if baseColumn-elementEndColumn(document.Elements[anchor.nameEnd]) > defaultMaximumAlignmentPadding {
			return nil
		}
	}
	replacements := make([]formatterReplacement, 0, 2*len(group))
	for _, anchor := range group {
		nameEnd := document.Elements[anchor.nameEnd]
		replacements = append(replacements, formatterReplacement{
			start: nameEnd.Span.End,
			end:   document.Elements[anchor.baseStart].Span.Start,
			text:  strings.Repeat(" ", baseColumn-elementEndColumn(nameEnd)),
		})
	}
	return append(replacements, alignNamedTypeContracts(document, group, baseColumn)...)
}

// alignNamedTypeContracts treats the first contract or default clause as the
// secondary anchor. Failure to fit it never discards the base-type column.
func alignNamedTypeContracts(document cst.Document, group []namedTypeAlignmentAnchor, baseColumn int) []formatterReplacement {
	prefixEnd := func(anchor namedTypeAlignmentAnchor) int {
		shift := baseColumn - document.Elements[anchor.baseStart].Token.Column
		return elementEndColumn(document.Elements[anchor.contractPrefix]) + shift
	}
	contracted := []namedTypeAlignmentAnchor{}
	contractColumn := 0
	for _, anchor := range group {
		if anchor.contractStart < 0 || anchor.contractPrefix < 0 {
			continue
		}
		contracted = append(contracted, anchor)
		if column := prefixEnd(anchor) + 1; column > contractColumn {
			contractColumn = column
		}
	}
	if len(contracted) < 2 {
		return nil
	}
	for _, anchor := range contracted {
		if contractColumn-prefixEnd(anchor) > defaultMaximumAlignmentPadding {
			return nil
		}
	}
	replacements := make([]formatterReplacement, 0, len(contracted))
	for _, anchor := range contracted {
		replacements = append(replacements, formatterReplacement{
			start: document.Elements[anchor.contractPrefix].Span.End,
			end:   document.Elements[anchor.contractStart].Span.Start,
			text:  strings.Repeat(" ", contractColumn-prefixEnd(anchor)),
		})
	}
	return replacements
}
