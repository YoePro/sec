package cst

import (
	"reflect"

	"sec/internal/ast"
	"sec/internal/lexer"
)

// Role records a parser-owned grammatical meaning on a concrete source
// element. Roles enrich the lossless token tape; they never replace its exact
// source spelling, trivia, or byte ranges.
type Role string

const (
	// GenericConstraintConjunction marks the real & token joining two valid
	// constraints in a generic parameter declaration.
	GenericConstraintConjunction Role = "generic-constraint-conjunction"
	// ContextualMatrixOperator marks an identifier-spelled x that the parser
	// resolved as an infix matrix-multiplication operator.
	ContextualMatrixOperator Role = "contextual-matrix-operator"
	// PostfixMutationAlias marks a statement-only ++ or -- token normalized by
	// the parser to its canonical compound assignment.
	PostfixMutationAlias Role = "postfix-mutation-alias"
	// UnitMetadataName marks a metadata declaration name inside an impl whose
	// target is a parsed unit declaration.
	UnitMetadataName Role = "unit-metadata-name"
	// UnitExpressionCompactToken marks a real operator or grouping delimiter in
	// a parser-confirmed structural unit expression.
	UnitExpressionCompactToken Role = "unit-expression-compact-token"
	// CallableParameterListOpen marks the real opening parenthesis of a parsed
	// function, lambda, or initializer parameter list.
	CallableParameterListOpen Role = "callable-parameter-list-open"
	// CallableParameterListClose marks the matching real closing parenthesis of
	// a complete parsed callable parameter list.
	CallableParameterListClose Role = "callable-parameter-list-close"
	// CallableParameterColon marks the declaration colon of a complete callable
	// parameter, excluding the colon-free shorthand ref self form.
	CallableParameterColon Role = "callable-parameter-colon"
	// CallableParameterTypeStart marks the first concrete type token after the
	// colon, including a variadic spread marker when present.
	CallableParameterTypeStart Role = "callable-parameter-type-start"
	// TypeArgumentListOpen marks the real opening bracket of a complete parsed
	// generic type-argument list, excluding sequence types and indexing.
	TypeArgumentListOpen Role = "type-argument-list-open"
	// TypeArgumentListClose marks the matching real closing bracket.
	TypeArgumentListClose Role = "type-argument-list-close"
	// LambdaCaptureListOpen marks the real opening parenthesis of a parsed
	// explicit lambda capture list.
	LambdaCaptureListOpen Role = "lambda-capture-list-open"
	// LambdaCaptureListClose marks the matching real closing parenthesis of a
	// complete parsed explicit lambda capture list.
	LambdaCaptureListClose Role = "lambda-capture-list-close"
	// CommaListOpen marks the real opening delimiter of a parsed
	// comma-separated call argument list, array literal, or struct literal
	// field list whose grammar permits a trailing comma.
	CommaListOpen Role = "comma-list-open"
	// CommaListClose marks the matching real closing delimiter.
	CommaListClose Role = "comma-list-close"
	// BinaryOperator marks the operator token of a parsed infix expression.
	BinaryOperator Role = "binary-operator"
	// AssignmentOperator marks the operator of an assignment statement and the
	// initialization operator (:= or :<-) of a let declaration.
	AssignmentOperator Role = "assignment-operator"
	// PrefixOperator marks the operator token of a parsed unary prefix
	// expression, including the <- consuming call-site marker.
	PrefixOperator Role = "prefix-operator"
	// ForeignQualifierSeparator marks each ':' of a parser-confirmed `::` in
	// a C:: or c:: foreign-qualified name. The qualification is one contiguous
	// name: no space separates the family, separators, and segments.
	//
	// Rules: rules/platform/ffi.md §5-6; rules/foundations/grammar.md ForeignTypeReference.
	ForeignQualifierSeparator Role = "foreign-qualifier-separator"
	// DeclarationColon marks the type-annotation colon of a let declaration.
	DeclarationColon Role = "declaration-colon"
	// SpacedKeyword marks a statement or control-flow keyword that is followed
	// by exactly one space when its operand continues on the same line.
	SpacedKeyword Role = "spaced-keyword"
	// NamedTypeBaseStart marks the first base-type token of a simple named-type
	// declaration such as `type Port int range 1..65535`.
	NamedTypeBaseStart Role = "named-type-base-start"
	// NamedTypeContractStart marks the first contract or default-clause token
	// of a simple named-type declaration.
	NamedTypeContractStart Role = "named-type-contract-start"
	// EmptyCollectionLiteralClose marks the closing brace of an empty
	// compiler-known collection literal, written directly after its opener.
	EmptyCollectionLiteralClose Role = "empty-collection-literal-close"
	// SpacedBefore marks a parser-confirmed token preceded by exactly one space
	// when it continues the same line, such as a named-type contract start or
	// default clause.
	SpacedBefore Role = "spaced-before"
	// ControlBodyOpen marks the real opening brace of a switch or match body,
	// which is not an executable BlockStatement.
	ControlBodyOpen Role = "control-body-open"
	// ExecutableBlockOpen marks the real opening brace of a parser-confirmed
	// executable block, excluding aggregate literals and structural declarations.
	ExecutableBlockOpen Role = "executable-block-open"
	// ExecutableBlockClose marks its matching real closing brace.
	ExecutableBlockClose Role = "executable-block-close"
	// DeclarationGroupSeparator marks a comma that separates two declarators
	// belonging to one parser-confirmed declaration group.
	DeclarationGroupSeparator Role = "declaration-group-separator"
	// MatchPatternPayloadOpen marks the opening parenthesis of a parsed variant
	// payload-binding pattern.
	MatchPatternPayloadOpen Role = "match-pattern-payload-open"
	// MatchPatternPayloadClose marks its real closing parenthesis.
	MatchPatternPayloadClose Role = "match-pattern-payload-close"
	// MatchPatternFieldsOpen marks the opening brace of a parsed shallow field
	// pattern.
	MatchPatternFieldsOpen Role = "match-pattern-fields-open"
	// MatchPatternFieldsClose marks its real closing brace.
	MatchPatternFieldsClose Role = "match-pattern-fields-close"
	// MatchPatternFieldSeparator marks a comma between parsed shallow fields.
	MatchPatternFieldSeparator Role = "match-pattern-field-separator"
	// MatchPatternFieldColon marks an explicit field-to-binding separator.
	MatchPatternFieldColon Role = "match-pattern-field-colon"
	// MatchGuardKeyword marks the where token of a parsed match arm or try
	// handler guard.
	MatchGuardKeyword Role = "match-guard-keyword"
	// HandlerArrow marks the real => token of a parsed match arm or try handler.
	HandlerArrow Role = "handler-arrow"
	// SwitchCaseSeparator marks a comma between parser-confirmed case items.
	SwitchCaseSeparator Role = "switch-case-separator"
	// SwitchCaseColon marks the colon terminating a valid case or default header.
	SwitchCaseColon Role = "switch-case-colon"
	// RedundantControlConditionDelimiter marks the complete outer parenthesis
	// pair around a parser-confirmed if, while, or switch condition.
	RedundantControlConditionDelimiter Role = "redundant-control-condition-delimiter"
	// TestDeclarationName marks the exact string token in a valid top-level
	// source-test header.
	TestDeclarationName Role = "test-declaration-name"
	// AvailabilityBlockOpen marks the real opening brace owned by an if or
	// while whose condition is a compiler-known state query: an ownership
	// availability test or a union `is Variant` / `is empty` state test.
	AvailabilityBlockOpen Role = "availability-block-open"
	// AttachedAttributeEnd marks the final real token of an argument-free
	// attribute that the parser attached to a declaration.
	AttachedAttributeEnd Role = "attached-attribute-end"
	// AttributedDeclarationStart marks the first real token of the declaration
	// owned by an attached attribute set.
	AttributedDeclarationStart Role = "attributed-declaration-start"
	// RangeOperator marks .. or ..< in a parser-confirmed range or slice.
	RangeOperator Role = "range-operator"
	// RangeOperatorOpenStart marks .. or ..< in a range without a lower bound,
	// such as the switch case pattern `..<0`; only its upper-bound side is
	// compact.
	RangeOperatorOpenStart Role = "range-operator-open-start"
	// RangeOperatorOpenEnd marks .. in a range without an upper bound; only
	// its lower-bound side is compact.
	RangeOperatorOpenEnd Role = "range-operator-open-end"
	// RangeStepKeyword marks contextual step in a parser-confirmed for range.
	RangeStepKeyword Role = "range-step-keyword"
	// StructFieldColon marks the real colon in a parser-confirmed struct or
	// register field; both share one structural alignment engine.
	StructFieldColon Role = "struct-field-colon"
	// StructFieldTypeStart marks the first real token of its declared type.
	StructFieldTypeStart Role = "struct-field-type-start"
	// StructFieldTag marks a parser-confirmed raw struct-field tag token.
	StructFieldTag Role = "struct-field-tag"
	// EnumValueAssignment marks the real = token of a complete explicit enum
	// member initializer.
	EnumValueAssignment Role = "enum-value-assignment"
)

// HasRole reports whether this concrete element carries a grammatical role.
func (e Element) HasRole(role Role) bool {
	for _, candidate := range e.Roles {
		if candidate == role {
			return true
		}
	}
	return false
}

// ApplyProgramRoles projects parser-owned grammatical facts onto matching
// real CST tokens. The parser remains the sole grammar authority; CST roles
// give source-to-source tools exact byte ownership without reparsing text.
//
// Rules:
//   - rules/tooling/formatter.md — §5 "Required syntax representation"
//   - rules/tooling/formatter.md — §25 "Malformed and incomplete source"
//   - rules/declarations/generics.md — §12 "Multiple constraints"
//   - rules/foundations/lexical_structure.md — §10 "Contextual operator x"
//   - rules/foundations/operators.md — "Increment and decrement aliases"
//   - rules/types/units.md — "Unit metadata"
//   - rules/types/units.md — "Structural unit expressions"
//   - rules/tooling/testing.md — §5.1 "Canonical form"
//   - rules/memory/ownership.md — §21 "is available and is not available"
//   - rules/tooling/formatter.md — §18 "Control flow"
//   - rules/tooling/formatter.md — §20 "Patterns and destructuring"
//   - rules/tooling/formatter.md — §23 "assert, ranges, and step"
//   - rules/tooling/formatter.md — §9(5–10) structural field alignment
//   - rules/tooling/formatter.md — §15(4) enum assignment alignment
//   - rules/tooling/formatter.md — §16(14–17) "attributes"
//   - rules/tooling/formatter.md — §16(12) "multiline capture lists"
//   - rules/tooling/formatter.md — §8(3–8) "brace placement"
//   - rules/tooling/formatter.md — §27(17–19) parenthesis corrections
func (d *Document) ApplyProgramRoles(program *ast.Program) {
	if d == nil || program == nil {
		return
	}
	indexes := make(map[concreteTokenKey]int, len(d.Elements))
	for index, element := range d.Elements {
		if element.Kind == Token {
			indexes[keyForToken(element.Token)] = index
		}
	}

	mark := func(token lexer.Token, role Role) {
		elementIndex, ok := indexes[keyForToken(token)]
		if !ok {
			return
		}
		d.Elements[elementIndex].Roles = appendUniqueRole(d.Elements[elementIndex].Roles, role)
	}
	markGroup := func(token lexer.Token, role Role) {
		elementIndex, ok := indexes[keyForToken(token)]
		if !ok {
			return
		}
		d.Elements[elementIndex].Roles = appendUniqueRole(d.Elements[elementIndex].Roles, role)
		for _, group := range d.Groups {
			if group.Open == elementIndex && group.Close >= 0 {
				d.Elements[group.Close].Roles = appendUniqueRole(d.Elements[group.Close].Roles, role)
				return
			}
		}
	}
	markGroupPair := func(token lexer.Token, openRole, closeRole Role) {
		elementIndex, ok := indexes[keyForToken(token)]
		if !ok {
			return
		}
		d.Elements[elementIndex].Roles = appendUniqueRole(d.Elements[elementIndex].Roles, openRole)
		for _, group := range d.Groups {
			if group.Open == elementIndex && group.Close >= 0 {
				d.Elements[group.Close].Roles = appendUniqueRole(d.Elements[group.Close].Roles, closeRole)
				return
			}
		}
	}
	markPreviousToken := func(token lexer.Token, expected lexer.TokenType, role Role) {
		elementIndex, ok := indexes[keyForToken(token)]
		if !ok {
			return
		}
		for index := elementIndex - 1; index >= 0; index-- {
			if d.Elements[index].Kind != Token {
				continue
			}
			if d.Elements[index].Token.Type == expected {
				d.Elements[index].Roles = appendUniqueRole(d.Elements[index].Roles, role)
			}
			return
		}
	}
	markNextGroup := func(token lexer.Token, expected lexer.TokenType, openRole, closeRole Role) {
		elementIndex, ok := indexes[keyForToken(token)]
		if !ok {
			return
		}
		for index := elementIndex + 1; index < len(d.Elements); index++ {
			if d.Elements[index].Kind != Token {
				continue
			}
			if d.Elements[index].Token.Type == expected {
				d.Elements[index].Roles = appendUniqueRole(d.Elements[index].Roles, openRole)
				for _, group := range d.Groups {
					if group.Open == index && group.Close >= 0 {
						d.Elements[group.Close].Roles = appendUniqueRole(d.Elements[group.Close].Roles, closeRole)
						return
					}
				}
			}
			return
		}
	}
	// nextTokenAtDepth returns the first token after start, outside any nested
	// delimiter group, that satisfies accept; it stops at a closer that would
	// leave the starting nesting level.
	nextTokenAtDepth := func(start lexer.Token, accept func(lexer.TokenType) bool) int {
		elementIndex, ok := indexes[keyForToken(start)]
		if !ok {
			return -1
		}
		depth := 0
		for index := elementIndex + 1; index < len(d.Elements); index++ {
			element := d.Elements[index]
			if element.Kind != Token {
				continue
			}
			typ := element.Token.Type
			if depth == 0 && accept(typ) {
				return index
			}
			switch typ {
			case lexer.LPAREN, lexer.LBRACKET, lexer.LBRACE:
				depth++
			case lexer.RPAREN, lexer.RBRACKET, lexer.RBRACE:
				if depth == 0 {
					return -1
				}
				depth--
			}
		}
		return -1
	}
	markIndex := func(index int, role Role) {
		if index >= 0 {
			d.Elements[index].Roles = appendUniqueRole(d.Elements[index].Roles, role)
		}
	}
	// markDeclarationHeader gives a word-structured declaration header one
	// canonical space between its words, from the declaration keyword up to the
	// opening body brace or the end of the line. Bracketed, parenthesized, and
	// angle-bracketed parts stay attached to the word on their left, member dots
	// and commas keep their own spacing, an enum-underlying colon binds left, and
	// the body brace follows one space.
	//
	// Rules:
	//   - rules/tooling/formatter.md — §3 canonical model, §6(4) declaration colon, §8(1) same-line braces
	//   - rules/foundations/grammar.md — EnumDeclaration, UnitDeclaration, InterfaceDeclaration, ImplDeclaration, SingleImport
	markDeclarationHeader := func(start lexer.Token) {
		startIndex, ok := indexes[keyForToken(start)]
		if !ok {
			return
		}
		line := start.Line
		depth := 0
		previous := -1
		for index := startIndex; index < len(d.Elements); index++ {
			element := d.Elements[index]
			if element.Kind != Token {
				if element.Kind == Comment {
					return
				}
				continue
			}
			if element.Token.Line != line {
				return
			}
			typ := element.Token.Type
			if depth == 0 && previous >= 0 {
				left := d.Elements[previous].Token.Type
				switch {
				case typ == lexer.LBRACE:
					markIndex(index, SpacedBefore)
					return
				case typ == lexer.COLON:
					markIndex(index, DeclarationColon)
				case left == lexer.COLON:
				case typ == lexer.LBRACKET || typ == lexer.LPAREN || typ == lexer.LT || typ == lexer.DOT || typ == lexer.COMMA ||
					left == lexer.DOT:
				case left == lexer.COMMA:
					markIndex(index, SpacedBefore)
				default:
					markIndex(previous, SpacedKeyword)
				}
			}
			switch typ {
			case lexer.LBRACKET, lexer.LPAREN, lexer.LT:
				depth++
			case lexer.RBRACKET, lexer.RPAREN, lexer.GT:
				if depth > 0 {
					depth--
				}
			}
			if depth == 0 {
				previous = index
			}
		}
	}
	// markTypeFirstDeclarator assigns spacing roles to `T mut: name`,
	// `T: name := value`, and the parenthesized group `T (name := value, ...)`.
	// The declarator's own name is the anchor; the colon binds to the type or
	// `mut` on its left, `mut` follows the type after one space, and a group's
	// opening parenthesis follows the type after one space.
	//
	// Rules:
	//   - rules/types/types.md — "Type-first declarations", "Parenthesized immutable type-first groups"
	//   - rules/tooling/formatter.md — §6(4) declaration colon, §3 canonical spacing
	markTypeFirstDeclarator := func(node *ast.LetStatement) {
		nameIndex, ok := indexes[keyForToken(node.Name.Token)]
		if !ok {
			return
		}
		previous := func(index int) int {
			for index--; index >= 0; index-- {
				if d.Elements[index].Kind == Token {
					return index
				}
			}
			return -1
		}
		before := previous(nameIndex)
		if before < 0 {
			return
		}
		switch d.Elements[before].Token.Type {
		case lexer.COLON:
			markIndex(before, DeclarationColon)
			if mutIndex := previous(before); mutIndex >= 0 && d.Elements[mutIndex].Token.Type == lexer.MUT {
				markIndex(mutIndex, SpacedBefore)
			}
		case lexer.LPAREN:
			markIndex(before, SpacedBefore)
		}
	}
	markAttachedAttributes := func(attributes []*ast.Attribute, declaration lexer.Token) {
		if len(attributes) == 0 || declaration.Type == lexer.EOF {
			return
		}
		// rules/tooling/formatter.md — §16(14): attributes are written one per
		// line, so an argument-free attribute also ends its line before the
		// next attribute.
		for index := 0; index+1 < len(attributes); index++ {
			current, next := attributes[index], attributes[index+1]
			if current == nil || current.Name == nil || len(current.Arguments) != 0 || next == nil {
				continue
			}
			mark(current.Name.Token, AttachedAttributeEnd)
			mark(next.Token, AttributedDeclarationStart)
		}
		last := attributes[len(attributes)-1]
		if last == nil || last.Name == nil || len(last.Arguments) != 0 {
			return
		}
		mark(last.Name.Token, AttachedAttributeEnd)
		mark(declaration, AttributedDeclarationStart)
	}
	markCallableParameters := func(open lexer.Token, parameters []*ast.Parameter) {
		mark(open, CallableParameterListOpen)
		for _, parameter := range parameters {
			if parameter == nil || parameter.Name == nil || parameter.Type == nil || parameter.Type.Invalid {
				return
			}
		}
		for _, parameter := range parameters {
			nameIndex, nameOK := indexes[keyForToken(parameter.Name.Token)]
			typeIndex, typeOK := indexes[keyForToken(parameter.Type.Token)]
			if !nameOK || !typeOK || typeIndex <= nameIndex {
				continue
			}
			colonIndex := -1
			for index := nameIndex + 1; index < typeIndex; index++ {
				if d.Elements[index].Kind == Token && d.Elements[index].Token.Type == lexer.COLON {
					colonIndex = index
					break
				}
			}
			if colonIndex < 0 {
				continue
			}
			typeStart := -1
			for index := colonIndex + 1; index <= typeIndex; index++ {
				if d.Elements[index].Kind == Token {
					typeStart = index
					break
				}
			}
			if typeStart < 0 {
				continue
			}
			d.Elements[colonIndex].Roles = appendUniqueRole(d.Elements[colonIndex].Roles, CallableParameterColon)
			d.Elements[typeStart].Roles = appendUniqueRole(d.Elements[typeStart].Roles, CallableParameterTypeStart)
		}
		markGroupPair(open, CallableParameterListOpen, CallableParameterListClose)
	}

	unitNames := map[string]bool{}
	for _, statement := range program.Statements {
		if declaration, ok := statement.(*ast.UnitDeclStatement); ok && declaration.Name != nil {
			unitNames[declaration.Name.Value] = true
		}
	}
	for _, statement := range program.Statements {
		implementation, ok := statement.(*ast.ImplStatement)
		if !ok || implementation.Target == nil || !unitNames[implementation.Target.Name] {
			continue
		}
		for _, member := range implementation.Members {
			if metadata, ok := member.(*ast.UnitMetadataDeclaration); ok {
				mark(metadata.Token, UnitMetadataName)
			}
		}
	}

	// markSpacingRoles marks the operator, keyword, and body-brace positions
	// whose same-line horizontal spacing is canonical.
	//
	// Rules:
	//   - rules/tooling/formatter.md — §6(4)–(8) "Indentation and basic whitespace"
	//   - rules/tooling/formatter.md — §8(1)–(3), §18(1) same-line braces and "} else {"
	//   - rules/tooling/formatter.md — §21(5) consuming call-site marker attachment
	markSpacingRoles := func(node any) {
		switch node := node.(type) {
		case *ast.InfixExpression:
			if node.Left != nil && node.Right != nil {
				mark(node.Token, BinaryOperator)
				if node.Operator == "not in" {
					markIndex(nextTokenAtDepth(node.Token, func(typ lexer.TokenType) bool { return typ == lexer.IN }), BinaryOperator)
				}
			}
		case *ast.PrefixExpression:
			if node.Right != nil {
				mark(node.Token, PrefixOperator)
			}
		case *ast.AssignmentStatement:
			if node.Target != nil && node.Value != nil && node.PostfixAlias.Type == "" {
				markIndex(nextTokenAtDepth(node.Token, isAssignmentOperatorToken), AssignmentOperator)
			}
		case *ast.LetStatement:
			if node.Name == nil {
				break
			}
			// Typed declarations such as `float: low := 1.0` start with their
			// type name, not a let keyword.
			if node.Token.Type == lexer.LET {
				mark(node.Token, SpacedKeyword)
			}
			if node.Mutable && node.Token.Type == lexer.LET {
				markIndex(nextTokenAtDepth(node.Token, func(typ lexer.TokenType) bool { return typ == lexer.MUT }), SpacedKeyword)
			}
			if node.Token.Type != lexer.LET {
				markTypeFirstDeclarator(node)
			}
			if node.Type != nil && !node.Type.Invalid {
				colon := nextTokenAtDepth(node.Name.Token, func(typ lexer.TokenType) bool { return true })
				if colon >= 0 && d.Elements[colon].Token.Type == lexer.COLON {
					markIndex(colon, DeclarationColon)
				}
			}
			if node.Value != nil && !node.SynthesizedDefault {
				markIndex(nextTokenAtDepth(node.Name.Token, func(typ lexer.TokenType) bool {
					return typ == lexer.DECLARE || typ == lexer.MOVE_DECLARE
				}), AssignmentOperator)
			}
		case *ast.TypeDeclStatement:
			// rules/tooling/formatter.md §3 and §6: one canonical space separates
			// the words of a named-type header, its sequential contracts, and its
			// default clause (rules/foundations/grammar.md "Type contracts",
			// "Default clause"). Gaps inside contract operands keep their roles.
			if node.Name == nil {
				break
			}
			mark(node.Token, SpacedKeyword)
			if len(node.GenericParameters) == 0 {
				mark(node.Name.Token, SpacedKeyword)
			} else {
				// After a generic parameter list the following header word
				// carries the space instead of the declared name.
				markIndex(nextTokenAtDepth(node.Name.Token, func(typ lexer.TokenType) bool { return typ != lexer.LBRACKET }), SpacedBefore)
			}
			if node.ErrorType {
				mark(node.ErrorToken, SpacedBefore)
			}
			// rules/tooling/formatter.md §7(6) and §9(2): simple named-type
			// declarations expose their base-type and first contract/default
			// anchors for homogeneous group alignment.
			if isSimpleNamedTypeDeclaration(node) {
				markIndex(nextTokenAtDepth(node.Name.Token, func(lexer.TokenType) bool { return true }), NamedTypeBaseStart)
				if contracts := typeDeclarationContracts(node.Contract); len(contracts) > 0 {
					token, _ := contractSpacingToken(contracts[0])
					mark(token, NamedTypeContractStart)
				} else if node.Default != nil {
					mark(node.DefaultToken, NamedTypeContractStart)
				}
			}
			for _, contract := range typeDeclarationContracts(node.Contract) {
				token, hasOperand := contractSpacingToken(contract)
				mark(token, SpacedBefore)
				if hasOperand {
					mark(token, SpacedKeyword)
				}
			}
			if node.Default != nil {
				mark(node.DefaultToken, SpacedBefore)
				mark(node.DefaultToken, SpacedKeyword)
			}
		case *ast.EnumDeclaration:
			if node.Name != nil {
				markDeclarationHeader(node.Token)
			}
		case *ast.UnitDeclStatement:
			markDeclarationHeader(node.Token)
		case *ast.InterfaceDeclaration:
			if node.Name != nil {
				markDeclarationHeader(node.Token)
			}
		case *ast.ImplStatement:
			markDeclarationHeader(node.Token)
		case *ast.ImportStatement:
			if node.PathToken.Type != lexer.EOF && node.Token.Type == lexer.IMPORT && node.Token.Line == node.PathToken.Line {
				markDeclarationHeader(node.Token)
			}
		case *ast.RegisterField:
			// rules/tooling/formatter.md §6(4): the register field colon binds
			// left. Register fields, including reserved `_` fields, are clients
			// of the same structural field-alignment engine as struct fields
			// (MD-013; md010-md014 correction §§ 5.20–5.23, 9.4; formatter.md
			// § 22(1), § 22(4)), so they carry the same colon and type anchors.
			if node.Name != nil {
				colon := nextTokenAtDepth(node.Name.Token, func(lexer.TokenType) bool { return true })
				if colon >= 0 && d.Elements[colon].Token.Type == lexer.COLON {
					markIndex(colon, DeclarationColon)
					// The type anchor is the first token after the colon:
					// `bit`, `bit[N]`, or a named field type.
					for typeStart := colon + 1; typeStart < len(d.Elements); typeStart++ {
						if d.Elements[typeStart].Kind != Token {
							continue
						}
						if d.Elements[typeStart].Token.Line == d.Elements[colon].Token.Line && d.Elements[typeStart].Token.Type != lexer.RBRACE {
							markIndex(colon, StructFieldColon)
							markIndex(typeStart, StructFieldTypeStart)
						}
						break
					}
				}
			}
		case *ast.CollectionLiteral:
			// rules/collections/collections.md §13.3 writes the canonical
			// empty literal as `list[T] {}`: one space before an empty brace pair.
			if node.Type != nil && !node.Invalid && node.Open.Type == lexer.LBRACE && node.Close.Type == lexer.RBRACE {
				mark(node.Open, SpacedBefore)
				mark(node.Close, EmptyCollectionLiteralClose)
			}
		case *ast.ReturnStatement:
			if node.Value != nil {
				mark(node.Token, SpacedKeyword)
			}
		case *ast.DiscardStatement:
			// rules/memory/ownership.md explicit discard statement keyword.
			if node.Token.Type == lexer.DISCARD && (node.Value != nil || node.Name != nil) {
				mark(node.Token, SpacedKeyword)
			}
		case *ast.DetachStatement:
			// rules/concurrency/tasks.md §22: contextual `detach handle [discard]`.
			if node.Token.Lexeme == "detach" && node.Value != nil {
				mark(node.Token, SpacedKeyword)
				if node.DiscardResult && node.DiscardToken.Type == lexer.DISCARD {
					mark(node.DiscardToken, SpacedBefore)
				}
			}
		case *ast.AssertStatement:
			// rules/errors/panic.md §15.1 and formatter.md §23(1) statement syntax.
			if node.Token.Type == lexer.ASSERT && node.Condition != nil {
				mark(node.Token, SpacedKeyword)
			}
		case *ast.PanicStatement:
			// rules/errors/panic.md §17 `panic "message"`.
			if node.Token.Type == lexer.PANIC && node.Message != nil {
				mark(node.Token, SpacedKeyword)
			}
		case *ast.PropertySetter:
			// rules/errors/errorhandling.md §24: `try set value ErrorType`.
			if node.Parameter != nil && !node.Invalid {
				mark(node.Token, SpacedKeyword)
				if node.ErrorType != nil {
					mark(node.Parameter.Token, SpacedKeyword)
				}
			}
		case *ast.InterfaceProperty:
			if node.SetterParameter != nil {
				mark(node.SetToken, SpacedKeyword)
				if node.SetterErrorType != nil {
					mark(node.SetterParameter.Token, SpacedKeyword)
				}
			}
		case *ast.IfStatement:
			mark(node.Token, SpacedKeyword)
			if node.Alternative != nil && node.Consequence != nil && node.Consequence.Token.Type == lexer.LBRACE {
				if closeIndex, ok := indexes[keyForToken(node.Consequence.Token)]; ok {
					for _, group := range d.Groups {
						if group.Open != closeIndex || group.Close < 0 {
							continue
						}
						for index := group.Close + 1; index < len(d.Elements); index++ {
							if d.Elements[index].Kind == Token {
								if d.Elements[index].Token.Type == lexer.ELSE {
									markIndex(index, SpacedKeyword)
									d.Elements[index].Roles = appendUniqueRole(d.Elements[index].Roles, BinaryOperator)
								}
								break
							}
						}
						break
					}
				}
			}
		case *ast.WhileStatement:
			mark(node.Token, SpacedKeyword)
		case *ast.ForStatement:
			mark(node.Token, SpacedKeyword)
			if len(node.Bindings) > 0 && node.Iterable != nil {
				last := node.Bindings[len(node.Bindings)-1].Token
				markIndex(nextTokenAtDepth(last, func(typ lexer.TokenType) bool { return typ == lexer.IN }), BinaryOperator)
			}
		case *ast.SwitchStatement:
			mark(node.Token, SpacedKeyword)
			markIndex(nextTokenAtDepth(node.Token, func(typ lexer.TokenType) bool { return typ == lexer.LBRACE }), ControlBodyOpen)
		case *ast.MatchExpression:
			if node.Token.Type == lexer.MATCH {
				mark(node.Token, SpacedKeyword)
				markIndex(nextTokenAtDepth(node.Token, func(typ lexer.TokenType) bool { return typ == lexer.LBRACE }), ControlBodyOpen)
			}
		}
	}
	visitProgramNodes(program, func(node any) {
		markSpacingRoles(node)
		switch node := node.(type) {
		case *ast.GenericParameter:
			for index, operator := range node.ConstraintOperators {
				// A separator whose right-hand constraint is recovered is part of an
				// uncertain region and deliberately receives no formatting role.
				if index+1 >= len(node.Constraints) || node.Constraints[index+1] == nil ||
					node.Constraints[index+1].Invalid {
					continue
				}
				mark(operator, GenericConstraintConjunction)
			}
		case *ast.Identifier:
			for _, separator := range node.ForeignSeparators {
				mark(separator, ForeignQualifierSeparator)
			}
		case *ast.TypeReference:
			for _, separator := range node.ForeignSeparators {
				mark(separator, ForeignQualifierSeparator)
			}
			// rules/types/types.md "Safe references" and "Function types";
			// rules/tooling/formatter.md §3 and §16(13): `ref T`, `ref mut T`,
			// and `fn(Params) Return` use one space between their words.
			if !node.Invalid && node.Ref && node.Token.Type == lexer.REF {
				mark(node.Token, SpacedKeyword)
				if node.MutableRef {
					markIndex(nextTokenAtDepth(node.Token, func(typ lexer.TokenType) bool { return true }), SpacedKeyword)
				}
			}
			if !node.Invalid && node.FunctionReturnType != nil && !node.FunctionReturnType.Invalid {
				mark(typeReferenceFirstToken(node.FunctionReturnType), SpacedBefore)
			}
			if !node.Invalid && (len(node.TypeArgs) > 0 || len(node.ConstArgs) > 0) &&
				node.TypeArgumentOpen.Type == lexer.LBRACKET && node.TypeArgumentClose.Type == lexer.RBRACKET {
				mark(node.TypeArgumentOpen, TypeArgumentListOpen)
				mark(node.TypeArgumentClose, TypeArgumentListClose)
			}
		case *ast.InfixExpression:
			if node.Operator == "x" && node.Left != nil && node.Right != nil {
				mark(node.Token, ContextualMatrixOperator)
			}
		case *ast.AssignmentStatement:
			if node.PostfixAlias.Type == lexer.INCREMENT || node.PostfixAlias.Type == lexer.DECREMENT {
				mark(node.PostfixAlias, PostfixMutationAlias)
			}
		case *ast.TestDeclaration:
			if !node.Invalid && node.Name != nil && node.Body != nil && node.Body.Token.Type == lexer.LBRACE {
				mark(node.Name.Token, TestDeclarationName)
			}
		case *ast.IfStatement:
			if isStateQueryCondition(node.Condition) &&
				node.Consequence != nil && node.Consequence.Token.Type == lexer.LBRACE {
				mark(node.Consequence.Token, AvailabilityBlockOpen)
			}
			if node.Consequence != nil && node.ConditionOpen.Type == lexer.LPAREN && node.ConditionClose.Type == lexer.RPAREN {
				mark(node.ConditionOpen, RedundantControlConditionDelimiter)
				mark(node.ConditionClose, RedundantControlConditionDelimiter)
			}
		case *ast.UnitExpression:
			switch node.Kind {
			case ast.UnitExpressionMultiply, ast.UnitExpressionDivide, ast.UnitExpressionPower:
				mark(node.Token, UnitExpressionCompactToken)
			case ast.UnitExpressionGroup:
				markGroup(node.Token, UnitExpressionCompactToken)
			}
		case *ast.FunctionDeclaration:
			// rules/tooling/formatter.md §16(1) and §16(4): `fn Name(...) Return`
			// keeps one space after `fn` and before the return type.
			if node.Name != nil && node.Token.Type == lexer.FN {
				mark(node.Token, SpacedKeyword)
			}
			// `extern "ABI" fn name(...)`: the extern keyword, ABI string, and
			// fn keyword are each followed by one space.
			if node.Name != nil && node.Token.Type == lexer.EXTERN {
				any := func(lexer.TokenType) bool { return true }
				mark(node.Token, SpacedKeyword)
				abi := nextTokenAtDepth(node.Token, any)
				if abi >= 0 && d.Elements[abi].Token.Type == lexer.STRING {
					markIndex(abi, SpacedKeyword)
					if fnIndex := nextTokenAtDepth(d.Elements[abi].Token, any); fnIndex >= 0 && d.Elements[fnIndex].Token.Type == lexer.FN {
						markIndex(fnIndex, SpacedKeyword)
					}
				}
			}
			if node.ReturnType != nil && !node.ReturnType.Invalid {
				mark(typeReferenceFirstToken(node.ReturnType), SpacedBefore)
			}
			markCallableParameters(node.ParameterOpen, node.Parameters)
			markAttachedAttributes(node.Attributes, node.Token)
		case *ast.TypeDeclStatement:
			markAttachedAttributes(node.Attributes, node.Token)
		case *ast.EnumDeclaration:
			markAttachedAttributes(node.Attributes, node.Token)
		case *ast.LambdaExpression:
			if node.CaptureOpen.Type == lexer.LPAREN {
				markGroupPair(node.CaptureOpen, LambdaCaptureListOpen, LambdaCaptureListClose)
			}
			markCallableParameters(node.ParameterOpen, node.Parameters)
		case *ast.BlockStatement:
			if node.Token.Type == lexer.LBRACE {
				markGroupPair(node.Token, ExecutableBlockOpen, ExecutableBlockClose)
			}
		case *ast.InitDeclaration:
			markCallableParameters(node.ParameterOpen, node.Parameters)
		case *ast.LetGroupStatement:
			for _, declaration := range node.Lets[1:] {
				if declaration != nil && declaration.Name != nil {
					markPreviousToken(declaration.Name.Token, lexer.COMMA, DeclarationGroupSeparator)
				}
			}
		case *ast.MatchPattern:
			switch {
			case node.Kind == ast.MatchPatternVariant && node.Binding != nil:
				markNextGroup(node.NameToken, lexer.LPAREN, MatchPatternPayloadOpen, MatchPatternPayloadClose)
			case node.Kind == ast.MatchPatternFields:
				markNextGroup(node.NameToken, lexer.LBRACE, MatchPatternFieldsOpen, MatchPatternFieldsClose)
				for index, field := range node.Fields {
					if field == nil {
						continue
					}
					if index > 0 {
						markPreviousToken(field.Token, lexer.COMMA, MatchPatternFieldSeparator)
					}
					if field.Binding != nil && field.Binding.Token != field.Token {
						markPreviousToken(field.Binding.Token, lexer.COLON, MatchPatternFieldColon)
					}
				}
			}
		case *ast.MatchArm:
			mark(node.WhereToken, MatchGuardKeyword)
			mark(node.ArrowToken, HandlerArrow)
		case *ast.TryHandler:
			if node.Guard != nil {
				mark(node.GuardToken, MatchGuardKeyword)
			}
			mark(node.ArrowToken, HandlerArrow)
		case *ast.CallExpression:
			// rules/tooling/formatter.md — §11(2), §17(2) multiline argument lists.
			if node.ArgumentsOpen.Type == lexer.LPAREN {
				markGroupPair(node.ArgumentsOpen, CommaListOpen, CommaListClose)
			}
		case *ast.OkExpression:
			if node.ArgumentsOpen.Type == lexer.LPAREN {
				markGroupPair(node.ArgumentsOpen, CommaListOpen, CommaListClose)
			}
		case *ast.ErrExpression:
			if node.ArgumentsOpen.Type == lexer.LPAREN {
				markGroupPair(node.ArgumentsOpen, CommaListOpen, CommaListClose)
			}
		case *ast.NewExpression:
			if node.ArgumentsOpen.Type == lexer.LPAREN {
				markGroupPair(node.ArgumentsOpen, CommaListOpen, CommaListClose)
			}
		case *ast.ArrayLiteral:
			// rules/tooling/formatter.md — §11(2) multiline collection elements.
			if node.Token.Type == lexer.LBRACKET {
				markGroupPair(node.Token, CommaListOpen, CommaListClose)
			}
		case *ast.StructLiteral:
			if node.Open.Type == lexer.LBRACE {
				markGroupPair(node.Open, CommaListOpen, CommaListClose)
			}
		case *ast.SwitchCase:
			if node.Body == nil || node.ColonToken.Type != lexer.COLON {
				break
			}
			mark(node.ColonToken, SwitchCaseColon)
			if !node.Default && len(node.Items) > 0 && node.Token.Type == lexer.CASE {
				mark(node.Token, SpacedKeyword)
			}
			for index := 1; index < len(node.Items); index++ {
				markPreviousToken(switchCaseItemToken(node.Items[index]), lexer.COMMA, SwitchCaseSeparator)
			}
		case *ast.WhileStatement:
			if isStateQueryCondition(node.Condition) && node.Body != nil && node.Body.Token.Type == lexer.LBRACE {
				mark(node.Body.Token, AvailabilityBlockOpen)
			}
			if node.Body != nil && node.ConditionOpen.Type == lexer.LPAREN && node.ConditionClose.Type == lexer.RPAREN {
				mark(node.ConditionOpen, RedundantControlConditionDelimiter)
				mark(node.ConditionClose, RedundantControlConditionDelimiter)
			}
		case *ast.SwitchStatement:
			if node.Subject != nil && node.SubjectOpen.Type == lexer.LPAREN && node.SubjectClose.Type == lexer.RPAREN {
				mark(node.SubjectOpen, RedundantControlConditionDelimiter)
				mark(node.SubjectClose, RedundantControlConditionDelimiter)
			}
		case *ast.RangeExpression:
			if node.Token.Type == lexer.RANGE || node.Token.Type == lexer.RANGE_EXCLUSIVE {
				switch {
				case node.Start == nil && node.End != nil:
					mark(node.Token, RangeOperatorOpenStart)
				case node.End == nil && node.Start != nil:
					mark(node.Token, RangeOperatorOpenEnd)
				default:
					mark(node.Token, RangeOperator)
				}
			}
		case *ast.SliceExpression:
			if node.RangeToken.Type == lexer.RANGE || node.RangeToken.Type == lexer.RANGE_EXCLUSIVE {
				mark(node.RangeToken, RangeOperator)
			}
		case *ast.ForStatement:
			if node.Step != nil && node.StepToken.Type == lexer.IDENT && node.StepToken.Lexeme == "step" {
				mark(node.StepToken, RangeStepKeyword)
			}
		case *ast.StructField:
			if node.Type != nil && !node.Type.Invalid {
				markPreviousToken(node.Type.Token, lexer.COLON, StructFieldColon)
				mark(node.Type.Token, StructFieldTypeStart)
				if node.TagToken.Type == lexer.RAW_STRING {
					mark(node.TagToken, StructFieldTag)
				}
			}
		case *ast.EnumValue:
			if !node.Invalid && node.Initializer != nil && node.InitializerToken.Type == lexer.ASSIGN {
				mark(node.InitializerToken, EnumValueAssignment)
			}
		}
	})
}

func switchCaseItemToken(item ast.SwitchCaseItem) lexer.Token {
	switch item := item.(type) {
	case *ast.SwitchValueCase:
		return item.Token
	case *ast.SwitchRangeCase:
		return item.Token
	case *ast.SwitchRelationalCase:
		return item.Token
	default:
		return lexer.Token{}
	}
}

type concreteTokenKey struct {
	line   int
	column int
	typ    lexer.TokenType
}

func keyForToken(token lexer.Token) concreteTokenKey {
	return concreteTokenKey{line: token.Line, column: token.Column, typ: token.Type}
}

func appendUniqueRole(roles []Role, role Role) []Role {
	for _, candidate := range roles {
		if candidate == role {
			return roles
		}
	}
	return append(roles, role)
}

func visitProgramNodes(program *ast.Program, visitNode func(any)) {
	var visit func(reflect.Value)
	visit = func(value reflect.Value) {
		if !value.IsValid() {
			return
		}
		if value.Kind() == reflect.Interface {
			if !value.IsNil() {
				visit(value.Elem())
			}
			return
		}
		if value.Kind() == reflect.Pointer {
			if value.IsNil() {
				return
			}
			if value.CanInterface() {
				visitNode(value.Interface())
			}
			if value.Type().Elem().PkgPath() == "sec/internal/ast" {
				visit(value.Elem())
			}
			return
		}
		switch value.Kind() {
		case reflect.Struct:
			if value.Type().PkgPath() != "sec/internal/ast" {
				return
			}
			for index := 0; index < value.NumField(); index++ {
				visit(value.Field(index))
			}
		case reflect.Slice, reflect.Array:
			for index := 0; index < value.Len(); index++ {
				visit(value.Index(index))
			}
		}
	}
	visit(reflect.ValueOf(program))
}

// isStateQueryCondition reports a condition that ends in a contextual state
// word, whose following body brace must stay separated from it.
//
// Rules:
//   - rules/tooling/formatter.md — § 8(1) braces on the construct header line, § 18 "Control flow"
//   - rules/declarations/unions.md — §8 "`is` tests for union state and active variant"
//   - rules/memory/ownership.md — §21 "is available and is not available"
//   - rules/platform/ffi.md — §11 "is null"
func isStateQueryCondition(condition ast.Expression) bool {
	switch condition.(type) {
	case *ast.AvailabilityExpression, *ast.StateTestExpression, *ast.NullTestExpression:
		return true
	}
	return false
}

// isAssignmentOperatorToken reports the statement assignment operators.
//
// Rules:
//   - rules/foundations/operators.md — assignment is not part of expression precedence
func isAssignmentOperatorToken(typ lexer.TokenType) bool {
	switch typ {
	case lexer.ASSIGN, lexer.MOVE_ASSIGN, lexer.PLUS_ASSIGN, lexer.MINUS_ASSIGN,
		lexer.ASTERISK_ASSIGN, lexer.SLASH_ASSIGN, lexer.PERCENT_ASSIGN,
		lexer.BIT_AND_ASSIGN, lexer.BIT_OR_ASSIGN, lexer.BIT_XOR_ASSIGN,
		lexer.SHIFT_LEFT_ASSIGN, lexer.SHIFT_RIGHT_ASSIGN:
		return true
	}
	return false
}

// typeDeclarationContracts flattens a parsed sequential contract conjunction
// in source order.
//
// Rule: rules/foundations/grammar.md — "Type contracts" (sequential contracts).
func typeDeclarationContracts(contract ast.Contract) []ast.Contract {
	if contract == nil {
		return nil
	}
	if list, ok := contract.(*ast.ContractList); ok {
		return list.Contracts
	}
	return []ast.Contract{contract}
}

// contractSpacingToken returns the contract's leading word and whether an
// operand follows it on the contract's own grammar.
//
// Rule: rules/foundations/grammar.md — "Type contracts".
func contractSpacingToken(contract ast.Contract) (lexer.Token, bool) {
	switch contract := contract.(type) {
	case *ast.RangeContract:
		return contract.Token, contract.Min != nil
	case *ast.MembershipContract:
		return contract.Token, true
	case *ast.MarkerContract:
		return contract.Token, contract.Value != nil
	case *ast.RegexContract:
		return contract.Token, contract.Pattern != nil
	default:
		return lexer.Token{}, false
	}
}

// isSimpleNamedTypeDeclaration reports whether a type declaration is a single
// line named type over an existing type, without generics, aggregate bodies,
// variants, or implements clauses.
//
// Rules:
//   - rules/tooling/formatter.md — §7(6) "Adjacent simple named-type declarations"
//   - rules/types/types.md — named type declarations
func isSimpleNamedTypeDeclaration(node *ast.TypeDeclStatement) bool {
	return node.Name != nil && len(node.GenericParameters) == 0 && node.BaseType != nil && !node.BaseType.Invalid &&
		node.StructType == nil && node.RegisterType == nil && !node.Union && len(node.Variants) == 0 &&
		len(node.UnionVariants) == 0 && len(node.Implements) == 0 && node.AssignedType == nil
}

// typeReferenceFirstToken returns the first source token of a type reference.
// Postfix sequence types (`int[2]`, `byte[]`) retain their bracket as Token, so
// the first token is that of their element type.
//
// Rule: rules/foundations/grammar.md — TypeReference and TypeSuffixes.
func typeReferenceFirstToken(typ *ast.TypeReference) lexer.Token {
	for typ != nil && !typ.Ref && typ.Token.Type == lexer.LBRACKET && typ.ElementType != nil {
		typ = typ.ElementType
	}
	if typ == nil {
		return lexer.Token{}
	}
	return typ.Token
}
