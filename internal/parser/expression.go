package parser

import (
	"fmt"
	"strings"

	"sec/internal/ast"
	compilerdiagnostics "sec/internal/diagnostics"
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
	lexer.SPREAD:      CALL,
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

	case lexer.PLUS, lexer.MINUS, lexer.NOT, lexer.BIT_NOT, lexer.MOVE_ASSIGN:
		left = p.parsePrefixExpression()

	case lexer.TRY:
		left = p.parseTryExpression()

	case lexer.NEW:
		left = p.parseNewExpression()

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

	case lexer.LBRACKET:
		left = p.parseArrayLiteral()

	case lexer.REF:
		left = p.parseRefExpression()

	case lexer.LPAREN:
		left = p.parseGroupedExpression()

	default:
		if p.curToken.Type == lexer.ILLEGAL {
			if message, ok := numericSuffixMigrationMessage(p.curToken); ok {
				p.addDiagnostic(compilerdiagnostics.ParserInvalidExpression, p.curToken, nil, nil, "%s", message)
				return p.invalidExpression(p.curToken, message, compilerdiagnostics.ParserInvalidExpression)
			}
		}
		if p.curToken.Type == lexer.ILLEGAL && isMalformedScientificExponent(p.curToken.Lexeme) {
			message := fmt.Sprintf(
				"malformed scientific exponent %q: expected at least one decimal digit at %d:%d",
				p.curToken.Lexeme,
				p.curToken.Line,
				p.curToken.Column,
			)
			p.addDiagnostic(compilerdiagnostics.ParserInvalidExpression, p.curToken, nil, nil, "%s", message)
			return p.invalidExpression(p.curToken, message, compilerdiagnostics.ParserInvalidExpression)
		}
		message := fmt.Sprintf(
			"no prefix parse function for %q at %d:%d",
			p.curToken.Type,
			p.curToken.Line,
			p.curToken.Column,
		)
		p.addDiagnostic(compilerdiagnostics.ParserInvalidExpression, p.curToken, nil, nil, "%s", message)
		invalid := p.invalidExpression(p.curToken, message, compilerdiagnostics.ParserInvalidExpression)
		if p.curToken.Type == lexer.IF {
			recovery := p.skipUnsupportedConditionalExpression()
			invalid.Recovery.Start = recovery.Start
			invalid.Recovery.End = recovery.End
			invalid.Recovery.Skipped = recovery.Skipped
		}
		return invalid
	}

	for p.peekToken.Type != lexer.EOF && currentPrecedence < p.peekPrecedence() {
		if p.stopBeforeBrace && p.peekToken.Type == lexer.LBRACE {
			return left
		}
		if p.peekToken.Type == lexer.SPREAD {
			p.nextToken()
			return &ast.SpreadExpression{Token: p.curToken, Value: left}
		}

		switch p.peekToken.Type {
		case lexer.LPAREN:
			p.nextToken()
			left = p.parseConversionExpression(left)

		case lexer.IDENT:
			if p.peekToken.Lexeme != "x" {
				return left
			}
			p.nextToken()
			left = p.parseInfixExpression(left)

		case lexer.LBRACKET:
			p.nextToken()
			left = p.parseBracketExpression(left)

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

// skipUnsupportedConditionalExpression keeps a currently unsupported `if`
// expression from breaking the surrounding statement and declaration shape.
// It consumes the complete if/else-if/else chain but leaves the next sibling
// statement untouched.
func (p *Parser) skipUnsupportedConditionalExpression() RecoveryEvent {
	start := p.curToken
	end := start
	skipped := 1
	delimiters := newDelimiterStack()
	seenBody := false
	allowContinuation := false

	for p.peekToken.Type != lexer.EOF {
		if delimiters.empty() && seenBody {
			if p.peekToken.Type == lexer.ELSE {
				p.nextToken()
				end = p.curToken
				skipped++
				allowContinuation = true
				continue
			}
			if !allowContinuation {
				break
			}
		}

		if !delimiters.canConsume(p.peekToken.Type) {
			break
		}
		p.nextToken()
		end = p.curToken
		skipped++
		if p.curToken.Type == lexer.LBRACE {
			seenBody = true
			allowContinuation = false
		}
		delimiters.consume(p.curToken.Type)
	}

	return p.recordSkippedRecovery(start, end, skipped, RecoveryProbable)
}

func (p *Parser) parseNewExpression() ast.Expression {
	expr := &ast.NewExpression{Token: p.curToken}
	if !p.expectPeekTypeStart() {
		return nil
	}
	expr.Type = p.parseTypeReference()
	if expr.Type == nil || expr.Type.Invalid {
		return nil
	}
	if !p.expectPeek(lexer.LPAREN) {
		p.addError("new construction requires an argument list at %d:%d", p.peekToken.Line, p.peekToken.Column)
		return nil
	}
	arguments, ok := p.parseCallArguments()
	if !ok {
		return nil
	}
	expr.Arguments = arguments
	return expr
}

func numericSuffixMigrationMessage(token lexer.Token) (string, bool) {
	if token.Lexeme == "" {
		return "", false
	}
	replacements := map[byte]struct {
		newSuffix string
		family    string
	}{
		'c': {newSuffix: "t", family: "char"},
		'd': {newSuffix: "m", family: "decimal"},
		'f': {newSuffix: "g", family: "binary float"},
	}
	replacement, ok := replacements[token.Lexeme[len(token.Lexeme)-1]]
	if !ok {
		return "", false
	}
	return fmt.Sprintf(
		"literal suffix '%c' was replaced by '%s' for %s at %d:%d",
		token.Lexeme[len(token.Lexeme)-1], replacement.newSuffix,
		replacement.family, token.Line, token.Column,
	), true
}

func isMalformedScientificExponent(lexeme string) bool {
	if lexeme == "" || !((lexeme[0] >= '0' && lexeme[0] <= '9') || lexeme[0] == '.') {
		return false
	}
	for index := len(lexeme) - 1; index >= 0; index-- {
		if lexeme[index] != 'e' && lexeme[index] != 'E' {
			continue
		}
		tail := lexeme[index+1:]
		return tail == "" || tail == "+" || tail == "-"
	}
	return false
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

	expr.Parameters = p.parseParameters(false)
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
	previousContext := p.recoveryContext
	p.recoveryContext = RecoveryContextMatchArm
	defer func() { p.recoveryContext = previousContext }()

	for {
		p.endRecoveryEpisode()
		p.nextToken()
		if p.curToken.Type == lexer.RBRACE {
			return arms
		}
		if p.curToken.Type == lexer.EOF {
			p.addError("unterminated match block")
			return nil
		}

		start := p.curToken
		diagnosticStart := len(p.diagnostics)
		arm := p.parseMatchArm()
		if arm == nil {
			recovery := p.skipMatchArm(start)
			pattern := p.invalidMatchPattern(start, diagnosticStart, recovery)
			arms = append(arms, &ast.MatchArm{
				Token: start, Pattern: pattern, Invalid: true, Recovery: pattern.Recovery,
			})
			continue
		}
		arms = append(arms, arm)
		p.endRecoveryEpisode()
	}
}

func (p *Parser) parseMatchArm() *ast.MatchArm {
	arm := &ast.MatchArm{Token: p.curToken}
	arm.Pattern = p.parseMatchPattern()
	if arm.Pattern == nil {
		return nil
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

// parseMatchPattern parses Sec's closed match-pattern grammar and rejects
// expression-like spellings before semantic variant resolution.
//
// Rules:
//   - rules/control-flow/flowcontrol_match.md — "Variant patterns"
//   - rules/control-flow/flowcontrol_match.md — "Whole-payload binding"
//   - rules/declarations/unions.md — "match"
func (p *Parser) parseMatchPattern() *ast.MatchPattern {
	start := p.curToken
	if start.Type == lexer.UNDERSCORE {
		return &ast.MatchPattern{Token: start, Kind: ast.MatchPatternCatchAll}
	}
	if start.Type != lexer.IDENT {
		p.addCanonicalMatchPatternError(start)
		return nil
	}
	if start.Lexeme == "empty" {
		if p.peekToken.Type == lexer.DOT || p.peekToken.Type == lexer.LPAREN || p.peekToken.Type == lexer.LBRACE {
			p.addError("the compiler-known empty match pattern cannot be qualified or bind a payload at %d:%d", start.Line, start.Column)
			return nil
		}
		return &ast.MatchPattern{Token: start, Kind: ast.MatchPatternEmpty, Name: "empty"}
	}

	parts := []string{start.Lexeme}
	for p.peekToken.Type == lexer.DOT {
		p.nextToken()
		if !p.expectPeek(lexer.IDENT) {
			p.addError("expected variant name after '.' in match pattern at %d:%d", p.peekToken.Line, p.peekToken.Column)
			return nil
		}
		parts = append(parts, p.curToken.Lexeme)
	}
	pattern := &ast.MatchPattern{Token: start, Kind: ast.MatchPatternVariant, Name: strings.Join(parts, "."), NameToken: p.curToken}
	switch p.peekToken.Type {
	case lexer.LPAREN:
		p.nextToken()
		p.nextToken()
		if p.curToken.Type == lexer.RPAREN {
			p.addError(
				"match variant %s cannot use empty payload parentheses; write %s for a payload-less variant or %s(binding) for a payload-bearing variant at %d:%d",
				pattern.Name,
				pattern.Name,
				pattern.Name,
				p.curToken.Line,
				p.curToken.Column,
			)
			return nil
		}
		pattern.Binding = p.parseMatchPatternBinding()
		if pattern.Binding == nil {
			p.skipRejectedMatchPayload()
			return nil
		}
		if p.peekToken.Type != lexer.RPAREN {
			if p.peekToken.Type == lexer.LPAREN || p.peekToken.Type == lexer.DOT {
				p.addError("nested match patterns are not part of Sec 0.1; bind the payload and use a where guard or another match at %d:%d", p.peekToken.Line, p.peekToken.Column)
			} else {
				p.addError("match variant payload must contain exactly one binding at %d:%d", p.peekToken.Line, p.peekToken.Column)
			}
			p.skipRejectedMatchPayload()
			return nil
		}
		p.nextToken()
	case lexer.LBRACE:
		p.nextToken()
		pattern.Kind = ast.MatchPatternFields
		pattern.Fields = p.parseMatchFieldPatterns()
		if pattern.Fields == nil {
			return nil
		}
	}
	return pattern
}

func (p *Parser) parseMatchPatternBinding() *ast.MatchPatternBinding {
	binding := &ast.MatchPatternBinding{Token: p.curToken, Mode: ast.MatchBindingValue}
	if p.curToken.Type == lexer.LPAREN {
		p.addError("nested match patterns are not part of Sec 0.1; bind the payload and use another match at %d:%d", p.curToken.Line, p.curToken.Column)
		return nil
	}
	if p.curToken.Type == lexer.REF {
		binding.Mode = ast.MatchBindingSharedRef
		if p.peekToken.Type == lexer.MUT {
			p.nextToken()
			binding.Mode = ast.MatchBindingMutableRef
		}
		if p.peekToken.Type != lexer.IDENT {
			p.addError("match payload reference must bind an identifier at %d:%d", p.peekToken.Line, p.peekToken.Column)
			return nil
		}
		p.nextToken()
	}
	if p.curToken.Type != lexer.IDENT && p.curToken.Type != lexer.UNDERSCORE {
		p.addError("match payload must bind an identifier, ref identifier, ref mut identifier, or _ at %d:%d", p.curToken.Line, p.curToken.Column)
		return nil
	}
	binding.Name = &ast.Identifier{Token: p.curToken, Value: p.curToken.Lexeme}
	return binding
}

// skipRejectedMatchPayload keeps the invalid arm as one recovery episode. The
// outer payload '(' was already consumed, so the first unmatched ')' is its
// boundary; nested parentheses are balanced while scanning toward it.
//
// Rule: rules/compiler/parser_recovery.md, match-arm synchronization.
func (p *Parser) skipRejectedMatchPayload() {
	depth := 0
	if p.curToken.Type == lexer.LPAREN {
		depth = 1
	}
	for p.peekToken.Type != lexer.EOF {
		switch p.peekToken.Type {
		case lexer.LPAREN:
			depth++
		case lexer.RPAREN:
			p.nextToken()
			if depth == 0 {
				return
			}
			depth--
			continue
		}
		p.nextToken()
	}
}

func (p *Parser) parseMatchFieldPatterns() []*ast.MatchFieldPattern {
	fields := []*ast.MatchFieldPattern{}
	for p.peekToken.Type != lexer.RBRACE && p.peekToken.Type != lexer.EOF {
		p.nextToken()
		if p.curToken.Type != lexer.IDENT {
			p.addError("expected field name in shallow match pattern at %d:%d", p.curToken.Line, p.curToken.Column)
			return nil
		}
		fieldName := &ast.Identifier{Token: p.curToken, Value: p.curToken.Lexeme}
		field := &ast.MatchFieldPattern{Token: p.curToken, Field: fieldName}
		if p.peekToken.Type == lexer.COLON {
			p.nextToken()
			p.nextToken()
			field.Binding = p.parseMatchPatternBinding()
			if field.Binding == nil {
				return nil
			}
		} else {
			field.Binding = &ast.MatchPatternBinding{Token: fieldName.Token, Name: &ast.Identifier{Token: fieldName.Token, Value: fieldName.Value}, Mode: ast.MatchBindingValue}
		}
		fields = append(fields, field)
		switch p.peekToken.Type {
		case lexer.COMMA:
			p.nextToken()
		case lexer.RBRACE:
		default:
			if p.peekToken.Line <= p.curToken.Line {
				p.addError("expected ',' or '}' after match field pattern at %d:%d", p.peekToken.Line, p.peekToken.Column)
				return nil
			}
		}
	}
	if p.peekToken.Type != lexer.RBRACE {
		p.addError("unterminated shallow match field pattern")
		return nil
	}
	p.nextToken()
	return fields
}

func (p *Parser) addCanonicalMatchPatternError(token lexer.Token) {
	switch token.Type {
	case lexer.TRUE, lexer.FALSE:
		p.addError("boolean match patterns are not part of Sec 0.1; use if/else at %d:%d", token.Line, token.Column)
	case lexer.INT, lexer.FLOAT, lexer.STRING, lexer.CHAR:
		p.addError("literal and range match patterns are not part of Sec 0.1; use switch at %d:%d", token.Line, token.Column)
	default:
		p.addError("expected _, empty, or a variant pattern at %d:%d", token.Line, token.Column)
	}
}

func (p *Parser) skipMatchArm(start lexer.Token) RecoveryEvent {
	return p.skipPatternEntry(start)
}

func (p *Parser) parseSpawnExpression() ast.Expression {
	expr := &ast.SpawnExpression{Token: p.curToken, Kind: "task"}
	if p.peekToken.Type == lexer.IDENT && isSpawnKindModifier(p.peekToken.Lexeme) {
		p.nextToken()
		expr.Kind = p.curToken.Lexeme
	}
	if p.peekToken.Type == lexer.LBRACE {
		p.nextToken()
		expr.Body = p.parseStatementBlock("spawn body")
		return expr
	}
	p.nextToken()
	expr.Value = p.parseExpression(PREFIX)
	if expr.Value == nil {
		return nil
	}
	return expr
}

func isSpawnKindModifier(value string) bool {
	switch value {
	case "task", "thread", "process":
		return true
	default:
		return false
	}
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

func (p *Parser) parseBracketExpression(left ast.Expression) ast.Expression {
	curToken := p.curToken
	peekToken := p.peekToken
	lexerState := p.l.Snapshot()
	errorCount := len(p.errors)
	warningCount := len(p.warnings)

	generic := p.parseExplicitGenericCallExpression(left)
	if generic != nil {
		return generic
	}
	if p.curToken.Type != lexer.RBRACKET || p.peekToken.Type != lexer.LPAREN {
		p.curToken = curToken
		p.peekToken = peekToken
		p.l.Restore(lexerState)
		p.rollbackErrors(errorCount)
		p.warnings = p.warnings[:warningCount]
		return p.parseIndexOrSliceExpression(left)
	}
	return generic
}

func (p *Parser) parseArrayLiteral() ast.Expression {
	lit := &ast.ArrayLiteral{Token: p.curToken}

	if p.peekToken.Type == lexer.RBRACKET {
		p.nextToken()
		return lit
	}

	for {
		p.nextToken()
		element := p.parseExpression(LOWEST)
		if element == nil {
			return nil
		}
		if _, alreadySpread := element.(*ast.SpreadExpression); !alreadySpread && p.peekToken.Type == lexer.SPREAD {
			p.nextToken()
			element = &ast.SpreadExpression{Token: p.curToken, Value: element}
		}
		lit.Elements = append(lit.Elements, element)

		switch p.peekToken.Type {
		case lexer.COMMA:
			p.nextToken()
			if p.peekToken.Type == lexer.RBRACKET {
				p.nextToken()
				return lit
			}
		case lexer.RBRACKET:
			p.nextToken()
			return lit
		default:
			p.addError("expected ',' or ']' after array literal element at %d:%d", p.peekToken.Line, p.peekToken.Column)
			return nil
		}
	}
}

func (p *Parser) parseIndexOrSliceExpression(left ast.Expression) ast.Expression {
	token := p.curToken

	if p.peekToken.Type == lexer.RANGE || p.peekToken.Type == lexer.RANGE_EXCLUSIVE {
		p.nextToken()
		return p.parseSliceExpressionAfterRange(left, token, nil)
	}

	if p.peekToken.Type == lexer.RBRACKET {
		if p.inRefExpression {
			p.addError("expected expression after ref; %s[] is a type, not storage at %d:%d", left.String(), token.Line, token.Column)
		} else {
			p.addError("empty index expression at %d:%d", p.peekToken.Line, p.peekToken.Column)
		}
		p.nextToken()
		return nil
	}

	p.nextToken()
	start := p.parseExpression(LOWEST)
	if start == nil {
		return nil
	}

	if p.peekToken.Type == lexer.RANGE || p.peekToken.Type == lexer.RANGE_EXCLUSIVE {
		p.nextToken()
		return p.parseSliceExpressionAfterRange(left, token, start)
	}

	if !p.expectPeek(lexer.RBRACKET) {
		return nil
	}
	return &ast.IndexExpression{Token: token, Left: left, Index: start}
}

func (p *Parser) parseSliceExpressionAfterRange(left ast.Expression, token lexer.Token, start ast.Expression) ast.Expression {
	expr := &ast.SliceExpression{
		Token:     token,
		Left:      left,
		Start:     start,
		Exclusive: p.curToken.Type == lexer.RANGE_EXCLUSIVE,
	}
	if p.isExpressionStart(p.peekToken.Type) {
		p.nextToken()
		expr.End = p.parseExpression(LOWEST)
		if expr.End == nil {
			return nil
		}
	}
	if !p.expectPeek(lexer.RBRACKET) {
		return nil
	}
	return expr
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
			// A try expression stops before a brace at its own expression level so
			// the brace can introduce handlers. That decision must not leak into a
			// nested call argument: Type { ... } here is a struct literal, while a
			// handler brace following the completed call is still seen by the outer
			// try parser after this flag is restored.
			previousStopBeforeBrace := p.stopBeforeBrace
			p.stopBeforeBrace = false
			arg = p.parseExpression(LOWEST)
			p.stopBeforeBrace = previousStopBeforeBrace
			if arg == nil {
				return nil, false
			}
		}
		if _, alreadySpread := arg.(*ast.SpreadExpression); !alreadySpread && p.peekToken.Type == lexer.SPREAD {
			p.nextToken()
			arg = &ast.SpreadExpression{Token: p.curToken, Value: arg}
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
	// try applies to the complete following expression. stopBeforeBrace keeps a
	// handler block distinct from postfix expression syntax.
	expr.Expression = p.parseExpression(LOWEST)
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
	previousContext := p.recoveryContext
	p.recoveryContext = RecoveryContextTryHandler
	defer func() { p.recoveryContext = previousContext }()

	for {
		p.endRecoveryEpisode()
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

		start := p.curToken
		diagnosticStart := len(p.diagnostics)
		handler := p.parseTryHandler()
		if handler == nil {
			recovery := p.skipTryHandler(start)
			pattern := p.invalidPattern(start, diagnosticStart, recovery)
			handlers = append(handlers, &ast.TryHandler{
				Token: start, Pattern: pattern, Invalid: true, Recovery: pattern.Recovery,
			})
			continue
		}
		handlers = append(handlers, handler)
		p.endRecoveryEpisode()

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
		p.nextToken()
		handler.BlockBody = p.parseStatementBlock("try handler")
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

func (p *Parser) skipTryHandler(start lexer.Token) RecoveryEvent {
	return p.skipPatternEntry(start)
}

// skipPatternEntry synchronizes before the next line's pattern or the closing
// brace. Delimiter depth prevents a malformed nested call or literal from
// being mistaken for a sibling arm.
func (p *Parser) skipPatternEntry(start lexer.Token) RecoveryEvent {
	end := start
	skipped := 1
	delimiters := newDelimiterStack()
	for p.curToken.Type != lexer.EOF {
		atTop := delimiters.empty()
		if atTop && p.peekToken.Type == lexer.RBRACE {
			break
		}
		if atTop && p.peekToken.Line > start.Line && (p.peekToken.Type == lexer.UNDERSCORE || p.isExpressionStart(p.peekToken.Type)) {
			break
		}
		if !delimiters.canConsume(p.peekToken.Type) {
			break
		}
		p.nextToken()
		end = p.curToken
		skipped++
		delimiters.consume(p.curToken.Type)
	}
	return p.recordSkippedRecovery(start, end, skipped, RecoveryProbable)
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

	for {
		for p.peekToken.Type == lexer.COMMENT {
			p.nextToken()
		}
		if p.peekToken.Type == lexer.RBRACE || p.peekToken.Type == lexer.EOF {
			break
		}
		p.nextToken()
		value := p.parseExpression(LOWEST)
		if value == nil {
			return nil
		}

		field := &ast.StructLiteralField{Token: expressionToken(value)}
		if spread, ok := value.(*ast.SpreadExpression); ok {
			field.Token = spread.Token
			field.Value = spread.Value
			field.Spread = true
		} else if p.peekToken.Type == lexer.SPREAD {
			p.nextToken()
			field.Token = p.curToken
			field.Value = value
			field.Spread = true
		} else {
			name, ok := value.(*ast.Identifier)
			if !ok {
				p.addError("expected struct literal field name or spread expression at %d:%d", expressionToken(value).Line, expressionToken(value).Column)
				return nil
			}
			field.Name = name
			if !p.expectPeek(lexer.COLON) {
				return nil
			}
			p.nextToken()
			field.Value = p.parseExpression(LOWEST)
			if field.Value == nil {
				return nil
			}
		}

		lit.Fields = append(lit.Fields, field)
		for p.peekToken.Type == lexer.COMMENT {
			p.nextToken()
		}

		switch p.peekToken.Type {
		case lexer.COMMA:
			p.nextToken()
			if p.peekToken.Type == lexer.RBRACE {
				break
			}
		case lexer.IDENT:
			// Multiline struct literals may separate fields by line layout
			// without commas. Newlines are trivia to the lexer, so the next
			// field identifier and its source line are the separator visible
			// to the parser.
			if p.peekToken.Line > expressionToken(field.Value).Line {
				continue
			}
			p.addError("expected ',' or '}' after struct literal field")
			return nil
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

	if p.peekToken.Type != lexer.IDENT && p.peekToken.Type != lexer.UNDERSCORE {
		p.addError(
			"expected next token to be %q, got %q at %d:%d",
			lexer.IDENT,
			p.peekToken.Type,
			p.peekToken.Line,
			p.peekToken.Column,
		)
		return nil
	}
	p.nextToken()

	expr.Property = &ast.Identifier{Token: p.curToken, Value: p.curToken.Lexeme}
	return expr
}

func (p *Parser) parseRefExpression() ast.Expression {
	expr := &ast.RefExpression{Token: p.curToken}
	if p.peekToken.Type == lexer.MUT {
		p.nextToken()
		expr.Mutable = true
	}
	if !p.isExpressionStart(p.peekToken.Type) {
		p.addError("expected expression after ref at %d:%d", p.peekToken.Line, p.peekToken.Column)
		return nil
	}
	p.nextToken()
	previousInRefExpression := p.inRefExpression
	p.inRefExpression = true
	expr.Value = p.parseExpression(PREFIX)
	p.inRefExpression = previousInRefExpression
	if expr.Value == nil {
		return nil
	}
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

	unit, unitExpression := p.parseUnit()
	if unit == "" || p.peekToken.Type != lexer.LPAREN {
		p.curToken = curToken
		p.peekToken = peekToken
		p.l.Restore(lexerState)
		p.rollbackErrors(errorCount)
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
			Token:          ident.Token,
			Name:           ident.Value,
			Unit:           unit,
			UnitExpression: unitExpression,
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
		lexer.SELF,
		lexer.INT,
		lexer.FLOAT,
		lexer.STRING,
		lexer.CHAR,
		lexer.INTERPSTRING,
		lexer.TRUE,
		lexer.FALSE,
		lexer.PLUS,
		lexer.MINUS,
		lexer.NOT,
		lexer.BIT_NOT,
		lexer.TRY,
		lexer.NEW,
		lexer.MATCH,
		lexer.SPAWN,
		lexer.AWAIT,
		lexer.FN,
		lexer.CAPTURE,
		lexer.AT,
		lexer.LBRACKET,
		lexer.REF,
		lexer.LPAREN:
		return true
	default:
		return false
	}
}

func (p *Parser) peekPrecedence() precedence {
	if p.contextualMatrixMultiplyAhead() {
		return PRODUCT
	}
	if p, ok := precedences[p.peekToken.Type]; ok {
		return p
	}

	return LOWEST
}

func (p *Parser) contextualMatrixMultiplyAhead() bool {
	if p.peekToken.Type != lexer.IDENT || p.peekToken.Lexeme != "x" {
		return false
	}
	state := p.l.Snapshot()
	next := p.l.NextToken()
	p.l.Restore(state)
	return p.isExpressionStart(next.Type)
}

func (p *Parser) curPrecedence() precedence {
	if p.curToken.Type == lexer.IDENT && p.curToken.Lexeme == "x" {
		return PRODUCT
	}
	if p, ok := precedences[p.curToken.Type]; ok {
		return p
	}

	return LOWEST
}
