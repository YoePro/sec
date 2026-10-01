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

// Complete callable parameters expose their declaration colon and first type
// token without assigning those roles to call arguments or recovered syntax.
//
// Rules:
//   - rules/tooling/formatter.md — §5 "Required syntax representation"
//   - rules/tooling/formatter.md — §16(2–3) multiline parameter alignment
func TestApplyProgramRolesMarksCallableParameterAlignmentAnchors(t *testing.T) {
	source := "fn Send(\n    value : int,\n    values: ...string,\n) void {\n    let item := Pair { value: 1 }\n}\n"
	program := parser.New(lexer.New(source)).ParseProgram()
	document := Build(source, "parameters.sec")
	document.ApplyProgramRoles(program)

	var colons int
	typeStarts := []string{}
	for _, element := range document.Elements {
		if element.HasRole(CallableParameterColon) {
			colons++
		}
		if element.HasRole(CallableParameterTypeStart) {
			typeStarts = append(typeStarts, element.Text)
		}
	}
	if colons != 2 || len(typeStarts) != 2 || typeStarts[0] != "int" || typeStarts[1] != "..." {
		t.Fatalf("parameter colons = %d, type starts = %v; elements = %+v", colons, typeStarts, document.Elements)
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

// Attached attribute layout is projected only from declarations to which the
// parser successfully attached a known attribute.
//
// Rules:
//   - rules/tooling/formatter.md — §5 "Required syntax representation"
//   - rules/tooling/formatter.md — §16(14–17) "attributes"
func TestApplyProgramRolesMarksAttachedAttributeBoundaries(t *testing.T) {
	source := "@noCopy type Session struct {}\n\n@noPanic fn Safe() void {\n}\n"
	program := parser.New(lexer.New(source)).ParseProgram()
	document := Build(source, "attributes.sec")
	document.ApplyProgramRoles(program)

	var attributeEnds, declarationStarts int
	for _, element := range document.Elements {
		if element.HasRole(AttachedAttributeEnd) {
			attributeEnds++
		}
		if element.HasRole(AttributedDeclarationStart) {
			declarationStarts++
		}
	}
	if attributeEnds != 2 || declarationStarts != 2 {
		t.Fatalf("attribute ends = %d, declaration starts = %d; elements = %+v", attributeEnds, declarationStarts, document.Elements)
	}
}

// Explicit enum assignments receive formatter roles only from complete parsed
// members; implicit members and legacy colon spellings remain outside them.
//
// Rules:
//   - rules/tooling/formatter.md — §5 "Required syntax representation"
//   - rules/tooling/formatter.md — §15(4) enum assignment alignment
func TestApplyProgramRolesMarksExplicitEnumAssignments(t *testing.T) {
	source := "enum Status uint8 {\nA = 1\nLongName = 2\nImplicit\nLegacy: 3\n}\n"
	program := parser.New(lexer.New(source)).ParseProgram()
	document := Build(source, "enum_assignments.sec")
	document.ApplyProgramRoles(program)

	assignments := 0
	for _, element := range document.Elements {
		if element.HasRole(EnumValueAssignment) {
			assignments++
			if element.Token.Type != lexer.ASSIGN {
				t.Fatalf("enum assignment role attached to %+v", element.Token)
			}
		}
	}
	if assignments != 2 {
		t.Fatalf("enum assignment roles = %d, want 2; elements = %+v", assignments, document.Elements)
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

	var parameterLists, parameterListClosers, otherParentheses int
	for _, element := range document.Elements {
		switch element.Token.Type {
		case lexer.LPAREN:
			if element.HasRole(CallableParameterListOpen) {
				parameterLists++
			} else {
				otherParentheses++
			}
		case lexer.RPAREN:
			if element.HasRole(CallableParameterListClose) {
				parameterListClosers++
			}
		}
	}
	if parameterLists != 3 || parameterListClosers != 3 || otherParentheses != 1 {
		t.Fatalf("parameter lists = %d, closers = %d, other parentheses = %d; elements = %+v", parameterLists, parameterListClosers, otherParentheses, document.Elements)
	}
}

// Generic argument brackets are parser-owned roles while fixed arrays and
// expression indexing retain ordinary bracket identity.
//
// Rules:
//   - rules/tooling/formatter.md — §5 "Required syntax representation"
//   - rules/tooling/formatter.md — §15(6–8) multiline generic lists
func TestApplyProgramRolesMarksOnlyTypeArgumentListDelimiters(t *testing.T) {
	source := "fn Use(value: Result[Value, Error], array: Value[4]) void {\n    discard array[0]\n}\n"
	program := parser.New(lexer.New(source)).ParseProgram()
	document := Build(source, "generic_arguments.sec")
	document.ApplyProgramRoles(program)

	var genericOpeners, genericClosers, otherOpeners, otherClosers int
	for _, element := range document.Elements {
		switch element.Token.Type {
		case lexer.LBRACKET:
			if element.HasRole(TypeArgumentListOpen) {
				genericOpeners++
			} else {
				otherOpeners++
			}
		case lexer.RBRACKET:
			if element.HasRole(TypeArgumentListClose) {
				genericClosers++
			} else {
				otherClosers++
			}
		}
	}
	if genericOpeners != 1 || genericClosers != 1 || otherOpeners != 2 || otherClosers != 2 {
		t.Fatalf("generic brackets = %d/%d, ordinary brackets = %d/%d; elements = %+v", genericOpeners, genericClosers, otherOpeners, otherClosers, document.Elements)
	}
}

// Explicit capture delimiters are parser-owned so formatting can distinguish
// them from ordinary calls and parenthesized expressions.
//
// Rules:
//   - rules/tooling/formatter.md — §5 "Required syntax representation"
//   - rules/tooling/formatter.md — §16(12) multiline capture lists
func TestApplyProgramRolesMarksLambdaCaptureListDelimiters(t *testing.T) {
	source := "fn Build() void {\n    let closure := capture(\n        first,\n        <-second,\n    ) fn() int {\n        return first + second\n    }\n}\n"
	program := parser.New(lexer.New(source)).ParseProgram()
	document := Build(source, "lambda_capture.sec")
	document.ApplyProgramRoles(program)

	var captureOpeners, captureClosers int
	for _, element := range document.Elements {
		if element.HasRole(LambdaCaptureListOpen) {
			captureOpeners++
		}
		if element.HasRole(LambdaCaptureListClose) {
			captureClosers++
		}
	}
	if captureOpeners != 1 || captureClosers != 1 {
		t.Fatalf("capture openers = %d, closers = %d; elements = %+v", captureOpeners, captureClosers, document.Elements)
	}
}

// Executable braces are distinguished from structural declaration and value
// literal braces before the formatter makes multiline layout decisions.
//
// Rules:
//   - rules/tooling/formatter.md — §5 "Required syntax representation"
//   - rules/tooling/formatter.md — §8(3–8) brace placement
func TestApplyProgramRolesMarksOnlyExecutableBlockDelimiters(t *testing.T) {
	source := "type Pair struct { Left: int, Right: int }\n\nfn Build() Pair { return Pair { Left: 1, Right: 2 } }\n"
	program := parser.New(lexer.New(source)).ParseProgram()
	document := Build(source, "executable_blocks.sec")
	document.ApplyProgramRoles(program)

	var executableOpeners, executableClosers, otherOpeners int
	for _, element := range document.Elements {
		switch element.Token.Type {
		case lexer.LBRACE:
			if element.HasRole(ExecutableBlockOpen) {
				executableOpeners++
			} else {
				otherOpeners++
			}
		case lexer.RBRACE:
			if element.HasRole(ExecutableBlockClose) {
				executableClosers++
			}
		}
	}
	if executableOpeners != 1 || executableClosers != 1 || otherOpeners != 2 {
		t.Fatalf("executable openers = %d, closers = %d, other openers = %d; elements = %+v", executableOpeners, executableClosers, otherOpeners, document.Elements)
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

// Range punctuation and contextual step receive formatting roles only after
// the parser has established their range, slice, and for-header meanings.
//
// Rules:
//   - rules/tooling/formatter.md — §23(5–8) ranges and step
func TestApplyProgramRolesMarksRangeAndStepTokens(t *testing.T) {
	source := "fn Visit(values: int[], step: int) void {\n    for index in 0 ..< values.Len step 2 {\n        let window := values[index .. index + 2]\n        let ordinary := step\n    }\n}\n"
	program := parser.New(lexer.New(source)).ParseProgram()
	document := Build(source, "ranges.sec")
	document.ApplyProgramRoles(program)

	var rangeOperators, stepKeywords, ordinaryStepIdentifiers int
	for _, element := range document.Elements {
		if element.HasRole(RangeOperator) {
			rangeOperators++
		}
		if element.Token.Type == lexer.IDENT && element.Token.Lexeme == "step" {
			if element.HasRole(RangeStepKeyword) {
				stepKeywords++
			} else {
				ordinaryStepIdentifiers++
			}
		}
	}
	if rangeOperators != 2 || stepKeywords != 1 || ordinaryStepIdentifiers != 2 {
		t.Fatalf("range operators = %d, step keywords = %d, ordinary step identifiers = %d; elements = %+v", rangeOperators, stepKeywords, ordinaryStepIdentifiers, document.Elements)
	}
}

// Struct-field alignment anchors come only from complete parser-owned fields;
// the same colon and type tokens in local declarations remain unclassified.
//
// Rules:
//   - rules/tooling/formatter.md — §9(1–2) syntactic alignment anchors
//   - rules/tooling/formatter.md — §9(5) struct-field alignment
func TestApplyProgramRolesMarksStructFieldAlignmentAnchors(t *testing.T) {
	source := "type Endpoint struct {\n    Host: string `json:\"host\"`,\n    Timeout: Duration `json:\"timeout\"`,\n}\n\nfn Local() void {\n    let value: string := \"\"\n}\n"
	program := parser.New(lexer.New(source)).ParseProgram()
	document := Build(source, "struct_fields.sec")
	document.ApplyProgramRoles(program)

	var fieldColons, fieldTypes, fieldTags, ordinaryColons int
	for _, element := range document.Elements {
		if element.Token.Type == lexer.COLON {
			if element.HasRole(StructFieldColon) {
				fieldColons++
			} else {
				ordinaryColons++
			}
		}
		if element.HasRole(StructFieldTypeStart) {
			fieldTypes++
		}
		if element.HasRole(StructFieldTag) {
			fieldTags++
		}
	}
	if fieldColons != 2 || fieldTypes != 2 || fieldTags != 2 || ordinaryColons != 1 {
		t.Fatalf("field colons = %d, field types = %d, field tags = %d, ordinary colons = %d; elements = %+v", fieldColons, fieldTypes, fieldTags, ordinaryColons, document.Elements)
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
