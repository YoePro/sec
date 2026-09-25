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
	// MatchGuardKeyword marks the contextual where token of a parsed match arm.
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
	// AvailabilityBlockOpen marks the real opening brace owned by an if whose
	// condition is a compiler-known ownership availability query.
	AvailabilityBlockOpen Role = "availability-block-open"
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

	visitProgramNodes(program, func(node any) {
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
			if _, ok := node.Condition.(*ast.AvailabilityExpression); ok &&
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
			mark(node.ParameterOpen, CallableParameterListOpen)
		case *ast.LambdaExpression:
			mark(node.ParameterOpen, CallableParameterListOpen)
		case *ast.InitDeclaration:
			mark(node.ParameterOpen, CallableParameterListOpen)
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
			mark(node.ArrowToken, HandlerArrow)
		case *ast.SwitchCase:
			if node.Body == nil || node.ColonToken.Type != lexer.COLON {
				break
			}
			mark(node.ColonToken, SwitchCaseColon)
			for index := 1; index < len(node.Items); index++ {
				markPreviousToken(switchCaseItemToken(node.Items[index]), lexer.COMMA, SwitchCaseSeparator)
			}
		case *ast.WhileStatement:
			if node.Body != nil && node.ConditionOpen.Type == lexer.LPAREN && node.ConditionClose.Type == lexer.RPAREN {
				mark(node.ConditionOpen, RedundantControlConditionDelimiter)
				mark(node.ConditionClose, RedundantControlConditionDelimiter)
			}
		case *ast.SwitchStatement:
			if node.Subject != nil && node.SubjectOpen.Type == lexer.LPAREN && node.SubjectClose.Type == lexer.RPAREN {
				mark(node.SubjectOpen, RedundantControlConditionDelimiter)
				mark(node.SubjectClose, RedundantControlConditionDelimiter)
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
