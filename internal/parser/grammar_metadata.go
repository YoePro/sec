package parser

import (
	"strings"

	"sec/internal/lexer"
)

// Grammar starts live here so normal parsing and recovery use the same
// categories. A start is a syntactic candidate, not proof that the complete
// construct is valid in its enclosing scope. Contextual and legacy recovery
// forms remain distinguishable from reserved keywords.
// Rules: rules/foundations/grammar.md — "Parser responsibilities";
// rules/compiler/parser_recovery.md — "A.8 Centralize grammar start predicates".

// isInterfaceMemberStart reports tokens that can reliably begin an interface
// member during later-line recovery, including contextual event requirements.
//
// Rules:
//   - rules/foundations/grammar.md — "Interface declaration", InterfaceMember production
//   - rules/compiler/parser_recovery.md — "Interface recovery"
func (p *Parser) isInterfaceMemberStart(token lexer.Token) bool {
	switch token.Type {
	case lexer.FN, lexer.MUT, lexer.CONSUME_ARROW, lexer.STATIC, lexer.PROPERTY:
		return true
	case lexer.IDENT:
		return token.Lexeme == contextualEvent
	default:
		return false
	}
}

// isImplMemberStart includes contextual init/event/unit metadata and attributes.
// It does not classify arbitrary identifiers as reliable member boundaries.
// Rules: rules/foundations/grammar.md — "Impl declaration", "Unit metadata inside impl";
// rules/compiler/parser_recovery.md — "Impl-member recovery", Appendix A.8.
func (p *Parser) isImplMemberStart(token lexer.Token) bool {
	switch token.Type {
	case lexer.TYPE, lexer.UNIT, lexer.ENUM, lexer.FN, lexer.FREE, lexer.PROPERTY, lexer.STRUCT, lexer.LET, lexer.STATIC, lexer.AT:
		return true
	case lexer.IDENT:
		return token.Lexeme == contextualInit || token.Lexeme == contextualEvent || p.isUnitMetadataName(token.Lexeme)
	default:
		return false
	}
}

// isUnitMetadataName recognizes contextual unit metadata fields without reserving
// them globally.
//
// Rules: rules/foundations/grammar.md — "Unit metadata inside impl".
func (p *Parser) isUnitMetadataName(name string) bool {
	switch normalizeUnitMetadataName(name) {
	case "dimension", "scale", "system", "longname", "symbol", "baseunit", "status", "kind", "transform", "offset", "origin", "logbase", "logfactor", "reference":
		return true
	default:
		return false
	}
}

// normalizeUnitMetadataName normalizes the accepted case/underscore metadata
// spellings.
//
// Rules: rules/corrections/applied/grammar-units-correction-20260818.md — canonical
// metadata fields and retained parser compatibility.
func normalizeUnitMetadataName(name string) string {
	return strings.ReplaceAll(strings.ToLower(name), "_", "")
}

// isContractStart recognizes contract starts using the canonical lexer-owned word
// inventory.
//
// Rules: rules/foundations/grammar.md — "Type contracts";
// rules/foundations/lexical_structure.md §7.3.
func (p *Parser) isContractStart(token lexer.Token) bool {
	if token.Type == lexer.RANGE_KW || token.Type == lexer.IN {
		return true
	}
	if token.Type != lexer.IDENT {
		return false
	}
	// `regex` is a reserved contract spelling like the other contract words
	// and follows ordinary whitespace and continuation rules; it has no
	// same-line requirement (MD-009).
	//
	// Rules:
	//   - rules/corrections/applied/missing-decisions-md001-md009-correction-20261003.md — §§ 10.1–10.14
	return lexer.IsContractWord(token.Lexeme)
}

// isTypeStart recognizes canonical and explicitly diagnosed legacy type starts.
//
// Rules: rules/foundations/grammar.md — "Type-reference grammar", "Prefix sequence
// types are invalid".
func isTypeStart(tokenType lexer.TokenType) bool {
	return tokenType == lexer.IDENT || tokenType == lexer.LT || tokenType == lexer.LBRACKET || tokenType == lexer.LPAREN || tokenType == lexer.VOID || tokenType == lexer.FN || tokenType == lexer.MUT || tokenType == lexer.CONSUME_ARROW || tokenType == lexer.REF
}

// isTypeNameToken recognizes a name at a type-versus-expression lookahead boundary.
//
// Rules: rules/foundations/grammar.md — "Type versus expression ambiguity".
func (p *Parser) isTypeNameToken(tokenType lexer.TokenType) bool {
	return tokenType == lexer.IDENT
}

// isStatementStart recognizes reliable token-only recovery starts, including
// diagnosed misplaced forms. Identifier-led candidates need contextual lookahead;
// expression starts are classified separately.
//
// Rules: rules/compiler/parser_recovery.md — "Statement recovery", Appendix A.8.
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
		lexer.ASSERT,
		lexer.PANIC,
		lexer.REF,
		lexer.LBRACKET,
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

// isDeclarationStart recognizes unambiguous declaration keywords for recovery and
// invalid-declaration retention. Identifier-led typed/test declarations require
// contextual lookahead.
//
// Rules: rules/compiler/parser_recovery.md — "Declaration recovery", Appendix A.8.
func isDeclarationStart(t lexer.TokenType) bool {
	switch t {
	case lexer.MODULE, lexer.IMPORT, lexer.TYPE, lexer.UNIT, lexer.ENUM,
		lexer.STRUCT, lexer.INTERFACE, lexer.IMPL, lexer.EXTERN, lexer.FN,
		lexer.PROPERTY, lexer.STATIC, lexer.LET, lexer.UNSAFE, lexer.AT:
		return true
	default:
		return false
	}
}

// assignmentOperators returns the statement-only operators; := is a binding
// delimiter and ++/-- are aliases normalized by their statement parser.
// Rules: rules/foundations/grammar.md — "Assignment statements".
func assignmentOperators() []lexer.TokenType {
	return []lexer.TokenType{
		lexer.ASSIGN, lexer.MOVE_ASSIGN, lexer.PLUS_ASSIGN, lexer.MINUS_ASSIGN,
		lexer.ASTERISK_ASSIGN, lexer.SLASH_ASSIGN, lexer.PERCENT_ASSIGN,
		lexer.BIT_AND_ASSIGN, lexer.BIT_OR_ASSIGN, lexer.BIT_XOR_ASSIGN,
		lexer.SHIFT_LEFT_ASSIGN, lexer.SHIFT_RIGHT_ASSIGN,
	}
}

// isAssignmentOperator shares the inventory used for expected-token help.
// Rules: rules/foundations/grammar.md — "Assignment statements".
func (p *Parser) isAssignmentOperator(t lexer.TokenType) bool {
	for _, operator := range assignmentOperators() {
		if t == operator {
			return true
		}
	}
	return false
}

// isExpressionStart recognizes value-expression prefixes shared by normal parsing
// and list recovery.
//
// Rules: rules/foundations/grammar.md — "Primary expressions", "Prefix expressions",
// "Qualified unit references".
func (p *Parser) isExpressionStart(t lexer.TokenType) bool {
	switch t {
	case lexer.IDENT,
		lexer.SELF,
		lexer.INT,
		lexer.FLOAT,
		lexer.STRING,
		lexer.RAW_STRING,
		lexer.CHAR,
		lexer.INTERPSTRING,
		lexer.TRUE,
		lexer.FALSE,
		lexer.NULL,
		lexer.LT,
		lexer.MOVE_ASSIGN,
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

// Contextual grammar spellings remain identifiers outside their owned context.
// Contract/type-name inventories stay in lexer; operator precedence stays in operators.
// Rules: rules/foundations/lexical_structure.md §§7.2,9,10.1;
// rules/foundations/grammar.md — "Contextual spellings", "Assembly statement".
const (
	contextualTarget      = "target"
	contextualTest        = "test"
	contextualDo          = "do"
	contextualDetach      = "detach"
	contextualNot         = "not"
	contextualIs          = "is"
	contextualX           = "x"
	contextualAvailable   = "available"
	contextualNone        = "None"
	contextualSome        = "Some"
	contextualEmpty       = "empty"
	contextualMove        = "move"
	contextualStep        = "step"
	contextualRegister    = "register"
	contextualError       = "error"
	contextualEvent       = "event"
	contextualUnreachable = "unreachable"
	contextualBit         = "bit"
	contextualInit        = "init"
	contextualSet         = "set"
	contextualUsing       = "using"
	contextualTask        = "task"
	contextualThread      = "thread"
	contextualProcess     = "process"
	contextualInputs      = "inputs"
	contextualOutputs     = "outputs"
	contextualClobbers    = "clobbers"
	contextualFlex        = "flex"
	contextualCallback    = "callback"
	contextualRead        = "read"
	contextualWrite       = "write"
	contextualClear       = "clear"
)

// isOuterDeclarationRecoveryStart preserves strong declaration boundaries even
// after an unmatched delimiter, but ambiguous statement/declaration prefixes
// cannot end a nested region. In particular a let inside a malformed function
// remains inside that function's skipped body.
// Rules: rules/compiler/parser_recovery.md — "Enclosing-boundary safety",
// "Declaration-header recovery", "Prefer grammar context".
func isOuterDeclarationRecoveryStart(t lexer.TokenType, nested bool) bool {
	if !isDeclarationStart(t) {
		return false
	}
	if nested {
		switch t {
		case lexer.LET, lexer.UNSAFE, lexer.AT:
			return false
		}
	}
	return true
}
