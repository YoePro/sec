package parser

import (
	"fmt"
	"strconv"
	"strings"

	"sec/internal/ast"
	"sec/internal/lexer"
)

type Parser struct {
	l *lexer.Lexer

	errors   []string
	warnings []string

	curToken  lexer.Token
	peekToken lexer.Token

	stopBeforeBrace bool
}

func New(l *lexer.Lexer) *Parser {
	p := &Parser{
		l:        l,
		errors:   []string{},
		warnings: []string{},
	}

	p.nextToken()
	p.nextToken()

	return p
}

func (p *Parser) Errors() []string {
	return p.errors
}

func (p *Parser) Warnings() []string {
	return p.warnings
}

func (p *Parser) ParseProgram() *ast.Program {
	program := &ast.Program{}
	for p.curToken.Type != lexer.EOF {
		p.skipComments()

		if p.curToken.Type == lexer.EOF {
			break
		}

		if p.isTargetDirectiveStart() && len(program.Statements) > 0 {
			p.addError("#target directive must appear before any code or declarations at %d:%d", p.curToken.Line, p.curToken.Column)
		}

		stmt := p.parseStatement()

		if stmt != nil {
			program.Statements = append(program.Statements, stmt)
			p.nextToken()
			continue
		}

		p.skipStatement()
	}
	return program
}

func (p *Parser) isTargetDirectiveStart() bool {
	return p.curToken.Type == lexer.HASH &&
		p.peekToken.Type == lexer.IDENT &&
		p.peekToken.Lexeme == "target"
}

func (p *Parser) parseStatement() ast.Statement {
	switch p.curToken.Type {
	case lexer.HASH:
		return p.parseCompilerDirective()

	case lexer.MODULE:
		return p.parseModuleStatement()

	case lexer.IMPORT:
		return p.parseImportStatement()

	case lexer.TYPE:
		return p.parseTypeDeclStatement()

	case lexer.ENUM:
		return p.parseEnumDeclaration()

	case lexer.FN:
		return p.parseFunctionDeclaration()

	case lexer.STRUCT:
		return p.parseStructStatement()

	case lexer.IMPL:
		return p.parseImplStatement()

	case lexer.LET:
		return p.parseLetStatement()

	case lexer.RETURN:
		return p.parseReturnStatement()

	case lexer.TRY:
		return p.parseTryAssignmentStatement()

	case lexer.DEFER:
		return p.parseDeferStatement()

	case lexer.DISCARD:
		return p.parseDiscardStatement()

	case lexer.IF:
		return p.parseIfStatement()

	case lexer.FOR:
		return p.parseForStatement()

	case lexer.WHILE:
		return p.parseWhileStatement()

	case lexer.SWITCH:
		return p.parseSwitchStatement()

	case lexer.ELSE:
		return p.parseUnexpectedElseStatement()

	case lexer.FALLTHROUGH:
		return &ast.FallthroughStatement{Token: p.curToken}

	case lexer.BREAK:
		return &ast.BreakStatement{Token: p.curToken}

	case lexer.CONTINUE:
		return &ast.ContinueStatement{Token: p.curToken}

	case lexer.UNSAFE:
		if p.peekToken.Type == lexer.FN {
			return p.parseUnsafeFunctionDeclaration()
		}
		return p.parseUnsafeStatement()

	case lexer.ASM:
		return p.parseAsmStatement()

	case lexer.MATCH:
		return p.parseMatchStatement()

	case lexer.SPAWN, lexer.AWAIT:
		return p.parseExpressionOrAssignmentStatement()

	case lexer.AT:
		return p.parseExpressionOrAssignmentStatement()

	case lexer.IDENT:
		if p.peekToken.Type == lexer.MUT || p.peekToken.Type == lexer.COLON || p.peekToken.Type == lexer.LT || p.peekToken.Type == lexer.LBRACKET {
			errorsBefore := len(p.errors)
			if stmt := p.parseTypedVariableDeclaration(); stmt != nil || len(p.errors) > errorsBefore {
				return stmt
			}
		}
		return p.parseExpressionOrAssignmentStatement()

	case lexer.LBRACKET:
		return p.parseTypedVariableDeclaration()

	case lexer.COMMENT:
		return p.parseCommentStatement()

	default:
		p.addError("unexpected token %q at %d:%d", p.curToken.Lexeme, p.curToken.Line, p.curToken.Column)
		return nil
	}
}

func (p *Parser) parseDiscardStatement() ast.Statement {
	stmt := &ast.DiscardStatement{Token: p.curToken}
	if p.peekToken.Type != lexer.IDENT {
		p.addError("discard requires identifier at %d:%d", p.peekToken.Line, p.peekToken.Column)
		return stmt
	}
	p.nextToken()
	stmt.Name = &ast.Identifier{Token: p.curToken, Value: p.curToken.Lexeme}
	return stmt
}

func (p *Parser) parseDeferStatement() ast.Statement {
	stmt := &ast.DeferStatement{Token: p.curToken}
	if p.peekToken.Type == lexer.RETURN {
		p.nextToken()
		returnStmt := p.parseReturnStatement()
		block := &ast.BlockStatement{Token: stmt.Token}
		if returnStmt != nil {
			block.Statements = append(block.Statements, returnStmt)
		}
		stmt.Body = block
		return stmt
	}
	if p.peekToken.Type != lexer.LBRACE {
		p.addError("defer requires a block at %d:%d", p.peekToken.Line, p.peekToken.Column)
		return stmt
	}
	p.nextToken()
	stmt.Body = p.parseStatementBlock("defer body")
	return stmt
}

func (p *Parser) parseCompilerDirective() ast.Statement {
	if p.peekToken.Type != lexer.IDENT {
		p.addError("expected compiler directive name after '#' at %d:%d", p.curToken.Line, p.curToken.Column)
		return nil
	}

	hashToken := p.curToken
	p.nextToken()
	switch p.curToken.Lexeme {
	case "target":
		return p.parseTargetDirective(hashToken)
	default:
		p.addError("unknown compiler directive #%s at %d:%d", p.curToken.Lexeme, p.curToken.Line, p.curToken.Column)
		return nil
	}
}

func (p *Parser) parseMatchStatement() ast.Statement {
	expr := p.parseMatchExpression()
	if expr == nil {
		return nil
	}
	return &ast.MatchStatement{Token: expr.Token, Match: expr}
}

func (p *Parser) parseIfStatement() ast.Statement {
	stmt := &ast.IfStatement{Token: p.curToken}

	if p.peekToken.Type == lexer.LBRACE {
		p.addError("if statement missing condition at %d:%d", p.peekToken.Line, p.peekToken.Column)
		p.nextToken()
		stmt.Consequence = p.parseStatementBlock("if body")
		return stmt
	}

	p.nextToken()
	previousStopBeforeBrace := p.stopBeforeBrace
	p.stopBeforeBrace = true
	stmt.Condition = p.parseExpression(LOWEST)
	p.stopBeforeBrace = previousStopBeforeBrace
	if stmt.Condition == nil {
		return nil
	}

	if p.peekToken.Type != lexer.LBRACE {
		p.addError("expected '{' after if condition at %d:%d", p.peekToken.Line, p.peekToken.Column)
		return stmt
	}
	p.nextToken()
	stmt.Consequence = p.parseStatementBlock("if body")
	if stmt.Consequence == nil {
		return nil
	}

	if p.peekToken.Type != lexer.ELSE {
		return stmt
	}

	p.nextToken()
	switch p.peekToken.Type {
	case lexer.IF:
		p.nextToken()
		elseIf := p.parseIfStatement()
		if elseIf == nil {
			return nil
		}
		elseIfStmt := elseIf.(*ast.IfStatement)
		stmt.Alternative = &ast.BlockStatement{
			Token:      elseIfStmt.Token,
			Statements: []ast.Statement{elseIf},
		}
	case lexer.LBRACE:
		p.nextToken()
		stmt.Alternative = p.parseStatementBlock("else body")
		if stmt.Alternative == nil {
			return nil
		}
	default:
		p.addError("expected 'if' or '{' after else at %d:%d", p.peekToken.Line, p.peekToken.Column)
		return nil
	}

	return stmt
}

func (p *Parser) parseForStatement() ast.Statement {
	stmt := &ast.ForStatement{Token: p.curToken}

	if p.peekToken.Type == lexer.LBRACE {
		p.nextToken()
		stmt.Body = p.parseStatementBlock("for body")
		return stmt
	}

	p.nextToken()
	first, ok := p.parseForBinding()
	if !ok {
		p.addError("for loop requires an iterable expression at %d:%d", p.curToken.Line, p.curToken.Column)
		p.skipMalformedForHeader(stmt)
		return stmt
	}
	stmt.Bindings = append(stmt.Bindings, first)

	for p.peekToken.Type == lexer.COMMA {
		p.nextToken()
		p.nextToken()
		next, ok := p.parseForBinding()
		if !ok {
			p.addError("expected loop binding after ',' at %d:%d", p.curToken.Line, p.curToken.Column)
			p.skipMalformedForHeader(stmt)
			return stmt
		}
		stmt.Bindings = append(stmt.Bindings, next)
	}

	if p.peekToken.Type == lexer.SEMICOLON || p.curToken.Type == lexer.SEMICOLON || p.peekToken.Type == lexer.DECLARE || p.peekToken.Type == lexer.ASSIGN {
		p.addError("C-style for loops are not supported; use a range or while at %d:%d", p.curToken.Line, p.curToken.Column)
		p.skipMalformedForHeader(stmt)
		return stmt
	}

	if p.peekToken.Type != lexer.IN {
		p.addError("condition-only for loops are not supported; use while at %d:%d", p.curToken.Line, p.curToken.Column)
		p.skipMalformedForHeader(stmt)
		return stmt
	}

	p.nextToken()
	if p.peekToken.Type == lexer.LBRACE || p.peekToken.Type == lexer.EOF {
		p.addError("for loop requires an iterable expression at %d:%d", p.peekToken.Line, p.peekToken.Column)
		p.skipMalformedForHeader(stmt)
		return stmt
	}

	p.nextToken()
	previousStopBeforeBrace := p.stopBeforeBrace
	p.stopBeforeBrace = true
	stmt.Iterable = p.parseRangeOrExpression()
	p.stopBeforeBrace = previousStopBeforeBrace
	if stmt.Iterable == nil {
		p.skipMalformedForHeader(stmt)
		return stmt
	}

	if p.peekToken.Type == lexer.IDENT && p.peekToken.Lexeme == "step" {
		p.nextToken()
		if p.peekToken.Type == lexer.LBRACE || p.peekToken.Type == lexer.EOF {
			p.addError("for range step requires an expression at %d:%d", p.peekToken.Line, p.peekToken.Column)
			p.skipMalformedForHeader(stmt)
			return stmt
		}
		p.nextToken()
		previousStopBeforeBrace = p.stopBeforeBrace
		p.stopBeforeBrace = true
		stmt.Step = p.parseExpression(LOWEST)
		p.stopBeforeBrace = previousStopBeforeBrace
		if stmt.Step == nil {
			p.skipMalformedForHeader(stmt)
			return stmt
		}
	}

	if p.peekToken.Type != lexer.LBRACE {
		p.addError("expected '{' after for iterable at %d:%d", p.peekToken.Line, p.peekToken.Column)
		p.skipMalformedForHeader(stmt)
		return stmt
	}
	p.nextToken()
	stmt.Body = p.parseStatementBlock("for body")

	return stmt
}

func (p *Parser) parseWhileStatement() ast.Statement {
	stmt := &ast.WhileStatement{Token: p.curToken}

	if p.peekToken.Type == lexer.LBRACE {
		p.addError("while statement missing condition at %d:%d", p.peekToken.Line, p.peekToken.Column)
		p.nextToken()
		stmt.Body = p.parseStatementBlock("while body")
		return stmt
	}

	p.nextToken()
	previousStopBeforeBrace := p.stopBeforeBrace
	p.stopBeforeBrace = true
	stmt.Condition = p.parseExpression(LOWEST)
	p.stopBeforeBrace = previousStopBeforeBrace
	if stmt.Condition == nil {
		return nil
	}

	if p.peekToken.Type != lexer.LBRACE {
		if p.peekToken.Type == lexer.ASSIGN {
			p.addWarning("assignment in while condition at %d:%d", p.peekToken.Line, p.peekToken.Column)
			p.skipUntilBlockStart()
			if p.curToken.Type != lexer.LBRACE {
				return stmt
			}
			stmt.Body = p.parseStatementBlock("while body")
			return stmt
		}
		p.addError("expected '{' after while condition at %d:%d", p.peekToken.Line, p.peekToken.Column)
		return stmt
	}
	p.nextToken()
	stmt.Body = p.parseStatementBlock("while body")
	if stmt.Body == nil {
		return nil
	}

	return stmt
}

func (p *Parser) parseForBinding() (ast.ForBinding, bool) {
	switch p.curToken.Type {
	case lexer.IDENT:
		return ast.ForBinding{Token: p.curToken, Name: p.curToken.Lexeme}, true
	case lexer.UNDERSCORE:
		return ast.ForBinding{Token: p.curToken, Name: "_", Discard: true}, true
	default:
		return ast.ForBinding{}, false
	}
}

func (p *Parser) skipMalformedForHeader(stmt *ast.ForStatement) {
	for p.curToken.Type != lexer.EOF && p.curToken.Type != lexer.LBRACE && p.curToken.Type != lexer.RBRACE {
		if p.peekToken.Type == lexer.LBRACE {
			p.nextToken()
			stmt.Body = p.parseStatementBlock("for body")
			return
		}
		p.nextToken()
	}

	if p.curToken.Type == lexer.LBRACE {
		stmt.Body = p.parseStatementBlock("for body")
	}
}

func (p *Parser) parseSwitchStatement() ast.Statement {
	stmt := &ast.SwitchStatement{Token: p.curToken}

	if p.peekToken.Type != lexer.LBRACE {
		p.nextToken()
		previousStopBeforeBrace := p.stopBeforeBrace
		p.stopBeforeBrace = true
		stmt.Subject = p.parseExpression(LOWEST)
		p.stopBeforeBrace = previousStopBeforeBrace
		if stmt.Subject == nil {
			return nil
		}
	}

	if p.peekToken.Type != lexer.LBRACE {
		p.addError("expected '{' after switch at %d:%d", p.peekToken.Line, p.peekToken.Column)
		return stmt
	}
	p.nextToken()
	p.nextToken()

	for p.curToken.Type != lexer.RBRACE && p.curToken.Type != lexer.EOF {
		if p.curToken.Type == lexer.COMMENT {
			p.nextToken()
			continue
		}

		switch {
		case p.curToken.Type == lexer.CASE:
			caseClause := p.parseSwitchCaseClause(false, stmt.Subject == nil)
			if caseClause != nil {
				stmt.Cases = append(stmt.Cases, caseClause)
			}
		case p.curToken.Type == lexer.DEFAULT:
			defaultClause := p.parseSwitchCaseClause(true, stmt.Subject == nil)
			if defaultClause != nil {
				if stmt.Default != nil {
					stmt.DuplicateDefaultTokens = append(stmt.DuplicateDefaultTokens, defaultClause.Token)
				} else {
					stmt.Default = defaultClause
				}
			}
			if p.curToken.Type != lexer.RBRACE && p.curToken.Type != lexer.EOF {
				stmt.DefaultNotFinalToken = p.curToken
			}
		default:
			p.addError("expected switch case or default at %d:%d", p.curToken.Line, p.curToken.Column)
			p.skipStatement()
			p.nextToken()
		}
	}

	if p.curToken.Type == lexer.EOF {
		p.addError("unterminated switch body")
		return nil
	}

	return stmt
}

func (p *Parser) parseSwitchCaseClause(isDefault bool, subjectless bool) *ast.SwitchCase {
	clause := &ast.SwitchCase{Token: p.curToken, Default: isDefault}

	if isDefault {
		if p.peekToken.Type != lexer.COLON {
			p.addError("expected ':' after default at %d:%d", p.peekToken.Line, p.peekToken.Column)
			p.skipSwitchClause()
			return clause
		}
		p.nextToken()
		p.nextToken()
		clause.Body = p.parseSwitchCaseBody()
		return clause
	}

	p.nextToken()
	for {
		item := p.parseSwitchCaseItem(subjectless)
		if item == nil {
			p.skipSwitchClause()
			return clause
		}
		clause.Items = append(clause.Items, item)

		switch p.peekToken.Type {
		case lexer.COMMA:
			p.nextToken()
			p.nextToken()
			continue
		case lexer.COLON:
			p.nextToken()
			p.nextToken()
			clause.Body = p.parseSwitchCaseBody()
			return clause
		default:
			p.addError("expected ',' or ':' after switch case item at %d:%d", p.peekToken.Line, p.peekToken.Column)
			p.skipSwitchClause()
			return clause
		}
	}
}

func (p *Parser) parseSwitchCaseItem(subjectless bool) ast.SwitchCaseItem {
	if subjectless {
		value := p.parseExpression(LOWEST)
		if value == nil {
			return nil
		}
		return &ast.SwitchValueCase{Token: expressionToken(value), Value: value}
	}

	switch p.curToken.Type {
	case lexer.LT, lexer.LTE, lexer.GT, lexer.GTE:
		token := p.curToken
		operator := p.curToken.Lexeme
		p.nextToken()
		value := p.parseExpression(COMPARE)
		if value == nil {
			return nil
		}
		return &ast.SwitchRelationalCase{Token: token, Operator: operator, Value: value}
	case lexer.RANGE, lexer.RANGE_EXCLUSIVE:
		rangeExpr := &ast.RangeExpression{Token: p.curToken, Exclusive: p.curToken.Type == lexer.RANGE_EXCLUSIVE}
		if p.isExpressionStart(p.peekToken.Type) {
			p.nextToken()
			rangeExpr.End = p.parseExpression(COMPARE)
		}
		return &ast.SwitchRangeCase{Token: rangeExpr.Token, Range: rangeExpr}
	default:
		value := p.parseRangeOrExpression()
		if value == nil {
			return nil
		}
		if rangeExpr, ok := value.(*ast.RangeExpression); ok {
			return &ast.SwitchRangeCase{Token: rangeExpr.Token, Range: rangeExpr}
		}
		return &ast.SwitchValueCase{Token: expressionToken(value), Value: value}
	}
}

func (p *Parser) parseSwitchCaseBody() *ast.BlockStatement {
	block := &ast.BlockStatement{Token: p.curToken}
	for p.curToken.Type != lexer.RBRACE && p.curToken.Type != lexer.EOF && p.curToken.Type != lexer.CASE && p.curToken.Type != lexer.DEFAULT {
		if p.curToken.Type == lexer.COMMENT {
			p.nextToken()
			continue
		}

		stmt := p.parseStatement()
		if stmt != nil {
			block.Statements = append(block.Statements, stmt)
			p.nextToken()
			continue
		}

		p.skipStatement()
	}
	return block
}

func (p *Parser) skipSwitchClause() {
	for p.curToken.Type != lexer.RBRACE && p.curToken.Type != lexer.EOF && p.curToken.Type != lexer.CASE && p.curToken.Type != lexer.DEFAULT {
		p.nextToken()
	}
}

func (p *Parser) parseUnsafeStatement() ast.Statement {
	stmt := &ast.UnsafeStatement{Token: p.curToken}

	if !p.expectPeek(lexer.LBRACE) {
		return nil
	}

	stmt.Body = p.parseStatementBlock("unsafe body")
	if stmt.Body == nil {
		return nil
	}

	return stmt
}

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
		case "inputs":
			inputs, ok := p.parseAsmInputs()
			if !ok {
				return nil
			}
			block.Inputs = inputs
		case "outputs":
			outputs, ok := p.parseAsmOutputs()
			if !ok {
				return nil
			}
			block.Outputs = outputs
		case "clobbers":
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
			if !p.expectPeek(lexer.RPAREN) {
				return nil, false
			}
		}
		outputs = append(outputs, output)

		p.nextToken()
	}
	return outputs, true
}

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

func isSwitchClauseStart(token lexer.Token) bool {
	return token.Type == lexer.CASE || token.Type == lexer.DEFAULT
}

func expressionToken(expr ast.Expression) lexer.Token {
	switch expr := expr.(type) {
	case *ast.Identifier:
		return expr.Token
	case *ast.IntegerLiteral:
		return expr.Token
	case *ast.FloatLiteral:
		return expr.Token
	case *ast.StringLiteral:
		return expr.Token
	case *ast.BooleanLiteral:
		return expr.Token
	case *ast.InterpolatedStringLiteral:
		return expr.Token
	case *ast.PrefixExpression:
		return expr.Token
	case *ast.InfixExpression:
		return expr.Token
	case *ast.ConversionExpression:
		return expr.Token
	case *ast.CallExpression:
		return expr.Token
	case *ast.RuntimeCallExpression:
		return expr.Token
	case *ast.TryExpression:
		return expr.Token
	case *ast.MatchExpression:
		return expr.Token
	case *ast.RangeExpression:
		return expr.Token
	case *ast.MemberExpression:
		return expr.Token
	case *ast.StructLiteral:
		return expr.Token
	default:
		return lexer.Token{}
	}
}

func (p *Parser) parseUnexpectedElseStatement() ast.Statement {
	stmt := &ast.InvalidStatement{Token: p.curToken}
	p.addError("else without matching if at %d:%d", p.curToken.Line, p.curToken.Column)

	if p.peekToken.Type == lexer.LBRACE {
		p.nextToken()
		p.parseStatementBlock("else body")
	}

	return stmt
}

func (p *Parser) parseModuleStatement() ast.Statement {
	stmt := &ast.ModuleStatement{
		Token: p.curToken,
	}

	if p.peekToken.Type != lexer.IDENT {
		p.addError("module declaration missing name at %d:%d", p.curToken.Line, p.curToken.Column)
		return nil
	}
	p.nextToken()

	stmt.Path = p.parseDottedPath()

	return stmt
}

func (p *Parser) parseTargetDirective(hashToken lexer.Token) ast.Statement {
	stmt := &ast.TargetDirective{Token: hashToken}

	if !p.expectPeek(lexer.LPAREN) {
		return nil
	}

	seen := map[string]bool{}
	for p.peekToken.Type != lexer.RPAREN && p.peekToken.Type != lexer.EOF {
		if !p.expectPeek(lexer.IDENT) {
			p.skipCompilerDirective()
			return nil
		}
		name := p.curToken.Lexeme
		if name != "os" && name != "arch" {
			p.addError("unknown #target argument %q at %d:%d", name, p.curToken.Line, p.curToken.Column)
			p.skipCompilerDirective()
			return nil
		}
		if seen[name] {
			p.addError("duplicate #target argument %q at %d:%d", name, p.curToken.Line, p.curToken.Column)
			p.skipCompilerDirective()
			return nil
		}
		seen[name] = true

		if !p.expectPeek(lexer.COLON) {
			p.skipCompilerDirective()
			return nil
		}
		if p.peekToken.Type != lexer.STRING {
			p.addError("#target arguments must be compile-time string literals at %d:%d", p.peekToken.Line, p.peekToken.Column)
			p.skipCompilerDirective()
			return nil
		}
		p.nextToken()

		value := trimStringQuotes(p.curToken.Lexeme)
		switch name {
		case "os":
			stmt.OS = value
		case "arch":
			stmt.Arch = value
		}

		switch p.peekToken.Type {
		case lexer.COMMA:
			p.nextToken()
		case lexer.RPAREN:
		default:
			p.addError("expected ',' or ')' in #target directive at %d:%d", p.peekToken.Line, p.peekToken.Column)
			p.skipCompilerDirective()
			return nil
		}
	}

	if !p.expectPeek(lexer.RPAREN) {
		return nil
	}
	if stmt.OS == "" || stmt.Arch == "" {
		p.addError("#target requires os and arch arguments at %d:%d", hashToken.Line, hashToken.Column)
		return nil
	}

	return stmt
}

func (p *Parser) skipCompilerDirective() {
	for p.curToken.Type != lexer.EOF && p.curToken.Type != lexer.RPAREN {
		p.nextToken()
	}
}

func (p *Parser) parseImportStatement() ast.Statement {
	stmt := &ast.ImportStatement{
		Token: p.curToken,
	}

	if p.peekToken.Type == lexer.IDENT {
		p.nextToken()
		stmt.Alias = p.curToken.Lexeme
	}

	if !p.expectPeek(lexer.STRING) {
		return nil
	}

	stmt.Path = trimStringQuotes(p.curToken.Lexeme)

	return stmt
}

func (p *Parser) parseCommentStatement() ast.Statement {
	return &ast.CommentStatement{
		Token: p.curToken,
		Text:  p.curToken.Lexeme,
	}
}

func (p *Parser) parseTypeDeclStatement() ast.Statement {
	stmt := &ast.TypeDeclStatement{
		Token: p.curToken,
	}

	if !p.expectPeek(lexer.IDENT) {
		return nil
	}

	stmt.Name = &ast.Identifier{
		Token: p.curToken,
		Value: p.curToken.Lexeme,
	}

	if p.isAttachedGenericListStart(stmt.Name) {
		p.nextToken()
		stmt.GenericParameters = p.parseGenericParameters()
		if stmt.GenericParameters == nil {
			return nil
		}
	}

	if p.peekToken.Type == lexer.ASSIGN {
		p.nextToken()

		if !p.expectPeekTypeStart() {
			return nil
		}

		assignedType := p.parseTypeReference()

		if p.peekToken.Type == lexer.RANGE_KW {
			p.nextToken()
			stmt.Contract = p.parseRangeContract()
			if stmt.Contract == nil {
				return nil
			}
		}

		if p.peekToken.Type == lexer.IDENT && !p.isStatementStart(p.peekToken.Type) {
			stmt.Variants = []*ast.Identifier{
				{Token: assignedType.Token, Value: assignedType.Name},
			}

			for p.peekToken.Type == lexer.IDENT && !p.isStatementStart(p.peekToken.Type) {
				p.nextToken()
				stmt.Variants = append(stmt.Variants, &ast.Identifier{
					Token: p.curToken,
					Value: p.curToken.Lexeme,
				})
			}

			stmt.AssignedType = nil
		} else {
			stmt.AssignedType = assignedType
		}

		return stmt
	}

	if p.peekToken.Type == lexer.STRUCT {
		p.nextToken()
		stmt.StructType = p.parseStructType()
		return stmt
	}

	if p.peekToken.Type == lexer.UNION {
		p.nextToken()
		stmt.Union = true
		if p.peekToken.Type == lexer.LBRACE {
			p.nextToken()
			stmt.UnionVariants = p.parseUnionType()
		}
		return stmt
	}

	if p.peekToken.Type == lexer.EOF || p.isStatementStart(p.peekToken.Type) {
		p.addError(
			"type declaration missing base type after %q at %d:%d",
			stmt.Name.Value,
			stmt.Name.Token.Line,
			stmt.Name.Token.Column+len([]rune(stmt.Name.Value)),
		)
		return nil
	}

	if !p.expectPeekTypeStart() {
		return nil
	}

	stmt.BaseType = p.parseTypeReference()

	if p.peekToken.Type == lexer.RANGE_KW {
		p.nextToken()
		stmt.Contract = p.parseRangeContract()
		if stmt.Contract == nil {
			return nil
		}
	}

	return stmt
}

func (p *Parser) parseEnumDeclaration() *ast.EnumDeclaration {
	enum := &ast.EnumDeclaration{Token: p.curToken}

	if !p.expectPeek(lexer.IDENT) {
		return nil
	}
	enum.Name = &ast.Identifier{Token: p.curToken, Value: p.curToken.Lexeme}

	if p.peekToken.Type != lexer.LBRACE {
		if !p.expectPeekTypeStart() {
			return nil
		}
		enum.UnderlyingType = p.parseTypeReference()
	}

	if !p.expectPeek(lexer.LBRACE) {
		return nil
	}

	seenValue := false
	for p.peekToken.Type != lexer.RBRACE && p.peekToken.Type != lexer.EOF {
		p.nextToken()
		if p.curToken.Type == lexer.COMMENT {
			continue
		}
		if p.curToken.Type != lexer.IDENT {
			p.addError("expected enum value name, got %q at %d:%d", p.curToken.Lexeme, p.curToken.Line, p.curToken.Column)
			p.skipCurrentBlock()
			return nil
		}

		seenValue = true
		value := &ast.EnumValue{
			Token: p.curToken,
			Name:  &ast.Identifier{Token: p.curToken, Value: p.curToken.Lexeme},
		}
		if p.peekToken.Type == lexer.ASSIGN {
			p.nextToken()
			p.nextToken()
			value.Initializer = p.parseExpression(LOWEST)
			if value.Initializer == nil {
				return nil
			}
		}
		enum.Values = append(enum.Values, value)

		switch p.peekToken.Type {
		case lexer.COMMA:
			p.nextToken()
		case lexer.RBRACE:
			continue
		default:
			p.addError("expected ',' or '}' after enum value at %d:%d", p.peekToken.Line, p.peekToken.Column)
			for p.peekToken.Type != lexer.RBRACE && p.peekToken.Type != lexer.EOF {
				p.nextToken()
			}
			return enum
		}
	}

	if !seenValue {
		p.addError("enum %s must declare at least one value", enum.Name.Value)
	}

	if !p.expectPeek(lexer.RBRACE) {
		return enum
	}

	return enum
}

func (p *Parser) parseFunctionDeclaration() *ast.FunctionDeclaration {
	fn := &ast.FunctionDeclaration{Token: p.curToken}

	if !p.expectPeek(lexer.IDENT) {
		return nil
	}
	fn.Name = &ast.Identifier{Token: p.curToken, Value: p.curToken.Lexeme}

	if p.isAttachedGenericListStart(fn.Name) {
		p.nextToken()
		fn.GenericParameters = p.parseGenericParameters()
		if fn.GenericParameters == nil {
			return nil
		}
	}

	if !p.expectPeek(lexer.LPAREN) {
		return nil
	}

	fn.Parameters = p.parseParameters()
	if fn.Parameters == nil {
		return nil
	}

	if !p.expectPeekTypeStart() {
		return nil
	}
	fn.ReturnType = p.parseTypeReference()

	fn.Body = p.parseFunctionBlockStatement()
	if fn.Body == nil {
		return nil
	}

	return fn
}

func (p *Parser) isAttachedGenericListStart(name *ast.Identifier) bool {
	if name == nil || p.peekToken.Type != lexer.LBRACKET {
		return false
	}
	return p.peekToken.Line == name.Token.Line &&
		p.peekToken.Column == name.Token.Column+len([]rune(name.Value))
}

func (p *Parser) parseGenericParameters() []*ast.GenericParameter {
	params := []*ast.GenericParameter{}

	if p.peekToken.Type == lexer.RBRACKET {
		p.addError("expected generic parameter name at %d:%d", p.peekToken.Line, p.peekToken.Column)
		p.nextToken()
		return nil
	}

	for {
		if p.peekToken.Type != lexer.IDENT {
			p.addError("expected generic parameter name at %d:%d", p.peekToken.Line, p.peekToken.Column)
			p.skipGenericParameterList()
			return nil
		}
		p.nextToken()
		param := &ast.GenericParameter{
			Token: p.curToken,
			Name:  &ast.Identifier{Token: p.curToken, Value: p.curToken.Lexeme},
		}

		if p.peekToken.Type == lexer.COLON {
			p.nextToken()
			if !p.expectPeekTypeStart() {
				p.addError("expected constraint type after ':' for generic parameter %s at %d:%d", param.Name.Value, p.peekToken.Line, p.peekToken.Column)
				p.skipGenericParameterList()
				return nil
			}
			param.Constraint = p.parseTypeReference()
		}

		params = append(params, param)

		switch p.peekToken.Type {
		case lexer.COMMA:
			p.nextToken()
			if p.peekToken.Type == lexer.RBRACKET {
				p.nextToken()
				return params
			}
		case lexer.RBRACKET:
			p.nextToken()
			return params
		default:
			p.addError("expected ',' or ']' after generic parameter %s at %d:%d", param.Name.Value, p.peekToken.Line, p.peekToken.Column)
			p.skipGenericParameterList()
			return nil
		}
	}
}

func (p *Parser) skipGenericParameterList() {
	for p.curToken.Type != lexer.EOF {
		switch p.curToken.Type {
		case lexer.RBRACKET:
			return
		case lexer.LBRACE, lexer.LPAREN:
			return
		}
		p.nextToken()
	}
}

func (p *Parser) parseUnsafeFunctionDeclaration() *ast.FunctionDeclaration {
	unsafeToken := p.curToken
	p.nextToken()
	fn := p.parseFunctionDeclaration()
	if fn == nil {
		return nil
	}
	fn.Token = unsafeToken
	fn.Unsafe = true
	return fn
}

func (p *Parser) parseParameters() []*ast.Parameter {
	parameters := []*ast.Parameter{}

	if p.peekToken.Type == lexer.RPAREN {
		p.nextToken()
		return parameters
	}

	for {
		if p.peekToken.Type == lexer.REF {
			p.nextToken()
		}
		ref := p.curToken.Type == lexer.REF

		if !p.expectPeek(lexer.IDENT) {
			return nil
		}

		parameter := &ast.Parameter{
			Token: p.curToken,
			Name:  &ast.Identifier{Token: p.curToken, Value: p.curToken.Lexeme},
			Ref:   ref,
		}

		if !p.expectPeek(lexer.COLON) {
			return nil
		}

		if !p.expectPeekTypeStart() {
			return nil
		}
		parameter.Type = p.parseTypeReference()
		parameters = append(parameters, parameter)

		switch p.peekToken.Type {
		case lexer.COMMA:
			p.nextToken()
			if p.peekToken.Type == lexer.RPAREN {
				p.nextToken()
				return parameters
			}
		case lexer.RPAREN:
			p.nextToken()
			return parameters
		default:
			p.addError("expected ',' or ')' after parameter at %d:%d", p.peekToken.Line, p.peekToken.Column)
			return nil
		}
	}
}

func (p *Parser) parseFunctionBlockStatement() *ast.BlockStatement {
	if !p.expectPeek(lexer.LBRACE) {
		return nil
	}

	return p.parseStatementBlock("function body")
}

func (p *Parser) parseStatementBlock(name string) *ast.BlockStatement {
	block := &ast.BlockStatement{Token: p.curToken}

	p.nextToken()
	for p.curToken.Type != lexer.RBRACE && p.curToken.Type != lexer.EOF {
		if p.curToken.Type == lexer.COMMENT {
			p.nextToken()
			continue
		}

		stmt := p.parseStatement()
		if stmt != nil {
			block.Statements = append(block.Statements, stmt)
			p.nextToken()
			continue
		}

		p.skipStatement()
	}

	if p.curToken.Type == lexer.EOF {
		p.addError("unterminated %s", name)
		return nil
	}

	return block
}

func (p *Parser) parseReturnStatement() ast.Statement {
	stmt := &ast.ReturnStatement{Token: p.curToken}

	if p.peekToken.Type == lexer.RBRACE || p.peekToken.Type == lexer.EOF || p.isReturnTerminator(p.peekToken.Type) || isSwitchClauseStart(p.peekToken) {
		return stmt
	}

	p.nextToken()
	stmt.Value = p.parseExpression(LOWEST)
	if stmt.Value == nil {
		return nil
	}

	return stmt
}

func (p *Parser) isReturnTerminator(t lexer.TokenType) bool {
	switch t {
	case lexer.MODULE,
		lexer.COMMENT,
		lexer.IMPORT,
		lexer.TYPE,
		lexer.ENUM,
		lexer.STRUCT,
		lexer.INTERFACE,
		lexer.IMPL,
		lexer.PROPERTY,
		lexer.FN,
		lexer.LET,
		lexer.RETURN,
		lexer.IF,
		lexer.FOR,
		lexer.WHILE,
		lexer.SWITCH,
		lexer.BREAK,
		lexer.CONTINUE,
		lexer.UNSAFE,
		lexer.ASM,
		lexer.DEFER:
		return true
	default:
		return false
	}
}

func (p *Parser) parseStructStatement() ast.Statement {
	stmt := &ast.StructStatement{
		Token: p.curToken,
	}

	if !p.expectPeek(lexer.IDENT) {
		return nil
	}

	stmt.Name = &ast.Identifier{
		Token: p.curToken,
		Value: p.curToken.Lexeme,
	}

	if !p.expectPeek(lexer.LBRACE) {
		return nil
	}

	stmt.Fields = p.parseStructFields()

	if !p.expectPeek(lexer.RBRACE) {
		return stmt
	}

	return stmt
}

func (p *Parser) parseStructType() *ast.StructType {
	structType := &ast.StructType{Token: p.curToken}

	if !p.expectPeek(lexer.LBRACE) {
		return nil
	}

	structType.Fields = p.parseStructFields()

	if !p.expectPeek(lexer.RBRACE) {
		return structType
	}

	return structType
}

func (p *Parser) parseUnionType() []*ast.UnionVariant {
	variants := []*ast.UnionVariant{}

	for p.peekToken.Type != lexer.RBRACE && p.peekToken.Type != lexer.EOF {
		p.nextToken()
		if p.curToken.Type == lexer.COMMENT {
			continue
		}
		if p.curToken.Type != lexer.IDENT {
			p.addError("expected union variant name at %d:%d", p.curToken.Line, p.curToken.Column)
			p.skipBraceBlock()
			return variants
		}

		variant := &ast.UnionVariant{
			Token: p.curToken,
			Name:  &ast.Identifier{Token: p.curToken, Value: p.curToken.Lexeme},
		}

		if p.peekToken.Type == lexer.LPAREN {
			p.nextToken()
			if !p.expectPeekTypeStart() {
				p.skipBraceBlock()
				return variants
			}
			variant.Payload = p.parseTypeReference()
			if !p.expectPeek(lexer.RPAREN) {
				p.skipBraceBlock()
				return variants
			}
		} else if p.peekToken.Type == lexer.LBRACE {
			p.nextToken()
			variant.PayloadFields = p.parseStructFields()
			if !p.expectPeek(lexer.RBRACE) {
				return variants
			}
		}

		variants = append(variants, variant)
		if p.peekToken.Type == lexer.COMMA {
			p.nextToken()
		}
	}

	if !p.expectPeek(lexer.RBRACE) {
		return variants
	}
	return variants
}

func (p *Parser) parseStructFields() []*ast.StructField {
	fields := []*ast.StructField{}

	if p.peekToken.Type == lexer.RBRACE {
		return fields
	}

	for p.peekToken.Type != lexer.RBRACE && p.peekToken.Type != lexer.EOF {
		p.nextToken()

		if p.curToken.Type == lexer.COMMENT {
			continue
		}

		if p.curToken.Type != lexer.IDENT {
			p.addError("expected struct field name, got %q", p.curToken.Lexeme)
			return fields
		}

		field := &ast.StructField{
			Token: p.curToken,
			Name: &ast.Identifier{
				Token: p.curToken,
				Value: p.curToken.Lexeme,
			},
		}

		if p.peekToken.Type != lexer.COLON {
			p.addError("missing ':' after struct field name %q at %d:%d", field.Name.Value, field.Name.Token.Line, field.Name.Token.Column)
			if p.skipMalformedStructField() {
				continue
			}
			return fields
		}

		p.nextToken()
		colon := p.curToken

		if p.peekToken.Type == lexer.COMMA || p.peekToken.Type == lexer.RBRACE || p.peekToken.Type == lexer.EOF {
			p.addError("missing type after ':' at %d:%d", colon.Line, colon.Column)
			return fields
		}

		if !p.expectPeekTypeStart() {
			return fields
		}

		field.Type = p.parseTypeReference()
		if p.peekToken.Type == lexer.RANGE_KW {
			p.nextToken()
			field.Contract = p.parseRangeContract()
		}
		if p.peekToken.Type == lexer.RAW_STRING {
			p.nextToken()
			tags, ok := p.parseStructTag(p.curToken)
			if !ok {
				for p.peekToken.Type != lexer.RBRACE && p.peekToken.Type != lexer.EOF {
					p.nextToken()
				}
				return fields
			}
			field.Tags = tags
		}
		fields = append(fields, field)

		switch p.peekToken.Type {
		case lexer.COMMA:
			p.nextToken()
			if p.peekToken.Type == lexer.RBRACE {
				return fields
			}
		case lexer.RBRACE:
			return fields
		default:
			p.addError("expected ',' or '}' after struct field at %d:%d", p.peekToken.Line, p.peekToken.Column)
			for p.peekToken.Type != lexer.RBRACE && p.peekToken.Type != lexer.EOF {
				p.nextToken()
			}
			return fields
		}
	}

	return fields
}

func (p *Parser) skipMalformedStructField() bool {
	for p.peekToken.Type != lexer.COMMA && p.peekToken.Type != lexer.RBRACE && p.peekToken.Type != lexer.EOF {
		p.nextToken()
	}

	if p.peekToken.Type == lexer.COMMA {
		p.nextToken()
		return true
	}

	return false
}

func (p *Parser) parseStructTag(token lexer.Token) ([]ast.StructTag, bool) {
	raw := strings.TrimPrefix(strings.TrimSuffix(token.Lexeme, "`"), "`")
	if strings.TrimSpace(raw) == "" {
		return nil, true
	}

	tags := []ast.StructTag{}
	for _, part := range strings.Fields(raw) {
		key, value, ok := strings.Cut(part, ":")
		if !ok || key == "" || len(value) < 2 || value[0] != '"' || value[len(value)-1] != '"' {
			p.addError("invalid struct field tag")
			return nil, false
		}

		tags = append(tags, ast.StructTag{
			Key:   key,
			Value: strings.Trim(value, `"`),
		})
	}

	return tags, true
}

func (p *Parser) parseImplStatement() ast.Statement {
	stmt := &ast.ImplStatement{Token: p.curToken}

	if !p.expectPeekTypeStart() {
		return nil
	}
	stmt.Target = p.parseTypeReference()

	if !p.expectPeek(lexer.LBRACE) {
		return nil
	}

	for p.peekToken.Type != lexer.RBRACE && p.peekToken.Type != lexer.EOF {
		p.nextToken()
		if p.curToken.Type == lexer.COMMENT {
			continue
		}

		switch p.curToken.Type {
		case lexer.TYPE:
			parsed := p.parseTypeDeclStatement()
			typeDecl, ok := parsed.(*ast.TypeDeclStatement)
			if !ok {
				continue
			}
			stmt.Members = append(stmt.Members, typeDecl)
		case lexer.ENUM:
			enum := p.parseEnumDeclaration()
			if enum == nil {
				continue
			}
			stmt.Members = append(stmt.Members, enum)
		case lexer.FN:
			fn := p.parseFunctionDeclaration()
			if fn == nil {
				continue
			}
			stmt.Members = append(stmt.Members, fn)
		case lexer.PROPERTY:
			property := p.parsePropertyDeclaration()
			if property == nil {
				continue
			}
			stmt.Members = append(stmt.Members, property)
		case lexer.STRUCT:
			p.addError("impl block may only contain type, enum, property, and fn declarations at %d:%d", p.curToken.Line, p.curToken.Column)
			return nil
		case lexer.LET:
			p.addError("impl block may only contain type, enum, property, and fn declarations at %d:%d", p.curToken.Line, p.curToken.Column)
			return nil
		default:
			p.addError("unexpected token %q in impl block at %d:%d", p.curToken.Lexeme, p.curToken.Line, p.curToken.Column)
			return nil
		}
	}

	if !p.expectPeek(lexer.RBRACE) {
		return stmt
	}

	return stmt
}

func (p *Parser) parsePropertyDeclaration() *ast.PropertyDeclaration {
	property := &ast.PropertyDeclaration{Token: p.curToken}

	if p.peekToken.Type != lexer.IDENT {
		p.nextToken()
		p.addError("property declaration missing name at %d:%d", p.curToken.Line, p.curToken.Column)
		if p.curToken.Type == lexer.LBRACE {
			p.skipCurrentBlock()
		}
		return nil
	}
	p.nextToken()
	property.Name = &ast.Identifier{Token: p.curToken, Value: p.curToken.Lexeme}

	if p.peekToken.Type != lexer.COLON {
		p.nextToken()
		p.addError(
			"expected ':' after property name %s at %d:%d",
			property.Name.Value,
			p.curToken.Line,
			p.curToken.Column,
		)
		if p.curToken.Type == lexer.LBRACE {
			p.skipCurrentBlock()
		}
		return nil
	}
	p.nextToken()

	if !isTypeStart(p.peekToken.Type) {
		p.nextToken()
		p.addError(
			"property %s missing type after ':' at %d:%d",
			property.Name.Value,
			p.curToken.Line,
			p.curToken.Column,
		)
		if p.curToken.Type == lexer.LBRACE {
			p.skipCurrentBlock()
		}
		return nil
	}
	p.nextToken()
	property.Type = p.parseTypeReference()

	if !p.expectPeek(lexer.LBRACE) {
		return nil
	}

	for p.peekToken.Type != lexer.RBRACE && p.peekToken.Type != lexer.EOF {
		p.nextToken()
		switch p.curToken.Type {
		case lexer.GET:
			if property.Getter != nil {
				p.addError("duplicate get in property %q", property.Name.Value)
				return nil
			}
			property.Getter = p.parseBlockStatement()
			if property.Getter == nil {
				p.skipPropertyRemainder()
				return nil
			}
		case lexer.SET:
			if property.Setter != nil {
				p.addError("duplicate set in property %q", property.Name.Value)
				return nil
			}
			property.Setter = p.parsePropertySetter(property.Name.Value, false)
			if property.Setter == nil {
				p.skipPropertyRemainder()
				return nil
			}
		case lexer.TRY:
			if !p.expectPeek(lexer.SET) {
				p.skipPropertyRemainder()
				return nil
			}
			if property.Setter != nil {
				p.addError("duplicate set in property %q", property.Name.Value)
				return nil
			}
			property.Setter = p.parsePropertySetter(property.Name.Value, true)
			if property.Setter == nil {
				p.skipPropertyRemainder()
				return nil
			}
		case lexer.COMMENT:
			continue
		default:
			p.addError("unexpected token %q in property block at %d:%d", p.curToken.Lexeme, p.curToken.Line, p.curToken.Column)
			return nil
		}
	}

	if property.Getter == nil && property.Setter == nil {
		p.addError("property %q must have get or set", property.Name.Value)
		return nil
	}

	if !p.expectPeek(lexer.RBRACE) {
		return property
	}

	return property
}

func (p *Parser) parsePropertySetter(propertyName string, fallible bool) *ast.PropertySetter {
	setter := &ast.PropertySetter{Token: p.curToken, Fallible: fallible}

	if p.peekToken.Type != lexer.IDENT {
		p.nextToken()
		p.addError(
			"setter for %s must declare value parameter at %d:%d",
			propertyName,
			p.curToken.Line,
			p.curToken.Column,
		)
		if p.curToken.Type == lexer.LBRACE {
			p.skipCurrentBlock()
		}
		return nil
	}
	p.nextToken()
	setter.Parameter = &ast.Identifier{Token: p.curToken, Value: p.curToken.Lexeme}

	setter.Body = p.parseBlockStatement()
	if setter.Body == nil {
		return nil
	}

	return setter
}

func (p *Parser) skipCurrentBlock() {
	if p.curToken.Type != lexer.LBRACE {
		return
	}

	depth := 1
	for depth > 0 {
		p.nextToken()
		switch p.curToken.Type {
		case lexer.EOF:
			return
		case lexer.LBRACE:
			depth++
		case lexer.RBRACE:
			depth--
		}
	}
}

func (p *Parser) skipBraceBlock() {
	if p.curToken.Type != lexer.LBRACE {
		return
	}
	depth := 1
	for depth > 0 {
		p.nextToken()
		switch p.curToken.Type {
		case lexer.EOF:
			p.addError("unterminated block")
			return
		case lexer.LBRACE:
			depth++
		case lexer.RBRACE:
			depth--
		}
	}
}

func (p *Parser) skipPropertyRemainder() {
	depth := 0
	for p.peekToken.Type != lexer.EOF {
		p.nextToken()
		switch p.curToken.Type {
		case lexer.LBRACE:
			depth++
		case lexer.RBRACE:
			if depth == 0 {
				return
			}
			depth--
		}
	}
}

func (p *Parser) parseBlockStatement() *ast.BlockStatement {
	if !p.expectPeek(lexer.LBRACE) {
		return nil
	}

	block := &ast.BlockStatement{Token: p.curToken}
	depth := 1
	for depth > 0 {
		p.nextToken()
		switch p.curToken.Type {
		case lexer.EOF:
			p.addError("unterminated block")
			return nil
		case lexer.LBRACE:
			depth++
		case lexer.RBRACE:
			depth--
		}
		if depth > 0 {
			block.Tokens = append(block.Tokens, p.curToken)
		}
	}

	return block
}

func (p *Parser) parseTypeReference() *ast.TypeReference {
	if p.curToken.Type == lexer.FN {
		return p.parseFunctionTypeReference()
	}

	if p.curToken.Type == lexer.LBRACKET {
		return p.parseSliceTypeReference()
	}

	ref := &ast.TypeReference{
		Token: p.curToken,
		Name:  p.curToken.Lexeme,
	}

	for p.peekToken.Type == lexer.DOT {
		p.nextToken()
		if !p.expectPeek(lexer.IDENT) {
			return ref
		}
		ref.Name += "." + p.curToken.Lexeme
	}

	if p.peekToken.Type == lexer.LT {
		p.nextToken()

		unit := p.parseUnit()
		if unit == "" {
			return ref
		}

		ref.Unit = unit
	}

	if p.peekToken.Type == lexer.LBRACKET {
		p.nextToken()
		ref.TypeArgs = p.parseTypeArgs()
	}

	return ref
}

func (p *Parser) parseFunctionTypeReference() *ast.TypeReference {
	ref := &ast.TypeReference{
		Token: p.curToken,
		Name:  "fn",
	}

	if !p.expectPeek(lexer.LPAREN) {
		return ref
	}

	for p.peekToken.Type != lexer.RPAREN && p.peekToken.Type != lexer.EOF {
		if !p.expectPeekTypeStart() {
			return ref
		}
		ref.FunctionParameterTypes = append(ref.FunctionParameterTypes, p.parseTypeReference())

		if p.peekToken.Type == lexer.COMMA {
			p.nextToken()
			continue
		}
	}

	if !p.expectPeek(lexer.RPAREN) {
		return ref
	}

	if !p.expectPeekTypeStart() {
		return ref
	}
	ref.FunctionReturnType = p.parseTypeReference()

	return ref
}

func (p *Parser) parseSliceTypeReference() *ast.TypeReference {
	ref := &ast.TypeReference{
		Token: p.curToken,
	}

	if p.peekToken.Type == lexer.INT {
		p.nextToken()
		length, ok := ast.ParseIntegerLiteralInt64(p.curToken.Lexeme)
		if !ok {
			p.addError("invalid array length %q at %d:%d", p.curToken.Lexeme, p.curToken.Line, p.curToken.Column)
			return ref
		}
		ref.ArrayLength = length
	}

	if !p.expectPeek(lexer.RBRACKET) {
		return ref
	}
	if !p.expectPeekTypeStart() {
		return ref
	}

	ref.ElementType = p.parseTypeReference()

	return ref
}

func (p *Parser) parseUnit() string {
	if p.curToken.Type != lexer.LT {
		p.addError("expected unit to start with '<', got %q", p.curToken.Lexeme)
		return ""
	}

	p.nextToken()

	unit := ""

	for p.curToken.Type != lexer.GT && p.curToken.Type != lexer.EOF {
		unit += p.curToken.Lexeme
		p.nextToken()
	}

	if p.curToken.Type != lexer.GT {
		p.addError("unterminated unit type")
		return ""
	}

	return unit
}

func (p *Parser) parseTypeArgs() []*ast.TypeReference {
	typeArgs := []*ast.TypeReference{}

	for p.peekToken.Type != lexer.RBRACKET && p.peekToken.Type != lexer.EOF {
		if !p.expectPeekTypeStart() {
			return typeArgs
		}

		typeArgs = append(typeArgs, p.parseTypeReference())

		if p.peekToken.Type == lexer.COMMA {
			p.nextToken()
			continue
		}
	}

	if !p.expectPeek(lexer.RBRACKET) {
		return typeArgs
	}

	return typeArgs
}

// Check range contract
func (p *Parser) parseRangeContract() ast.Contract {
	contract := &ast.RangeContract{
		Token: p.curToken,
	}

	contract.Min = p.parseOptionalRangeBound()

	if !p.expectPeekRangeOperator() {
		return nil
	}

	contract.Exclusive = p.curToken.Type == lexer.RANGE_EXCLUSIVE

	if p.isAtTypeDeclEnd() {
		if !p.requireRangeBound(contract) {
			return nil
		}

		return contract
	}

	if !p.isRangeBoundStart(p.peekToken.Type) {
		if !p.requireRangeBound(contract) {
			return nil
		}

		return contract
	}

	contract.Max = p.parseOptionalRangeBound()

	if !p.requireRangeBound(contract) {
		return nil
	}

	return contract
}

func (p *Parser) parseOptionalRangeBound() ast.Expression {
	switch p.peekToken.Type {
	case lexer.INT, lexer.FLOAT:
		p.nextToken()
		return p.parseNumberLiteral()

	case lexer.MINUS:
		p.nextToken()
		minusToken := p.curToken

		if !p.expectPeekNumber() {
			return nil
		}

		return &ast.PrefixExpression{
			Token:    minusToken,
			Operator: minusToken.Lexeme,
			Right:    p.parseNumberLiteral(),
		}

	default:
		return nil
	}
}

func (p *Parser) isRangeBoundStart(t lexer.TokenType) bool {
	return t == lexer.INT || t == lexer.FLOAT || t == lexer.MINUS
}

func (p *Parser) parseNumberLiteral() ast.Expression {
	switch p.curToken.Type {
	case lexer.INT:
		value, ok := ast.ParseIntegerLiteralInt64(p.curToken.Lexeme)
		if !ok {
			p.addError("could not parse integer %q", p.curToken.Lexeme)
			return nil
		}

		return &ast.IntegerLiteral{
			Token: p.curToken,
			Value: value,
		}

	case lexer.FLOAT:
		value, ok := ast.ParseFloatLiteralFloat64(p.curToken.Lexeme)
		if !ok {
			p.addError("could not parse float %q", p.curToken.Lexeme)
			return nil
		}

		return &ast.FloatLiteral{
			Token: p.curToken,
			Value: value,
		}

	default:
		p.addError("expected number, got %q", p.curToken.Lexeme)
		return nil
	}
}

func (p *Parser) parseDottedPath() string {
	path := p.curToken.Lexeme

	for p.peekToken.Type == lexer.DOT {
		p.nextToken()

		if !p.expectPeek(lexer.IDENT) {
			return path
		}

		path += "." + p.curToken.Lexeme
	}

	return path
}

func (p *Parser) expectPeek(t lexer.TokenType) bool {
	if p.peekToken.Type == t {
		p.nextToken()
		return true
	}

	p.addError(
		"expected next token to be %q, got %q at %d:%d",
		t,
		p.peekToken.Type,
		p.peekToken.Line,
		p.peekToken.Column,
	)

	return false
}

func (p *Parser) expectPeekNumber() bool {
	if p.peekToken.Type == lexer.INT || p.peekToken.Type == lexer.FLOAT {
		p.nextToken()
		return true
	}

	p.addError(
		"expected next token to be number, got %q at %d:%d",
		p.peekToken.Type,
		p.peekToken.Line,
		p.peekToken.Column,
	)

	return false
}

func (p *Parser) expectPeekTypeStart() bool {
	if isTypeStart(p.peekToken.Type) {
		p.nextToken()
		return true
	}

	p.addError(
		"expected next token to be type, got %q at %d:%d",
		p.peekToken.Type,
		p.peekToken.Line,
		p.peekToken.Column,
	)

	return false
}

func isTypeStart(tokenType lexer.TokenType) bool {
	return tokenType == lexer.IDENT || tokenType == lexer.LBRACKET || tokenType == lexer.VOID || tokenType == lexer.FN
}

func (p *Parser) nextToken() {
	p.curToken = p.peekToken
	p.peekToken = p.l.NextToken()
}

func (p *Parser) addError(format string, args ...any) {
	p.errors = append(p.errors, fmt.Sprintf(format, args...))
}

func (p *Parser) addWarning(format string, args ...any) {
	p.warnings = append(p.warnings, fmt.Sprintf(format, args...))
}

func trimStringQuotes(s string) string {
	if unquoted, err := strconv.Unquote(s); err == nil {
		return unquoted
	}

	return s
}

func (p *Parser) skipStatement() {
	p.nextToken()

	for !p.isAtEnd() && !p.isStatementStart(p.curToken.Type) {
		p.nextToken()
	}
}

func (p *Parser) skipUntilBlockStart() {
	for !p.isAtEnd() && p.curToken.Type != lexer.LBRACE {
		p.nextToken()
	}
}

func (p *Parser) isAtEnd() bool {
	return p.curToken.Type == lexer.EOF
}

func (p *Parser) isStatementStart(t lexer.TokenType) bool {
	switch t {
	case lexer.MODULE,
		lexer.IMPORT,
		lexer.TYPE,
		lexer.ENUM,
		lexer.STRUCT,
		lexer.INTERFACE,
		lexer.IMPL,
		lexer.PROPERTY,
		lexer.FN,
		lexer.LET,
		lexer.RETURN,
		lexer.IF,
		lexer.FOR,
		lexer.WHILE,
		lexer.MATCH,
		lexer.SWITCH,
		lexer.BREAK,
		lexer.CONTINUE,
		lexer.UNSAFE,
		lexer.ASM,
		lexer.DEFER:
		return true
	default:
		return false
	}
}

func (p *Parser) isAtTypeDeclEnd() bool {
	return p.peekToken.Type == lexer.EOF || p.isStatementStart(p.peekToken.Type)
}

func (p *Parser) parseAssignmentStatement() ast.Statement {
	stmt := &ast.AssignmentStatement{Token: p.curToken}

	stmt.Target = p.parseExpression(LOWEST)
	if stmt.Target == nil {
		return nil
	}

	if !p.expectPeekAssignmentOperator() {
		return nil
	}

	stmt.Operator = p.curToken.Lexeme
	p.nextToken()
	stmt.Value = p.parseExpression(LOWEST)
	if stmt.Value == nil {
		return nil
	}

	return stmt
}

func (p *Parser) parseTryAssignmentStatement() ast.Statement {
	stmt := &ast.TryAssignmentStatement{Token: p.curToken}

	p.nextToken()
	target := p.parseExpression(LOWEST)
	if target == nil {
		return nil
	}

	if !p.isAssignmentOperator(p.peekToken.Type) {
		tryExpr := &ast.TryExpression{Token: stmt.Token, Expression: target}
		if p.peekToken.Type == lexer.LBRACE {
			p.nextToken()
			tryExpr.Handlers = p.parseTryHandlerBlock()
			if tryExpr.Handlers == nil {
				return nil
			}
		}
		return &ast.ExpressionStatement{Token: stmt.Token, Expression: tryExpr}
	}

	if !p.expectPeekAssignmentOperator() {
		return nil
	}

	assignment := &ast.AssignmentStatement{
		Token:    expressionToken(target),
		Target:   target,
		Operator: p.curToken.Lexeme,
	}
	p.nextToken()

	previousStopBeforeBrace := p.stopBeforeBrace
	p.stopBeforeBrace = true
	assignment.Value = p.parseExpression(LOWEST)
	p.stopBeforeBrace = previousStopBeforeBrace
	if assignment.Value == nil {
		return nil
	}
	stmt.Assignment = assignment
	if p.peekToken.Type == lexer.LBRACE {
		p.nextToken()
		stmt.Handlers = p.parseTryHandlerBlock()
		if stmt.Handlers == nil {
			return nil
		}
	}
	return stmt
}

func (p *Parser) parseExpressionOrAssignmentStatement() ast.Statement {
	token := p.curToken
	expr := p.parseExpression(LOWEST)
	if expr == nil {
		return nil
	}

	if !p.isAssignmentOperator(p.peekToken.Type) {
		return &ast.ExpressionStatement{Token: token, Expression: expr}
	}

	stmt := &ast.AssignmentStatement{Token: token, Target: expr}
	p.nextToken()
	stmt.Operator = p.curToken.Lexeme
	p.nextToken()
	stmt.Value = p.parseExpression(LOWEST)
	if stmt.Value == nil {
		return nil
	}

	return stmt
}

func (p *Parser) parseExpressionStatement() ast.Statement {
	stmt := &ast.ExpressionStatement{Token: p.curToken}
	stmt.Expression = p.parseExpression(LOWEST)
	if stmt.Expression == nil {
		return nil
	}
	return stmt
}

func (p *Parser) expectPeekAssignmentOperator() bool {
	if p.isAssignmentOperator(p.peekToken.Type) {
		p.nextToken()
		return true
	}

	p.addError(
		"expected next token to be assignment operator, got %q at %d:%d",
		p.peekToken.Type,
		p.peekToken.Line,
		p.peekToken.Column,
	)

	return false
}

func (p *Parser) isAssignmentOperator(t lexer.TokenType) bool {
	switch t {
	case lexer.ASSIGN,
		lexer.PLUS_ASSIGN,
		lexer.MINUS_ASSIGN,
		lexer.ASTERISK_ASSIGN,
		lexer.SLASH_ASSIGN,
		lexer.PERCENT_ASSIGN,
		lexer.BIT_AND_ASSIGN,
		lexer.BIT_OR_ASSIGN,
		lexer.BIT_XOR_ASSIGN,
		lexer.SHIFT_LEFT_ASSIGN,
		lexer.SHIFT_RIGHT_ASSIGN:
		return true
	default:
		return false
	}
}

// parseLetStatement parses a variable declaration.
//
// Supported forms:
//
//	let value: int
//	let mut value: int
//	let value := 42
//	let mut value := 42
//	let value: int := 42
//
// Parsing order:
//
//  1. Consume the mandatory 'let' keyword.
//  2. Optionally consume 'mut'.
//  3. Parse the variable identifier.
//  4. Optionally parse an explicit type declaration after ':'.
//  5. Optionally parse an initializer after ':='.
//
// Notes:
//
//   - Variables are immutable unless the 'mut' keyword is present.
//   - Type declarations and initializers are independent.
//   - The parser only verifies syntax. It does not verify that the type
//     exists, that the initializer matches the declared type, or that the
//     variable is used correctly. Those checks belong to semantic analysis.
//
// Grammar:
//
//	LetStatement
//	    := "let"
//	       ["mut"]
//	       Identifier
//	       [ ":" TypeReference ]
//	       [ ":=" Expression ]
func (p *Parser) parseLetStatement() ast.Statement {
	token := p.curToken
	mutable := false

	if p.peekToken.Type == lexer.MUT {
		p.nextToken()
		mutable = true
	}

	first := p.parseLetDeclarator(token, mutable, nil)
	if first == nil {
		return nil
	}
	if !p.letDeclaratorMayOmitInitializer(first) {
		p.addError("let declaration requires initializer for %q at %d:%d", first.Name.Value, first.Name.Token.Line, first.Name.Token.Column)
		p.skipDeclarationRest()
		return nil
	}

	lets := []*ast.LetStatement{first}
	for p.peekToken.Type == lexer.COMMA {
		p.nextToken()
		if p.peekToken.Type == lexer.EOF || p.isStatementStart(p.peekToken.Type) {
			break
		}

		next := p.parseLetDeclarator(token, mutable, nil)
		if next == nil {
			return nil
		}
		if !p.letDeclaratorMayOmitInitializer(next) {
			p.addError("let declaration requires initializer for %q at %d:%d", next.Name.Value, next.Name.Token.Line, next.Name.Token.Column)
			p.skipDeclarationRest()
			return nil
		}
		lets = append(lets, next)
	}

	if len(lets) == 1 {
		return first
	}

	return &ast.LetGroupStatement{Token: token, Lets: lets}
}

func (p *Parser) letDeclaratorMayOmitInitializer(stmt *ast.LetStatement) bool {
	if stmt.Value != nil {
		return true
	}

	return stmt.Type != nil
}

func (p *Parser) parseTypedVariableDeclaration() ast.Statement {
	token := p.curToken
	typ := p.parseTypeReference()

	mutable := false
	if p.peekToken.Type == lexer.MUT {
		p.nextToken()
		mutable = true
	}

	if p.peekToken.Type != lexer.COLON {
		return nil
	}
	p.nextToken()

	first := p.parseLetDeclarator(token, mutable, typ)
	if first == nil {
		return nil
	}

	if !mutable && first.Value == nil {
		p.addError("immutable typed declaration requires initializer for %q at %d:%d", first.Name.Value, first.Name.Token.Line, first.Name.Token.Column)
		p.skipDeclarationRest()
		return nil
	}

	lets := []*ast.LetStatement{first}
	for p.peekToken.Type == lexer.COMMA {
		p.nextToken()
		if p.peekToken.Type == lexer.EOF || p.isStatementStart(p.peekToken.Type) {
			break
		}

		next := p.parseLetDeclarator(token, mutable, typ)
		if next == nil {
			return nil
		}
		if !mutable && next.Value == nil {
			p.addError("immutable typed declaration requires initializer for %q at %d:%d", next.Name.Value, next.Name.Token.Line, next.Name.Token.Column)
			p.skipDeclarationRest()
			return nil
		}
		lets = append(lets, next)
	}

	if len(lets) == 1 {
		return first
	}

	return &ast.LetGroupStatement{Token: token, Lets: lets}
}

func (p *Parser) skipDeclarationRest() {
	for p.peekToken.Type != lexer.EOF && !p.isStatementStart(p.peekToken.Type) {
		p.nextToken()
	}
	if p.peekToken.Type == lexer.EOF {
		p.nextToken()
	}
}

func (p *Parser) parseLetDeclarator(token lexer.Token, mutable bool, inheritedType *ast.TypeReference) *ast.LetStatement {
	if !p.expectPeek(lexer.IDENT) {
		return nil
	}

	stmt := &ast.LetStatement{
		Token:   token,
		Mutable: mutable,
		Name: &ast.Identifier{
			Token: p.curToken,
			Value: p.curToken.Lexeme,
		},
		Type: inheritedType,
	}

	if stmt.Type == nil && p.peekToken.Type == lexer.COLON {

		p.nextToken()
		colon := p.curToken

		if p.peekToken.Type == lexer.EOF || (p.isStatementStart(p.peekToken.Type) && !isTypeStart(p.peekToken.Type)) {
			p.addError(
				"let statement missing type after ':' at %d:%d",
				colon.Line,
				colon.Column,
			)
			return nil
		}

		if !p.expectPeekTypeStart() {
			return nil
		}

		stmt.Type = p.parseTypeReference()
	}

	if p.peekToken.Type == lexer.DECLARE {

		p.nextToken()
		p.nextToken()

		stmt.Value = p.parseExpression(LOWEST)

		if stmt.Value == nil {
			return nil
		}
	}

	if p.peekToken.Type == lexer.ASSIGN {
		p.addError(
			"let initializer must use ':=', got '=' at %d:%d",
			p.peekToken.Line,
			p.peekToken.Column,
		)
		return nil
	}

	return stmt
}

/*
func (p *Parser) parseExpression() ast.Expression {
	switch p.curToken.Type {
	case lexer.IDENT:
		return &ast.Identifier{Token: p.curToken, Value: p.curToken.Lexeme}
	case lexer.INT, lexer.FLOAT:
		return p.parseNumberLiteral()
	case lexer.STRING:
		return &ast.StringLiteral{Token: p.curToken, Value: trimStringQuotes(p.curToken.Lexeme)}
	default:
		p.addError("unexpected expression %q", p.curToken.Lexeme)
		return nil
	}
}
*/

func (p *Parser) peekIs(t lexer.TokenType) bool {
	return p.peekToken.Type == t
}

func (p *Parser) currentIs(t lexer.TokenType) bool {
	return p.curToken.Type == t
}

func (p *Parser) skipComments() {
	for p.curToken.Type == lexer.COMMENT {
		p.nextToken()
	}
}
