package parser

import (
	"strings"

	"sec/internal/ast"
	"sec/internal/diagnostics"
	"sec/internal/lexer"
)

// interpolationLexer anchors a fragment's tokens and diagnostics in its parent
// source, including rune-based columns for nested interpolations.
// Rules: rules/foundations/lexical_structure.md — "14.3 Interpolated strings".
func interpolationLexer(source string, token lexer.Token, offset int) *lexer.Lexer {
	l := lexer.NewWithFile(source, token.File)
	state := l.Snapshot()
	state.Line = token.Line
	state.Column = token.Column + offset
	l.Restore(state)
	return l
}

// parseInterpolatedStringLiteral splits text and balanced expressions without
// changing the original token. Each expression uses the ordinary Pratt parser.
// Rules: rules/foundations/lexical_structure.md — "14.3 Interpolated strings", "15. Escapes";
// rules/compiler/parser_recovery.md — "Recovery goals", "Invalid AST nodes".
func (p *Parser) parseInterpolatedStringLiteral() ast.Expression {
	token := p.curToken
	literal := &ast.InterpolatedStringLiteral{Token: token, Value: token.Lexeme}
	source := []rune(token.Lexeme)
	position := func(offset int) lexer.Token {
		return lexer.Token{File: token.File, Line: token.Line, Column: token.Column + offset}
	}
	textStart := 2
	var text strings.Builder
	flush := func(end int) {
		if end > textStart {
			start := position(textStart)
			start.Type = lexer.STRING
			start.Lexeme = string(source[textStart:end])
			literal.Parts = append(literal.Parts, ast.InterpolatedStringPart{Token: start, End: position(end), Text: text.String()})
		}
		text.Reset()
	}
	for i := 2; i < len(source)-1; {
		if source[i] == '\\' {
			start := i
			i += 2
			if source[start+1] == 'u' && i < len(source)-1 && source[i] == '{' {
				for i < len(source)-1 && source[i] != '}' {
					i++
				}
				i++
			}
			text.WriteString(string(source[start:i]))
			continue
		}
		if (source[i] == '{' || source[i] == '}') && source[i+1] == source[i] {
			text.WriteRune(source[i])
			i += 2
			continue
		}
		if source[i] != '{' {
			text.WriteRune(source[i])
			i++
			continue
		}
		flush(i)
		start := i
		scanner := interpolationLexer(string(source[i+1:len(source)-1]), token, i+1)
		depth := 1
		for depth > 0 {
			next := scanner.NextToken()
			if next.Type == lexer.EOF {
				break
			}
			if next.Type == lexer.LBRACE {
				depth++
			}
			if next.Type == lexer.RBRACE {
				depth--
			}
		}
		end := i + 1 + scanner.Snapshot().Pos
		child := newParser(interpolationLexer(string(source[i+1:end-1]), token, i+1), true)
		child.recoveryContext = p.recoveryContext
		expression := child.parseExpression(LOWEST)
		if child.peekToken.Type != lexer.EOF {
			child.addDiagnostic(diagnostics.ParserUnexpectedToken, child.peekToken, nil, nil, "expected end of interpolation expression, got %q", child.peekToken.Lexeme)
		}
		if len(child.diagnostics) != 0 || expression == nil {
			expression = child.invalidExpression(child.curToken, "invalid interpolation expression", diagnostics.ParserInvalidExpression)
			invalid := expression.(*ast.InvalidExpression)
			invalid.Recovery.Start = position(i + 1)
			invalid.Recovery.End = position(end - 1)
			if len(child.diagnostics) > 0 {
				invalid.Recovery.DiagnosticID = child.diagnostics[0].ID
			}
		}
		for _, diagnostic := range child.diagnostics {
			p.endRecoveryEpisode()
			p.addDiagnostic(diagnostic.ID, diagnostic.Primary, diagnostic.Expected, diagnostic.Unexpected, "%s", diagnostic.Message)
		}
		p.warnings = append(p.warnings, child.warnings...)
		opening := position(start)
		opening.Type, opening.Lexeme = lexer.LBRACE, "{"
		literal.Parts = append(literal.Parts, ast.InterpolatedStringPart{Token: opening, End: position(end), Expression: expression})
		i, textStart = end, end
	}
	flush(len(source) - 1)
	return literal
}
