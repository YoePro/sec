package formatter

import (
	"sort"
	"strings"

	"sec/internal/ast"
	"sec/internal/cst"
	"sec/internal/lexer"
	"sec/internal/parser"
)

// formatPropertyAccessorLayout places every accessor of an impl property with
// accessor bodies on its own line and the property's closing brace on its own
// line. Compressed forms such as `property X: int { get {` ... `} }` otherwise
// put two structural braces on one physical line, which the line-oriented
// indentation pass cannot balance. Only line breaks are inserted; the later
// passes own indentation and executable-block expansion.
//
// Rules:
//   - rules/tooling/formatter.md — §16(7) property layout, §16(9) fallible setter
//   - rules/tooling/formatter.md — §8(1), §8(4) executable blocks are multiline
//   - rules/tooling/formatter.md — §25 "Malformed and incomplete source"
func formatPropertyAccessorLayout(text string) string {
	document := cst.Build(text, "")
	if hasUncertainConcreteSyntax(document) {
		return text
	}
	program := parser.New(lexer.New(text)).ParseProgram()
	byOffset := map[int]int{}
	for index, element := range document.Elements {
		if element.Kind == cst.Token {
			byOffset[element.Span.Start] = index
		}
	}
	closeOf := map[int]int{}
	for _, group := range document.Groups {
		if group.Close >= 0 {
			closeOf[group.Open] = group.Close
		}
	}
	// previousToken returns the nearest real token before index and whether a
	// line break or comment separates them.
	previousToken := func(index int) (int, bool) {
		separated := false
		for current := index - 1; current >= 0; current-- {
			element := document.Elements[current]
			switch element.Kind {
			case cst.Token:
				return current, separated
			case cst.Whitespace:
				if strings.ContainsAny(element.Text, "\n\r") {
					separated = true
				}
			default:
				separated = true
			}
		}
		return -1, true
	}

	breaks := map[int]int{}
	breakBefore := func(index int) {
		previous, separated := previousToken(index)
		if previous < 0 || separated {
			return
		}
		breaks[document.Elements[previous].Span.End] = document.Elements[index].Span.Start
	}
	for _, statement := range program.Statements {
		impl, ok := statement.(*ast.ImplStatement)
		if !ok {
			continue
		}
		for _, member := range impl.Members {
			property, ok := member.(*ast.PropertyDeclaration)
			if !ok || property.Name == nil {
				continue
			}
			accessors := []int{}
			if property.Getter != nil && property.Getter.Token.Type == lexer.LBRACE {
				if open, ok := byOffset[property.Getter.Token.ByteStart]; ok {
					if keyword, _ := previousToken(open); keyword >= 0 {
						accessors = append(accessors, keyword)
					}
				}
			}
			if property.Setter != nil && property.Setter.Body != nil {
				if keyword, ok := byOffset[property.Setter.Token.ByteStart]; ok {
					// §16(9): a fallible setter starts at its try keyword.
					if previous, separated := previousToken(keyword); previous >= 0 && !separated &&
						document.Elements[previous].Token.Type == lexer.TRY {
						keyword = previous
					}
					accessors = append(accessors, keyword)
				}
			}
			if len(accessors) == 0 {
				continue
			}
			sort.Ints(accessors)
			bodyOpen, _ := previousToken(accessors[0])
			if bodyOpen < 0 || document.Elements[bodyOpen].Token.Type != lexer.LBRACE {
				continue
			}
			bodyClose, ok := closeOf[bodyOpen]
			if !ok {
				continue
			}
			for _, accessor := range accessors {
				breakBefore(accessor)
			}
			breakBefore(bodyClose)
		}
	}

	starts := make([]int, 0, len(breaks))
	for start := range breaks {
		starts = append(starts, start)
	}
	sort.Sort(sort.Reverse(sort.IntSlice(starts)))
	for _, start := range starts {
		text = text[:start] + "\n" + text[breaks[start]:]
	}
	return text
}
