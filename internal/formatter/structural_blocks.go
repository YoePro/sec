package formatter

import (
	"sort"
	"strings"

	"sec/internal/ast"
	"sec/internal/cst"
	"sec/internal/lexer"
	"sec/internal/parser"
)

// formatStructuralBlockLayout makes every non-empty single-line structural
// declaration block multiline: struct, enum, union, register, interface, and
// impl blocks. Comma-separated blocks (struct fields, enum values, union
// variants) place each item on its own line with the multiline trailing
// comma; comma-free blocks (interface, impl, register) move their content off
// the brace lines, and later passes expand executable member bodies. Empty
// blocks stay compact, and blocks holding comments or several comma-free
// members on one line are left for structural formatting. Only line breaks
// and the trailing comma are inserted; indentation is owned by later passes.
//
// Rules:
//   - rules/tooling/formatter.md — § 15(1) structural declaration blocks, § 8(7) empty blocks, § 11(2)–(3)
func formatStructuralBlockLayout(text string) string {
	if !strings.Contains(text, "{") {
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
	replacements := []formatterReplacement{}
	expand := func(token lexer.Token, commaSeparated bool) {
		start, ok := byOffset[token.ByteStart]
		if !ok {
			return
		}
		open := structuralBlockOpen(document, start)
		if open < 0 {
			return
		}
		close, ok := closeOf[open]
		if !ok {
			return
		}
		if replacement, ok := structuralBlockReplacements(text, document, open, close, commaSeparated); ok {
			replacements = append(replacements, replacement...)
		}
	}
	var visitStatement func(statement ast.Statement)
	visitStatement = func(statement ast.Statement) {
		switch node := statement.(type) {
		case *ast.TypeDeclStatement:
			switch {
			case node.StructType != nil || node.Union:
				expand(node.Token, true)
			case node.RegisterType != nil:
				expand(node.Token, false)
			}
		case *ast.EnumDeclaration:
			expand(node.Token, true)
		case *ast.InterfaceDeclaration:
			expand(node.Token, false)
		case *ast.ImplStatement:
			expand(node.Token, false)
			for _, member := range node.Members {
				if nested, ok := member.(ast.Statement); ok {
					visitStatement(nested)
				}
			}
		}
	}
	for _, statement := range program.Statements {
		visitStatement(statement)
	}
	sort.SliceStable(replacements, func(i, j int) bool { return replacements[i].start > replacements[j].start })
	for _, replacement := range replacements {
		text = text[:replacement.start] + replacement.text + text[replacement.end:]
	}
	return text
}

// structuralBlockOpen returns the declaration's body brace: the first `{` at
// delimiter depth zero after the declaration keyword.
func structuralBlockOpen(document cst.Document, start int) int {
	depth := 0
	for index := start + 1; index < len(document.Elements); index++ {
		element := document.Elements[index]
		if element.Kind != cst.Token {
			continue
		}
		switch element.Token.Type {
		case lexer.LPAREN, lexer.LBRACKET:
			depth++
		case lexer.RPAREN, lexer.RBRACKET:
			depth--
		case lexer.LBRACE:
			if depth == 0 {
				return index
			}
		}
	}
	return -1
}

// structuralBlockReplacements computes the line breaks of one single-line
// block, or reports false when the block must stay unchanged.
func structuralBlockReplacements(text string, document cst.Document, open, close int, commaSeparated bool) ([]formatterReplacement, bool) {
	openSpan, closeSpan := document.Elements[open].Span, document.Elements[close].Span
	body := text[openSpan.End:closeSpan.Start]
	if strings.TrimSpace(body) == "" || strings.ContainsAny(body, "\n\r") {
		return nil, false
	}
	commas := []int{}
	depth := 0
	last := -1
	for index := open + 1; index < close; index++ {
		element := document.Elements[index]
		switch element.Kind {
		case cst.Comment:
			return nil, false
		case cst.Token:
			last = index
			switch element.Token.Type {
			case lexer.LPAREN, lexer.LBRACKET, lexer.LBRACE:
				depth++
			case lexer.RPAREN, lexer.RBRACKET, lexer.RBRACE:
				depth--
			case lexer.COMMA:
				if depth == 0 {
					commas = append(commas, index)
				}
			}
		}
	}
	if last < 0 {
		return nil, false
	}
	breakAt := func(at int, prefix string) formatterReplacement {
		end := at
		for end < len(text) && isHorizontalFormatterByte(text[end]) {
			end++
		}
		start := at
		for start > openSpan.End && isHorizontalFormatterByte(text[start-1]) {
			start--
		}
		return formatterReplacement{start: start, end: end, text: prefix + "\n"}
	}
	replacements := []formatterReplacement{breakAt(openSpan.End, "")}
	if commaSeparated {
		for _, comma := range commas {
			if comma == last {
				continue
			}
			replacements = append(replacements, breakAt(document.Elements[comma].Span.End, ""))
		}
		trailing := ""
		if document.Elements[last].Token.Type != lexer.COMMA {
			trailing = ","
		}
		replacements = append(replacements, breakAt(document.Elements[last].Span.End, trailing))
	} else {
		replacements = append(replacements, breakAt(document.Elements[last].Span.End, ""))
	}
	return replacements, true
}
