package parser

import (
	"sec/internal/ast"
	"sec/internal/lexer"
)

type precedence int

const (
	LOWEST  precedence = iota
	OR                 // ||
	AND                // &&
	BIT_OR             // |
	BIT_XOR            // ^
	BIT_AND            // &
	EQUALS             // == !=
	COMPARE            // < <= > >=
	SHIFT              // << >>
	SUM                // + -
	PRODUCT            // * / %
	PREFIX             // -x !x
	CALL               // Type(value)
	MEMBER             // value.field
)

var precedences = map[lexer.TokenType]precedence{
	lexer.OR:          OR,
	lexer.AND:         AND,
	lexer.BIT_OR:      BIT_OR,
	lexer.BIT_XOR:     BIT_XOR,
	lexer.BIT_AND:     BIT_AND,
	lexer.EQ:          EQUALS,
	lexer.NEQ:         EQUALS,
	lexer.LT:          COMPARE,
	lexer.LTE:         COMPARE,
	lexer.GT:          COMPARE,
	lexer.GTE:         COMPARE,
	lexer.IN:          COMPARE,
	lexer.SHIFT_LEFT:  SHIFT,
	lexer.SHIFT_RIGHT: SHIFT,
	lexer.PLUS:        SUM,
	lexer.MINUS:       SUM,
	lexer.SLASH:       PRODUCT,
	lexer.ASTERISK:    PRODUCT,
	lexer.PERCENT:     PRODUCT,
	lexer.LPAREN:      CALL,
	lexer.LBRACKET:    CALL,
	lexer.LBRACE:      CALL,
	lexer.DOT:         MEMBER,
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
	case lexer.IDENT, lexer.SELF:
		left = p.parseIdentifierExpression()

	case lexer.INT:
		left = p.parseIntegerLiteral()

	case lexer.FLOAT:
		left = p.parseFloatLiteral()

	case lexer.STRING:
		left = p.parseStringLiteral()

	case lexer.CHAR:
		left = p.parseCharLiteral()

	case lexer.INTERPSTRING:
		left = p.parseInterpolatedStringLiteral()

	case lexer.TRUE, lexer.FALSE:
		left = p.parseBooleanLiteral()

	case lexer.MINUS, lexer.NOT, lexer.BIT_NOT:
		left = p.parsePrefixExpression()

	case lexer.TRY:
		left = p.parseTryExpression()

	case lexer.SPAWN:
		left = p.parseSpawnExpression()

	case lexer.AWAIT:
		left = p.parseAwaitExpression()

	case lexer.MATCH:
		left = p.parseMatchExpression()

	case lexer.FN:
		left = p.parseLambdaExpression(nil)

	case lexer.CAPTURE:
		left = p.parseCaptureLambdaExpression()

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

		case lexer.LBRACKET:
			p.nextToken()
			left = p.parseExplicitGenericCallExpression(left)

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
			lexer.BIT_AND,
			lexer.BIT_OR,
			lexer.BIT_XOR,
			lexer.SHIFT_LEFT,
			lexer.SHIFT_RIGHT,
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

func (p *Parser) parseCaptureLambdaExpression() ast.Expression {
	captures := []ast.LambdaCapture{}

	if !p.expectPeek(lexer.LPAREN) {
		return nil
	}

	for p.peekToken.Type != lexer.RPAREN && p.peekToken.Type != lexer.EOF {
		if !p.expectPeek(lexer.IDENT) {
			return nil
		}
		captures = append(captures, ast.LambdaCapture{
			Name: &ast.Identifier{Token: p.curToken, Value: p.curToken.Lexeme},
		})

		if p.peekToken.Type == lexer.COMMA {
			p.nextToken()
			continue
		}
	}

	if !p.expectPeek(lexer.RPAREN) {
		return nil
	}
	if !p.expectPeek(lexer.FN) {
		p.addError("capture must be followed by lambda at %d:%d", p.peekToken.Line, p.peekToken.Column)
		return nil
	}

	return p.parseLambdaExpression(captures)
}

func (p *Parser) parseLambdaExpression(captures []ast.LambdaCapture) ast.Expression {
	expr := &ast.LambdaExpression{Token: p.curToken, Captures: captures}

	if !p.expectPeek(lexer.LPAREN) {
		return nil
	}

	expr.Parameters = p.parseParameters()
	if expr.Parameters == nil {
		return nil
	}

	if !isTypeStart(p.peekToken.Type) {
		p.addError("lambda return type is required at %d:%d", p.peekToken.Line, p.peekToken.Column)
		return nil
	}
	p.nextToken()
	expr.ReturnType = p.parseTypeReference()

	expr.Body = p.parseFunctionBlockStatement()
	if expr.Body == nil {
		return nil
	}

	return expr
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
	if p.curToken.Type == lexer.UNDERSCORE {
		arm.Pattern = &ast.Identifier{Token: p.curToken, Value: "_"}
	} else {
		pattern := p.parseExpression(LOWEST)
		if pattern == nil {
			return nil
		}
		arm.Pattern = pattern
	}

	if p.peekToken.Type == lexer.WHERE {
		p.nextToken()
		p.nextToken()
		guard := p.parseExpression(LOWEST)
		if guard == nil {
			return nil
		}
		arm.Guard = guard
	}

	if p.peekToken.Type != lexer.ARROW {
		p.addError("expected '=>' after match pattern at %d:%d", p.peekToken.Line, p.peekToken.Column)
		return nil
	}
	p.nextToken()

	switch p.peekToken.Type {
	case lexer.LBRACE:
		p.nextToken()
		arm.BlockBody = p.parseStatementBlock("match arm")
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

func (p *Parser) parseSpawnExpression() ast.Expression {
	expr := &ast.SpawnExpression{Token: p.curToken}
	if p.peekToken.Type != lexer.LBRACE {
		p.addError("spawn requires a block at %d:%d", p.peekToken.Line, p.peekToken.Column)
		return expr
	}
	p.nextToken()
	expr.Body = p.parseStatementBlock("spawn body")
	return expr
}

func (p *Parser) parseAwaitExpression() ast.Expression {
	expr := &ast.AwaitExpression{Token: p.curToken}
	p.nextToken()
	expr.Value = p.parseExpression(PREFIX)
	if expr.Value == nil {
		return nil
	}
	return expr
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

		args, ok := p.parseCallArguments()
		if !ok {
			return nil
		}
		return &ast.CallExpression{
			Token:     expressionToken(left),
			Callee:    left,
			Arguments: args,
		}
	}

	args, ok := p.parseCallArguments()
	if !ok {
		return nil
	}

	if ident.Value == "Ok" || ident.Value == "Err" {
		if ident.Value == "Ok" && len(args) == 0 {
			return &ast.OkExpression{Token: ident.Token, Arguments: args}
		}
		if ident.Value == "Ok" {
			var value ast.Expression
			if len(args) > 0 {
				value = args[0]
			}
			return &ast.OkExpression{Token: ident.Token, Value: value, Arguments: args}
		}
		var value ast.Expression
		if len(args) > 0 {
			value = args[0]
		}
		return &ast.ErrExpression{Token: ident.Token, Value: value, Arguments: args}
	}

	return &ast.CallExpression{
		Token:     ident.Token,
		Callee:    ident,
		Function:  ident,
		Arguments: args,
	}
}

func (p *Parser) parseExplicitGenericCallExpression(left ast.Expression) ast.Expression {
	typeArgs := p.parseTypeArgs()
	if typeArgs == nil {
		return nil
	}

	if p.peekToken.Type == lexer.LBRACE {
		ref, ok := typeReferenceFromExpression(left)
		if !ok {
			p.addError(
				"expected struct literal type before generic arguments at %d:%d",
				p.curToken.Line,
				p.curToken.Column,
			)
			return nil
		}
		ref.TypeArgs = typeArgs
		p.nextToken()
		return p.parseStructLiteralWithType(ref)
	}

	if p.peekToken.Type == lexer.DOT {
		p.nextToken()
		member := p.parseMemberExpression(left)
		if member == nil {
			return nil
		}
		if p.peekToken.Type != lexer.LPAREN {
			p.addError(
				"generic union variant arguments must be followed by call at %d:%d",
				p.peekToken.Line,
				p.peekToken.Column,
			)
			return nil
		}
		p.nextToken()
		args, ok := p.parseCallArguments()
		if !ok {
			return nil
		}
		return &ast.CallExpression{
			Token:            expressionToken(member),
			Callee:           member,
			GenericArguments: typeArgs,
			Arguments:        args,
		}
	}

	if p.peekToken.Type != lexer.LPAREN {
		p.addError(
			"generic arguments must be followed by call at %d:%d",
			p.peekToken.Line,
			p.peekToken.Column,
		)
		return nil
	}
	p.nextToken()

	args, ok := p.parseCallArguments()
	if !ok {
		return nil
	}

	switch callee := left.(type) {
	case *ast.Identifier:
		return &ast.CallExpression{
			Token:            callee.Token,
			Callee:           callee,
			Function:         callee,
			GenericArguments: typeArgs,
			Arguments:        args,
		}
	case *ast.MemberExpression:
		return &ast.CallExpression{
			Token:            callee.Token,
			Callee:           callee,
			GenericArguments: typeArgs,
			Arguments:        args,
		}
	default:
		p.addError(
			"expected callable before generic arguments at %d:%d",
			p.curToken.Line,
			p.curToken.Column,
		)
		return nil
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
		var arg ast.Expression
		if p.curToken.Type == lexer.UNDERSCORE {
			arg = &ast.Identifier{Token: p.curToken, Value: "_"}
		} else {
			arg = p.parseExpression(LOWEST)
			if arg == nil {
				return nil, false
			}
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
		if p.curToken.Type == lexer.COMMENT {
			continue
		}
		if p.curToken.Type == lexer.MATCH {
			return p.parseExplicitTryMatchHandlerBlock()
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

func (p *Parser) parseExplicitTryMatchHandlerBlock() []*ast.TryHandler {
	if !p.expectPeek(lexer.LBRACE) {
		return nil
	}

	handlers := p.parseTryHandlerBlock()
	if handlers == nil {
		return nil
	}

	if p.peekToken.Type != lexer.RBRACE {
		p.addError("expected '}' after try match handler block at %d:%d", p.peekToken.Line, p.peekToken.Column)
		return handlers
	}
	p.nextToken()
	return handlers
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
	ref, ok := typeReferenceFromExpression(left)
	if !ok {
		p.addError("expected struct literal type before '{' at %d:%d", p.curToken.Line, p.curToken.Column)
		return nil
	}

	return p.parseStructLiteralWithType(ref)
}

func (p *Parser) parseStructLiteralWithType(ref *ast.TypeReference) ast.Expression {
	lit := &ast.StructLiteral{
		Token: ref.Token,
		Type:  ref,
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

func typeReferenceFromExpression(expr ast.Expression) (*ast.TypeReference, bool) {
	switch expr := expr.(type) {
	case *ast.Identifier:
		return &ast.TypeReference{Token: expr.Token, Name: expr.Value}, true
	case *ast.MemberExpression:
		left, ok := typeReferenceFromExpression(expr.Object)
		if !ok {
			return nil, false
		}
		return &ast.TypeReference{Token: left.Token, Name: left.Name + "." + expr.Property.Value}, true
	default:
		return nil, false
	}
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
	bigValue, ok := ast.ParseIntegerLiteralLexeme(p.curToken.Lexeme)
	if !ok {
		p.addError("could not parse integer %q", p.curToken.Lexeme)
		return nil
	}

	var value int64
	if bigValue.IsInt64() {
		value = bigValue.Int64()
	}

	return &ast.IntegerLiteral{
		Token:    p.curToken,
		Value:    value,
		BigValue: bigValue,
	}
}

func (p *Parser) parseFloatLiteral() ast.Expression {
	value, ok := ast.ParseFloatLiteralFloat64(p.curToken.Lexeme)
	if !ok {
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

func (p *Parser) parseCharLiteral() ast.Expression {
	return &ast.CharLiteral{
		Token: p.curToken,
		Value: trimCharQuotes(p.curToken.Lexeme),
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
	if p.curToken.Type == lexer.LT {
		if conversion := p.parseUnitConversionExpression(left); conversion != nil {
			return conversion
		}
	}

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

func (p *Parser) parseUnitConversionExpression(left ast.Expression) ast.Expression {
	ident, ok := left.(*ast.Identifier)
	if !ok {
		return nil
	}

	curToken := p.curToken
	peekToken := p.peekToken
	lexerState := p.l.Snapshot()
	errorCount := len(p.errors)
	warningCount := len(p.warnings)

	unit := p.parseUnit()
	if unit == "" || p.peekToken.Type != lexer.LPAREN {
		p.curToken = curToken
		p.peekToken = peekToken
		p.l.Restore(lexerState)
		p.errors = p.errors[:errorCount]
		p.warnings = p.warnings[:warningCount]
		return nil
	}

	p.nextToken()
	args, ok := p.parseCallArguments()
	if !ok {
		return nil
	}
	if len(args) != 1 {
		p.addError("conversion to %s<%s> expects 1 argument at %d:%d", ident.Value, unit, ident.Token.Line, ident.Token.Column)
		return nil
	}

	return &ast.ConversionExpression{
		Token: ident.Token,
		Type: &ast.TypeReference{
			Token: ident.Token,
			Name:  ident.Value,
			Unit:  unit,
		},
		Value: args[0],
	}
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
