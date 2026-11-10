package parser

import (
	"sec/internal/ast"
	compilerdiagnostics "sec/internal/diagnostics"
	"sec/internal/lexer"
)

// parseAsmStatement parses the existing string and structured assembly statement forms.
// Rules: rules/platform/inline_assembly.md — §§15,17–18,30,35–37.
func (p *Parser) parseAsmStatement() ast.Statement {
	stmt := &ast.AsmStatement{Token: p.curToken}

	switch p.peekToken.Type {
	case lexer.LBRACE:
		p.nextToken()
		block := p.parseAsmBlock()
		if block == nil {
			return nil
		}
		stmt.Block = block
		return stmt
	case lexer.STRING:
		p.nextToken()
		stmt.Template = &ast.StringLiteral{Token: p.curToken, Value: trimStringQuotes(p.curToken.Lexeme)}
		return stmt
	case lexer.LPAREN:
		p.nextToken()
		if !p.expectPeek(lexer.STRING) {
			return nil
		}
		stmt.Template = &ast.StringLiteral{Token: p.curToken, Value: trimStringQuotes(p.curToken.Lexeme)}
		if !p.expectPeek(lexer.RPAREN) {
			return nil
		}
		return stmt
	default:
		p.addError("asm statement requires string template at %d:%d", p.peekToken.Line, p.peekToken.Column)
		return nil
	}
}

// parseAsmBlock retains the template and explicit operand/clobber sections.
// Rules: rules/platform/inline_assembly.md — §§15,17–18,30,35–37.
func (p *Parser) parseAsmBlock() *ast.AsmBlock {
	block := &ast.AsmBlock{Token: p.curToken}

	p.nextToken()
	if p.curToken.Type != lexer.STRING {
		p.addError("asm block requires string template at %d:%d", p.curToken.Line, p.curToken.Column)
		p.skipCurrentBlock()
		return nil
	}
	block.Template = &ast.StringLiteral{Token: p.curToken, Value: trimStringQuotes(p.curToken.Lexeme)}

	p.nextToken()
	for p.curToken.Type != lexer.RBRACE && p.curToken.Type != lexer.EOF {
		if p.curToken.Type == lexer.COMMENT {
			p.nextToken()
			continue
		}
		if p.curToken.Type != lexer.IDENT {
			p.addError("expected asm section at %d:%d", p.curToken.Line, p.curToken.Column)
			p.skipCurrentBlock()
			return nil
		}

		switch p.curToken.Lexeme {
		case contextualInputs:
			inputs, ok := p.parseAsmInputs()
			if !ok {
				return nil
			}
			block.Inputs = inputs
		case contextualOutputs:
			outputs, ok := p.parseAsmOutputs()
			if !ok {
				return nil
			}
			block.Outputs = outputs
		case contextualClobbers:
			clobbers, ok := p.parseAsmClobbers()
			if !ok {
				return nil
			}
			block.Clobbers = clobbers
		default:
			p.addError("unknown asm section %q at %d:%d", p.curToken.Lexeme, p.curToken.Line, p.curToken.Column)
			p.skipCurrentBlock()
			return nil
		}
	}

	return block
}

// parseAsmInputs parses register-bound Sec input expressions.
// Rules: rules/platform/inline_assembly.md — §§15,17–18,30,35–37.
func (p *Parser) parseAsmInputs() ([]ast.AsmOperand, bool) {
	if !p.expectPeek(lexer.COLON) {
		return nil, false
	}

	inputs := []ast.AsmOperand{}
	p.nextToken()
	for p.curToken.Type != lexer.RBRACE && p.curToken.Type != lexer.EOF {
		if p.curToken.Type == lexer.COMMENT || p.curToken.Type == lexer.COMMA {
			p.nextToken()
			continue
		}
		if p.curToken.Type == lexer.IDENT && p.peekToken.Type == lexer.COLON {
			return inputs, true
		}
		if p.curToken.Type != lexer.IDENT {
			p.addError("expected asm input at %d:%d", p.curToken.Line, p.curToken.Column)
			return nil, false
		}
		register := p.curToken.Lexeme
		if !p.expectPeek(lexer.LPAREN) {
			return nil, false
		}
		p.nextToken()
		value := p.parseExpression(LOWEST)
		if value == nil {
			return nil, false
		}
		if !p.expectPeek(lexer.RPAREN) {
			return nil, false
		}
		inputs = append(inputs, ast.AsmOperand{Register: register, Value: value})
		p.nextToken()
	}
	return inputs, true
}

// parseAsmOutputs retains register outputs and optional source bindings.
// Rules: rules/platform/inline_assembly.md — §§15,17–18,30,35–37.
func (p *Parser) parseAsmOutputs() ([]ast.AsmOutput, bool) {
	if !p.expectPeek(lexer.COLON) {
		return nil, false
	}

	outputs := []ast.AsmOutput{}
	p.nextToken()
	for p.curToken.Type != lexer.RBRACE && p.curToken.Type != lexer.EOF {
		if p.curToken.Type == lexer.COMMENT || p.curToken.Type == lexer.COMMA {
			p.nextToken()
			continue
		}
		if p.curToken.Type == lexer.IDENT && p.peekToken.Type == lexer.COLON {
			return outputs, true
		}
		if p.curToken.Type != lexer.IDENT {
			p.addError("expected asm output at %d:%d", p.curToken.Line, p.curToken.Column)
			return nil, false
		}
		output := ast.AsmOutput{Register: p.curToken.Lexeme}
		if p.peekToken.Type == lexer.LPAREN {
			p.nextToken()
			if !p.expectPeek(lexer.IDENT) {
				return nil, false
			}
			output.Name = p.curToken.Lexeme
			output.Token = p.curToken
			if !p.expectPeek(lexer.RPAREN) {
				return nil, false
			}
		}
		outputs = append(outputs, output)

		p.nextToken()
	}
	return outputs, true
}

// parseAsmClobbers retains explicitly declared destroyed machine resources.
// Rules: rules/platform/inline_assembly.md — §§15,17–18,30,35–37.
func (p *Parser) parseAsmClobbers() ([]string, bool) {
	if !p.expectPeek(lexer.COLON) {
		return nil, false
	}

	clobbers := []string{}
	p.nextToken()
	for p.curToken.Type != lexer.RBRACE && p.curToken.Type != lexer.EOF {
		if p.curToken.Type == lexer.COMMENT || p.curToken.Type == lexer.COMMA {
			p.nextToken()
			continue
		}
		if p.curToken.Type == lexer.IDENT && p.peekToken.Type == lexer.COLON {
			return clobbers, true
		}
		if p.curToken.Type != lexer.IDENT {
			p.addError("expected asm clobber at %d:%d", p.curToken.Line, p.curToken.Column)
			return nil, false
		}
		clobbers = append(clobbers, p.curToken.Lexeme)
		p.nextToken()
	}
	return clobbers, true
}

// parseUnsupportedVolatileAsm diagnoses C-style modifiers without reserving
// volatile as a Sec keyword. The existing asm parser consumes the rejected
// statement, so recovery preserves the following declaration and closing block.
// Rules: rules/platform/inline_assembly.md — §46;
// rules/compiler/parser_recovery.md — Invalid AST nodes, Diagnostic deduplication.
func (p *Parser) parseUnsupportedVolatileAsm() (ast.Statement, bool) {
	start := p.curToken
	postfix := start.Type == lexer.ASM && p.peekToken.Type == lexer.IDENT && p.peekToken.Lexeme == "volatile"
	prefix := start.Type == lexer.IDENT && start.Lexeme == "volatile" && p.peekToken.Type == lexer.ASM
	attribute := start.Type == lexer.AT && p.peekToken.Type == lexer.IDENT && p.peekToken.Lexeme == "volatile" && p.tokenAfterPeek() == lexer.ASM
	if !postfix && !prefix && !attribute {
		return nil, false
	}
	message := "C-style volatile asm syntax is not supported; use ordinary asm inside an explicit unsafe context"
	p.addDiagnostic(compilerdiagnostics.ParserReservedSyntax, start, nil, nil, "%s", message)
	if attribute {
		p.nextToken()
	}
	p.nextToken()
	if postfix {
		p.curToken = start
	}
	p.parseAsmStatement()
	return &ast.InvalidStatement{Token: start, Message: message, Recovery: &ast.RecoveryInfo{
		DiagnosticID: compilerdiagnostics.ParserReservedSyntax, Message: message,
		Start: start, End: p.curToken, Reported: true,
	}}, true
}
