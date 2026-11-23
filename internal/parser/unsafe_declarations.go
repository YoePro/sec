package parser

import (
	"sec/internal/ast"
	"sec/internal/lexer"
)

// parseUnsafeFunctionDeclaration retains the caller-facing unsafe contract on
// an ordinary or foreign function; the body remains a normal explicit body.
// Rules: rules/memory/unsafe.md — §§6–7,10–11;
// rules/foundations/grammar.md — §36.
func (p *Parser) parseUnsafeFunctionDeclaration() *ast.FunctionDeclaration {
	defer p.withRecoveryContext(RecoveryContextFunction)()
	unsafeToken := p.curToken
	p.nextToken()

	var fn *ast.FunctionDeclaration
	switch p.curToken.Type {
	case lexer.FN:
		fn = p.parseFunctionDeclaration()
	case lexer.EXTERN:
		fn = p.parseExternFunctionDeclaration()
	default:
		p.addError("unsafe function declaration must be followed by fn or extern at %d:%d", p.curToken.Line, p.curToken.Column)
		return nil
	}
	if fn == nil {
		return nil
	}
	fn.Token = unsafeToken
	fn.Unsafe = true
	return fn
}

// parseUnsafeImplMethod admits the ordinary unsafe method form, preserving
// the same AST and safety marker as an attributed method. Runtime unsafe
// statements and foreign declarations remain invalid impl members.
// Rules: rules/memory/unsafe.md — §§6–7;
// rules/foundations/grammar.md — §§28,36.
func (p *Parser) parseUnsafeImplMethod() ast.ImplMember {
	if p.peekToken.Type == lexer.FN {
		if fn := p.parseUnsafeFunctionDeclaration(); fn != nil {
			return fn
		}
		return nil
	}
	start, diagnosticStart := p.curToken, len(p.diagnostics)
	recovery := p.skipInvalidImplMember()
	return p.invalidMember(start, diagnosticStart, recovery, "unsafe inside impl must modify fn")
}
