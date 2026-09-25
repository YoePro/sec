package cst

import (
	"testing"

	"sec/internal/lexer"
	"sec/internal/parser"
)

// Parser-owned roles distinguish a generic constraint conjunction from the
// same lexical token in an ordinary expression without losing source bytes.
//
// Rules:
//   - rules/tooling/formatter.md — §5 "Required syntax representation"
//   - rules/declarations/generics.md — §12 "Multiple constraints"
func TestApplyProgramRolesMarksOnlyGenericConstraintConjunctions(t *testing.T) {
	source := "fn Save[T: First&Second](value: int, mask: int) void {\n    discard value&mask\n}\n"
	program := parser.New(lexer.New(source)).ParseProgram()
	document := Build(source, "roles.sec")
	document.ApplyProgramRoles(program)
	if document.Text() != source {
		t.Fatalf("role annotation changed source: %q", document.Text())
	}

	var conjunctions, ordinary int
	for _, element := range document.Elements {
		if element.Token.Type != lexer.BIT_AND {
			continue
		}
		if element.HasRole(GenericConstraintConjunction) {
			conjunctions++
		} else {
			ordinary++
		}
	}
	if conjunctions != 1 || ordinary != 1 {
		t.Fatalf("constraint conjunctions = %d, ordinary bitwise operators = %d; elements = %+v", conjunctions, ordinary, document.Elements)
	}
}

func TestApplyProgramRolesMarksOnlyContextualMatrixOperators(t *testing.T) {
	source := "fn Multiply(x: matrix[float32, 2, 2], left: matrix[float32, 2, 2], right: matrix[float32, 2, 2]) void {\n    discard left x right\n    discard x\n}\n"
	program := parser.New(lexer.New(source)).ParseProgram()
	document := Build(source, "matrix.sec")
	document.ApplyProgramRoles(program)

	var operators, identifiers int
	for _, element := range document.Elements {
		if element.Token.Lexeme != "x" {
			continue
		}
		if element.HasRole(ContextualMatrixOperator) {
			operators++
		} else {
			identifiers++
		}
	}
	if operators != 1 || identifiers != 2 {
		t.Fatalf("matrix operators = %d, ordinary x identifiers = %d; elements = %+v", operators, identifiers, document.Elements)
	}
}

func TestApplyProgramRolesMarksOnlyValidPostfixMutationAliases(t *testing.T) {
	source := "fn Update() void {\n    value++\n    value--\n    let old := value++\n}\n"
	program := parser.New(lexer.New(source)).ParseProgram()
	document := Build(source, "postfix.sec")
	document.ApplyProgramRoles(program)

	var aliases, invalidExpressions int
	for _, element := range document.Elements {
		if element.Token.Type != lexer.INCREMENT && element.Token.Type != lexer.DECREMENT {
			continue
		}
		if element.HasRole(PostfixMutationAlias) {
			aliases++
		} else {
			invalidExpressions++
		}
	}
	if aliases != 2 || invalidExpressions != 1 {
		t.Fatalf("postfix aliases = %d, invalid expression uses = %d; elements = %+v", aliases, invalidExpressions, document.Elements)
	}
}

func TestApplyProgramRolesScopesUnitMetadataNames(t *testing.T) {
	source := "unit Meter decimal physical\n\nimpl Meter {\n    long_name: \"Meter\"\n}\n\nimpl Ordinary {\n    long_name: \"ordinary\"\n}\n"
	program := parser.New(lexer.New(source)).ParseProgram()
	document := Build(source, "units.sec")
	document.ApplyProgramRoles(program)

	var unitMetadata, ordinaryMembers int
	for _, element := range document.Elements {
		if element.Text != "long_name" {
			continue
		}
		if element.HasRole(UnitMetadataName) {
			unitMetadata++
		} else {
			ordinaryMembers++
		}
	}
	if unitMetadata != 1 || ordinaryMembers != 1 {
		t.Fatalf("unit metadata names = %d, ordinary members = %d; elements = %+v", unitMetadata, ordinaryMembers, document.Elements)
	}
}

func TestApplyProgramRolesMarksOnlyTopLevelTestDeclarationNames(t *testing.T) {
	source := "test   \"keeps  spacing\"   {\n}\n\nfn test(name: string) void {\n    discard test(\"ordinary\")\n}\n"
	program := parser.New(lexer.New(source)).ParseProgram()
	document := Build(source, "source_test.sec")
	document.ApplyProgramRoles(program)

	var testNames, ordinaryStrings int
	for _, element := range document.Elements {
		if element.Token.Type != lexer.STRING {
			continue
		}
		if element.HasRole(TestDeclarationName) {
			testNames++
		} else {
			ordinaryStrings++
		}
	}
	if testNames != 1 || ordinaryStrings != 1 {
		t.Fatalf("test names = %d, ordinary strings = %d; elements = %+v", testNames, ordinaryStrings, document.Elements)
	}
}

func TestApplyProgramRolesMarksOnlyAvailabilityConditionBlocks(t *testing.T) {
	source := "fn Check(value: int, available: bool) void {\n    if value is available{\n    }\n    if value is not available {\n    }\n    if available{\n    }\n}\n"
	program := parser.New(lexer.New(source)).ParseProgram()
	document := Build(source, "availability.sec")
	document.ApplyProgramRoles(program)

	var availabilityBlocks, otherBlocks int
	for _, element := range document.Elements {
		if element.Token.Type != lexer.LBRACE {
			continue
		}
		if element.HasRole(AvailabilityBlockOpen) {
			availabilityBlocks++
		} else {
			otherBlocks++
		}
	}
	if availabilityBlocks != 2 || otherBlocks != 2 {
		t.Fatalf("availability blocks = %d, other blocks = %d; elements = %+v", availabilityBlocks, otherBlocks, document.Elements)
	}
}

// Structural unit operators and grouping delimiters receive compact-spacing
// roles only when the parser has accepted them inside a unit annotation. The
// same tokens in an ordinary expression retain their ordinary CST identity.
//
// Rules:
//   - rules/tooling/formatter.md — §5 "Required syntax representation"
//   - rules/tooling/formatter.md — §22(8) compact unit operators
//   - rules/types/units.md — "Structural unit expressions"
func TestApplyProgramRolesMarksOnlyStructuralUnitExpressionTokens(t *testing.T) {
	source := "type Flux decimal<( kg * m ) / ( s ^ 2 * A )>\n\nfn Calculate(left: int, right: int, scale: int) int {\n    return (left * right) / (scale ^ 2)\n}\n"
	program := parser.New(lexer.New(source)).ParseProgram()
	document := Build(source, "unit_expression.sec")
	document.ApplyProgramRoles(program)

	var unitTokens, ordinaryTokens int
	for _, element := range document.Elements {
		switch element.Token.Type {
		case lexer.LPAREN, lexer.RPAREN, lexer.ASTERISK, lexer.SLASH, lexer.BIT_XOR:
			if element.HasRole(UnitExpressionCompactToken) {
				unitTokens++
			} else {
				ordinaryTokens++
			}
		}
	}
	if unitTokens != 8 || ordinaryTokens == 0 {
		t.Fatalf("unit tokens = %d, ordinary tokens = %d; elements = %+v", unitTokens, ordinaryTokens, document.Elements)
	}
}

func TestApplyProgramRolesMarksCallableParameterListOpeners(t *testing.T) {
	source := "type Buffer struct {}\n\nimpl Buffer {\n    init (size: uint) {\n    }\n}\n\nfn Apply (value: int) int {\n    let transform := fn (item: int) int {\n        return item\n    }\n    return transform(value)\n}\n"
	program := parser.New(lexer.New(source)).ParseProgram()
	document := Build(source, "callables.sec")
	document.ApplyProgramRoles(program)

	var parameterLists, otherParentheses int
	for _, element := range document.Elements {
		if element.Token.Type != lexer.LPAREN {
			continue
		}
		if element.HasRole(CallableParameterListOpen) {
			parameterLists++
		} else {
			otherParentheses++
		}
	}
	if parameterLists != 3 || otherParentheses != 1 {
		t.Fatalf("parameter lists = %d, other parentheses = %d; elements = %+v", parameterLists, otherParentheses, document.Elements)
	}
}

func TestApplyProgramRolesMarksOnlyDeclarationGroupSeparators(t *testing.T) {
	source := "fn Build() void {\n    let first := Pair(1, 2), second := 3, third := 4\n}\n"
	program := parser.New(lexer.New(source)).ParseProgram()
	document := Build(source, "declaration_groups.sec")
	document.ApplyProgramRoles(program)

	var groupSeparators, expressionCommas int
	for _, element := range document.Elements {
		if element.Token.Type != lexer.COMMA {
			continue
		}
		if element.HasRole(DeclarationGroupSeparator) {
			groupSeparators++
		} else {
			expressionCommas++
		}
	}
	if groupSeparators != 2 || expressionCommas != 1 {
		t.Fatalf("group separators = %d, expression commas = %d; elements = %+v", groupSeparators, expressionCommas, document.Elements)
	}
}

func TestApplyProgramRolesMarksMatchFormattingTokens(t *testing.T) {
	source := "fn Test(value: Shape, ready: bool) int {\n    return match value {\n        Shape.Circle (ref mut circle)where ready=>1\n        Rectangle{width : w,height:ref h}=>2\n    }\n}\n"
	program := parser.New(lexer.New(source)).ParseProgram()
	document := Build(source, "match_formatting.sec")
	document.ApplyProgramRoles(program)

	counts := map[Role]int{}
	for _, element := range document.Elements {
		for _, role := range element.Roles {
			counts[role]++
		}
	}
	want := map[Role]int{
		MatchPatternPayloadOpen:    1,
		MatchPatternPayloadClose:   1,
		MatchPatternFieldsOpen:     1,
		MatchPatternFieldsClose:    1,
		MatchPatternFieldSeparator: 1,
		MatchPatternFieldColon:     2,
		MatchGuardKeyword:          1,
		HandlerArrow:               2,
	}
	for role, expected := range want {
		if counts[role] != expected {
			t.Fatalf("role %s count = %d, want %d; elements = %+v", role, counts[role], expected, document.Elements)
		}
	}
}

func TestApplyProgramRolesDoesNotMarkRecoveredMatchArrow(t *testing.T) {
	source := "fn Test(value: Shape) int {\n    return match value {\n        Shape.Left | Shape.Right=>1\n        _=>0\n    }\n}\n"
	program := parser.New(lexer.New(source)).ParseProgram()
	document := Build(source, "invalid_match_formatting.sec")
	document.ApplyProgramRoles(program)

	var formatted, recovered int
	for _, element := range document.Elements {
		if element.Token.Type != lexer.ARROW {
			continue
		}
		if element.HasRole(HandlerArrow) {
			formatted++
		} else {
			recovered++
		}
	}
	if formatted != 1 || recovered != 1 {
		t.Fatalf("formatted arrows = %d, recovered arrows = %d; elements = %+v", formatted, recovered, document.Elements)
	}
}

func TestApplyProgramRolesMarksOnlyValidSwitchHeaderPunctuation(t *testing.T) {
	source := "fn Test(value: int) void {\n    switch value {\n    case 1 , 2 :\n        return\n    case 3, :\n        return\n    default :\n        return\n    }\n}\n"
	program := parser.New(lexer.New(source)).ParseProgram()
	document := Build(source, "switch_formatting.sec")
	document.ApplyProgramRoles(program)

	var separators, colons int
	for _, element := range document.Elements {
		if element.HasRole(SwitchCaseSeparator) {
			separators++
		}
		if element.HasRole(SwitchCaseColon) {
			colons++
		}
	}
	if separators != 1 || colons != 2 {
		t.Fatalf("switch separators = %d, colons = %d; elements = %+v", separators, colons, document.Elements)
	}
}

func TestApplyProgramRolesMarksWholeControlConditionParentheses(t *testing.T) {
	source := "fn Test(ready: bool, fallback: bool, enabled: bool, value: int) void {\n    if (ready) {\n        return\n    }\n    while (ready) {\n        break\n    }\n    switch (value) {\n    default:\n        return\n    }\n    if (ready || fallback) && enabled {\n        return\n    }\n}\n"
	program := parser.New(lexer.New(source)).ParseProgram()
	document := Build(source, "control_parentheses.sec")
	document.ApplyProgramRoles(program)

	count := 0
	for _, element := range document.Elements {
		if element.HasRole(RedundantControlConditionDelimiter) {
			count++
		}
	}
	if count != 6 {
		t.Fatalf("control-condition delimiter count = %d, want 6; elements = %+v", count, document.Elements)
	}
}

// A separator before a recovered missing constraint belongs to the uncertain
// region and must not authorize a formatter rewrite.
func TestApplyProgramRolesDoesNotMarkRecoveredConstraintSeparator(t *testing.T) {
	source := "fn Broken[T: First & ]() void {}\n"
	program := parser.New(lexer.New(source)).ParseProgram()
	document := Build(source, "recovery.sec")
	document.ApplyProgramRoles(program)
	for _, element := range document.Elements {
		if element.HasRole(GenericConstraintConjunction) {
			t.Fatalf("recovered separator received a formatting role: %+v", element)
		}
	}
}
