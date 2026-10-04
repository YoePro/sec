package formatter

import (
	"sort"
	"strings"

	"sec/internal/cst"
	"sec/internal/lexer"
	"sec/internal/parser"
)

// preferredLineWidth is the fixed Sec 0.1 preferred width (§ 32(2)).
const preferredLineWidth = 120

// formatCallableParameterListLayout normalizes parser-owned function, method,
// initializer, and lambda parameter lists that are written across lines:
//
//   - a list whose first parameter starts on the line after `(` is a
//     deliberate multiline form and becomes the canonical one: one parameter
//     per line, a trailing comma, and `)` on its own line (§ 16(2), § 11(3));
//   - a mixed list whose first parameter hugs `(` becomes single-line when the
//     whole line then fits the preferred width (§ 16(1), § 3(7)), and the
//     canonical multiline form otherwise.
//
// Single-line lists, lists with comments, and lists with a parameter that
// itself spans lines are left unchanged; indentation is applied by the
// line-based indentation pass.
//
// Rules:
//   - rules/tooling/formatter.md — § 3(6)–(7), § 11(2)–(5), § 16(1)–(2), § 32(3), § 32(6)
func formatCallableParameterListLayout(text string) string {
	if !strings.Contains(text, "(") || !strings.Contains(text, "\n") {
		return text
	}
	document := cst.Build(text, "")
	if hasUncertainConcreteSyntax(document) {
		return text
	}
	p := parser.New(lexer.New(text))
	program := p.ParseProgram()
	if len(p.Errors()) > 0 || program == nil {
		return text
	}
	document.ApplyProgramRoles(program)

	replacements := []formatterReplacement{}
	for _, group := range document.Groups {
		if group.Close < 0 || !document.Elements[group.Open].HasRole(cst.CallableParameterListOpen) {
			continue
		}
		if replacement, ok := parameterListLayoutReplacement(text, document, group.Open, group.Close); ok {
			replacements = append(replacements, replacement)
		}
	}
	sort.SliceStable(replacements, func(i, j int) bool { return replacements[i].start > replacements[j].start })
	for _, replacement := range replacements {
		text = text[:replacement.start] + replacement.text + text[replacement.end:]
	}
	return text
}

func parameterListLayoutReplacement(text string, document cst.Document, open, close int) (formatterReplacement, bool) {
	interiorStart, interiorEnd := document.Elements[open].Span.End, document.Elements[close].Span.Start
	interior := text[interiorStart:interiorEnd]
	if !strings.Contains(interior, "\n") || strings.TrimSpace(interior) == "" {
		return formatterReplacement{}, false
	}
	// Split at top-level commas; comments keep their author's layout.
	items := []string{}
	depth := 0
	itemStart := interiorStart
	for index := open + 1; index < close; index++ {
		element := document.Elements[index]
		if element.Kind == cst.Comment {
			return formatterReplacement{}, false
		}
		if element.Kind != cst.Token {
			continue
		}
		switch element.Token.Type {
		case lexer.LPAREN, lexer.LBRACKET, lexer.LBRACE:
			depth++
		case lexer.RPAREN, lexer.RBRACKET, lexer.RBRACE:
			depth--
		case lexer.COMMA:
			if depth == 0 {
				items = append(items, text[itemStart:element.Span.Start])
				itemStart = element.Span.End
			}
		}
	}
	items = append(items, text[itemStart:interiorEnd])
	parameters := make([]string, 0, len(items))
	// blankBefore marks a parameter preceded by a blank line, which splits
	// alignment groups and is preserved (§ 9(8)).
	blankBefore := map[int]bool{}
	for index, item := range items {
		leading := item[:len(item)-len(strings.TrimLeft(item, " \t\r\n"))]
		if index > 0 && strings.Count(leading, "\n") >= 2 {
			blankBefore[len(parameters)] = true
		}
		item = strings.TrimSpace(item)
		if item == "" {
			if index == len(items)-1 {
				continue // existing trailing comma
			}
			return formatterReplacement{}, false
		}
		if strings.ContainsAny(item, "\n\r") {
			return formatterReplacement{}, false
		}
		parameters = append(parameters, item)
	}
	if len(parameters) == 0 {
		return formatterReplacement{}, false
	}

	firstBreak := strings.TrimLeft(interior, " \t")
	deliberateMultiline := strings.HasPrefix(firstBreak, "\n") || strings.HasPrefix(firstBreak, "\r")
	if !deliberateMultiline && len(blankBefore) == 0 {
		lineStart := strings.LastIndexByte(text[:interiorStart], '\n') + 1
		lineEnd := strings.IndexByte(text[interiorEnd:], '\n')
		if lineEnd < 0 {
			lineEnd = len(text)
		} else {
			lineEnd += interiorEnd
		}
		singleLine := strings.TrimLeft(text[lineStart:interiorStart], " \t")
		indent := len(text[lineStart:interiorStart]) - len(singleLine)
		width := indent + len(singleLine) + len(strings.Join(parameters, ", ")) + len(strings.TrimRight(text[interiorEnd:lineEnd], " \t"))
		if width <= preferredLineWidth {
			return formatterReplacement{start: interiorStart, end: interiorEnd, text: strings.Join(parameters, ", ")}, true
		}
	}
	var multiline strings.Builder
	multiline.WriteString("\n")
	for index, parameter := range parameters {
		if blankBefore[index] {
			multiline.WriteString("\n")
		}
		multiline.WriteString(parameter)
		multiline.WriteString(",\n")
	}
	return formatterReplacement{start: interiorStart, end: interiorEnd, text: multiline.String()}, true
}
