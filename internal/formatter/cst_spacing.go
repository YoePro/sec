package formatter

import (
	"sort"
	"strings"

	"sec/internal/cst"
	"sec/internal/lexer"
	"sec/internal/parser"
)

type formatterReplacement struct {
	start int
	end   int
	text  string
}

// formatCSTRoles formats only parser-proven concrete operator roles.
// Identical token spellings in other grammatical positions remain untouched.
//
// Rules:
//   - rules/tooling/formatter.md — §5 "Required syntax representation"
//   - rules/tooling/formatter.md — §6 "Horizontal whitespace"
//   - rules/declarations/generics.md — §12 "Multiple constraints"
//   - rules/foundations/lexical_structure.md — §10 "Contextual operator x"
//   - rules/foundations/operators.md — "Matrix multiplication operator x"
//   - rules/foundations/operators.md — "Increment and decrement aliases"
//   - rules/types/units.md — "Unit metadata"
//   - rules/types/units.md — "Structural unit expressions"
//   - rules/tooling/testing.md — §5.1 "Canonical form"
//   - rules/tooling/testing.md — §41 "Formatter requirements"
//   - rules/memory/ownership.md — §21 "is available and is not available"
//   - rules/tooling/formatter.md — §18 "Control flow"
//   - rules/tooling/formatter.md — §20 "Patterns and destructuring"
func formatCSTRoles(text string) string {
	program := parser.New(lexer.New(text)).ParseProgram()
	document := cst.Build(text, "")
	document.ApplyProgramRoles(program)

	replacements := []formatterReplacement{}
	removedHorizontalTrivia := map[[2]int]bool{}
	removeHorizontalTrivia := func(start, end int) {
		if start >= end {
			return
		}
		span := [2]int{start, end}
		if removedHorizontalTrivia[span] {
			return
		}
		removedHorizontalTrivia[span] = true
		replacements = append(replacements, formatterReplacement{start: start, end: end})
	}
	for _, element := range document.Elements {
		replacementText := ""
		start := element.Span.Start
		end := element.Span.End
		switch {
		case element.HasRole(cst.GenericConstraintConjunction):
			replacementText = " & "
		case element.HasRole(cst.ContextualMatrixOperator):
			replacementText = " x "
		case element.HasRole(cst.PostfixMutationAlias):
			for start > 0 && isHorizontalFormatterByte(text[start-1]) {
				start--
			}
			replacementText = " += 1"
			if element.Token.Type == lexer.DECREMENT {
				replacementText = " -= 1"
			}
			replacements = append(replacements, formatterReplacement{
				start: start,
				end:   end,
				text:  replacementText,
			})
			continue
		case element.HasRole(cst.UnitExpressionCompactToken):
			before := start
			for before > 0 && isHorizontalFormatterByte(text[before-1]) {
				before--
			}
			after := end
			for after < len(text) && isHorizontalFormatterByte(text[after]) {
				after++
			}
			removeHorizontalTrivia(before, start)
			removeHorizontalTrivia(end, after)
			continue
		case element.HasRole(cst.CallableParameterListOpen):
			for start > 0 && isHorizontalFormatterByte(text[start-1]) {
				start--
			}
			replacements = append(replacements, formatterReplacement{
				start: start,
				end:   end,
				text:  element.Text,
			})
			continue
		case element.HasRole(cst.DeclarationGroupSeparator):
			for start > 0 && isHorizontalFormatterByte(text[start-1]) {
				start--
			}
			for end < len(text) && isHorizontalFormatterByte(text[end]) {
				end++
			}
			replacementText = element.Text
			if end < len(text) && text[end] != '\n' && text[end] != '\r' {
				replacementText += " "
			}
			replacements = append(replacements, formatterReplacement{
				start: start,
				end:   end,
				text:  replacementText,
			})
			continue
		case element.HasRole(cst.MatchPatternPayloadOpen):
			for start > 0 && isHorizontalFormatterByte(text[start-1]) {
				start--
			}
			for end < len(text) && isHorizontalFormatterByte(text[end]) {
				end++
			}
			replacements = append(replacements, formatterReplacement{start: start, end: end, text: element.Text})
			continue
		case element.HasRole(cst.MatchPatternPayloadClose):
			tokenStart := start
			for start > 0 && isHorizontalFormatterByte(text[start-1]) {
				start--
			}
			if start == 0 || text[start-1] == '\n' || text[start-1] == '\r' {
				start = tokenStart
			}
			replacements = append(replacements, formatterReplacement{start: start, end: end, text: element.Text})
			continue
		case element.HasRole(cst.MatchPatternFieldsOpen):
			for start > 0 && isHorizontalFormatterByte(text[start-1]) {
				start--
			}
			for end < len(text) && isHorizontalFormatterByte(text[end]) {
				end++
			}
			replacementText = " " + element.Text
			if end < len(text) && text[end] != '\n' && text[end] != '\r' && text[end] != '}' {
				replacementText += " "
			}
			replacements = append(replacements, formatterReplacement{start: start, end: end, text: replacementText})
			continue
		case element.HasRole(cst.MatchPatternFieldsClose):
			tokenStart := start
			for start > 0 && isHorizontalFormatterByte(text[start-1]) {
				start--
			}
			sameLine := start > 0 && text[start-1] != '\n' && text[start-1] != '\r'
			if !sameLine {
				start = tokenStart
			}
			replacementText = element.Text
			if sameLine && text[start-1] != '{' {
				replacementText = " " + replacementText
			}
			replacements = append(replacements, formatterReplacement{start: start, end: end, text: replacementText})
			continue
		case element.HasRole(cst.MatchPatternFieldSeparator):
			for start > 0 && isHorizontalFormatterByte(text[start-1]) {
				start--
			}
			for end < len(text) && isHorizontalFormatterByte(text[end]) {
				end++
			}
			replacementText = element.Text
			if end < len(text) && text[end] != '\n' && text[end] != '\r' {
				replacementText += " "
			}
			replacements = append(replacements, formatterReplacement{start: start, end: end, text: replacementText})
			continue
		case element.HasRole(cst.MatchPatternFieldColon):
			for start > 0 && isHorizontalFormatterByte(text[start-1]) {
				start--
			}
			for end < len(text) && isHorizontalFormatterByte(text[end]) {
				end++
			}
			replacements = append(replacements, formatterReplacement{start: start, end: end, text: ": "})
			continue
		case element.HasRole(cst.MatchGuardKeyword):
			for start > 0 && isHorizontalFormatterByte(text[start-1]) {
				start--
			}
			for end < len(text) && isHorizontalFormatterByte(text[end]) {
				end++
			}
			replacements = append(replacements, formatterReplacement{start: start, end: end, text: " where "})
			continue
		case element.HasRole(cst.HandlerArrow):
			for start > 0 && isHorizontalFormatterByte(text[start-1]) {
				start--
			}
			for end < len(text) && isHorizontalFormatterByte(text[end]) {
				end++
			}
			replacements = append(replacements, formatterReplacement{start: start, end: end, text: " => "})
			continue
		case element.HasRole(cst.SwitchCaseSeparator):
			for start > 0 && isHorizontalFormatterByte(text[start-1]) {
				start--
			}
			for end < len(text) && isHorizontalFormatterByte(text[end]) {
				end++
			}
			replacementText = element.Text
			if end < len(text) && text[end] != '\n' && text[end] != '\r' {
				replacementText += " "
			}
			replacements = append(replacements, formatterReplacement{start: start, end: end, text: replacementText})
			continue
		case element.HasRole(cst.SwitchCaseColon):
			for start > 0 && isHorizontalFormatterByte(text[start-1]) {
				start--
			}
			replacements = append(replacements, formatterReplacement{start: start, end: end, text: element.Text})
			continue
		case element.HasRole(cst.UnitMetadataName):
			canonical, known := canonicalUnitMetadataName(element.Text)
			if !known || canonical == element.Text {
				continue
			}
			replacements = append(replacements, formatterReplacement{
				start: start,
				end:   end,
				text:  canonical,
			})
			continue
		case element.HasRole(cst.TestDeclarationName):
			for start > 0 && isHorizontalFormatterByte(text[start-1]) {
				start--
			}
			for end < len(text) && isHorizontalFormatterByte(text[end]) {
				end++
			}
			replacements = append(replacements, formatterReplacement{
				start: start,
				end:   end,
				text:  " " + element.Text + " ",
			})
			continue
		case element.HasRole(cst.AvailabilityBlockOpen):
			for start > 0 && isHorizontalFormatterByte(text[start-1]) {
				start--
			}
			replacements = append(replacements, formatterReplacement{
				start: start,
				end:   end,
				text:  " {",
			})
			continue
		default:
			continue
		}

		for start > 0 && isHorizontalFormatterByte(text[start-1]) {
			start--
		}
		for end < len(text) && isHorizontalFormatterByte(text[end]) {
			end++
		}
		replacements = append(replacements, formatterReplacement{
			start: start,
			end:   end,
			text:  replacementText,
		})
	}

	sort.Slice(replacements, func(i, j int) bool { return replacements[i].start > replacements[j].start })
	for _, replacement := range replacements {
		text = text[:replacement.start] + replacement.text + text[replacement.end:]
	}
	return text
}

// canonicalUnitMetadataName maps every accepted migration spelling to the
// closed canonical metadata inventory.
//
// Rules:
//   - rules/types/units.md — "Unit metadata"
func canonicalUnitMetadataName(name string) (string, bool) {
	normalized := strings.ReplaceAll(strings.ToLower(name), "_", "")
	names := map[string]string{
		"longname": "LongName", "symbol": "Symbol", "baseunit": "BaseUnit",
		"status": "Status", "dimension": "Dimension", "kind": "Kind",
		"scale": "Scale", "system": "System", "transform": "Transform",
		"offset": "Offset", "origin": "Origin", "logbase": "LogBase",
		"logfactor": "LogFactor", "reference": "Reference",
	}
	canonical, ok := names[normalized]
	return canonical, ok
}

func isHorizontalFormatterByte(value byte) bool {
	return value == ' ' || value == '\t'
}
