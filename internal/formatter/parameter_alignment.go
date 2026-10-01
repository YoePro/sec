package formatter

import (
	"strings"

	"sec/internal/cst"
)

type parameterAlignmentAnchor struct {
	colon     int
	typeStart int
	line      int
	groupOpen int
}

// parameterAlignmentReplacements aligns type starts in contiguous multiline
// callable parameter groups using parser-owned CST anchors. Single-line lists,
// recovered parameters, blank lines, and excessive padding remain structural
// boundaries rather than formatter guesses; comment-bearing lists without
// complete parser roles stay conservatively unchanged.
//
// Rules:
//   - rules/tooling/formatter.md — §9(1–2), §9(8–10)
//   - rules/tooling/formatter.md — §16(2–3) "multiline parameter lists"
//   - rules/tooling/formatter.md — §21(11) ownership markers as prefixes
func parameterAlignmentReplacements(document cst.Document) []formatterReplacement {
	anchors := parameterAlignmentAnchors(document)
	replacements := []formatterReplacement{}
	group := []parameterAlignmentAnchor{}
	flush := func() {
		replacements = append(replacements, alignParameterGroup(document, group)...)
		group = group[:0]
	}

	for _, anchor := range anchors {
		if len(group) > 0 && !parameterAnchorsAreContiguous(document, group[len(group)-1], anchor) {
			flush()
		}
		group = append(group, anchor)
	}
	flush()
	return replacements
}

func parameterAlignmentAnchors(document cst.Document) []parameterAlignmentAnchor {
	anchors := []parameterAlignmentAnchor{}
	for typeIndex, element := range document.Elements {
		if !element.HasRole(cst.CallableParameterTypeStart) {
			continue
		}
		colonIndex := typeIndex - 1
		for colonIndex >= 0 && document.Elements[colonIndex].Kind != cst.Token {
			colonIndex--
		}
		if colonIndex < 0 || !document.Elements[colonIndex].HasRole(cst.CallableParameterColon) ||
			document.Elements[colonIndex].Token.Line != element.Token.Line {
			continue
		}
		groupOpen, groupClose := callableParameterGroup(document, colonIndex)
		if groupOpen < 0 || document.Elements[groupOpen].Token.Line == document.Elements[groupClose].Token.Line {
			continue
		}
		anchors = append(anchors, parameterAlignmentAnchor{
			colon: colonIndex, typeStart: typeIndex, line: element.Token.Line, groupOpen: groupOpen,
		})
	}
	return anchors
}

func callableParameterGroup(document cst.Document, element int) (int, int) {
	for _, group := range document.Groups {
		if group.Open < element && element < group.Close &&
			document.Elements[group.Open].HasRole(cst.CallableParameterListOpen) {
			return group.Open, group.Close
		}
	}
	return -1, -1
}

func parameterAnchorsAreContiguous(document cst.Document, previous, current parameterAlignmentAnchor) bool {
	if current.groupOpen != previous.groupOpen || current.line != previous.line+1 {
		return false
	}
	for index := previous.typeStart + 1; index < current.colon; index++ {
		element := document.Elements[index]
		if element.Kind == cst.Comment {
			return false
		}
		if element.Kind == cst.Whitespace && strings.Count(strings.ReplaceAll(element.Text, "\r\n", "\n"), "\n") > 1 {
			return false
		}
	}
	return true
}

func alignParameterGroup(document cst.Document, group []parameterAlignmentAnchor) []formatterReplacement {
	if len(group) == 0 {
		return nil
	}
	maximumColonColumn := 0
	for _, anchor := range group {
		if column := document.Elements[anchor.colon].Token.Column; column > maximumColonColumn {
			maximumColonColumn = column
		}
	}
	typeColumn := maximumColonColumn + 2
	for _, anchor := range group {
		colon := document.Elements[anchor.colon]
		if typeColumn-(colon.Token.Column+1) > defaultMaximumAlignmentPadding {
			return unalignedParameterReplacements(document, group)
		}
	}

	replacements := make([]formatterReplacement, 0, len(group))
	for _, anchor := range group {
		colon := document.Elements[anchor.colon]
		typeStart := document.Elements[anchor.typeStart]
		replacements = append(replacements, formatterReplacement{
			start: colon.Span.End,
			end:   typeStart.Span.Start,
			text:  strings.Repeat(" ", typeColumn-(colon.Token.Column+1)),
		})
	}
	return replacements
}

func unalignedParameterReplacements(document cst.Document, group []parameterAlignmentAnchor) []formatterReplacement {
	replacements := make([]formatterReplacement, 0, len(group))
	for _, anchor := range group {
		replacements = append(replacements, formatterReplacement{
			start: document.Elements[anchor.colon].Span.End,
			end:   document.Elements[anchor.typeStart].Span.Start,
			text:  " ",
		})
	}
	return replacements
}
