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

	errors []string

	curToken  lexer.Token
	peekToken lexer.Token
}

func New(l *lexer.Lexer) *Parser {
	p := &Parser{
		l:      l,
		errors: []string{},
	}

	p.nextToken()
	p.nextToken()

	return p
}

func (p *Parser) Errors() []string {
	return p.errors
}

func (p *Parser) ParseProgram() *ast.Program {
	program := &ast.Program{}
	for p.curToken.Type != lexer.EOF {
		p.skipComments()

		if p.curToken.Type == lexer.EOF {
			break
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

func (p *Parser) parseStatement() ast.Statement {
	switch p.curToken.Type {
	case lexer.MODULE:
		return p.parseModuleStatement()

	case lexer.IMPORT:
		return p.parseImportStatement()

	case lexer.TYPE:
		return p.parseTypeDeclStatement()

	case lexer.ENUM:
		return p.parseEnumDeclaration()

	case lexer.STRUCT:
		return p.parseStructStatement()

	case lexer.IMPL:
		return p.parseImplStatement()

	case lexer.LET:
		return p.parseLetStatement()

	case lexer.IDENT:
		if p.peekToken.Type == lexer.MUT || p.peekToken.Type == lexer.COLON || p.peekToken.Type == lexer.LT || p.peekToken.Type == lexer.LBRACKET {
			errorsBefore := len(p.errors)
			if stmt := p.parseTypedVariableDeclaration(); stmt != nil || len(p.errors) > errorsBefore {
				return stmt
			}
		}
		return p.parseAssignmentStatement()

	case lexer.LBRACKET:
		return p.parseTypedVariableDeclaration()

	case lexer.COMMENT:
		return p.parseCommentStatement()

	default:
		p.addError("unexpected token %q at %d:%d", p.curToken.Lexeme, p.curToken.Line, p.curToken.Column)
		return nil
	}
}

func (p *Parser) parseModuleStatement() ast.Statement {
	stmt := &ast.ModuleStatement{
		Token: p.curToken,
	}

	if !p.expectPeek(lexer.IDENT) {
		return nil
	}

	stmt.Path = p.parseDottedPath()

	return stmt
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
			p.addError("expected ',' or '}' after struct field")
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

func (p *Parser) parseSliceTypeReference() *ast.TypeReference {
	ref := &ast.TypeReference{
		Token: p.curToken,
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
		value, err := strconv.ParseInt(p.curToken.Lexeme, 10, 64)
		if err != nil {
			p.addError("could not parse integer %q", p.curToken.Lexeme)
			return nil
		}

		return &ast.IntegerLiteral{
			Token: p.curToken,
			Value: value,
		}

	case lexer.FLOAT:
		value, err := strconv.ParseFloat(p.curToken.Lexeme, 64)
		if err != nil {
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
	return tokenType == lexer.IDENT || tokenType == lexer.LBRACKET
}

func (p *Parser) nextToken() {
	p.curToken = p.peekToken
	p.peekToken = p.l.NextToken()
}

func (p *Parser) addError(format string, args ...any) {
	p.errors = append(p.errors, fmt.Sprintf(format, args...))
}

func trimStringQuotes(s string) string {
	if len(s) >= 2 && s[0] == '"' && s[len(s)-1] == '"' {
		return s[1 : len(s)-1]
	}

	return s
}

func (p *Parser) skipStatement() {
	p.nextToken()

	for !p.isAtEnd() && !p.isStatementStart(p.curToken.Type) {
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

	return stmt.Mutable && stmt.Type != nil
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

		if p.peekToken.Type == lexer.EOF || p.isStatementStart(p.peekToken.Type) {
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
