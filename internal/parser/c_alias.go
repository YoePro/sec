package parser

import (
	"sec/internal/ast"
	compilerdiagnostics "sec/internal/diagnostics"
	"sec/internal/lexer"
)

// cAliasAttribute is the compiler-known standalone declaration of MD-021.
const cAliasAttribute = "cAlias"

// peekIsCAlias reports whether the token after the current `@` or the
// peeked `@` starts a standalone @cAlias declaration.
// Rules: rules/foundations/grammar.md — § 126(1).
func (p *Parser) peekIsCAlias() bool {
	return p.peekToken.Type == lexer.IDENT && p.peekToken.Lexeme == cAliasAttribute
}

// tokenAfterPeekIsCAlias reports whether `@ cAlias` follows: the peeked
// token is `@` and the one after it is the cAlias identifier.
// Rules: rules/foundations/grammar.md — § 126(1); rules/foundations/attributes.md — § 95(1).
func (p *Parser) tokenAfterPeekIsCAlias() bool {
	if p.peekToken.Type != lexer.AT {
		return false
	}
	curToken, peekToken := p.curToken, p.peekToken
	lexerState := p.l.Snapshot()
	errorCount, warningCount := len(p.errors), len(p.warnings)
	p.nextToken()
	result := p.peekIsCAlias()
	p.curToken, p.peekToken = curToken, peekToken
	p.l.Restore(lexerState)
	p.rollbackErrors(errorCount)
	p.warnings = p.warnings[:warningCount]
	return result
}

// parseCAliasDeclaration parses `@cAlias(name: "c::...", target: Type)`
// with the current token on `@`. The declaration attaches to nothing; its
// target is a TypeReference, an exception to the attribute-argument
// expression model. Outside module scope it is reported and still parsed so
// that the following source stays aligned.
//
// Rules:
//   - rules/foundations/grammar.md — § 126(1)
//   - rules/foundations/attributes.md — § 95(1)
//   - rules/compiler/parser_recovery.md — § 27(2)
func (p *Parser) parseCAliasDeclaration() ast.Statement {
	topLevel := p.recoveryContext == RecoveryContextTopLevel
	defer p.withRecoveryContext(RecoveryContextAttribute)()
	declaration := &ast.CAliasDeclaration{Token: p.curToken}
	p.nextToken()
	declaration.Keyword = p.curToken
	invalid := func(token lexer.Token, format string, args ...any) {
		declaration.Invalid = true
		p.addDiagnostic(compilerdiagnostics.AttributeInvalidArgument, token, nil, nil, format+" at %d:%d", append(args, token.Line, token.Column)...)
	}
	if !topLevel {
		declaration.Invalid = true
		p.addDiagnostic(compilerdiagnostics.AttributeNotAllowedOnTarget, declaration.Token, nil, nil,
			"@cAlias is a standalone module-scope declaration and cannot appear here at %d:%d", declaration.Token.Line, declaration.Token.Column)
	}
	if p.peekToken.Type != lexer.LPAREN {
		invalid(declaration.Keyword, "@cAlias requires the arguments (name: \"c::...\", target: Type)")
		return declaration
	}
	p.nextToken()
	seen := map[string]lexer.Token{}
	for {
		p.skipPeekComments()
		if p.peekToken.Type == lexer.RPAREN {
			p.nextToken()
			declaration.Close = p.curToken
			break
		}
		if p.peekToken.Type != lexer.IDENT || p.tokenAfterPeek() != lexer.COLON {
			invalid(p.peekToken, "@cAlias takes only the named arguments name and target")
			p.recoverAttributeArguments()
			return declaration
		}
		p.nextToken()
		label := p.curToken
		p.nextToken()
		if previous, duplicate := seen[label.Lexeme]; duplicate {
			declaration.Invalid = true
			p.addDiagnostic(compilerdiagnostics.AttributeDuplicateArgument, label, nil, nil,
				"duplicate argument %s in @cAlias at %d:%d; first given at %d:%d", label.Lexeme, label.Line, label.Column, previous.Line, previous.Column)
		}
		seen[label.Lexeme] = label
		p.skipPeekComments()
		switch label.Lexeme {
		case "name":
			p.nextToken()
			value := p.parseExpression(LOWEST)
			literal, ok := value.(*ast.StringLiteral)
			if !ok {
				invalid(label, "the name argument of @cAlias must be a string literal such as \"c::stddef::size_t\"")
			} else {
				declaration.NameLabel, declaration.Name = label, literal
			}
		case "target":
			p.nextToken()
			declaration.TargetLabel = label
			declaration.Target = p.parseTypeReference()
			if declaration.Target == nil || declaration.Target.Invalid {
				declaration.Invalid = true
			}
		default:
			invalid(label, "@cAlias has no argument named %s; expected name and target", label.Lexeme)
			p.recoverAttributeArguments()
			return declaration
		}
		p.skipPeekComments()
		if p.peekToken.Type == lexer.COMMA {
			p.nextToken()
			continue
		}
		if p.peekToken.Type != lexer.RPAREN {
			unexpected := p.peekToken
			declaration.Invalid = true
			p.addDiagnostic(compilerdiagnostics.AttributeInvalidArgument, unexpected, []lexer.TokenType{lexer.COMMA, lexer.RPAREN}, &unexpected,
				"expected ',' or ')' in @cAlias arguments at %d:%d", unexpected.Line, unexpected.Column)
			p.recoverAttributeArguments()
			return declaration
		}
	}
	for _, required := range []string{"name", "target"} {
		if _, found := seen[required]; !found {
			invalid(declaration.Keyword, "@cAlias requires the named argument %s", required)
		}
	}
	return declaration
}
