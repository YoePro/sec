package parser

import (
	"strconv"

	"sec/internal/ast"
	"sec/internal/lexer"
)

type precedence int

const (
	LOWEST  precedence = iota
	OR                 // ||
	AND                // &&
	EQUALS             // == !=
	COMPARE            // < <= > >=
	SUM                // + -
	PRODUCT            // * / %
	PREFIX             // -x !x
	CALL               // Type(value)
	MEMBER             // value.field
)

var precedences = map[lexer.TokenType]precedence{
	lexer.OR:       OR,
	lexer.AND:      AND,
	lexer.EQ:       EQUALS,
	lexer.NEQ:      EQUALS,
	lexer.LT:       COMPARE,
	lexer.LTE:      COMPARE,
	lexer.GT:       COMPARE,
	lexer.GTE:      COMPARE,
	lexer.IN:       COMPARE,
	lexer.PLUS:     SUM,
	lexer.MINUS:    SUM,
	lexer.SLASH:    PRODUCT,
	lexer.ASTERISK: PRODUCT,
	lexer.PERCENT:  PRODUCT,
	lexer.LPAREN:   CALL,
	lexer.LBRACE:   CALL,
	lexer.DOT:      MEMBER,
}

// parseExpression parses a value-producing expression using Pratt parsing.
//
// The parser starts by parsing the expression prefix found at curToken.
// Examples of prefix expressions are:
//   - integer literals: 10
//   - float literals: 10.5
//   - string literals: "hello"
//   - boolean literals: true
//   - identifiers: value
//   - unary expressions: -value, !ok
//
// After the prefix expression has been parsed, the parser continues while the
// next token has stronger precedence than the current precedence level.
// This lets the parser correctly group expressions such as:
//
//	10 + 5 * 3
//
// as:
//
//	10 + (5 * 3)
//
// rather than:
//
//	(10 + 5) * 3
//
// This function should only validate expression syntax. It should not check
// whether identifiers exist, whether operators are valid for a type, or whether
// the resulting expression is type-correct. Those checks belong in semantic
// analysis.
func (p *Parser) parseExpression(currentPrecedence precedence) ast.Expression {
	var left ast.Expression

	switch p.curToken.Type {
	case lexer.IDENT:
		left = p.parseIdentifierExpression()

	case lexer.INT:
		left = p.parseIntegerLiteral()

	case lexer.FLOAT:
		left = p.parseFloatLiteral()

	case lexer.STRING:
		left = p.parseStringLiteral()

	case lexer.INTERPSTRING:
		left = p.parseInterpolatedStringLiteral()

	case lexer.TRUE, lexer.FALSE:
		left = p.parseBooleanLiteral()

	case lexer.MINUS, lexer.NOT:
		left = p.parsePrefixExpression()

	case lexer.TRY:
		left = p.parseTryExpression()

	case lexer.MATCH:
		left = p.parseMatchExpression()

	case lexer.AT:
		left = p.parseRuntimeCallExpression()

	case lexer.LPAREN:
		left = p.parseGroupedExpression()

	default:
		p.addError(
			"no prefix parse function for %q at %d:%d",
			p.curToken.Type,
			p.curToken.Line,
			p.curToken.Column,
		)
		return nil
	}

	for p.peekToken.Type != lexer.EOF && currentPrecedence < p.peekPrecedence() {
		if p.stopBeforeBrace && p.peekToken.Type == lexer.LBRACE {
			return left
		}

		switch p.peekToken.Type {
		case lexer.LPAREN:
			p.nextToken()
			left = p.parseConversionExpression(left)

		case lexer.LBRACE:
			p.nextToken()
			left = p.parseStructLiteralExpression(left)

		case lexer.DOT:
			p.nextToken()
			left = p.parseMemberExpression(left)

		case lexer.PLUS,
			lexer.MINUS,
			lexer.SLASH,
			lexer.ASTERISK,
			lexer.PERCENT,
			lexer.EQ,
			lexer.NEQ,
			lexer.AND,
			lexer.OR,
			lexer.LT,
			lexer.LTE,
			lexer.GT,
			lexer.GTE,
			lexer.IN:

			p.nextToken()
			if p.curToken.Type == lexer.IN {
				left = p.parseInExpression(left)
			} else {
				left = p.parseInfixExpression(left)
			}

		default:
			return left
		}
	}

	return left
}

func (p *Parser) parseMatchExpression() *ast.MatchExpression {
	expr := &ast.MatchExpression{Token: p.curToken}
	p.nextToken()

	previousStopBeforeBrace := p.stopBeforeBrace
	p.stopBeforeBrace = true
	expr.Subject = p.parseExpression(PREFIX)
	p.stopBeforeBrace = previousStopBeforeBrace
	if expr.Subject == nil {
		return nil
	}

	if p.peekToken.Type != lexer.LBRACE {
		p.addError("expected '{' after match expression at %d:%d", p.peekToken.Line, p.peekToken.Column)
		return nil
	}
	p.nextToken()

	expr.Arms = p.parseMatchArmBlock()
	if expr.Arms == nil {
		return nil
	}
	return expr
}

func (p *Parser) parseMatchArmBlock() []*ast.MatchArm {
	arms := []*ast.MatchArm{}

	for {
		p.nextToken()
		if p.curToken.Type == lexer.RBRACE {
			return arms
		}
		if p.curToken.Type == lexer.EOF {
			p.addError("unterminated match block")
			return nil
		}

		arm := p.parseMatchArm()
		if arm == nil {
			p.skipMatchArm()
			if p.curToken.Type == lexer.RBRACE {
				return arms
			}
			continue
		}
		arms = append(arms, arm)
	}
}

func (p *Parser) parseMatchArm() *ast.MatchArm {
	arm := &ast.MatchArm{Token: p.curToken}
	pattern := p.parseExpression(LOWEST)
	if pattern == nil {
		return nil
	}
	arm.Pattern = pattern

	if p.peekToken.Type != lexer.ARROW {
		p.addError("expected '=>' after match pattern at %d:%d", p.peekToken.Line, p.peekToken.Column)
		return nil
	}
	p.nextToken()

	switch p.peekToken.Type {
	case lexer.LBRACE:
		arm.BlockBody = p.parseBlockStatement()
	case lexer.RETURN:
		p.nextToken()
		returnStmt := p.parseReturnStatement()
		if returnStmt == nil {
			return nil
		}
		arm.ReturnBody = returnStmt.(*ast.ReturnStatement)
	default:
		p.nextToken()
		body := p.parseExpression(LOWEST)
		if body == nil {
			return nil
		}
		arm.Body = body
	}

	return arm
}

func (p *Parser) skipMatchArm() {
	for p.curToken.Type != lexer.RBRACE && p.curToken.Type != lexer.EOF {
		if p.peekToken.Type == lexer.RBRACE {
			p.nextToken()
			return
		}
		p.nextToken()
	}
}

func (p *Parser) parseConversionExpression(left ast.Expression) ast.Expression {
	ident, ok := left.(*ast.Identifier)
	if !ok {
		member, memberOK := left.(*ast.MemberExpression)
		if memberOK {
			args, ok := p.parseCallArguments()
			if !ok {
				return nil
			}
			return &ast.CallExpression{
				Token:     member.Token,
				Callee:    member,
				Arguments: args,
			}
		}

		p.addError(
			"expected conversion target before '(' at %d:%d",
			p.curToken.Line,
			p.curToken.Column,
		)
		return nil
	}

	args, ok := p.parseCallArguments()
	if !ok {
		return nil
	}

	if ident.Value == "Ok" || ident.Value == "Err" {
		if len(args) != 1 {
			p.addError("%s expects 1 argument at %d:%d", ident.Value, ident.Token.Line, ident.Token.Column)
			return nil
		}
		if ident.Value == "Ok" {
			return &ast.OkExpression{Token: ident.Token, Value: args[0]}
		}
		return &ast.ErrExpression{Token: ident.Token, Value: args[0]}
	}

	return &ast.CallExpression{
		Token:     ident.Token,
		Callee:    ident,
		Function:  ident,
		Arguments: args,
	}
}

func (p *Parser) parseRuntimeCallExpression() ast.Expression {
	expr := &ast.RuntimeCallExpression{Token: p.curToken}

	if !p.expectPeek(lexer.IDENT) {
		return nil
	}
	name := p.curToken.Lexeme

	for p.peekToken.Type == lexer.DOT {
		p.nextToken()
		if !p.expectPeek(lexer.IDENT) {
			return nil
		}
		name += "." + p.curToken.Lexeme
	}
	expr.Name = name

	if !p.expectPeek(lexer.LPAREN) {
		return nil
	}

	args, ok := p.parseCallArguments()
	if !ok {
		return nil
	}
	expr.Arguments = args
	return expr
}

func (p *Parser) parseCallArguments() ([]ast.Expression, bool) {
	args := []ast.Expression{}

	if p.peekToken.Type == lexer.RPAREN {
		p.nextToken()
		return args, true
	}

	for {
		p.nextToken()
		arg := p.parseExpression(LOWEST)
		if arg == nil {
			return nil, false
		}
		args = append(args, arg)

		switch p.peekToken.Type {
		case lexer.COMMA:
			p.nextToken()
			if p.peekToken.Type == lexer.RPAREN {
				p.nextToken()
				return args, true
			}
		case lexer.RPAREN:
			p.nextToken()
			return args, true
		default:
			p.addError("expected ',' or ')' after argument at %d:%d", p.peekToken.Line, p.peekToken.Column)
			return nil, false
		}
	}
}

func (p *Parser) parseTryExpression() ast.Expression {
	expr := &ast.TryExpression{Token: p.curToken}
	p.nextToken()

	previousStopBeforeBrace := p.stopBeforeBrace
	p.stopBeforeBrace = true
	expr.Expression = p.parseExpression(PREFIX)
	p.stopBeforeBrace = previousStopBeforeBrace
	if expr.Expression == nil {
		return nil
	}

	if p.peekToken.Type == lexer.LBRACE {
		p.nextToken()
		expr.Handlers = p.parseTryHandlerBlock()
		if expr.Handlers == nil {
			return nil
		}
	}
	return expr
}

func (p *Parser) parseTryHandlerBlock() []*ast.TryHandler {
	handlers := []*ast.TryHandler{}

	for {
		p.nextToken()
		if p.curToken.Type == lexer.RBRACE {
			return handlers
		}
		if p.curToken.Type == lexer.EOF {
			p.addError("unterminated try handler block")
			return nil
		}

		handler := p.parseTryHandler()
		if handler == nil {
			p.skipTryHandler()
			if p.curToken.Type == lexer.RBRACE {
				return handlers
			}
			continue
		}
		handlers = append(handlers, handler)

		if p.isTryHandlerBlockRecoveryStart(p.peekToken.Type) {
			p.addError("expected '}' after try handler block before %q at %d:%d", p.peekToken.Lexeme, p.peekToken.Line, p.peekToken.Column)
			return handlers
		}
	}
}

func (p *Parser) isTryHandlerBlockRecoveryStart(t lexer.TokenType) bool {
	switch t {
	case lexer.RETURN,
		lexer.LET,
		lexer.IF,
		lexer.FOR,
		lexer.MATCH,
		lexer.TRY:
		return true
	default:
		return false
	}
}

func (p *Parser) parseTryHandler() *ast.TryHandler {
	handler := &ast.TryHandler{Token: p.curToken}
	pattern := p.parseExpression(LOWEST)
	if pattern == nil {
		return nil
	}
	handler.Pattern = pattern

	if p.peekToken.Type != lexer.ARROW {
		p.addError("expected '=>' after try handler pattern at %d:%d", p.peekToken.Line, p.peekToken.Column)
		return nil
	}
	p.nextToken()

	switch p.peekToken.Type {
	case lexer.LBRACE:
		handler.BlockBody = p.parseBlockStatement()
	case lexer.RETURN:
		p.nextToken()
		returnStmt := p.parseReturnStatement()
		if returnStmt == nil {
			return nil
		}
		handler.ReturnBody = returnStmt.(*ast.ReturnStatement)
	default:
		p.nextToken()
		body := p.parseExpression(LOWEST)
		if body == nil {
			return nil
		}
		handler.Body = body
	}

	return handler
}

func (p *Parser) skipTryHandler() {
	for p.curToken.Type != lexer.RBRACE && p.curToken.Type != lexer.EOF {
		if p.peekToken.Type == lexer.RBRACE {
			p.nextToken()
			return
		}
		if p.peekToken.Type == lexer.ARROW {
			p.nextToken()
			continue
		}
		p.nextToken()
	}
}

func (p *Parser) parseStructLiteralExpression(left ast.Expression) ast.Expression {
	ident, ok := left.(*ast.Identifier)
	if !ok {
		p.addError("expected struct literal type before '{' at %d:%d", p.curToken.Line, p.curToken.Column)
		return nil
	}

	lit := &ast.StructLiteral{
		Token: ident.Token,
		Type:  &ast.TypeReference{Token: ident.Token, Name: ident.Value},
	}

	for p.peekToken.Type != lexer.RBRACE && p.peekToken.Type != lexer.EOF {
		if !p.expectPeek(lexer.IDENT) {
			return nil
		}

		field := &ast.StructLiteralField{
			Token: p.curToken,
			Name:  &ast.Identifier{Token: p.curToken, Value: p.curToken.Lexeme},
		}

		if !p.expectPeek(lexer.COLON) {
			return nil
		}

		p.nextToken()
		field.Value = p.parseExpression(LOWEST)
		if field.Value == nil {
			return nil
		}

		lit.Fields = append(lit.Fields, field)

		switch p.peekToken.Type {
		case lexer.COMMA:
			p.nextToken()
			if p.peekToken.Type == lexer.RBRACE {
				break
			}
		case lexer.RBRACE:
			break
		default:
			p.addError("expected ',' or '}' after struct literal field")
			return nil
		}
	}

	if !p.expectPeek(lexer.RBRACE) {
		return nil
	}

	return lit
}

func (p *Parser) parseMemberExpression(left ast.Expression) ast.Expression {
	expr := &ast.MemberExpression{
		Token:  p.curToken,
		Object: left,
	}

	if !p.expectPeek(lexer.IDENT) {
		return nil
	}

	expr.Property = &ast.Identifier{Token: p.curToken, Value: p.curToken.Lexeme}
	return expr
}

func (p *Parser) parseIdentifierExpression() ast.Expression {
	return &ast.Identifier{
		Token: p.curToken,
		Value: p.curToken.Lexeme,
	}
}

func (p *Parser) parseIntegerLiteral() ast.Expression {
	value, err := strconv.ParseInt(p.curToken.Lexeme, 10, 64)
	if err != nil {
		p.addError("could not parse integer %q", p.curToken.Lexeme)
		return nil
	}

	return &ast.IntegerLiteral{
		Token: p.curToken,
		Value: value,
	}
}

func (p *Parser) parseFloatLiteral() ast.Expression {
	value, err := strconv.ParseFloat(p.curToken.Lexeme, 64)
	if err != nil {
		p.addError("could not parse float %q", p.curToken.Lexeme)
		return nil
	}

	return &ast.FloatLiteral{
		Token: p.curToken,
		Value: value,
	}
}

func (p *Parser) parseStringLiteral() ast.Expression {
	return &ast.StringLiteral{
		Token: p.curToken,
		Value: trimStringQuotes(p.curToken.Lexeme),
	}
}

func (p *Parser) parseInterpolatedStringLiteral() ast.Expression {
	return &ast.InterpolatedStringLiteral{
		Token: p.curToken,
		Value: p.curToken.Lexeme,
	}
}

func (p *Parser) parseBooleanLiteral() ast.Expression {
	return &ast.BooleanLiteral{
		Token: p.curToken,
		Value: p.curToken.Type == lexer.TRUE,
	}
}

func (p *Parser) parsePrefixExpression() ast.Expression {
	expr := &ast.PrefixExpression{
		Token:    p.curToken,
		Operator: p.curToken.Lexeme,
	}

	p.nextToken()

	expr.Right = p.parseExpression(PREFIX)

	return expr
}

func (p *Parser) parseGroupedExpression() ast.Expression {
	p.nextToken()

	expr := p.parseExpression(LOWEST)
	if expr == nil {
		return nil
	}

	if !p.expectPeek(lexer.RPAREN) {
		return nil
	}

	return expr
}

func (p *Parser) parseInfixExpression(left ast.Expression) ast.Expression {
	expr := &ast.InfixExpression{
		Token:    p.curToken,
		Left:     left,
		Operator: p.curToken.Lexeme,
	}

	prec := p.curPrecedence()

	p.nextToken()

	expr.Right = p.parseExpression(prec)

	return expr
}

func (p *Parser) parseInExpression(left ast.Expression) ast.Expression {
	expr := &ast.InfixExpression{
		Token:    p.curToken,
		Left:     left,
		Operator: p.curToken.Lexeme,
	}

	p.nextToken()
	expr.Right = p.parseRangeOrExpression()
	if expr.Right == nil {
		return nil
	}

	return expr
}

func (p *Parser) parseRangeOrExpression() ast.Expression {
	if p.curToken.Type == lexer.RANGE || p.curToken.Type == lexer.RANGE_EXCLUSIVE {
		rangeExpr := &ast.RangeExpression{
			Token:     p.curToken,
			Exclusive: p.curToken.Type == lexer.RANGE_EXCLUSIVE,
		}
		if p.isExpressionStart(p.peekToken.Type) {
			p.nextToken()
			rangeExpr.End = p.parseExpression(COMPARE)
		}
		return rangeExpr
	}

	start := p.parseExpression(COMPARE)
	if start == nil {
		return nil
	}

	if p.peekToken.Type != lexer.RANGE && p.peekToken.Type != lexer.RANGE_EXCLUSIVE {
		return start
	}

	p.nextToken()
	rangeExpr := &ast.RangeExpression{
		Token:     p.curToken,
		Start:     start,
		Exclusive: p.curToken.Type == lexer.RANGE_EXCLUSIVE,
	}
	if p.isExpressionStart(p.peekToken.Type) {
		p.nextToken()
		rangeExpr.End = p.parseExpression(COMPARE)
	}
	return rangeExpr
}

func (p *Parser) isExpressionStart(t lexer.TokenType) bool {
	switch t {
	case lexer.IDENT,
		lexer.INT,
		lexer.FLOAT,
		lexer.STRING,
		lexer.INTERPSTRING,
		lexer.TRUE,
		lexer.FALSE,
		lexer.MINUS,
		lexer.NOT,
		lexer.TRY,
		lexer.MATCH,
		lexer.AT,
		lexer.LPAREN:
		return true
	default:
		return false
	}
}

func (p *Parser) peekPrecedence() precedence {
	if p, ok := precedences[p.peekToken.Type]; ok {
		return p
	}

	return LOWEST
}

func (p *Parser) curPrecedence() precedence {
	if p, ok := precedences[p.curToken.Type]; ok {
		return p
	}

	return LOWEST
}
