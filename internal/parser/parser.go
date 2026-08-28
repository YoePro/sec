package parser

import (
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"unicode"

	"sec/internal/ast"
	compilerdiagnostics "sec/internal/diagnostics"
	"sec/internal/lexer"
	"sec/internal/modules"
)

type Diagnostic struct {
	ID         string
	Message    string
	Primary    lexer.Token
	Expected   []lexer.TokenType
	Unexpected *lexer.Token
	Context    RecoveryContext
	Episode    int
}

type RecoveryContext string

const (
	RecoveryContextTopLevel   RecoveryContext = "top-level"
	RecoveryContextBlock      RecoveryContext = "block"
	RecoveryContextMatchArm   RecoveryContext = "match-arm"
	RecoveryContextTryHandler RecoveryContext = "try-handler"
	RecoveryContextMember     RecoveryContext = "declaration-member"
)

type RecoveryKind string

const (
	RecoveryInsertMissingToken RecoveryKind = "insert-missing-token"
	RecoverySkipTokens         RecoveryKind = "skip-tokens"
)

type RecoveryConfidence string

const (
	RecoveryExact       RecoveryConfidence = "exact"
	RecoveryUnambiguous RecoveryConfidence = "unambiguous"
	RecoveryProbable    RecoveryConfidence = "probable"
	RecoveryUnknown     RecoveryConfidence = "unknown"
)

type RecoveryEvent struct {
	Kind         RecoveryKind
	Confidence   RecoveryConfidence
	DiagnosticID string
	Start        lexer.Token
	End          lexer.Token
	Expected     []lexer.TokenType
	Skipped      int
	Context      RecoveryContext
	Episode      int
	// diagnosticIndex associates the repair with the parser diagnostic that
	// caused it. It is internal bookkeeping for speculative rollback.
	diagnosticIndex int
}

type ParseResult struct {
	Program     *ast.Program
	Diagnostics []Diagnostic
	Warnings    []string
	Recovery    []RecoveryEvent
	HasErrors   bool
	Fatal       bool
}

type Parser struct {
	l *lexer.Lexer

	errors            []string
	warnings          []string
	diagnostics       []Diagnostic
	recovery          []RecoveryEvent
	limitReached      bool
	diagnosticKeys    map[diagnosticKey]struct{}
	recoveryContext   RecoveryContext
	activeEpisode     int
	nextEpisode       int
	episodePrimary    diagnosticLocation
	hasEpisodePrimary bool
	lexerDiagnostics  int

	curToken  lexer.Token
	peekToken lexer.Token

	stopBeforeBrace bool
	inRefExpression bool
}

type diagnosticKey struct {
	id      string
	file    string
	line    int
	column  int
	context RecoveryContext
}

type diagnosticLocation struct {
	file    string
	line    int
	column  int
	context RecoveryContext
}

const maxParserDiagnostics = 100

func New(l *lexer.Lexer) *Parser {
	p := &Parser{
		l:               l,
		errors:          []string{},
		warnings:        []string{},
		diagnosticKeys:  map[diagnosticKey]struct{}{},
		recoveryContext: RecoveryContextTopLevel,
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

func (p *Parser) Diagnostics() []Diagnostic {
	result := make([]Diagnostic, len(p.diagnostics))
	for i, diagnostic := range p.diagnostics {
		result[i] = diagnostic
		result[i].Expected = append([]lexer.TokenType(nil), diagnostic.Expected...)
		if diagnostic.Unexpected != nil {
			unexpected := *diagnostic.Unexpected
			result[i].Unexpected = &unexpected
		}
	}
	return result
}

func (p *Parser) RecoveryEvents() []RecoveryEvent {
	result := make([]RecoveryEvent, len(p.recovery))
	for i, event := range p.recovery {
		result[i] = event
		result[i].Expected = append([]lexer.TokenType(nil), event.Expected...)
	}
	return result
}

// Parse returns the canonical parser result. ParseProgram remains available as
// a compatibility API while compiler and tooling consumers migrate.
func (p *Parser) Parse() ParseResult {
	program := p.ParseProgram()
	return ParseResult{
		Program:     program,
		Diagnostics: p.Diagnostics(),
		Warnings:    append([]string(nil), p.warnings...),
		Recovery:    p.RecoveryEvents(),
		HasErrors:   len(p.errors) > 0,
		Fatal:       false,
	}
}

func (p *Parser) ParseProgram() *ast.Program {
	program := &ast.Program{}
	for p.curToken.Type != lexer.EOF {
		p.endRecoveryEpisode()
		p.skipComments()

		if p.curToken.Type == lexer.EOF {
			break
		}

		if p.isTargetDirectiveStart() && len(program.Statements) > 0 {
			p.addError("#target directive must appear before any code or declarations at %d:%d", p.curToken.Line, p.curToken.Column)
		}

		if p.curToken.Type == lexer.IMPORT && p.peekToken.Type == lexer.LPAREN {
			program.Statements = append(program.Statements, p.parseImportGroup()...)
			p.nextToken()
			continue
		}

		start := p.curToken
		diagnosticStart := len(p.diagnostics)
		stmt := p.parseStatement()

		if parsedStatementPresent(stmt) {
			program.Statements = append(program.Statements, stmt)
			p.endRecoveryEpisode()
			p.nextToken()
			continue
		}

		recovery := p.skipStatement()
		if isDeclarationStart(start.Type) {
			program.Statements = append(program.Statements, p.invalidDeclaration(start, diagnosticStart, recovery))
		} else {
			program.Statements = append(program.Statements, p.invalidStatement(start, diagnosticStart, recovery))
		}
	}
	return program
}

func parsedStatementPresent(stmt ast.Statement) bool {
	if stmt == nil {
		return false
	}
	value := reflect.ValueOf(stmt)
	switch value.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		return !value.IsNil()
	default:
		return true
	}
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

	case lexer.UNIT:
		return p.parseUnitDeclStatement()

	case lexer.ENUM:
		return p.parseEnumDeclaration()

	case lexer.INTERFACE:
		return p.parseInterfaceDeclaration()

	case lexer.EXTERN:
		return p.parseExternFunctionDeclaration()

	case lexer.FN:
		return p.parseFunctionDeclaration()

	case lexer.FREE:
		return p.parseUnsupportedFreeStatement()

	case lexer.STRUCT:
		return p.parseStructStatement()

	case lexer.IMPL:
		return p.parseImplStatement()

	case lexer.PROPERTY:
		p.addError("property declarations are only valid inside impl blocks at %d:%d", p.curToken.Line, p.curToken.Column)
		p.skipUntilBlockStart()
		p.skipCurrentBlock()
		return nil

	case lexer.LET:
		return p.parseLetStatement()

	case lexer.STATIC:
		return p.parseStaticStatement()

	case lexer.RETURN:
		return p.parseReturnStatement()

	case lexer.TRY:
		return p.parseTryAssignmentStatement()

	case lexer.DEFER:
		return p.parseDeferStatement()

	case lexer.DISCARD:
		return p.parseDiscardStatement()

	case lexer.CANCEL:
		return &ast.CancelStatement{Token: p.curToken}

	case lexer.IF:
		return p.parseIfStatement()

	case lexer.FOR:
		return p.parseForStatement()

	case lexer.WHILE:
		return p.parseWhileStatement()

	case lexer.SWITCH:
		return p.parseSwitchStatement()

	case lexer.SELECT:
		return p.parseSelectStatement()

	case lexer.ELSE:
		return p.parseUnexpectedElseStatement()

	case lexer.FALLTHROUGH:
		return &ast.FallthroughStatement{Token: p.curToken}

	case lexer.BREAK:
		return &ast.BreakStatement{Token: p.curToken}

	case lexer.CONTINUE:
		return &ast.ContinueStatement{Token: p.curToken}

	case lexer.UNSAFE:
		if p.peekToken.Type == lexer.FN || p.peekToken.Type == lexer.EXTERN {
			return p.parseUnsafeFunctionDeclaration()
		}
		return p.parseUnsafeStatement()

	case lexer.ASM:
		return p.parseAsmStatement()

	case lexer.MATCH:
		return p.parseMatchStatement()

	case lexer.SPAWN, lexer.AWAIT:
		return p.parseExpressionOrAssignmentStatement()

	case lexer.ILLEGAL:
		return p.parseExpressionOrAssignmentStatement()

	case lexer.AT:
		if p.peekToken.Type == lexer.IDENT && p.peekToken.Lexeme == "address" {
			return p.parseAddressedLetStatement()
		}
		if p.peekToken.Type == lexer.IDENT && p.peekToken.Lexeme == "link_name" {
			return p.parseLinkNameExternDeclaration()
		}
		if p.peekToken.Type == lexer.IDENT && p.peekToken.Lexeme == "noCopy" {
			return p.parseNoCopyDeclaration()
		}
		return p.parseExpressionOrAssignmentStatement()

	case lexer.IDENT:
		if p.curToken.Lexeme == "detach" {
			return p.parseDetachStatement()
		}
		if p.peekToken.Type == lexer.MUT || p.peekToken.Type == lexer.COLON || p.looksLikeTypedVariableDeclaration() {
			errorsBefore := len(p.errors)
			if stmt := p.parseTypedVariableDeclaration(); stmt != nil || len(p.errors) > errorsBefore {
				return stmt
			}
		}
		return p.parseExpressionOrAssignmentStatement()

	case lexer.REF:
		errorsBefore := len(p.errors)
		if stmt := p.parseTypedVariableDeclaration(); stmt != nil || len(p.errors) > errorsBefore {
			return stmt
		}
		return p.parseExpressionOrAssignmentStatement()

	case lexer.SELF:
		return p.parseExpressionOrAssignmentStatement()

	case lexer.LBRACKET:
		return p.parseTypedVariableDeclaration()

	case lexer.COMMENT:
		return p.parseCommentStatement()

	default:
		unexpected := p.curToken
		p.addDiagnostic(
			compilerdiagnostics.ParserUnexpectedToken,
			unexpected,
			nil,
			&unexpected,
			"unexpected token %q at %d:%d",
			unexpected.Lexeme,
			unexpected.Line,
			unexpected.Column,
		)
		return nil
	}
}

func (p *Parser) parseUnsupportedFreeStatement() ast.Statement {
	stmt := &ast.InvalidStatement{
		Token:   p.curToken,
		Message: "free operations are reserved for destruction but are not implemented yet",
	}
	if p.peekToken.Type == lexer.LBRACE {
		p.nextToken()
		p.skipCurrentBlock()
		return stmt
	}
	p.skipInvalidImplMember()
	return stmt
}

func (p *Parser) looksLikeTypedVariableDeclaration() bool {
	if !p.isTypeNameToken(p.curToken.Type) || (p.peekToken.Type != lexer.LT && p.peekToken.Type != lexer.LBRACKET && p.peekToken.Type != lexer.LPAREN) {
		return false
	}

	curToken := p.curToken
	peekToken := p.peekToken
	lexerState := p.l.Snapshot()
	errorCount := len(p.errors)
	warningCount := len(p.warnings)

	_ = p.parseTypeReference()
	ok := p.peekToken.Type == lexer.MUT || p.peekToken.Type == lexer.COLON || p.looksLikeTypedVariableGroupAfterParsedType()

	p.curToken = curToken
	p.peekToken = peekToken
	p.l.Restore(lexerState)
	p.rollbackErrors(errorCount)
	p.warnings = p.warnings[:warningCount]

	return ok
}

func (p *Parser) looksLikeTypedVariableGroupAfterParsedType() bool {
	if p.peekToken.Type != lexer.LPAREN {
		return false
	}
	if p.peekToken.Line > p.curToken.Line {
		return true
	}

	curToken := p.curToken
	peekToken := p.peekToken
	lexerState := p.l.Snapshot()
	errorCount := len(p.errors)
	warningCount := len(p.warnings)

	p.nextToken()
	if p.peekToken.Type == lexer.COMMENT {
		p.nextToken()
	}
	ok := p.peekToken.Type == lexer.IDENT
	if ok {
		if p.peekToken.Line > p.curToken.Line {
			p.curToken = curToken
			p.peekToken = peekToken
			p.l.Restore(lexerState)
			p.rollbackErrors(errorCount)
			p.warnings = p.warnings[:warningCount]
			return true
		}
		p.nextToken()
		ok = p.peekToken.Type == lexer.DECLARE
	}

	p.curToken = curToken
	p.peekToken = peekToken
	p.l.Restore(lexerState)
	p.rollbackErrors(errorCount)
	p.warnings = p.warnings[:warningCount]

	return ok
}

func (p *Parser) parseDiscardStatement() ast.Statement {
	stmt := &ast.DiscardStatement{Token: p.curToken}
	if p.peekToken.Type == lexer.RBRACE || p.peekToken.Type == lexer.EOF {
		p.addError("discard requires expression at %d:%d", p.peekToken.Line, p.peekToken.Column)
		return stmt
	}
	p.nextToken()
	stmt.Value = p.parseExpression(LOWEST)
	if ident, ok := stmt.Value.(*ast.Identifier); ok {
		stmt.Name = ident
	}
	return stmt
}

func (p *Parser) parseDetachStatement() ast.Statement {
	stmt := &ast.DetachStatement{Token: p.curToken}
	if p.peekToken.Type == lexer.RBRACE || p.peekToken.Type == lexer.EOF {
		p.addError("detach requires task or thread handle at %d:%d", p.peekToken.Line, p.peekToken.Column)
		return stmt
	}
	p.nextToken()
	stmt.Value = p.parseExpression(LOWEST)
	if p.peekToken.Type == lexer.DISCARD {
		p.nextToken()
		stmt.DiscardResult = true
	}
	return stmt
}

func (p *Parser) parseDeferStatement() ast.Statement {
	stmt := &ast.DeferStatement{Token: p.curToken}
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
			unexpected := p.peekToken
			p.addDiagnostic(
				compilerdiagnostics.ParserInvalidAssignmentExpr,
				unexpected,
				[]lexer.TokenType{lexer.LBRACE},
				&unexpected,
				"assignment in while condition at %d:%d",
				unexpected.Line,
				unexpected.Column,
			)
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
	start, end, skipped := p.curToken, p.curToken, 0
	delimiters := newDelimiterStack()
	for p.curToken.Type != lexer.EOF {
		if delimiters.empty() && p.curToken.Type == lexer.LBRACE {
			if skipped > 0 {
				p.recordSkippedRecovery(start, end, skipped, RecoveryProbable)
			}
			stmt.Body = p.parseStatementBlock("for body")
			return
		}
		if delimiters.empty() && p.curToken.Type == lexer.RBRACE {
			break
		}
		if !delimiters.canConsume(p.curToken.Type) {
			break
		}
		end = p.curToken
		skipped++
		delimiters.consume(p.curToken.Type)
		if delimiters.empty() && p.peekToken.Type == lexer.LBRACE {
			p.recordSkippedRecovery(start, end, skipped, RecoveryProbable)
			p.nextToken()
			stmt.Body = p.parseStatementBlock("for body")
			return
		}
		p.nextToken()
	}
	if skipped > 0 {
		p.recordSkippedRecovery(start, end, skipped, RecoveryProbable)
	} else {
		p.endRecoveryEpisode()
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

func (p *Parser) parseSelectStatement() ast.Statement {
	stmt := &ast.SelectStatement{Token: p.curToken}
	if !p.expectPeek(lexer.LBRACE) {
		return stmt
	}
	p.nextToken()

	for p.curToken.Type != lexer.RBRACE && p.curToken.Type != lexer.EOF {
		if p.curToken.Type == lexer.COMMENT {
			p.nextToken()
			continue
		}

		branch := p.parseSelectBranch()
		if branch != nil {
			if branch.Kind == ast.SelectDefaultBranch {
				if hasSelectDefault(stmt) {
					stmt.DuplicateDefaultTokens = append(stmt.DuplicateDefaultTokens, branch.Token)
				}
				if p.curToken.Type != lexer.RBRACE && p.curToken.Type != lexer.EOF {
					stmt.DefaultNotFinalToken = p.curToken
				}
			}
			if branch.Kind == ast.SelectTimeoutBranch && hasSelectDefault(stmt) {
				stmt.UnreachableTimeoutToken = branch.Token
			}
			stmt.Branches = append(stmt.Branches, branch)
			continue
		}

		p.skipSelectBranch()
	}

	if p.curToken.Type == lexer.EOF {
		p.addError("unterminated select body")
		return nil
	}
	return stmt
}

func hasSelectDefault(stmt *ast.SelectStatement) bool {
	for _, branch := range stmt.Branches {
		if branch != nil && branch.Kind == ast.SelectDefaultBranch {
			return true
		}
	}
	return false
}

func (p *Parser) parseSelectBranch() *ast.SelectBranch {
	branch := &ast.SelectBranch{Token: p.curToken, Kind: ast.SelectOperationBranch}

	switch p.curToken.Type {
	case lexer.DEFAULT:
		branch.Kind = ast.SelectDefaultBranch
		if !p.expectPeek(lexer.ARROW) {
			return branch
		}
	case lexer.AFTER:
		branch.Kind = ast.SelectTimeoutBranch
		p.nextToken()
		branch.Value = p.parseExpression(LOWEST)
		if branch.Value == nil {
			return branch
		}
		if !p.expectPeek(lexer.ARROW) {
			return branch
		}
	default:
		if p.curToken.Type == lexer.IDENT && p.peekToken.Type == lexer.DECLARE {
			branch.Binding = &ast.Identifier{Token: p.curToken, Value: p.curToken.Lexeme}
			p.nextToken()
			p.nextToken()
		}
		branch.Value = p.parseExpression(LOWEST)
		if branch.Value == nil {
			return branch
		}
		if !p.expectPeek(lexer.ARROW) {
			return branch
		}
	}

	if !p.expectPeek(lexer.LBRACE) {
		return branch
	}
	branch.Body = p.parseStatementBlock("select branch")
	if branch.Body == nil {
		return branch
	}
	p.nextToken()
	return branch
}

func (p *Parser) skipSelectBranch() RecoveryEvent {
	start, end, skipped := p.curToken, p.curToken, 0
	delimiters := newDelimiterStack()
	for p.curToken.Type != lexer.EOF {
		if delimiters.empty() && (p.curToken.Type == lexer.RBRACE || p.curToken.Type == lexer.DEFAULT || p.curToken.Type == lexer.AFTER || p.curToken.Type == lexer.IDENT || p.curToken.Type == lexer.AWAIT) {
			break
		}
		if !delimiters.canConsume(p.curToken.Type) {
			break
		}
		end = p.curToken
		skipped++
		delimiters.consume(p.curToken.Type)
		p.nextToken()
	}
	if skipped == 0 {
		p.endRecoveryEpisode()
		return RecoveryEvent{}
	}
	return p.recordSkippedRecovery(start, end, skipped, RecoveryProbable)
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
			if !subjectless && p.peekToken.Type == lexer.OR {
				p.addError("use ',' between switch case values; '||' creates a boolean expression at %d:%d", p.peekToken.Line, p.peekToken.Column)
				p.skipSwitchClause()
				return clause
			}
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

func (p *Parser) skipSwitchClause() RecoveryEvent {
	start, end, skipped := p.curToken, p.curToken, 0
	delimiters := newDelimiterStack()
	for p.curToken.Type != lexer.EOF {
		if delimiters.empty() && (p.curToken.Type == lexer.RBRACE || p.curToken.Type == lexer.CASE || p.curToken.Type == lexer.DEFAULT) {
			break
		}
		if !delimiters.canConsume(p.curToken.Type) {
			break
		}
		end = p.curToken
		skipped++
		delimiters.consume(p.curToken.Type)
		p.nextToken()
	}
	if skipped == 0 {
		p.endRecoveryEpisode()
		return RecoveryEvent{}
	}
	return p.recordSkippedRecovery(start, end, skipped, RecoveryProbable)
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
	case *ast.InvalidExpression:
		return expr.Token
	case *ast.Identifier:
		return expr.Token
	case *ast.IntegerLiteral:
		return expr.Token
	case *ast.FloatLiteral:
		return expr.Token
	case *ast.StringLiteral:
		return expr.Token
	case *ast.CharLiteral:
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
	case *ast.SpreadExpression:
		return expr.Token
	case *ast.RangeExpression:
		return expr.Token
	case *ast.MemberExpression:
		return expr.Token
	case *ast.ArrayLiteral:
		return expr.Token
	case *ast.IndexExpression:
		return expr.Token
	case *ast.SliceExpression:
		return expr.Token
	case *ast.RefExpression:
		return expr.Token
	case *ast.StructLiteral:
		return expr.Token
	default:
		return lexer.Token{}
	}
}

func (p *Parser) parseUnexpectedElseStatement() ast.Statement {
	stmt := &ast.InvalidStatement{Token: p.curToken}
	unexpected := p.curToken
	p.addDiagnostic(
		compilerdiagnostics.ParserMisplacedKeyword,
		unexpected,
		nil,
		&unexpected,
		"else without matching if at %d:%d",
		unexpected.Line,
		unexpected.Column,
	)

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

	if !isPathSegmentToken(p.peekToken.Type) {
		p.addError("module declaration missing name at %d:%d", p.curToken.Line, p.curToken.Column)
		return nil
	}
	p.nextToken()

	nameToken := p.curToken
	stmt.Path = p.parseDottedPath()
	if strings.Contains(stmt.Path, ".") {
		p.addError("module declaration must contain one identifier, got %q at %d:%d", stmt.Path, nameToken.Line, nameToken.Column)
	}

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

func (p *Parser) skipCompilerDirective() RecoveryEvent {
	start, end, skipped := p.curToken, p.curToken, 1
	delimiters := newDelimiterStack(lexer.RPAREN)
	for p.peekToken.Type != lexer.EOF && !delimiters.empty() {
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
	p.validateImportPath(stmt.Path, p.curToken)

	return stmt
}

func (p *Parser) parseImportGroup() []ast.Statement {
	importToken := p.curToken
	p.nextToken()
	imports := []ast.Statement{}

	for {
		p.nextToken()
		for p.curToken.Type == lexer.COMMENT {
			p.nextToken()
		}
		if p.curToken.Type == lexer.RPAREN {
			return imports
		}
		if p.curToken.Type == lexer.EOF {
			p.addError("unterminated import group at %d:%d", importToken.Line, importToken.Column)
			return imports
		}

		stmt := &ast.ImportStatement{Token: importToken}
		if p.curToken.Type == lexer.IDENT {
			stmt.Alias = p.curToken.Lexeme
			if !p.expectPeek(lexer.STRING) {
				return imports
			}
		} else if p.curToken.Type != lexer.STRING {
			p.addError("expected import path or alias, got %q at %d:%d", p.curToken.Lexeme, p.curToken.Line, p.curToken.Column)
			return imports
		}
		stmt.Path = trimStringQuotes(p.curToken.Lexeme)
		p.validateImportPath(stmt.Path, p.curToken)
		imports = append(imports, stmt)
	}
}

func (p *Parser) validateImportPath(path string, token lexer.Token) {
	if err := modules.ValidateImportPath(path); err != nil {
		p.addError("invalid import path %q: %s at %d:%d", path, err, token.Line, token.Column)
	}
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
	if p.reversedTypeDeclarationKind() {
		kind := p.peekToken
		p.nextToken()
		name := "Name"
		if p.peekToken.Type == lexer.IDENT {
			name = p.peekToken.Lexeme
		}
		p.addDiagnostic(
			compilerdiagnostics.ParserMisplacedKeyword,
			kind,
			[]lexer.TokenType{lexer.IDENT},
			&kind,
			"type declaration name must come before %s; write 'type %s %s' at %d:%d",
			kind.Lexeme,
			name,
			kind.Lexeme,
			kind.Line,
			kind.Column,
		)
		return nil
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

		if p.isContractStart(p.peekToken) {
			stmt.Contract = p.parseContractSequence()
			if stmt.Contract == nil {
				return nil
			}
		}
		if p.peekToken.Type == lexer.DEFAULT {
			p.nextToken()
			stmt.DefaultToken = p.curToken
			if !p.expectPeekExpressionStart() {
				return nil
			}
			stmt.Default = p.parseExpression(LOWEST)
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

	if p.peekToken.Type == lexer.ENUM {
		p.nextToken()
		enum := &ast.EnumDeclaration{Token: stmt.Token, Name: stmt.Name}
		if !p.parseEnumUnderlying(enum) {
			return nil
		}
		return p.parseEnumBody(enum)
	}

	if p.peekToken.Type == lexer.STRUCT {
		p.nextToken()
		if p.peekToken.Type == lexer.IMPLEMENTS {
			p.rejectTypeImplementsClause(stmt.Name.Value)
		}
		stmt.StructType = p.parseStructType()
		return stmt
	}

	if p.peekToken.Type == lexer.IDENT && p.peekToken.Lexeme == "register" {
		p.nextToken()
		stmt.RegisterType = p.parseRegisterType()
		if p.peekToken.Type == lexer.IMPLEMENTS {
			p.rejectTypeImplementsClause(stmt.Name.Value)
		}
		return stmt
	}

	if p.peekToken.Type == lexer.UNION {
		p.nextToken()
		stmt.Union = true
		// rules/errors/errorhandling.md section 3.2 places the concrete-error
		// marker after the union kind: `type Name union error { ... }`.
		if p.peekToken.Type == lexer.IDENT && p.peekToken.Lexeme == "error" {
			p.nextToken()
			stmt.ErrorType = true
			stmt.ErrorToken = p.curToken
		}
		if p.peekToken.Type == lexer.IMPLEMENTS {
			p.rejectTypeImplementsClause(stmt.Name.Value)
		}
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

	if p.peekToken.Type == lexer.IMPLEMENTS {
		p.rejectTypeImplementsClause(stmt.Name.Value)
	}

	if p.isContractStart(p.peekToken) {
		stmt.Contract = p.parseContractSequence()
		if stmt.Contract == nil {
			return nil
		}
	}
	if p.peekToken.Type == lexer.DEFAULT {
		p.nextToken()
		stmt.DefaultToken = p.curToken
		if !p.expectPeekExpressionStart() {
			return nil
		}
		stmt.Default = p.parseExpression(LOWEST)
	}

	return stmt
}

func (p *Parser) rejectTypeImplementsClause(typeName string) {
	p.nextToken()
	p.addError("interface conformance for %s belongs on its primary impl; use impl %s implements Interface at %d:%d", typeName, typeName, p.curToken.Line, p.curToken.Column)
	_ = p.parseImplementsList()
}

func (p *Parser) reversedTypeDeclarationKind() bool {
	if p.peekToken.Type == lexer.STRUCT || p.peekToken.Type == lexer.UNION {
		return true
	}
	return p.peekToken.Type == lexer.IDENT && p.peekToken.Lexeme == "register"
}

func (p *Parser) parseUnitDeclStatement() ast.Statement {
	stmt := &ast.UnitDeclStatement{Token: p.curToken}

	if !p.expectPeek(lexer.IDENT) {
		return nil
	}
	stmt.Name = &ast.Identifier{Token: p.curToken, Value: p.curToken.Lexeme}

	stmt.BaseType = &ast.TypeReference{Token: stmt.Name.Token, Name: "decimal"}

	if p.peekToken.Type == lexer.EOF || p.peekToken.Line > stmt.Name.Token.Line {
		return stmt
	}

	if p.peekToken.Type == lexer.IDENT && isUnitCategoryName(p.peekToken.Lexeme) {
		p.nextToken()
		stmt.Category = p.curToken.Lexeme
		return stmt
	}

	if !isTypeStart(p.peekToken.Type) {
		return stmt
	}
	p.nextToken()
	stmt.BaseType = p.parseTypeReference()

	if p.peekToken.Type == lexer.IDENT && p.peekToken.Line == stmt.Name.Token.Line {
		p.nextToken()
		if isUnitCategoryName(p.curToken.Lexeme) {
			stmt.Category = p.curToken.Lexeme
		} else {
			p.addError("unit %s category must be physical, currency, information, ratio, or other, got %q at %d:%d", stmt.Name.Value, p.curToken.Lexeme, p.curToken.Line, p.curToken.Column)
		}
	}

	return stmt
}

func isUnitCategoryName(name string) bool {
	switch name {
	case "physical", "currency", "information", "ratio", "other":
		return true
	default:
		return false
	}
}

func (p *Parser) parseInterfaceDeclaration() ast.Statement {
	stmt := &ast.InterfaceDeclaration{Token: p.curToken}

	if !p.expectPeek(lexer.IDENT) {
		return nil
	}
	stmt.Name = &ast.Identifier{Token: p.curToken, Value: p.curToken.Lexeme}

	if p.isAttachedGenericListStart(stmt.Name) {
		p.nextToken()
		stmt.GenericParameters = p.parseGenericParameters()
		if stmt.GenericParameters == nil {
			return nil
		}
	}

	if p.peekToken.Type == lexer.IMPLEMENTS {
		p.nextToken()
		stmt.Implements = p.parseImplementsList()
		if stmt.Implements == nil {
			return nil
		}
	}

	if !p.expectPeek(lexer.LBRACE) {
		return nil
	}
	previousContext := p.recoveryContext
	p.recoveryContext = RecoveryContextMember
	defer func() { p.recoveryContext = previousContext }()

	for p.peekToken.Type != lexer.RBRACE && p.peekToken.Type != lexer.EOF {
		p.endRecoveryEpisode()
		p.nextToken()
		if p.curToken.Type == lexer.COMMENT {
			continue
		}
		switch p.curToken.Type {
		case lexer.FN:
			fn := p.parseFunctionSignature()
			if fn == nil {
				return nil
			}
			stmt.Methods = append(stmt.Methods, fn)
		case lexer.MUT:
			if !p.expectPeek(lexer.FN) {
				return nil
			}
			fn := p.parseFunctionSignature()
			if fn == nil {
				return nil
			}
			fn.ReceiverCapability = ast.ReceiverMutable
			stmt.Methods = append(stmt.Methods, fn)
		case lexer.CONSUME_ARROW:
			if !p.expectPeek(lexer.FN) {
				return nil
			}
			fn := p.parseFunctionSignature()
			if fn == nil {
				return nil
			}
			fn.ReceiverCapability = ast.ReceiverConsuming
			stmt.Methods = append(stmt.Methods, fn)
		case lexer.STATIC:
			// rules/declarations/static.md, sections 11-12; properties.md,
			// section 10. Interfaces preserve the static/instance category.
			if p.peekToken.Type == lexer.FN {
				p.nextToken()
				fn := p.parseFunctionSignature()
				if fn == nil {
					return nil
				}
				fn.Static = true
				stmt.Methods = append(stmt.Methods, fn)
				break
			}
			if p.peekToken.Type == lexer.PROPERTY {
				p.nextToken()
				property := p.parseInterfaceProperty()
				if property == nil {
					return nil
				}
				property.Static = true
				stmt.Properties = append(stmt.Properties, property)
				break
			}
			p.addError("static inside interface must modify fn or property at %d:%d", p.curToken.Line, p.curToken.Column)
			return nil
		case lexer.PROPERTY:
			property := p.parseInterfaceProperty()
			if property == nil {
				return nil
			}
			stmt.Properties = append(stmt.Properties, property)
		case lexer.IDENT:
			if p.curToken.Lexeme != "event" {
				if p.peekToken.Type == lexer.COLON {
					p.addError("interfaces cannot require stored field %q; use a property or method at %d:%d", p.curToken.Lexeme, p.curToken.Line, p.curToken.Column)
					line := p.curToken.Line
					for p.peekToken.Type != lexer.EOF && p.peekToken.Type != lexer.RBRACE && p.peekToken.Line == line {
						p.nextToken()
					}
					continue
				} else {
					p.addError("interface block may only contain method, property, event, and permitted associated requirements at %d:%d", p.curToken.Line, p.curToken.Column)
				}
				p.skipCurrentBlock()
				return nil
			}
			event := p.parseInterfaceEvent()
			if event == nil {
				return nil
			}
			stmt.Events = append(stmt.Events, event)
		default:
			p.addError("interface block may only contain fn, property, and event requirements at %d:%d", p.curToken.Line, p.curToken.Column)
			p.skipCurrentBlock()
			return nil
		}
	}

	if !p.expectPeek(lexer.RBRACE) {
		return stmt
	}

	return stmt
}

func (p *Parser) parseInterfaceEvent() *ast.InterfaceEvent {
	event := &ast.InterfaceEvent{Token: p.curToken}
	if p.peekToken.Type != lexer.IDENT {
		p.nextToken()
		p.addError("event requirement missing name at %d:%d", p.curToken.Line, p.curToken.Column)
		return nil
	}
	p.nextToken()
	event.Name = &ast.Identifier{Token: p.curToken, Value: p.curToken.Lexeme}
	if !p.expectPeek(lexer.LBRACKET) {
		return event
	}
	if !p.expectPeekTypeStart() {
		return event
	}
	event.Payload = p.parseTypeReference()
	if !p.expectPeek(lexer.RBRACKET) {
		return event
	}
	return event
}

func (p *Parser) parseImplementsList() []*ast.TypeReference {
	interfaces := []*ast.TypeReference{}
	if !p.expectPeekTypeStart() {
		return nil
	}

	for {
		interfaces = append(interfaces, p.parseTypeReference())
		if p.peekToken.Type != lexer.COMMA {
			return interfaces
		}
		p.nextToken()
		if !p.expectPeekTypeStart() {
			return nil
		}
	}
}

func (p *Parser) parseInterfaceProperty() *ast.InterfaceProperty {
	property := &ast.InterfaceProperty{Token: p.curToken}
	if !p.expectPeek(lexer.IDENT) {
		return nil
	}
	property.Name = &ast.Identifier{Token: p.curToken, Value: p.curToken.Lexeme}

	if !p.expectPeek(lexer.COLON) {
		return nil
	}
	if !p.expectPeekTypeStart() {
		return nil
	}
	property.Type = p.parseTypeReference()

	if !p.expectPeek(lexer.LBRACE) {
		return nil
	}
	previousContext := p.recoveryContext
	p.recoveryContext = RecoveryContextMember
	defer func() { p.recoveryContext = previousContext }()

	for p.peekToken.Type != lexer.RBRACE && p.peekToken.Type != lexer.EOF {
		p.endRecoveryEpisode()
		p.nextToken()
		switch p.curToken.Type {
		case lexer.GET:
			if property.RequiresGet {
				p.addError("duplicate get in interface property %q", property.Name.Value)
				return nil
			}
			property.RequiresGet = true
		case lexer.SET, lexer.IDENT:
			if p.curToken.Lexeme != "set" {
				p.addError("unexpected token %q in interface property %s at %d:%d", p.curToken.Lexeme, property.Name.Value, p.curToken.Line, p.curToken.Column)
				return nil
			}
			if !p.parseInterfacePropertySetter(property, false) {
				continue
			}
		case lexer.TRY:
			if !p.expectPeekContextualSet() {
				return nil
			}
			if !p.parseInterfacePropertySetter(property, true) {
				continue
			}
		case lexer.COMMENT:
			continue
		default:
			p.addError("unexpected token %q in interface property %s at %d:%d", p.curToken.Lexeme, property.Name.Value, p.curToken.Line, p.curToken.Column)
			return nil
		}
	}

	if !property.RequiresGet && !property.RequiresSet {
		p.addError("interface property %q must require get or set", property.Name.Value)
		return nil
	}

	if !p.expectPeek(lexer.RBRACE) {
		return property
	}
	return property
}

func (p *Parser) parseInterfacePropertySetter(property *ast.InterfaceProperty, fallible bool) bool {
	if property.RequiresSet {
		p.addError("duplicate set in interface property %q", property.Name.Value)
		if p.peekToken.Type == lexer.IDENT {
			p.nextToken()
		}
		return false
	}
	property.RequiresSet = true
	property.SetterFallible = fallible
	property.SetToken = p.curToken
	if p.peekToken.Type != lexer.IDENT {
		p.addError(
			"setter for %s must declare value parameter at %d:%d",
			property.Name.Value,
			property.SetToken.Line,
			property.SetToken.Column,
		)
		return false
	}
	p.nextToken()
	property.SetterParameter = &ast.Identifier{Token: p.curToken, Value: p.curToken.Lexeme}
	return true
}

func (p *Parser) parseEnumDeclaration() *ast.EnumDeclaration {
	enum := &ast.EnumDeclaration{Token: p.curToken}

	if !p.expectPeek(lexer.IDENT) {
		return nil
	}
	enum.Name = &ast.Identifier{Token: p.curToken, Value: p.curToken.Lexeme}

	if !p.parseEnumUnderlying(enum) {
		return nil
	}

	return p.parseEnumBody(enum)
}

func (p *Parser) parseEnumUnderlying(enum *ast.EnumDeclaration) bool {
	hadColon := false
	if p.peekToken.Type == lexer.COLON {
		p.nextToken()
		hadColon = true
	}
	if p.peekToken.Type == lexer.LBRACE {
		if hadColon {
			p.addError("expected enum underlying type after ':' at %d:%d", p.curToken.Line, p.curToken.Column)
			return false
		}
		return true
	}
	if p.peekToken.Type == lexer.IDENT && p.peekToken.Lexeme == "error" {
		p.nextToken()
		enum.ErrorType = true
		enum.ErrorToken = p.curToken
		return true
	}
	if !p.expectPeekTypeStart() {
		return false
	}
	if p.curToken.Lexeme != "bit" {
		enum.UnderlyingType = p.parseTypeReference()
		p.parseOptionalEnumErrorMarker(enum)
		return true
	}

	enum.BitUnderlying = true
	enum.UnderlyingBitWidth = 1
	if p.peekToken.Type != lexer.LBRACKET {
		p.parseOptionalEnumErrorMarker(enum)
		return true
	}
	p.nextToken()
	width, ok := p.parseRegisterWidth("enum bit width")
	if !ok {
		return false
	}
	enum.UnderlyingBitWidth = width
	if !p.expectPeek(lexer.RBRACKET) {
		return false
	}
	p.parseOptionalEnumErrorMarker(enum)
	return true
}

// parseOptionalEnumErrorMarker retains the canonical post-representation
// marker from rules/declarations/enums.md. `error` is a marker here, not an
// integer representation type.
func (p *Parser) parseOptionalEnumErrorMarker(enum *ast.EnumDeclaration) {
	if p.peekToken.Type != lexer.IDENT || p.peekToken.Lexeme != "error" {
		return
	}
	p.nextToken()
	enum.ErrorType = true
	enum.ErrorToken = p.curToken
}

func (p *Parser) parseEnumBody(enum *ast.EnumDeclaration) *ast.EnumDeclaration {
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
		if p.peekToken.Type == lexer.DEFAULT {
			p.nextToken()
			value.Default = true
			value.DefaultToken = p.curToken
		}
		if p.peekToken.Type == lexer.ASSIGN || p.peekToken.Type == lexer.COLON {
			if p.peekToken.Type == lexer.COLON {
				p.addWarning("enum initializer ':' is non-canonical; sec fmt will rewrite it to '=' at %d:%d", p.peekToken.Line, p.peekToken.Column)
			}
			p.nextToken()
			p.nextToken()
			value.Initializer = p.parseExpression(LOWEST)
			if value.Initializer == nil {
				return nil
			}
		}
		enum.Values = append(enum.Values, value)
		p.skipPeekComments()

		switch p.peekToken.Type {
		case lexer.COMMA:
			p.nextToken()
		case lexer.RBRACE:
			continue
		default:
			if p.peekToken.Type == lexer.IDENT && p.peekToken.Line > p.curToken.Line {
				continue
			}
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

func (p *Parser) parseExternFunctionDeclaration() *ast.FunctionDeclaration {
	externToken := p.curToken
	if !p.expectPeek(lexer.STRING) {
		p.addError("extern declaration requires ABI string at %d:%d", p.peekToken.Line, p.peekToken.Column)
		return nil
	}
	abi := trimStringQuotes(p.curToken.Lexeme)
	if !p.expectPeek(lexer.FN) {
		return nil
	}

	fn := p.parseFunctionSignature()
	if fn == nil {
		return nil
	}
	fn.Token = externToken
	fn.Extern = true
	fn.ABI = abi
	if p.peekToken.Type == lexer.LBRACE {
		fn.Body = p.parseFunctionBlockStatement()
		if fn.Body == nil {
			return nil
		}
		p.addError("extern function declarations may not have a Sec body at %d:%d", externToken.Line, externToken.Column)
	}
	return fn
}

func (p *Parser) parseFunctionSignature() *ast.FunctionDeclaration {
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

func (p *Parser) skipGenericParameterList() RecoveryEvent {
	start, end, skipped := p.curToken, p.curToken, 1
	delimiters := newDelimiterStack(lexer.RBRACKET)
	for p.peekToken.Type != lexer.EOF && !delimiters.empty() {
		if delimiters.depth() == 1 && (p.peekToken.Type == lexer.LBRACE || p.peekToken.Type == lexer.LPAREN) {
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

func (p *Parser) parseUnsafeFunctionDeclaration() *ast.FunctionDeclaration {
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

func (p *Parser) parseParameters() []*ast.Parameter {
	parameters := []*ast.Parameter{}

	if p.peekToken.Type == lexer.RPAREN {
		p.nextToken()
		return parameters
	}

	for {
		consuming := false
		if p.peekToken.Type == lexer.CONSUME_ARROW {
			p.nextToken()
			consuming = true
		}
		if p.peekToken.Type == lexer.REF {
			if consuming {
				p.addError("consuming parameter cannot use ref type at %d:%d", p.peekToken.Line, p.peekToken.Column)
				return nil
			}
			p.nextToken()
		}
		ref := p.curToken.Type == lexer.REF
		mutableRef := false
		if ref && p.peekToken.Type == lexer.MUT {
			p.nextToken()
			mutableRef = true
		}

		if p.peekToken.Type != lexer.IDENT && p.peekToken.Type != lexer.SELF {
			p.addError("expected next token to be %q, got %q at %d:%d", lexer.IDENT, p.peekToken.Type, p.peekToken.Line, p.peekToken.Column)
			return nil
		}
		p.nextToken()

		parameter := &ast.Parameter{
			Token:      p.curToken,
			Name:       &ast.Identifier{Token: p.curToken, Value: p.curToken.Lexeme},
			Ref:        ref,
			MutableRef: mutableRef,
			Consuming:  consuming,
		}

		if ref && p.curToken.Type == lexer.SELF && p.peekToken.Type != lexer.COLON {
			parameter.Type = &ast.TypeReference{Token: p.curToken, Name: "self", Ref: true, MutableRef: mutableRef}
			parameters = append(parameters, parameter)
			switch p.peekToken.Type {
			case lexer.COMMA:
				p.nextToken()
				if p.peekToken.Type == lexer.RPAREN {
					p.nextToken()
					return parameters
				}
				continue
			case lexer.RPAREN:
				p.nextToken()
				return parameters
			default:
				p.addError("expected ',' or ')' after parameter at %d:%d", p.peekToken.Line, p.peekToken.Column)
				return nil
			}
		}

		if !p.expectPeek(lexer.COLON) {
			return nil
		}

		if !p.expectPeekTypeStart() {
			return nil
		}
		parameter.Type = p.parseTypeReference()
		if ref {
			parameter.Type.Ref = true
			parameter.Type.MutableRef = mutableRef
		}
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
	previousContext := p.recoveryContext
	p.recoveryContext = RecoveryContextBlock
	defer func() { p.recoveryContext = previousContext }()

	p.nextToken()
	for p.curToken.Type != lexer.RBRACE && p.curToken.Type != lexer.EOF {
		p.endRecoveryEpisode()
		if p.curToken.Type == lexer.COMMENT {
			p.nextToken()
			continue
		}

		start := p.curToken
		diagnosticStart := len(p.diagnostics)
		stmt := p.parseStatement()
		if parsedStatementPresent(stmt) {
			block.Statements = append(block.Statements, stmt)
			p.endRecoveryEpisode()
			p.nextToken()
			continue
		}

		recovery := p.skipStatement()
		block.Statements = append(block.Statements, p.invalidStatement(start, diagnosticStart, recovery))
	}

	if p.curToken.Type == lexer.EOF {
		p.addError("unterminated %s", name)
		return nil
	}

	return block
}

func (p *Parser) parseReturnStatement() ast.Statement {
	stmt := &ast.ReturnStatement{Token: p.curToken}

	if p.peekToken.Type == lexer.RBRACE || p.peekToken.Type == lexer.EOF || isSwitchClauseStart(p.peekToken) {
		return stmt
	}
	if p.peekToken.Line > p.curToken.Line && p.isReturnTerminator(p.peekToken.Type) {
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
		lexer.CANCEL,
		lexer.CONTINUE,
		lexer.UNSAFE,
		lexer.ASM,
		lexer.DEFER,
		lexer.DISCARD:
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

func (p *Parser) parseRegisterType() *ast.RegisterType {
	registerType := &ast.RegisterType{
		Token:           p.curToken,
		AllocationOrder: "lsb-first",
	}
	allocationOrderSet := false

	if !p.expectPeek(lexer.LBRACKET) {
		return registerType
	}
	widthExpression, width, ok := p.parseRegisterWidthExpression("register width")
	if !ok {
		return registerType
	}
	registerType.Width = width
	registerType.WidthExpression = widthExpression
	if !p.expectPeek(lexer.RBRACKET) {
		return registerType
	}
	for p.peekToken.Type == lexer.IDENT {
		modifierToken := p.peekToken
		p.nextToken()
		modifier, ok := p.parseRegisterTypeModifier()
		if !ok {
			p.addError("unknown register modifier %q at %d:%d", modifierToken.Lexeme, modifierToken.Line, modifierToken.Column)
			return registerType
		}
		switch modifier {
		case "lsb-first", "msb-first":
			if allocationOrderSet {
				if registerType.AllocationOrder == modifier {
					p.addError("duplicate register allocation modifier %q at %d:%d", modifier, modifierToken.Line, modifierToken.Column)
				} else {
					p.addError("conflicting register allocation modifiers %q and %q at %d:%d", registerType.AllocationOrder, modifier, modifierToken.Line, modifierToken.Column)
				}
				return registerType
			}
			registerType.AllocationOrder = modifier
			allocationOrderSet = true
		case "little-endian", "big-endian":
			if registerType.ByteOrder != "" {
				if registerType.ByteOrder == modifier {
					p.addError("duplicate register byte-order modifier %q at %d:%d", modifier, modifierToken.Line, modifierToken.Column)
				} else {
					p.addError("conflicting register byte-order modifiers %q and %q at %d:%d", registerType.ByteOrder, modifier, modifierToken.Line, modifierToken.Column)
				}
				return registerType
			}
			registerType.ByteOrder = modifier
		}
	}
	if !p.expectPeek(lexer.LBRACE) {
		return registerType
	}

	registerType.Fields = p.parseRegisterFields()
	if !p.expectPeek(lexer.RBRACE) {
		return registerType
	}

	return registerType
}

func (p *Parser) parseRegisterTypeModifier() (string, bool) {
	prefix := p.curToken.Lexeme
	if prefix != "lsb" && prefix != "msb" && prefix != "little" && prefix != "big" {
		return "", false
	}
	if !p.expectPeek(lexer.MINUS) || !p.expectPeek(lexer.IDENT) {
		return "", false
	}
	suffix := p.curToken.Lexeme
	if (prefix == "lsb" || prefix == "msb") && suffix == "first" {
		return prefix + "-first", true
	}
	if (prefix == "little" || prefix == "big") && suffix == "endian" {
		return prefix + "-endian", true
	}
	return "", false
}

func (p *Parser) parseRegisterFields() []*ast.RegisterField {
	fields := []*ast.RegisterField{}
	if p.peekToken.Type == lexer.RBRACE {
		return fields
	}

	for p.peekToken.Type != lexer.RBRACE && p.peekToken.Type != lexer.EOF {
		p.nextToken()
		if p.curToken.Type == lexer.COMMENT {
			continue
		}
		if p.curToken.Type != lexer.IDENT && p.curToken.Type != lexer.UNDERSCORE {
			p.addError("expected register field name, got %q", p.curToken.Lexeme)
			return fields
		}

		field := &ast.RegisterField{
			Token:  p.curToken,
			Name:   &ast.Identifier{Token: p.curToken, Value: p.curToken.Lexeme},
			Access: ast.RegisterReadWrite,
		}
		if p.peekToken.Type != lexer.COLON {
			p.addError("missing ':' after register field name %q at %d:%d", field.Name.Value, field.Name.Token.Line, field.Name.Token.Column)
			if p.skipMalformedStructField() {
				continue
			}
			return fields
		}
		p.nextToken()

		if !p.expectPeek(lexer.IDENT) {
			return fields
		}
		if p.curToken.Lexeme == "bit" {
			field.Width = 1
			if p.peekToken.Type == lexer.LBRACKET {
				p.nextToken()
				widthExpression, width, ok := p.parseRegisterWidthExpression("bit field width")
				if !ok {
					return fields
				}
				field.Width = width
				field.WidthExpression = widthExpression
				if !p.expectPeek(lexer.RBRACKET) {
					return fields
				}
			}
			if p.peekToken.Type == lexer.LT {
				p.nextToken()
				field.Unit, field.UnitExpression = p.parseUnit()
			}
		} else {
			field.Type = p.parseTypeReference()
		}
		if isRegisterFieldAccessPrefix(p.peekToken) {
			field.Access = p.parseRegisterFieldAccessModifier()
			if isRegisterFieldAccessPrefix(p.peekToken) {
				p.addError("register field %s has more than one access modifier at %d:%d", field.Name.Value, p.peekToken.Line, p.peekToken.Column)
				return fields
			}
		}

		fields = append(fields, field)
		p.skipPeekComments()
		switch p.peekToken.Type {
		case lexer.COMMA:
			p.nextToken()
			if p.peekToken.Type == lexer.RBRACE {
				return fields
			}
		case lexer.RBRACE:
			return fields
		default:
			if (p.peekToken.Type == lexer.IDENT || p.peekToken.Type == lexer.UNDERSCORE) && p.peekToken.Line > p.curToken.Line {
				continue
			}
			p.addError("expected ',' or '}' after register field at %d:%d", p.peekToken.Line, p.peekToken.Column)
			for p.peekToken.Type != lexer.RBRACE && p.peekToken.Type != lexer.EOF {
				p.nextToken()
			}
			return fields
		}
	}

	return fields
}

func isRegisterFieldAccessPrefix(token lexer.Token) bool {
	if token.Type != lexer.IDENT {
		return false
	}
	return token.Lexeme == "read" || token.Lexeme == "write" || token.Lexeme == "clear"
}

// parseRegisterFieldAccessModifier parses the closed hyphenated modifier set
// owned by rules/declarations/registers.md. The AST stores one normalized fact
// so Sema, tooling, and lowering do not reconstruct it from tokens.
func (p *Parser) parseRegisterFieldAccessModifier() ast.RegisterFieldAccess {
	p.nextToken()
	prefix := p.curToken
	if !p.expectPeek(lexer.MINUS) || !p.expectPeek(lexer.IDENT) {
		return ast.RegisterFieldAccess("")
	}
	modifier := prefix.Lexeme + "-" + p.curToken.Lexeme
	if p.peekToken.Type == lexer.MINUS && (modifier == "write-one" || modifier == "write-zero" || modifier == "clear-on") {
		p.nextToken()
		if !p.expectPeek(lexer.IDENT) {
			return ast.RegisterFieldAccess("")
		}
		modifier += "-" + p.curToken.Lexeme
	}
	switch ast.RegisterFieldAccess(modifier) {
	case ast.RegisterReadWrite,
		ast.RegisterReadOnly,
		ast.RegisterWriteOnly,
		ast.RegisterWriteOneClear,
		ast.RegisterWriteZeroClear,
		ast.RegisterClearOnRead:
		return ast.RegisterFieldAccess(modifier)
	default:
		p.addError("unknown register field access modifier %q at %d:%d", modifier, prefix.Line, prefix.Column)
		return ast.RegisterFieldAccess("")
	}
}

func (p *Parser) parseRegisterWidth(kind string) (int64, bool) {
	if p.peekToken.Type == lexer.MINUS {
		p.nextToken()
		minus := p.curToken
		if !p.expectPeek(lexer.INT) {
			return 0, false
		}
		width, ok := ast.ParseIntegerLiteralInt64("-" + p.curToken.Lexeme)
		if !ok {
			p.addError("invalid %s %q at %d:%d", kind, "-"+p.curToken.Lexeme, minus.Line, minus.Column)
			return 0, false
		}
		return width, true
	}

	if !p.expectPeek(lexer.INT) {
		return 0, false
	}
	width, ok := ast.ParseIntegerLiteralInt64(p.curToken.Lexeme)
	if !ok {
		p.addError("invalid %s %q at %d:%d", kind, p.curToken.Lexeme, p.curToken.Line, p.curToken.Column)
		return 0, false
	}
	return width, true
}

// parseRegisterWidthExpression preserves the complete constant expression
// accepted by rules/declarations/registers.md. Positivity and constantness are
// semantic properties and are deliberately checked by Sema, not reconstructed
// in the parser. The cached int64 value keeps literal AST consumers stable.
func (p *Parser) parseRegisterWidthExpression(kind string) (ast.Expression, int64, bool) {
	if p.peekToken.Type == lexer.RBRACKET || p.peekToken.Type == lexer.EOF {
		p.addError("missing %s at %d:%d", kind, p.peekToken.Line, p.peekToken.Column)
		return nil, 0, false
	}
	p.nextToken()
	expression := p.parseExpression(LOWEST)
	if expression == nil {
		return nil, 0, false
	}
	width := int64(0)
	if value, ok := parserRegisterLiteralWidth(expression); ok {
		width = value
	}
	return expression, width, true
}

// parserRegisterLiteralWidth only supplies the compatibility cache above. It does
// not decide whether an expression is a legal compile-time constant; that
// decision remains in Sema where named constants are available.
func parserRegisterLiteralWidth(expression ast.Expression) (int64, bool) {
	switch expression := expression.(type) {
	case *ast.IntegerLiteral:
		return ast.ParseIntegerLiteralInt64(expression.Token.Lexeme)
	case *ast.PrefixExpression:
		if expression.Operator != "+" && expression.Operator != "-" {
			return 0, false
		}
		value, ok := parserRegisterLiteralWidth(expression.Right)
		if !ok {
			return 0, false
		}
		if expression.Operator == "-" {
			value = -value
		}
		return value, true
	}
	return 0, false
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
		if p.peekToken.Type == lexer.DEFAULT {
			p.nextToken()
			variant.Default = true
			variant.DefaultToken = p.curToken
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
		if p.isContractStart(p.peekToken) {
			field.Contract = p.parseContractSequence()
		}
		if p.peekToken.Type == lexer.RAW_STRING {
			p.nextToken()
			tags, ok := p.parseStructTag(p.curToken)
			if !ok {
				// rules/declarations/struct.md, Field-tag recovery;
				// correction16.md keeps the valid field and synchronizes at its
				// comma instead of discarding every later struct field.
				fields = append(fields, field)
				if p.skipMalformedStructField() {
					continue
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
	start, end, skipped := p.curToken, p.curToken, 1
	delimiters := newDelimiterStack()
	for p.peekToken.Type != lexer.EOF {
		if delimiters.empty() && (p.peekToken.Type == lexer.COMMA || p.peekToken.Type == lexer.RBRACE) {
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
	continued := p.peekToken.Type == lexer.COMMA
	if continued {
		p.nextToken()
		end = p.curToken
		skipped++
	}
	p.recordSkippedRecovery(start, end, skipped, RecoveryProbable)
	return continued
}

// parseStructTag implements the open-key field-tag grammar from
// rules/declarations/struct.md. correction16.md requires separators to be
// recognized only outside quoted values and malformed quote structure to fail.
func (p *Parser) parseStructTag(token lexer.Token) ([]ast.StructTag, bool) {
	if len(token.Lexeme) < 2 || token.Lexeme[0] != '`' || token.Lexeme[len(token.Lexeme)-1] != '`' {
		p.addError("invalid struct field tag")
		return nil, false
	}
	raw := []rune(token.Lexeme[1 : len(token.Lexeme)-1])
	if len(raw) == 0 {
		return nil, true
	}

	tags := []ast.StructTag{}
	for offset := 0; ; {
		for offset < len(raw) && unicode.IsSpace(raw[offset]) {
			offset++
		}
		if offset == len(raw) {
			return tags, true
		}

		keyStart := offset
		for offset < len(raw) && raw[offset] != ':' && !unicode.IsSpace(raw[offset]) && raw[offset] != '"' {
			offset++
		}
		if offset == keyStart || offset >= len(raw) || raw[offset] != ':' {
			p.addError("invalid struct field tag")
			return nil, false
		}
		key := string(raw[keyStart:offset])
		offset++
		if offset >= len(raw) || raw[offset] != '"' {
			p.addError("invalid struct field tag")
			return nil, false
		}
		offset++

		var value strings.Builder
		closed := false
		for offset < len(raw) {
			ch := raw[offset]
			if ch == '"' {
				offset++
				closed = true
				break
			}
			if ch == '\\' {
				if offset+1 >= len(raw) {
					break
				}
				// Struct tags are nested in a raw Sec literal. Preserve the
				// escape spelling while using it only to find the closing quote.
				value.WriteRune(ch)
				offset++
				value.WriteRune(raw[offset])
				offset++
				continue
			}
			value.WriteRune(ch)
			offset++
		}
		if !closed || offset < len(raw) && !unicode.IsSpace(raw[offset]) {
			p.addError("invalid struct field tag")
			return nil, false
		}

		tags = append(tags, ast.StructTag{
			Key:   key,
			Value: value.String(),
		})
	}
}

func (p *Parser) parseImplStatement() ast.Statement {
	stmt := &ast.ImplStatement{Token: p.curToken}

	if p.peekToken.Type == lexer.EXTENDS {
		p.nextToken()
		stmt.Extends = true
	}
	if !p.expectPeekTypeStart() {
		return nil
	}
	stmt.Target = p.parseTypeReference()
	if p.peekToken.Type == lexer.IMPLEMENTS {
		p.nextToken()
		stmt.Implements = p.parseImplementsList()
		if stmt.Implements == nil {
			return nil
		}
		if stmt.Extends {
			p.addError("impl extends %s cannot declare interface conformance; place implements on the primary impl at %d:%d", stmt.Target.Name, p.curToken.Line, p.curToken.Column)
		}
	}

	if p.peekToken.Type == lexer.FOR {
		p.nextToken()
		invalid := &ast.InvalidStatement{
			Token:   p.curToken,
			Message: "separate impl Interface for Type syntax is not supported; use impl Type implements Interface",
		}
		p.skipUntilBlockStart()
		p.skipCurrentBlock()
		return invalid
	}

	if !p.expectPeek(lexer.LBRACE) {
		return nil
	}
	previousContext := p.recoveryContext
	p.recoveryContext = RecoveryContextMember
	defer func() { p.recoveryContext = previousContext }()

	for p.peekToken.Type != lexer.RBRACE && p.peekToken.Type != lexer.EOF {
		p.endRecoveryEpisode()
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
		case lexer.UNIT:
			parsed := p.parseUnitDeclStatement()
			unitDecl, ok := parsed.(*ast.UnitDeclStatement)
			if !ok {
				continue
			}
			stmt.Members = append(stmt.Members, unitDecl)
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
		case lexer.IDENT:
			if p.curToken.Lexeme == "init" && p.peekToken.Type == lexer.LPAREN {
				initializer := p.parseInitDeclaration()
				if initializer != nil {
					stmt.Members = append(stmt.Members, initializer)
				}
				continue
			}
			if p.curToken.Lexeme == "event" {
				event := p.parseEventDeclaration()
				if event == nil {
					continue
				}
				stmt.Members = append(stmt.Members, event)
				continue
			}
			message := "impl block may only contain type, unit, enum, property, event, init, and fn declarations"
			if p.peekToken.Type == lexer.COLON {
				if p.isUnitMetadataName(p.curToken.Lexeme) {
					stmt.Members = append(stmt.Members, p.parseUnitMetadataDeclaration())
					continue
				}
				message = "stored fields are not allowed inside impl"
			} else if p.isAssignmentOperator(p.peekToken.Type) {
				message = "executable statements are not allowed inside impl"
			}
			start, diagnosticStart := p.curToken, len(p.diagnostics)
			recovery := p.skipInvalidImplMember()
			stmt.Members = append(stmt.Members, p.invalidMember(start, diagnosticStart, recovery, message))
		case lexer.STATIC:
			if p.peekToken.Type == lexer.FN {
				p.nextToken()
				fn := p.parseFunctionDeclaration()
				if fn == nil {
					continue
				}
				fn.Static = true
				stmt.Members = append(stmt.Members, fn)
				continue
			}
			if p.peekToken.Type == lexer.LET {
				parsed := p.parseStaticStatement()
				if let, ok := parsed.(*ast.LetStatement); ok {
					stmt.Members = append(stmt.Members, let)
				}
				continue
			}
			if p.peekToken.Type == lexer.PROPERTY {
				// rules/declarations/static.md, sections 11-12. A static
				// property is an impl member with no implicit receiver.
				p.nextToken()
				property := p.parsePropertyDeclaration()
				if property == nil {
					continue
				}
				property.Static = true
				stmt.Members = append(stmt.Members, property)
				continue
			}
			start, diagnosticStart := p.curToken, len(p.diagnostics)
			message := "static inside impl must modify fn, let, or property"
			recovery := p.skipInvalidImplMember()
			stmt.Members = append(stmt.Members, p.invalidMember(start, diagnosticStart, recovery, message))
		case lexer.FREE:
			start, diagnosticStart := p.curToken, len(p.diagnostics)
			message := "free operations are reserved for destruction but are not implemented yet"
			var recovery RecoveryEvent
			if p.peekToken.Type == lexer.LBRACE {
				p.nextToken()
				recovery = p.skipCurrentBlockRecovery(start)
			} else {
				recovery = p.skipInvalidImplMember()
			}
			stmt.Members = append(stmt.Members, p.invalidMember(start, diagnosticStart, recovery, message))
		case lexer.PROPERTY:
			property := p.parsePropertyDeclaration()
			if property == nil {
				continue
			}
			stmt.Members = append(stmt.Members, property)
		case lexer.STRUCT:
			start, diagnosticStart := p.curToken, len(p.diagnostics)
			message := "struct declarations inside impl must use type Name struct"
			recovery := p.skipInvalidImplMember()
			stmt.Members = append(stmt.Members, p.invalidMember(start, diagnosticStart, recovery, message))
		case lexer.LET:
			start, diagnosticStart := p.curToken, len(p.diagnostics)
			message := "variable declarations are not allowed inside impl"
			recovery := p.skipInvalidImplMember()
			stmt.Members = append(stmt.Members, p.invalidMember(start, diagnosticStart, recovery, message))
		default:
			message := "impl block may only contain type, unit, enum, property, event, and fn declarations"
			if p.curToken.Type == lexer.IDENT && p.peekToken.Type == lexer.COLON {
				if p.isUnitMetadataName(p.curToken.Lexeme) {
					stmt.Members = append(stmt.Members, p.parseUnitMetadataDeclaration())
					continue
				}
				message = "stored fields are not allowed inside impl"
			} else if p.curToken.Type == lexer.IDENT && p.isAssignmentOperator(p.peekToken.Type) {
				message = "executable statements are not allowed inside impl"
			}
			start, diagnosticStart := p.curToken, len(p.diagnostics)
			recovery := p.skipInvalidImplMember()
			stmt.Members = append(stmt.Members, p.invalidMember(start, diagnosticStart, recovery, message))
		}
	}

	if !p.expectPeek(lexer.RBRACE) {
		return stmt
	}

	return stmt
}

func (p *Parser) parseInitDeclaration() *ast.InitDeclaration {
	initializer := &ast.InitDeclaration{Token: p.curToken}
	// curToken is contextual identifier `init`, peekToken is `(`.
	p.nextToken()
	initializer.Parameters = p.parseParameters()
	if initializer.Parameters == nil {
		return nil
	}
	if isTypeStart(p.peekToken.Type) {
		p.nextToken()
		initializer.ErrorType = p.parseTypeReference()
	}
	initializer.Body = p.parseFunctionBlockStatement()
	if initializer.Body == nil {
		return nil
	}
	return initializer
}

func (p *Parser) isUnitMetadataName(name string) bool {
	switch normalizeUnitMetadataName(name) {
	case "dimension", "scale", "system", "longname", "symbol", "baseunit", "status", "kind", "transform", "offset", "origin", "logbase", "logfactor", "reference":
		return true
	default:
		return false
	}
}

func normalizeUnitMetadataName(name string) string {
	return strings.ReplaceAll(strings.ToLower(name), "_", "")
}

func (p *Parser) parseUnitMetadataDeclaration() *ast.UnitMetadataDeclaration {
	metadata := &ast.UnitMetadataDeclaration{
		Token: p.curToken,
		Name:  p.curToken.Lexeme,
	}
	startLine := p.curToken.Line
	p.nextToken()
	for p.peekToken.Type != lexer.EOF {
		if p.peekToken.Type == lexer.RBRACE {
			return metadata
		}
		if p.peekToken.Line > startLine {
			return metadata
		}
		p.nextToken()
		metadata.Value = append(metadata.Value, p.curToken)
	}
	return metadata
}

func (p *Parser) skipInvalidImplMember() RecoveryEvent {
	return p.skipInvalidImplMemberFrom(p.curToken)
}

func (p *Parser) skipInvalidImplMemberFrom(start lexer.Token) RecoveryEvent {
	end := p.curToken
	skipped := 1
	if start != p.curToken {
		skipped++
	}
	startLine := start.Line
	delimiters := newDelimiterStack()
	delimiters.consume(p.curToken.Type)

	for p.peekToken.Type != lexer.EOF {
		if delimiters.empty() && p.peekToken.Type == lexer.RBRACE {
			break
		}
		if delimiters.empty() && p.peekToken.Line > startLine && p.isImplMemberStart(p.peekToken.Type) {
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

func (p *Parser) skipCurrentBlockRecovery(start lexer.Token) RecoveryEvent {
	if p.curToken.Type != lexer.LBRACE {
		return RecoveryEvent{}
	}
	return p.skipBalancedBlockFrom(start, false)
}

func (p *Parser) isImplMemberStart(t lexer.TokenType) bool {
	switch t {
	case lexer.TYPE, lexer.UNIT, lexer.ENUM, lexer.FN, lexer.FREE, lexer.PROPERTY, lexer.STRUCT, lexer.LET, lexer.STATIC:
		return true
	default:
		return false
	}
}

func (p *Parser) parseEventDeclaration() *ast.EventDeclaration {
	event := &ast.EventDeclaration{Token: p.curToken}
	if p.peekToken.Type != lexer.IDENT {
		p.nextToken()
		p.addError("event declaration missing name at %d:%d", p.curToken.Line, p.curToken.Column)
		p.skipInvalidImplMember()
		return nil
	}
	p.nextToken()
	event.Name = &ast.Identifier{Token: p.curToken, Value: p.curToken.Lexeme}
	if p.peekToken.Type != lexer.IDENT || p.peekToken.Lexeme != "using" {
		p.nextToken()
		p.addError("event %s must specify storage with using at %d:%d", event.Name.Value, p.curToken.Line, p.curToken.Column)
		p.skipInvalidImplMember()
		return event
	}
	p.nextToken()
	if p.peekToken.Type != lexer.IDENT {
		p.nextToken()
		p.addError("event %s using missing storage field at %d:%d", event.Name.Value, p.curToken.Line, p.curToken.Column)
		p.skipInvalidImplMember()
		return event
	}
	p.nextToken()
	event.Storage = &ast.Identifier{Token: p.curToken, Value: p.curToken.Lexeme}
	if p.peekToken.Type == lexer.LBRACE {
		p.nextToken()
		p.skipCurrentBlock()
	}
	return event
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
		} else if p.peekToken.Type == lexer.LBRACE {
			p.nextToken()
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
				if p.peekToken.Type == lexer.LBRACE {
					p.nextToken()
					p.skipCurrentBlock()
				}
				p.skipPropertyRemainder()
				return nil
			}
			if !p.expectPeek(lexer.LBRACE) {
				p.skipPropertyRemainder()
				return nil
			}
			property.Getter = p.parseStatementBlock("property getter")
			if property.Getter == nil {
				p.skipPropertyRemainder()
				return nil
			}
		case lexer.SET, lexer.IDENT:
			if p.curToken.Lexeme != "set" {
				p.addError("unexpected token %q in property %s at %d:%d", p.curToken.Lexeme, property.Name.Value, p.curToken.Line, p.curToken.Column)
				p.skipPropertyRemainder()
				return nil
			}
			if property.Setter != nil {
				p.addError("duplicate set in property %q", property.Name.Value)
				if p.peekToken.Type == lexer.IDENT {
					p.nextToken()
				}
				if p.peekToken.Type == lexer.LBRACE {
					p.nextToken()
					p.skipCurrentBlock()
				}
				p.skipPropertyRemainder()
				return nil
			}
			property.Setter = p.parsePropertySetter(property.Name.Value, false)
			if property.Setter == nil {
				p.skipPropertyRemainder()
				return nil
			}
		case lexer.TRY:
			if !p.expectPeekContextualSet() {
				p.skipPropertyRemainder()
				return nil
			}
			if property.Setter != nil {
				p.addError("duplicate set in property %q", property.Name.Value)
				if p.peekToken.Type == lexer.IDENT {
					p.nextToken()
				}
				if p.peekToken.Type == lexer.LBRACE {
					p.nextToken()
					p.skipCurrentBlock()
				}
				p.skipPropertyRemainder()
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
			p.skipPropertyRemainder()
			return nil
		}
	}

	if property.Getter == nil && property.Setter == nil {
		p.addError("property %q must have get or set", property.Name.Value)
		if p.peekToken.Type == lexer.RBRACE {
			p.nextToken()
		}
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

	if !p.expectPeek(lexer.LBRACE) {
		return nil
	}
	setter.Body = p.parseStatementBlock("property setter")
	if setter.Body == nil {
		return nil
	}

	return setter
}

func (p *Parser) skipCurrentBlock() RecoveryEvent {
	if p.curToken.Type != lexer.LBRACE {
		return RecoveryEvent{}
	}
	return p.skipBalancedBlock(false)
}

func (p *Parser) skipBalancedBlock(reportUnterminated bool) RecoveryEvent {
	return p.skipBalancedBlockFrom(p.curToken, reportUnterminated)
}

func (p *Parser) skipBalancedBlockFrom(start lexer.Token, reportUnterminated bool) RecoveryEvent {
	end, skipped := p.curToken, 1
	if start != p.curToken {
		skipped++
	}
	delimiters := newDelimiterStack(lexer.RBRACE)
	for !delimiters.empty() {
		p.nextToken()
		if p.curToken.Type == lexer.EOF {
			if reportUnterminated {
				p.addError("unterminated block")
			}
			break
		}
		if !delimiters.canConsume(p.curToken.Type) {
			break
		}
		end = p.curToken
		skipped++
		delimiters.consume(p.curToken.Type)
	}
	return p.recordSkippedRecovery(start, end, skipped, RecoveryProbable)
}

func (p *Parser) skipBraceBlock() RecoveryEvent {
	if p.curToken.Type != lexer.LBRACE {
		return RecoveryEvent{}
	}
	return p.skipBalancedBlock(true)
}

func (p *Parser) skipPropertyRemainder() RecoveryEvent {
	start, end, skipped := p.curToken, p.curToken, 1
	delimiters := newDelimiterStack(lexer.RBRACE)
	for p.peekToken.Type != lexer.EOF && !delimiters.empty() {
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

func (p *Parser) parseTypeReference() *ast.TypeReference {
	if p.curToken.Type == lexer.REF {
		return p.parseReferenceTypeReference()
	}

	if p.curToken.Type == lexer.LT {
		token := p.curToken
		unit, unitExpression := p.parseUnit()
		return &ast.TypeReference{
			Token:          token,
			Name:           "",
			Unit:           unit,
			UnitExpression: unitExpression,
			UnitOnly:       true,
		}
	}

	if p.curToken.Type == lexer.FN {
		return p.parseFunctionTypeReference()
	}
	if p.curToken.Type == lexer.MUT || p.curToken.Type == lexer.CONSUME_ARROW {
		return p.parseCapabilityFunctionTypeReference()
	}

	if p.curToken.Type == lexer.LBRACKET {
		return p.parsePrefixSequenceTypeReference()
	}

	if p.curToken.Type == lexer.LPAREN {
		return p.parseParenthesizedTypeReference()
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

		unit, unitExpression := p.parseUnit()
		if unit == "" {
			return ref
		}

		ref.Unit = unit
		ref.UnitExpression = unitExpression
	}

	return p.parsePostfixTypeReference(ref)
}

func (p *Parser) parseParenthesizedTypeReference() *ast.TypeReference {
	token := p.curToken
	if !p.expectPeekTypeStart() {
		return p.invalidTypeReference(token, "")
	}
	ref := p.parseTypeReference()
	if !p.expectPeek(lexer.RPAREN) {
		return p.markInvalidTypeReference(ref)
	}
	return p.parsePostfixTypeReference(ref)
}

func (p *Parser) parseReferenceTypeReference() *ast.TypeReference {
	refToken := p.curToken
	mutable := false
	if p.peekToken.Type == lexer.MUT {
		p.nextToken()
		mutable = true
	}
	if !p.expectPeekTypeStart() {
		ref := p.invalidTypeReference(refToken, "ref")
		ref.Ref = true
		ref.MutableRef = mutable
		return ref
	}
	inner := p.parseTypeReference()
	inner.Ref = true
	inner.MutableRef = mutable
	inner.Token = refToken
	return inner
}

func (p *Parser) parseFunctionTypeReference() *ast.TypeReference {
	ref := &ast.TypeReference{
		Token:              p.curToken,
		Name:               "fn",
		FunctionCapability: ast.CallableShared,
	}

	if !p.expectPeek(lexer.LPAREN) {
		return p.markInvalidTypeReference(ref)
	}

	for p.peekToken.Type != lexer.RPAREN && p.peekToken.Type != lexer.EOF {
		if !p.expectPeekTypeStart() {
			return p.markInvalidTypeReference(ref)
		}
		ref.FunctionParameterTypes = append(ref.FunctionParameterTypes, p.parseTypeReference())

		if p.peekToken.Type == lexer.COMMA {
			p.nextToken()
			continue
		}
	}

	if !p.expectPeek(lexer.RPAREN) {
		return p.markInvalidTypeReference(ref)
	}

	if !p.expectPeekTypeStart() {
		return p.markInvalidTypeReference(ref)
	}
	ref.FunctionReturnType = p.parseTypeReference()

	return ref
}

// parseCapabilityFunctionTypeReference implements the callable type prefixes
// from rules/declarations/lambda-functions.md. Lambda expressions themselves
// still begin with plain fn; mut fn and -> fn are type syntax only.
func (p *Parser) parseCapabilityFunctionTypeReference() *ast.TypeReference {
	prefix := p.curToken
	capability := ast.CallableMutable
	if prefix.Type == lexer.CONSUME_ARROW {
		capability = ast.CallableConsuming
	}
	if !p.expectPeek(lexer.FN) {
		return p.invalidTypeReference(prefix, prefix.Lexeme)
	}
	ref := p.parseFunctionTypeReference()
	ref.Token = prefix
	ref.FunctionCapability = capability
	return ref
}

type typeSequenceSuffix struct {
	slice            bool
	length           int64
	lengthExpression ast.Expression
	token            lexer.Token
}

func (p *Parser) parsePostfixTypeReference(ref *ast.TypeReference) *ast.TypeReference {
	suffixes := []typeSequenceSuffix{}

	for p.peekToken.Type == lexer.LBRACKET {
		p.nextToken()
		token := p.curToken
		if isCollectionShapedTypeName(ref.Name) {
			ref = p.parseCollectionShapedTypeReferenceArgs(ref, token)
			if ref == nil {
				return p.invalidTypeReference(token, "")
			}
			continue
		}
		if ref.Name == "Event" || ref.Name == "EventStorage" {
			ref = p.parseEventTypeReferenceArgs(ref, token)
			if ref == nil {
				return p.invalidTypeReference(token, "")
			}
			continue
		}

		switch p.peekToken.Type {
		case lexer.RBRACKET:
			p.nextToken()
			suffixes = append(suffixes, typeSequenceSuffix{slice: true, token: token})
		case lexer.INT:
			p.nextToken()
			_, suffix := ast.SplitNumericLiteralSuffix(p.curToken.Lexeme)
			if suffix == "t" || suffix == "r" {
				bigValue, _ := ast.ParseIntegerLiteralLexeme(p.curToken.Lexeme)
				lengthExpr := &ast.IntegerLiteral{Token: p.curToken, BigValue: bigValue}
				if !p.expectPeek(lexer.RBRACKET) {
					return ref
				}
				suffixes = append(suffixes, typeSequenceSuffix{lengthExpression: lengthExpr, token: token})
				continue
			}
			length, ok := ast.ParseIntegerLiteralInt64(p.curToken.Lexeme)
			if !ok {
				bigValue, _ := ast.ParseIntegerLiteralLexeme(p.curToken.Lexeme)
				lengthExpr := &ast.IntegerLiteral{Token: p.curToken, BigValue: bigValue}
				if !p.expectPeek(lexer.RBRACKET) {
					return ref
				}
				suffixes = append(suffixes, typeSequenceSuffix{lengthExpression: lengthExpr, token: token})
				continue
			}
			if !p.expectPeek(lexer.RBRACKET) {
				return ref
			}
			suffixes = append(suffixes, typeSequenceSuffix{length: length, token: token})
		case lexer.MINUS, lexer.FLOAT, lexer.TRUE, lexer.FALSE:
			p.nextToken()
			lengthExpr := p.parseExpression(LOWEST)
			if lengthExpr == nil {
				return ref
			}
			if !p.expectPeek(lexer.RBRACKET) {
				return ref
			}
			suffixes = append(suffixes, typeSequenceSuffix{lengthExpression: lengthExpr, token: token})
		default:
			ref.TypeArgs = p.parseTypeArgs()
		}
	}

	for i := len(suffixes) - 1; i >= 0; i-- {
		suffix := suffixes[i]
		ref = &ast.TypeReference{
			Token:                 suffix.token,
			ElementType:           ref,
			Slice:                 suffix.slice,
			ArrayLength:           suffix.length,
			ArrayLengthExpression: suffix.lengthExpression,
		}
	}

	return ref
}

func (p *Parser) parseCollectionShapedTypeReferenceArgs(ref *ast.TypeReference, token lexer.Token) *ast.TypeReference {
	typeCount := collectionShapedTypeArgumentCount(ref.Name)
	for i := 0; i < typeCount; i++ {
		if !p.expectPeekTypeStart() {
			return ref
		}
		ref.TypeArgs = append(ref.TypeArgs, p.parseTypeReference())
		if i < typeCount-1 {
			if !p.expectPeek(lexer.COMMA) {
				return ref
			}
			continue
		}
		if p.peekToken.Type == lexer.COMMA {
			p.nextToken()
		}
	}

	for p.peekToken.Type != lexer.RBRACKET && p.peekToken.Type != lexer.EOF {
		p.nextToken()
		arg := p.parseExpression(LOWEST)
		if arg == nil {
			return ref
		}
		ref.ConstArgs = append(ref.ConstArgs, arg)
		if p.peekToken.Type == lexer.COMMA {
			p.nextToken()
			continue
		}
		break
	}

	if !p.expectPeek(lexer.RBRACKET) {
		return ref
	}
	_ = token
	return ref
}

func isCollectionShapedTypeName(name string) bool {
	switch name {
	case "list", "map", "set", "vector", "matrix", "tensor", "tensor_view", "Shape", "Strides", "TensorLayout":
		return true
	default:
		return false
	}
}

func collectionShapedTypeArgumentCount(name string) int {
	switch name {
	case "Shape", "Strides", "TensorLayout":
		return 0
	case "map":
		return 2
	default:
		return 1
	}
}

func (p *Parser) parseEventTypeReferenceArgs(ref *ast.TypeReference, token lexer.Token) *ast.TypeReference {
	if !p.expectPeekTypeStart() {
		return ref
	}
	ref.TypeArgs = []*ast.TypeReference{p.parseTypeReference()}
	if p.peekToken.Type == lexer.COMMA {
		p.nextToken()
		if p.peekToken.Type != lexer.INT {
			p.nextToken()
			p.addError("%s capacity must be an integer literal at %d:%d", ref.Name, p.curToken.Line, p.curToken.Column)
			return ref
		}
		p.nextToken()
		capacity, ok := ast.ParseIntegerLiteralInt64(p.curToken.Lexeme)
		if !ok {
			p.addError("invalid %s capacity %q at %d:%d", ref.Name, p.curToken.Lexeme, p.curToken.Line, p.curToken.Column)
			return ref
		}
		ref.EventCapacity = capacity
		ref.EventCapacitySet = true
	}
	if !p.expectPeek(lexer.RBRACKET) {
		return ref
	}
	_ = token
	return ref
}

func (p *Parser) parsePrefixSequenceTypeReference() *ast.TypeReference {
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
	} else {
		ref.Slice = true
	}

	if !p.expectPeek(lexer.RBRACKET) {
		return p.markInvalidTypeReference(ref)
	}
	if !p.expectPeekTypeStart() {
		return p.markInvalidTypeReference(ref)
	}

	ref.ElementType = p.parseTypeReference()

	return ref
}

func (p *Parser) parseUnit() (string, *ast.UnitExpression) {
	if p.curToken.Type != lexer.LT {
		p.addError("expected unit to start with '<', got %q", p.curToken.Lexeme)
		return "", nil
	}

	p.nextToken()

	unit := ""
	tokens := []lexer.Token{}

	for p.curToken.Type != lexer.GT && p.curToken.Type != lexer.EOF {
		unit += p.curToken.Lexeme
		tokens = append(tokens, p.curToken)
		p.nextToken()
	}

	if p.curToken.Type != lexer.GT {
		p.addError("unterminated unit type")
		return "", nil
	}

	expression, err := parseUnitExpressionTokens(tokens)
	if err != nil {
		p.addError("invalid unit expression %s: %s", unit, err)
		return unit, nil
	}
	expression.Source = unit
	return unit, expression
}

// unitSyntaxParser implements the structural grammar in
// rules/types/units.md, "Structural unit expressions". It constructs a
// first-class AST while the outer parser keeps the exact compact source
// spelling used by formatter and diagnostics.
type unitSyntaxParser struct {
	tokens []lexer.Token
	pos    int
}

func parseUnitExpressionTokens(tokens []lexer.Token) (*ast.UnitExpression, error) {
	if len(tokens) == 0 {
		return nil, fmt.Errorf("expected a unit factor")
	}
	p := unitSyntaxParser{tokens: tokens}
	expression, err := p.parseProduct()
	if err != nil {
		return nil, err
	}
	if p.pos != len(p.tokens) {
		return nil, fmt.Errorf("unexpected %q", p.tokens[p.pos].Lexeme)
	}
	return expression, nil
}

func (p *unitSyntaxParser) parseProduct() (*ast.UnitExpression, error) {
	left, err := p.parseFactor()
	if err != nil {
		return nil, err
	}
	for p.pos < len(p.tokens) && (p.tokens[p.pos].Type == lexer.ASTERISK || p.tokens[p.pos].Type == lexer.SLASH) {
		op := p.tokens[p.pos]
		p.pos++
		right, err := p.parseFactor()
		if err != nil {
			return nil, err
		}
		kind := ast.UnitExpressionMultiply
		if op.Type == lexer.SLASH {
			kind = ast.UnitExpressionDivide
		}
		left = &ast.UnitExpression{Token: op, Kind: kind, Left: left, Right: right}
	}
	return left, nil
}

func (p *unitSyntaxParser) parseFactor() (*ast.UnitExpression, error) {
	atom, err := p.parseAtom()
	if err != nil {
		return nil, err
	}
	if p.pos >= len(p.tokens) || p.tokens[p.pos].Type != lexer.BIT_XOR {
		return atom, nil
	}
	token := p.tokens[p.pos]
	p.pos++
	sign := 1
	if p.pos < len(p.tokens) && (p.tokens[p.pos].Type == lexer.PLUS || p.tokens[p.pos].Type == lexer.MINUS) {
		if p.tokens[p.pos].Type == lexer.MINUS {
			sign = -1
		}
		p.pos++
	}
	if p.pos >= len(p.tokens) || p.tokens[p.pos].Type != lexer.INT {
		return nil, fmt.Errorf("unit exponent must be a signed integer")
	}
	exponent64, err := strconv.ParseInt(p.tokens[p.pos].Lexeme, 10, 32)
	if err != nil || exponent64 == 0 {
		return nil, fmt.Errorf("unit exponent must be a non-zero signed integer")
	}
	p.pos++
	return &ast.UnitExpression{Token: token, Kind: ast.UnitExpressionPower, Left: atom, Exponent: sign * int(exponent64)}, nil
}

func (p *unitSyntaxParser) parseAtom() (*ast.UnitExpression, error) {
	if p.pos >= len(p.tokens) {
		return nil, fmt.Errorf("expected a unit factor")
	}
	token := p.tokens[p.pos]
	if token.Type == lexer.LPAREN {
		p.pos++
		expression, err := p.parseProduct()
		if err != nil {
			return nil, err
		}
		if p.pos >= len(p.tokens) || p.tokens[p.pos].Type != lexer.RPAREN {
			return nil, fmt.Errorf("expected ')' in unit expression")
		}
		p.pos++
		return &ast.UnitExpression{Token: token, Kind: ast.UnitExpressionGroup, Left: expression}, nil
	}
	if token.Type == lexer.INT && token.Lexeme == "1" {
		p.pos++
		return &ast.UnitExpression{Token: token, Kind: ast.UnitExpressionIdentity}, nil
	}
	if token.Type != lexer.IDENT {
		return nil, fmt.Errorf("expected a unit name, 1, or parenthesized unit expression")
	}
	name := token.Lexeme
	p.pos++
	for p.pos+1 < len(p.tokens) && p.tokens[p.pos].Type == lexer.DOT && p.tokens[p.pos+1].Type == lexer.IDENT {
		name += "." + p.tokens[p.pos+1].Lexeme
		p.pos += 2
	}
	return &ast.UnitExpression{Token: token, Kind: ast.UnitExpressionName, Name: name}, nil
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

func (p *Parser) parseContractSequence() ast.Contract {
	contracts := []ast.Contract{}
	var firstToken lexer.Token
	for p.isContractStart(p.peekToken) {
		p.nextToken()
		if len(contracts) == 0 {
			firstToken = p.curToken
		}
		contract := p.parseCurrentContract()
		if contract == nil {
			return nil
		}
		contracts = append(contracts, contract)
	}
	if len(contracts) == 0 {
		return nil
	}
	if len(contracts) == 1 {
		return contracts[0]
	}
	return &ast.ContractList{Token: firstToken, Contracts: contracts}
}

func (p *Parser) parseCurrentContract() ast.Contract {
	switch {
	case p.curToken.Type == lexer.RANGE_KW:
		return p.parseRangeContract()
	case p.curToken.Type == lexer.IN:
		return p.parseMembershipContract()
	case p.curToken.Type == lexer.IDENT:
		switch p.curToken.Lexeme {
		case "multipleOf":
			return p.parseValueContract("multipleOf")
		case "notEmpty", "unique", "finite", "odd", "even":
			return &ast.MarkerContract{Token: p.curToken, Name: p.curToken.Lexeme}
		}
	}
	p.addError("unknown contract %q at %d:%d", p.curToken.Lexeme, p.curToken.Line, p.curToken.Column)
	return nil
}

func (p *Parser) isContractStart(token lexer.Token) bool {
	if token.Type == lexer.RANGE_KW || token.Type == lexer.IN {
		return true
	}
	return token.Type == lexer.IDENT && isNamedContract(token.Lexeme)
}

func isNamedContract(name string) bool {
	switch name {
	case "multipleOf", "notEmpty", "unique", "finite", "odd", "even":
		return true
	default:
		return false
	}
}

func (p *Parser) parseMembershipContract() ast.Contract {
	contract := &ast.MembershipContract{Token: p.curToken}
	if !p.expectPeek(lexer.LBRACKET) {
		return nil
	}
	for p.peekToken.Type != lexer.RBRACKET && p.peekToken.Type != lexer.EOF {
		if !p.expectPeekExpressionStart() {
			return nil
		}
		value := p.parseExpression(LOWEST)
		if value == nil {
			return nil
		}
		contract.Values = append(contract.Values, value)
		if p.peekToken.Type == lexer.COMMA {
			p.nextToken()
			continue
		}
	}
	if len(contract.Values) == 0 {
		p.addError("membership contract requires at least one value at %d:%d", contract.Token.Line, contract.Token.Column)
		return nil
	}
	if !p.expectPeek(lexer.RBRACKET) {
		return nil
	}
	return contract
}

func (p *Parser) parseValueContract(name string) ast.Contract {
	contract := &ast.MarkerContract{Token: p.curToken, Name: name}
	if !p.expectPeekExpressionStart() {
		return nil
	}
	contract.Value = p.parseExpression(LOWEST)
	if contract.Value == nil {
		return nil
	}
	return contract
}

func (p *Parser) expectPeekExpressionStart() bool {
	if p.isExpressionStart(p.peekToken.Type) {
		p.nextToken()
		return true
	}
	unexpected := p.peekToken
	p.addDiagnostic(
		compilerdiagnostics.ParserInvalidExpression,
		unexpected,
		nil,
		&unexpected,
		"expected next token to be expression, got %q at %d:%d",
		unexpected.Type,
		unexpected.Line,
		unexpected.Column,
	)
	return false
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

	case lexer.PLUS, lexer.MINUS:
		p.nextToken()
		operatorToken := p.curToken

		if !p.expectPeekNumber() {
			return nil
		}

		return &ast.PrefixExpression{
			Token:    operatorToken,
			Operator: operatorToken.Lexeme,
			Right:    p.parseNumberLiteral(),
		}

	default:
		return nil
	}
}

func (p *Parser) isRangeBoundStart(t lexer.TokenType) bool {
	return t == lexer.INT || t == lexer.FLOAT || t == lexer.PLUS || t == lexer.MINUS
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

		if !isPathSegmentToken(p.peekToken.Type) {
			p.addError("expected next token to be %q, got %q at %d:%d", lexer.IDENT, p.peekToken.Type, p.peekToken.Line, p.peekToken.Column)
			return path
		}
		p.nextToken()

		path += "." + p.curToken.Lexeme
	}

	return path
}

func isPathSegmentToken(tokenType lexer.TokenType) bool {
	switch tokenType {
	case lexer.IDENT,
		lexer.IMPL,
		lexer.INTERFACE,
		lexer.IMPLEMENTS,
		lexer.TYPE,
		lexer.UNIT,
		lexer.STRUCT,
		lexer.ENUM,
		lexer.UNION,
		lexer.FN,
		lexer.LET,
		lexer.FOR,
		lexer.WHILE,
		lexer.MATCH,
		lexer.SWITCH:
		return true
	default:
		return false
	}
}

func (p *Parser) expectPeek(t lexer.TokenType) bool {
	if p.peekToken.Type == t {
		p.nextToken()
		if isClosingDelimiter(t) {
			p.endRecoveryEpisode()
		}
		return true
	}

	unexpected := p.peekToken
	p.addDiagnostic(
		compilerdiagnostics.ParserMissingToken,
		unexpected,
		[]lexer.TokenType{t},
		&unexpected,
		"expected next token to be %q, got %q at %d:%d",
		t,
		unexpected.Type,
		unexpected.Line,
		unexpected.Column,
	)

	return false
}

func (p *Parser) expectPeekNumber() bool {
	if p.peekToken.Type == lexer.INT || p.peekToken.Type == lexer.FLOAT {
		p.nextToken()
		return true
	}

	unexpected := p.peekToken
	p.addDiagnostic(
		compilerdiagnostics.ParserMissingToken,
		unexpected,
		[]lexer.TokenType{lexer.INT, lexer.FLOAT},
		&unexpected,
		"expected next token to be number, got %q at %d:%d",
		unexpected.Type,
		unexpected.Line,
		unexpected.Column,
	)

	return false
}

func (p *Parser) expectPeekTypeStart() bool {
	if isTypeStart(p.peekToken.Type) {
		p.nextToken()
		return true
	}

	unexpected := p.peekToken
	p.addDiagnostic(
		compilerdiagnostics.ParserInvalidTypeReference,
		unexpected,
		nil,
		&unexpected,
		"expected next token to be type, got %q at %d:%d",
		unexpected.Type,
		unexpected.Line,
		unexpected.Column,
	)

	return false
}

func (p *Parser) expectPeekContextualSet() bool {
	if (p.peekToken.Type == lexer.IDENT || p.peekToken.Type == lexer.SET) && p.peekToken.Lexeme == "set" {
		p.nextToken()
		return true
	}
	p.addError("expected 'set', got %q at %d:%d", p.peekToken.Lexeme, p.peekToken.Line, p.peekToken.Column)
	return false
}

func isTypeStart(tokenType lexer.TokenType) bool {
	return tokenType == lexer.IDENT || tokenType == lexer.SET || tokenType == lexer.LT || tokenType == lexer.LBRACKET || tokenType == lexer.LPAREN || tokenType == lexer.VOID || tokenType == lexer.FN || tokenType == lexer.MUT || tokenType == lexer.CONSUME_ARROW || tokenType == lexer.REF
}

func (p *Parser) isTypeNameToken(tokenType lexer.TokenType) bool {
	return tokenType == lexer.IDENT || tokenType == lexer.SET
}

func (p *Parser) nextToken() {
	p.curToken = p.peekToken
	p.peekToken = p.l.NextToken()
	p.collectLexerDiagnostics()
}

func (p *Parser) collectLexerDiagnostics() {
	diagnostics := p.l.Diagnostics()
	for p.lexerDiagnostics < len(diagnostics) {
		diagnostic := diagnostics[p.lexerDiagnostics]
		p.lexerDiagnostics++
		p.addDiagnostic(diagnostic.ID, diagnostic.Primary, nil, nil, "%s", diagnostic.Message)
	}
}

func (p *Parser) addError(format string, args ...any) {
	p.addDiagnostic(compilerdiagnostics.ParserSyntaxError, p.curToken, nil, nil, format, args...)
}

func (p *Parser) addDiagnostic(id string, primary lexer.Token, expected []lexer.TokenType, unexpected *lexer.Token, format string, args ...any) {
	if p.limitReached {
		return
	}
	key := diagnosticKey{id: id, file: primary.File, line: primary.Line, column: primary.Column, context: p.recoveryContext}
	if _, duplicate := p.diagnosticKeys[key]; duplicate {
		return
	}
	location := diagnosticLocation{file: primary.File, line: primary.Line, column: primary.Column, context: p.recoveryContext}
	if p.activeEpisode != 0 && p.hasEpisodePrimary && p.episodePrimary == location {
		return
	}
	if len(p.diagnostics) == maxParserDiagnostics-1 {
		id = compilerdiagnostics.ParserRecoveryLimit
		expected = nil
		unexpected = nil
		format = "parser diagnostic limit reached after %d errors at %d:%d"
		args = []any{maxParserDiagnostics, primary.Line, primary.Column}
		p.limitReached = true
		key.id = id
	}
	episode := p.ensureRecoveryEpisode()
	if !p.hasEpisodePrimary {
		p.episodePrimary = location
		p.hasEpisodePrimary = true
	}
	message := fmt.Sprintf(format, args...)
	diagnosticIndex := len(p.diagnostics)
	p.errors = append(p.errors, message)
	p.diagnostics = append(p.diagnostics, Diagnostic{
		ID:         id,
		Message:    message,
		Primary:    primary,
		Expected:   append([]lexer.TokenType(nil), expected...),
		Unexpected: unexpected,
		Context:    p.recoveryContext,
		Episode:    episode,
	})
	p.diagnosticKeys[key] = struct{}{}
	if id == compilerdiagnostics.ParserMissingToken && len(expected) > 0 {
		p.recovery = append(p.recovery, RecoveryEvent{
			Kind:            RecoveryInsertMissingToken,
			Confidence:      RecoveryUnambiguous,
			DiagnosticID:    id,
			diagnosticIndex: diagnosticIndex,
			Start:           primary,
			End:             primary,
			Expected:        append([]lexer.TokenType(nil), expected...),
			Context:         p.recoveryContext,
			Episode:         episode,
		})
	}
}

func (p *Parser) ensureRecoveryEpisode() int {
	if p.activeEpisode == 0 {
		p.nextEpisode++
		p.activeEpisode = p.nextEpisode
	}
	return p.activeEpisode
}

func (p *Parser) endRecoveryEpisode() {
	p.activeEpisode = 0
	p.episodePrimary = diagnosticLocation{}
	p.hasEpisodePrimary = false
}

func isClosingDelimiter(t lexer.TokenType) bool {
	return t == lexer.RPAREN || t == lexer.RBRACKET || t == lexer.RBRACE
}

func (p *Parser) rollbackErrors(count int) {
	p.errors = p.errors[:count]
	p.diagnostics = p.diagnostics[:count]
	if count < maxParserDiagnostics {
		p.limitReached = false
	}
	out := p.recovery[:0]
	for _, event := range p.recovery {
		if event.diagnosticIndex >= 0 && event.diagnosticIndex >= count {
			continue
		}
		out = append(out, event)
	}
	p.recovery = out
	p.diagnosticKeys = make(map[diagnosticKey]struct{}, len(p.diagnostics))
	p.nextEpisode = 0
	for _, diagnostic := range p.diagnostics {
		p.diagnosticKeys[diagnosticKey{
			id: diagnostic.ID, file: diagnostic.Primary.File,
			line: diagnostic.Primary.Line, column: diagnostic.Primary.Column,
			context: diagnostic.Context,
		}] = struct{}{}
		if diagnostic.Episode > p.nextEpisode {
			p.nextEpisode = diagnostic.Episode
		}
	}
	p.endRecoveryEpisode()
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

func trimCharQuotes(s string) string {
	if unquoted, err := strconv.Unquote(s); err == nil {
		return unquoted
	}

	return s
}

func (p *Parser) skipStatement() RecoveryEvent {
	start := p.curToken
	end := start
	skipped := 1
	delimiters := newDelimiterStack()
	delimiters.consume(p.curToken.Type)

	for p.peekToken.Type != lexer.EOF {
		if p.recoveryContext == RecoveryContextTopLevel && p.peekToken.Line > start.Line && isDeclarationStart(p.peekToken.Type) {
			p.nextToken()
			break
		}
		if delimiters.empty() && p.peekToken.Type == lexer.RBRACE {
			p.nextToken()
			break
		}
		if delimiters.empty() && p.peekToken.Line > start.Line && p.isStatementStart(p.peekToken.Type) {
			p.nextToken()
			break
		}
		if !delimiters.canConsume(p.peekToken.Type) {
			p.nextToken()
			break
		}
		p.nextToken()
		end = p.curToken
		skipped++
		delimiters.consume(p.curToken.Type)
	}
	if p.peekToken.Type == lexer.EOF && p.curToken.Type != lexer.EOF {
		p.nextToken()
	}
	return p.recordSkippedRecovery(start, end, skipped, RecoveryProbable)
}

func (p *Parser) invalidStatement(start lexer.Token, diagnosticStart int, recovery RecoveryEvent) *ast.InvalidStatement {
	message := "invalid statement"
	diagnosticID := compilerdiagnostics.ParserInvalidStatement
	if diagnosticStart < len(p.diagnostics) {
		diagnostic := p.diagnostics[diagnosticStart]
		message = diagnostic.Message
		diagnosticID = diagnostic.ID
	}
	return &ast.InvalidStatement{
		Token:   start,
		Message: message,
		Recovery: &ast.RecoveryInfo{
			DiagnosticID: diagnosticID,
			Message:      message,
			Start:        recovery.Start,
			End:          recovery.End,
			Skipped:      recovery.Skipped,
		},
	}
}

func (p *Parser) invalidDeclaration(start lexer.Token, diagnosticStart int, recovery RecoveryEvent) *ast.InvalidDeclaration {
	message := "invalid declaration"
	if diagnosticStart < len(p.diagnostics) {
		message = p.diagnostics[diagnosticStart].Message
	}
	return &ast.InvalidDeclaration{
		Token:   start,
		Message: message,
		Recovery: &ast.RecoveryInfo{
			DiagnosticID: compilerdiagnostics.ParserInvalidDeclaration,
			Message:      message,
			Start:        recovery.Start,
			End:          recovery.End,
			Skipped:      recovery.Skipped,
		},
	}
}

func (p *Parser) invalidMember(start lexer.Token, diagnosticStart int, recovery RecoveryEvent, fallback string) *ast.InvalidMember {
	message := fallback
	if diagnosticStart < len(p.diagnostics) {
		message = p.diagnostics[diagnosticStart].Message
	}
	return &ast.InvalidMember{
		Token:   start,
		Message: message,
		Recovery: &ast.RecoveryInfo{
			DiagnosticID: compilerdiagnostics.ParserInvalidBlockMember,
			Message:      message,
			Start:        recovery.Start,
			End:          recovery.End,
			Skipped:      recovery.Skipped,
		},
	}
}

func (p *Parser) invalidPattern(start lexer.Token, diagnosticStart int, recovery RecoveryEvent) *ast.InvalidPattern {
	message := "invalid pattern"
	if diagnosticStart < len(p.diagnostics) {
		message = p.diagnostics[diagnosticStart].Message
	}
	return &ast.InvalidPattern{
		Token:   start,
		Message: message,
		Recovery: &ast.RecoveryInfo{
			DiagnosticID: compilerdiagnostics.ParserInvalidPattern,
			Message:      message,
			Start:        recovery.Start,
			End:          recovery.End,
			Skipped:      recovery.Skipped,
		},
	}
}

func (p *Parser) invalidMatchPattern(start lexer.Token, diagnosticStart int, recovery RecoveryEvent) *ast.MatchPattern {
	message := "invalid match pattern"
	if diagnosticStart < len(p.diagnostics) {
		message = p.diagnostics[diagnosticStart].Message
	}
	return &ast.MatchPattern{
		Token: start, Kind: ast.MatchPatternInvalid, Invalid: true,
		Recovery: &ast.RecoveryInfo{
			DiagnosticID: compilerdiagnostics.ParserInvalidPattern,
			Message:      message, Start: recovery.Start, End: recovery.End, Skipped: recovery.Skipped,
		},
	}
}

func (p *Parser) recordSkippedRecovery(start, end lexer.Token, skipped int, confidence RecoveryConfidence) RecoveryEvent {
	event := RecoveryEvent{
		Kind:            RecoverySkipTokens,
		Confidence:      confidence,
		Start:           start,
		End:             end,
		Skipped:         skipped,
		Context:         p.recoveryContext,
		Episode:         p.activeEpisode,
		diagnosticIndex: -1,
	}
	if p.activeEpisode != 0 && len(p.diagnostics) > 0 {
		diagnosticIndex := len(p.diagnostics) - 1
		diagnostic := p.diagnostics[diagnosticIndex]
		if diagnostic.Episode == p.activeEpisode {
			event.DiagnosticID = diagnostic.ID
			event.diagnosticIndex = diagnosticIndex
		}
	}
	p.recovery = append(p.recovery, event)
	p.endRecoveryEpisode()
	return event
}

func (p *Parser) invalidExpression(token lexer.Token, message string, diagnosticID string) *ast.InvalidExpression {
	return &ast.InvalidExpression{
		Token:   token,
		Message: message,
		Recovery: &ast.RecoveryInfo{
			DiagnosticID: diagnosticID,
			Message:      message,
			Start:        token,
			End:          token,
		},
	}
}

func (p *Parser) invalidTypeReference(token lexer.Token, name string) *ast.TypeReference {
	message := "invalid type reference"
	diagnosticID := compilerdiagnostics.ParserInvalidTypeReference
	if len(p.diagnostics) > 0 {
		diagnostic := p.diagnostics[len(p.diagnostics)-1]
		message = diagnostic.Message
		diagnosticID = diagnostic.ID
	}
	return &ast.TypeReference{
		Token:   token,
		Name:    name,
		Invalid: true,
		Recovery: &ast.RecoveryInfo{
			DiagnosticID: diagnosticID,
			Message:      message,
			Start:        token,
			End:          token,
		},
	}
}

func (p *Parser) markInvalidTypeReference(ref *ast.TypeReference) *ast.TypeReference {
	if ref == nil {
		return p.invalidTypeReference(p.curToken, "")
	}
	invalid := p.invalidTypeReference(ref.Token, ref.Name)
	ref.Invalid = true
	ref.Recovery = invalid.Recovery
	return ref
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
		lexer.HASH,
		lexer.IMPORT,
		lexer.TYPE,
		lexer.UNIT,
		lexer.ENUM,
		lexer.STRUCT,
		lexer.INTERFACE,
		lexer.IMPL,
		lexer.PROPERTY,
		lexer.EXTERN,
		lexer.FN,
		lexer.FREE,
		lexer.LET,
		lexer.STATIC,
		lexer.RETURN,
		lexer.IF,
		lexer.FOR,
		lexer.WHILE,
		lexer.MATCH,
		lexer.SWITCH,
		lexer.SELECT,
		lexer.TRY,
		lexer.SPAWN,
		lexer.AWAIT,
		lexer.BREAK,
		lexer.CANCEL,
		lexer.CONTINUE,
		lexer.FALLTHROUGH,
		lexer.UNSAFE,
		lexer.ASM,
		lexer.DEFER,
		lexer.DISCARD,
		lexer.ELSE,
		lexer.AT,
		lexer.SELF,
		lexer.COMMENT:
		return true
	default:
		return false
	}
}

func isDeclarationStart(t lexer.TokenType) bool {
	switch t {
	case lexer.MODULE, lexer.IMPORT, lexer.TYPE, lexer.UNIT, lexer.ENUM,
		lexer.STRUCT, lexer.INTERFACE, lexer.IMPL, lexer.EXTERN, lexer.FN,
		lexer.PROPERTY, lexer.STATIC:
		return true
	default:
		return false
	}
}

func (p *Parser) isAtTypeDeclEnd() bool {
	return p.peekToken.Type == lexer.EOF || p.isStatementStart(p.peekToken.Type)
}

func (p *Parser) parseAssignmentStatement() ast.Statement {
	stmt := &ast.AssignmentStatement{Token: p.curToken, Ownership: ast.OwnershipCopy}

	stmt.Target = p.parseExpression(LOWEST)
	if stmt.Target == nil {
		return nil
	}

	if !p.expectPeekAssignmentOperator() {
		return nil
	}

	stmt.Operator = p.curToken.Lexeme
	if p.curToken.Type == lexer.MOVE_ASSIGN {
		stmt.Ownership = ast.OwnershipMove
	}
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
	previousStopBeforeBrace := p.stopBeforeBrace
	p.stopBeforeBrace = true
	target := p.parseExpression(LOWEST)
	p.stopBeforeBrace = previousStopBeforeBrace
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
		Token:     expressionToken(target),
		Target:    target,
		Operator:  p.curToken.Lexeme,
		Ownership: ast.OwnershipCopy,
	}
	if p.curToken.Type == lexer.MOVE_ASSIGN {
		assignment.Ownership = ast.OwnershipMove
	}
	p.nextToken()

	previousStopBeforeBrace = p.stopBeforeBrace
	p.stopBeforeBrace = true
	assignment.Value = p.parseExpression(LOWEST)
	p.stopBeforeBrace = previousStopBeforeBrace
	if assignment.Value == nil {
		return nil
	}
	stmt.Assignment = assignment
	if p.peekToken.Type != lexer.LBRACE {
		p.addError("try assignment requires a handler block at %d:%d", stmt.Token.Line, stmt.Token.Column)
		p.skipDeclarationRest()
		return nil
	}
	p.nextToken()
	stmt.Handlers = p.parseTryHandlerBlock()
	if stmt.Handlers == nil {
		return nil
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

	stmt := &ast.AssignmentStatement{Token: token, Target: expr, Ownership: ast.OwnershipCopy}
	p.nextToken()
	stmt.Operator = p.curToken.Lexeme
	if p.curToken.Type == lexer.MOVE_ASSIGN {
		stmt.Ownership = ast.OwnershipMove
	}
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

	unexpected := p.peekToken
	p.addDiagnostic(
		compilerdiagnostics.ParserMissingToken,
		unexpected,
		[]lexer.TokenType{
			lexer.ASSIGN,
			lexer.MOVE_ASSIGN,
			lexer.PLUS_ASSIGN,
			lexer.MINUS_ASSIGN,
			lexer.ASTERISK_ASSIGN,
			lexer.SLASH_ASSIGN,
			lexer.PERCENT_ASSIGN,
			lexer.BIT_AND_ASSIGN,
			lexer.BIT_OR_ASSIGN,
			lexer.BIT_XOR_ASSIGN,
			lexer.SHIFT_LEFT_ASSIGN,
			lexer.SHIFT_RIGHT_ASSIGN,
		},
		&unexpected,
		"expected next token to be assignment operator, got %q at %d:%d",
		unexpected.Type,
		unexpected.Line,
		unexpected.Column,
	)

	return false
}

func (p *Parser) isAssignmentOperator(t lexer.TokenType) bool {
	switch t {
	case lexer.ASSIGN,
		lexer.MOVE_ASSIGN,
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
//	       [ ":=" Expression | ":<-" Expression | "<-" Expression ]
func (p *Parser) parseLetStatement() ast.Statement {
	token := p.curToken
	mutable := false

	if p.peekToken.Type == lexer.MUT {
		p.nextToken()
		mutable = true
	}

	first := p.parseLetDeclarator(token, mutable, nil, nil)
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

		next := p.parseLetDeclarator(token, mutable, nil, nil)
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

func (p *Parser) parseAddressedLetStatement() ast.Statement {
	addressToken := p.curToken
	p.nextToken()
	if !p.expectPeek(lexer.LPAREN) {
		return nil
	}
	p.nextToken()
	address := p.parseExpression(LOWEST)
	if address == nil {
		return nil
	}
	if !p.expectPeek(lexer.RPAREN) {
		return nil
	}
	if !p.expectPeek(lexer.LET) {
		p.addError("@address must annotate a let declaration at %d:%d", p.peekToken.Line, p.peekToken.Column)
		return nil
	}

	stmt := p.parseLetStatement()
	if group, ok := stmt.(*ast.LetGroupStatement); ok {
		p.addError("@address cannot annotate grouped let declarations at %d:%d", group.Token.Line, group.Token.Column)
		return nil
	}
	letStmt, ok := stmt.(*ast.LetStatement)
	if !ok || letStmt == nil {
		return nil
	}
	letStmt.Address = address
	letStmt.AddressToken = addressToken
	return letStmt
}

func (p *Parser) parseNoCopyDeclaration() ast.Statement {
	attributes := []*ast.Attribute{}
	var first lexer.Token

	for {
		attributeToken := p.curToken
		if !p.expectPeek(lexer.IDENT) {
			return nil
		}
		nameToken := p.curToken
		if nameToken.Lexeme != "noCopy" {
			p.addError("unknown attribute @%s at %d:%d", nameToken.Lexeme, attributeToken.Line, attributeToken.Column)
			return nil
		}
		if len(attributes) > 0 {
			p.addError(
				"duplicate attribute @noCopy at %d:%d; first declared at %d:%d",
				attributeToken.Line,
				attributeToken.Column,
				first.Line,
				first.Column,
			)
		} else {
			first = attributeToken
		}
		attributes = append(attributes, &ast.Attribute{
			Token: attributeToken,
			Name:  &ast.Identifier{Token: nameToken, Value: nameToken.Lexeme},
		})

		if p.peekToken.Type == lexer.LPAREN {
			argumentToken := p.peekToken
			p.addError("@noCopy does not take arguments at %d:%d", argumentToken.Line, argumentToken.Column)
			p.consumeAttributeArguments()
		}

		p.skipPeekComments()
		if p.peekToken.Type != lexer.AT {
			break
		}
		p.nextToken()
		if p.peekToken.Type != lexer.IDENT || p.peekToken.Lexeme != "noCopy" {
			p.addError("@noCopy cannot be combined with an unsupported attribute at %d:%d", p.curToken.Line, p.curToken.Column)
			return nil
		}
	}

	p.skipPeekComments()
	if p.peekToken.Type != lexer.TYPE && p.peekToken.Type != lexer.ENUM {
		p.addError(
			"@noCopy may only annotate a nominal type declaration at %d:%d",
			p.peekToken.Line,
			p.peekToken.Column,
		)
		return nil
	}

	p.nextToken()
	stmt := p.parseStatement()
	switch stmt := stmt.(type) {
	case *ast.TypeDeclStatement:
		stmt.Attributes = attributes
	case *ast.EnumDeclaration:
		stmt.Attributes = attributes
	default:
		p.addError("@noCopy may only annotate a nominal type declaration at %d:%d", p.curToken.Line, p.curToken.Column)
		return nil
	}
	return stmt
}

func (p *Parser) consumeAttributeArguments() RecoveryEvent {
	p.nextToken()
	start, end, skipped := p.curToken, p.curToken, 1
	delimiters := newDelimiterStack(lexer.RPAREN)
	for !delimiters.empty() && p.peekToken.Type != lexer.EOF {
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

func (p *Parser) parseLinkNameExternDeclaration() ast.Statement {
	annotationToken := p.curToken
	p.nextToken()
	if !p.expectPeek(lexer.LPAREN) {
		return nil
	}
	if p.peekToken.Type != lexer.STRING {
		p.addError("@link_name requires a string literal at %d:%d", p.peekToken.Line, p.peekToken.Column)
		return nil
	}
	p.nextToken()
	linkName := trimStringQuotes(p.curToken.Lexeme)
	if linkName == "" {
		p.addError("@link_name requires a non-empty symbol name at %d:%d", p.curToken.Line, p.curToken.Column)
		return nil
	}
	if !p.expectPeek(lexer.RPAREN) {
		return nil
	}
	if p.peekToken.Type != lexer.EXTERN {
		p.addError("@link_name must annotate an extern declaration at %d:%d", p.peekToken.Line, p.peekToken.Column)
		return nil
	}
	p.nextToken()

	fn := p.parseExternFunctionDeclaration()
	if fn == nil {
		return nil
	}
	fn.LinkName = linkName
	fn.Token = annotationToken
	return fn
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
	var contract ast.Contract
	if p.isContractStart(p.peekToken) {
		contract = p.parseContractSequence()
		if contract == nil {
			return nil
		}
	}

	if p.peekToken.Type == lexer.LPAREN {
		return p.parseTypedVariableGroupDeclaration(token, typ, contract)
	}

	mutable := false
	if p.peekToken.Type == lexer.MUT {
		p.nextToken()
		mutable = true
	}

	if p.peekToken.Type != lexer.COLON {
		if mutable && p.peekToken.Type == lexer.IDENT {
			p.addError("typed mutable declaration requires ':' after mut; write %s mut: %s at %d:%d", parserTypeReferenceName(typ), p.peekToken.Lexeme, p.peekToken.Line, p.peekToken.Column)
			p.skipDeclarationRest()
			return nil
		}
		return nil
	}
	p.nextToken()

	first := p.parseLetDeclarator(token, mutable, typ, contract)
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

		next := p.parseLetDeclarator(token, mutable, typ, contract)
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

func (p *Parser) parseTypedVariableGroupDeclaration(token lexer.Token, typ *ast.TypeReference, contract ast.Contract) ast.Statement {
	if !p.expectPeek(lexer.LPAREN) {
		return nil
	}

	lets := []*ast.LetStatement{}
	for p.peekToken.Type != lexer.RPAREN && p.peekToken.Type != lexer.EOF {
		if p.peekToken.Type == lexer.COMMENT {
			p.nextToken()
			continue
		}
		let := p.parseLetDeclarator(token, false, typ, contract)
		if let == nil {
			return nil
		}
		if let.Value == nil {
			p.addError("immutable typed declaration requires initializer for %q at %d:%d", let.Name.Value, let.Name.Token.Line, let.Name.Token.Column)
			p.skipDeclarationRest()
			return nil
		}
		lets = append(lets, let)

		switch p.peekToken.Type {
		case lexer.COMMA:
			p.nextToken()
			if p.peekToken.Type == lexer.RPAREN {
				p.nextToken()
				if len(lets) == 1 {
					return lets[0]
				}
				return &ast.LetGroupStatement{Token: token, Lets: lets}
			}
		case lexer.RPAREN:
			p.nextToken()
			if len(lets) == 1 {
				return lets[0]
			}
			return &ast.LetGroupStatement{Token: token, Lets: lets}
		default:
			p.addError("expected ',' or ')' after typed declaration at %d:%d", p.peekToken.Line, p.peekToken.Column)
			p.skipDeclarationRest()
			return nil
		}
	}

	if p.peekToken.Type == lexer.RPAREN {
		p.nextToken()
	}
	if len(lets) == 0 {
		p.addError("typed declaration group requires at least one declaration at %d:%d", token.Line, token.Column)
		return nil
	}
	if len(lets) == 1 {
		return lets[0]
	}
	return &ast.LetGroupStatement{Token: token, Lets: lets}
}

func parserTypeReferenceName(ref *ast.TypeReference) string {
	if ref == nil {
		return "type"
	}
	name := ref.Name
	if name == "" && ref.UnitOnly {
		name = "<" + ref.Unit + ">"
	}
	if name == "" {
		name = ref.Token.Lexeme
	}
	if len(ref.TypeArgs) > 0 || len(ref.ConstArgs) > 0 {
		args := make([]string, 0, len(ref.TypeArgs)+len(ref.ConstArgs))
		for _, arg := range ref.TypeArgs {
			args = append(args, parserTypeReferenceName(arg))
		}
		for _, arg := range ref.ConstArgs {
			args = append(args, arg.String())
		}
		name += "[" + strings.Join(args, ", ") + "]"
	}
	if ref.Ref {
		prefix := "ref "
		if ref.MutableRef {
			prefix = "ref mut "
		}
		name = prefix + name
	}
	return name
}

func (p *Parser) skipDeclarationRest() RecoveryEvent {
	start, end, skipped := p.curToken, p.curToken, 1
	delimiters := newDelimiterStack()
	for p.peekToken.Type != lexer.EOF {
		if delimiters.empty() && (p.peekToken.Type == lexer.RBRACE || p.isStatementStart(p.peekToken.Type)) {
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
	if p.peekToken.Type == lexer.EOF {
		p.nextToken()
	}
	return p.recordSkippedRecovery(start, end, skipped, RecoveryProbable)
}

func (p *Parser) parseLetDeclarator(token lexer.Token, mutable bool, inheritedType *ast.TypeReference, inheritedContract ast.Contract) *ast.LetStatement {
	if !p.expectPeek(lexer.IDENT) {
		return nil
	}

	stmt := &ast.LetStatement{
		Token:     token,
		Mutable:   mutable,
		Ownership: ast.OwnershipCopy,
		Name: &ast.Identifier{
			Token: p.curToken,
			Value: p.curToken.Lexeme,
		},
		Type:     inheritedType,
		Contract: inheritedContract,
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
		if p.isContractStart(p.peekToken) {
			stmt.Contract = p.parseContractSequence()
			if stmt.Contract == nil {
				return nil
			}
		}
	}

	initializerErrorCount := len(p.errors)
	switch p.peekToken.Type {
	case lexer.DECLARE:
		p.nextToken()
		p.nextToken()
		stmt.Value = p.parseExpression(LOWEST)
	case lexer.MOVE_DECLARE:
		if stmt.Type != nil {
			p.addError("typed move initializer must use '<-', got ':<-' at %d:%d", p.peekToken.Line, p.peekToken.Column)
			return nil
		}
		stmt.Ownership = ast.OwnershipMove
		p.nextToken()
		p.nextToken()
		stmt.Value = p.parseExpression(LOWEST)
	case lexer.MOVE_ASSIGN:
		if stmt.Type == nil {
			p.addError("inferred move initializer must use ':<-', got '<-' at %d:%d", p.peekToken.Line, p.peekToken.Column)
			return nil
		}
		stmt.Ownership = ast.OwnershipMove
		p.nextToken()
		p.nextToken()
		stmt.Value = p.parseExpression(LOWEST)
	}
	if stmt.Value == nil && len(p.errors) > initializerErrorCount {
		return nil
	}
	if stmt.Value == nil && (p.curToken.Type == lexer.DECLARE || p.curToken.Type == lexer.MOVE_DECLARE || p.curToken.Type == lexer.MOVE_ASSIGN) {
		return nil
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

func (p *Parser) parseStaticStatement() ast.Statement {
	token := p.curToken
	if p.peekToken.Type == lexer.FN {
		p.nextToken()
		fn := p.parseFunctionDeclaration()
		if fn != nil {
			fn.Token = token
			fn.Static = true
		}
		return fn
	}
	if p.peekToken.Type != lexer.LET {
		p.addError("static must modify a declaration, got %q at %d:%d", p.peekToken.Lexeme, p.peekToken.Line, p.peekToken.Column)
		return nil
	}
	p.nextToken()
	stmt := p.parseLetStatement()
	switch stmt := stmt.(type) {
	case *ast.LetStatement:
		stmt.Static = true
		stmt.Token = token
		return stmt
	case *ast.LetGroupStatement:
		stmt.Token = token
		for _, let := range stmt.Lets {
			let.Static = true
			let.Token = token
		}
		return stmt
	default:
		return stmt
	}
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

func (p *Parser) skipPeekComments() {
	for p.peekToken.Type == lexer.COMMENT {
		p.nextToken()
	}
}
