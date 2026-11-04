package formatter

import (
	"sort"
	"strings"
	"unicode/utf8"

	"sec/internal/cst"
	"sec/internal/lexer"
	"sec/internal/parser"
)

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
// Single-line lists that fit, lists with comments, and lists with a parameter that
// itself spans lines are left unchanged. Run after structural indentation and
// executable-block expansion so width decisions use canonical header columns.
//
// Rules:
//   - rules/tooling/formatter.md — §4(9–13) effective preferred width
//   - rules/tooling/formatter.md — § 3(6)–(7), § 11(2)–(5), § 16(1)–(2), § 32(3), § 32(6)
func formatCallableParameterListLayout(text string, preferredLineWidth uint64, indentationWidth int) string {
	if !strings.Contains(text, "(") {
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
		if replacement, ok := parameterListLayoutReplacement(text, document, group.Open, group.Close, preferredLineWidth, indentationWidth); ok {
			replacements = append(replacements, replacement)
		}
	}
	sort.SliceStable(replacements, func(i, j int) bool { return replacements[i].start > replacements[j].start })
	for _, replacement := range replacements {
		text = text[:replacement.start] + replacement.text + text[replacement.end:]
	}
	return text
}

// parameterListLayoutReplacement uses canonical header columns and real CST
// delimiters to choose a compact or multiline parameter list. Comments and
// multiline parameter values retain their layout; deliberate lists stay open.
// Rules: rules/tooling/formatter.md — §3(7),§4(9–13),§9(8),§16(1–2),§25.
func parameterListLayoutReplacement(text string, document cst.Document, open, close int, preferredLineWidth uint64, indentationWidth int) (formatterReplacement, bool) {
	interiorStart, interiorEnd := document.Elements[open].Span.End, document.Elements[close].Span.Start
	interior := text[interiorStart:interiorEnd]
	if strings.TrimSpace(interior) == "" {
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
		width := indent + utf8.RuneCountInString(singleLine) + utf8.RuneCountInString(strings.Join(parameters, ", ")) + utf8.RuneCountInString(strings.TrimRight(text[interiorEnd:lineEnd], " \t"))
		if uint64(width) <= preferredLineWidth {
			return formatterReplacement{start: interiorStart, end: interiorEnd, text: strings.Join(parameters, ", ")}, true
		}
	}
	var multiline strings.Builder
	lineStart := strings.LastIndexByte(text[:interiorStart], '\n') + 1
	header := text[lineStart:interiorStart]
	baseIndent := header[:len(header)-len(strings.TrimLeft(header, " \t"))]
	multiline.WriteString("\n")
	for index, parameter := range parameters {
		if blankBefore[index] {
			multiline.WriteString("\n")
		}
		multiline.WriteString(baseIndent + strings.Repeat(" ", indentationWidth) + parameter)
		multiline.WriteString(",\n")
	}
	multiline.WriteString(baseIndent)
	return formatterReplacement{start: interiorStart, end: interiorEnd, text: multiline.String()}, true
}
