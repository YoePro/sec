package main

import (
	"reflect"
	"strings"

	"sec/internal/ast"
	"sec/internal/lexer"
	"sec/internal/parser"
	"sec/internal/sema"
)

// subjectCompletionKind is a value position whose expected type comes from a
// subject or from the other operand rather than from a declaration.
type subjectCompletionKind int

const (
	subjectCompletionNone subjectCompletionKind = iota
	// subjectCompletionCase is a `case` item of a subject switch.
	subjectCompletionCase
	// subjectCompletionMatchArm is the pattern at the start of a match arm.
	subjectCompletionMatchArm
	// subjectCompletionComparison is the right operand of `==` or `!=`.
	subjectCompletionComparison
)

// subjectCompletionPlaceholder stands in for the value being typed so the
// surrounding switch, match, or comparison parses.
const subjectCompletionPlaceholder = "__sec_completion"

// subjectCompletionKindAt classifies the non-member completion position from
// the text of the current line before the typed prefix.
func subjectCompletionKindAt(text string, prefixStart int) subjectCompletionKind {
	lineStart := strings.LastIndex(text[:prefixStart], "\n") + 1
	before := strings.TrimSpace(text[lineStart:prefixStart])
	switch {
	case before == "":
		if !strings.Contains(text[:lineStart], "match") {
			return subjectCompletionNone
		}
		return subjectCompletionMatchArm
	case before == "case" || (strings.HasPrefix(before, "case ") && strings.HasSuffix(before, ",") && !strings.Contains(before, ":")):
		return subjectCompletionCase
	case strings.HasSuffix(before, "==") || strings.HasSuffix(before, "!="):
		return subjectCompletionComparison
	}
	return subjectCompletionNone
}

// subjectCompletionParseText makes the position parse: the case or match-arm
// line in progress is blanked (keeping every offset), and a comparison gets a
// placeholder right operand plus an empty body when its if/while header is
// still open.
func subjectCompletionParseText(text string, prefixStart int, offset int, kind subjectCompletionKind) string {
	lineStart := strings.LastIndex(text[:prefixStart], "\n") + 1
	lineEnd := strings.IndexByte(text[offset:], '\n')
	if lineEnd < 0 {
		lineEnd = len(text)
	} else {
		lineEnd += offset
	}
	switch kind {
	case subjectCompletionCase, subjectCompletionMatchArm:
		return text[:lineStart] + strings.Repeat(" ", lineEnd-lineStart) + text[lineEnd:]
	case subjectCompletionComparison:
		rest := text[offset:lineEnd]
		suffix := ""
		header := strings.TrimSpace(text[lineStart:prefixStart])
		if (strings.HasPrefix(header, "if ") || strings.HasPrefix(header, "while ")) && !strings.Contains(rest, "{") {
			suffix = " {}"
		}
		return text[:prefixStart] + subjectCompletionPlaceholder + rest + suffix + text[lineEnd:]
	}
	return text
}

// subjectCompletionItems answers completion at a switch case, a match arm
// pattern, or the right operand of `==`/`!=` with only the values of the
// expected type: the subject's type, or the left operand's type.
//
//   - enum: the qualified members (`FileMode.Read`); in case and match
//     positions the members already covered by another case or arm are left
//     out;
//   - union, Option, and Result in a match arm: the bare variant patterns
//     (`Circle`, `Some`, `Ok`) not yet covered, and `_`;
//   - comparison and case positions also offer bool literals and visible
//     symbols and functions of the expected type.
//
// The boolean result reports whether the position was recognized; callers
// fall back to ordinary global completion otherwise.
//
// Rules:
//   - rules/tooling/lsp.md — "Completion" (expected type, decision 2026-10-03)
//   - rules/control-flow/flowcontrol_switch.md, flowcontrol_match.md — case and pattern forms
func subjectCompletionItems(uri string, text string, offset int, context completionContext, overlay sourceOverlay) ([]completionItem, bool) {
	prefixStart := offset - len(context.Prefix)
	if prefixStart < 0 || prefixStart > len(text) {
		return nil, false
	}
	kind := subjectCompletionKindAt(text, prefixStart)
	if kind == subjectCompletionNone {
		return nil, false
	}
	parseText := subjectCompletionParseText(text, prefixStart, offset, kind)
	l := lexer.New(parseText)
	if uri != "" {
		l = lexer.NewWithFile(parseText, pathFromURI(uri))
	}
	result := parser.New(l).Parse()
	if result.Program == nil || result.Fatal {
		return nil, false
	}
	prepareProgramForLSP(result.Program, pathFromURI(uri), overlay)
	analyzer := newLSPAnalyzer(uri, result.Program)
	analyzer.Analyze(result.Program)

	var expected sema.Type
	covered := map[string]bool{}
	switch kind {
	case subjectCompletionCase:
		statement := innermostSwitchAt(result.Program, parseText, offset)
		if statement == nil || statement.Subject == nil {
			return nil, false
		}
		subjectType, ok := subjectExpressionType(analyzer, statement.Subject)
		if !ok {
			return nil, false
		}
		expected = subjectType
		for _, switchCase := range statement.Cases {
			for _, item := range switchCase.Items {
				if value, ok := item.(*ast.SwitchValueCase); ok {
					covered[expressionSourceName(value.Value)] = true
				}
			}
		}
	case subjectCompletionMatchArm:
		match := innermostMatchArmPositionAt(result.Program, parseText, offset)
		if match == nil || match.Subject == nil {
			return nil, false
		}
		subjectType, ok := subjectExpressionType(analyzer, match.Subject)
		if !ok {
			return nil, false
		}
		expected = subjectType
		for _, arm := range match.Arms {
			if arm != nil && arm.Pattern != nil && arm.Guard == nil {
				covered[arm.Pattern.Name] = true
			}
		}
	case subjectCompletionComparison:
		left := comparisonLeftOperandAt(result.Program, parseText, prefixStart)
		if left == nil {
			return nil, false
		}
		leftType, ok := subjectExpressionType(analyzer, left)
		if !ok {
			return nil, false
		}
		expected = leftType
	}
	if expected.Kind == sema.InvalidType || expected.Kind == "" {
		return nil, false
	}

	items := []completionItem{}
	seen := map[string]bool{}
	add := func(item completionItem) {
		if item.Label == "" || seen[item.Label] || covered[item.Label] || !completionLabelMatches(item.Label, context.Prefix) {
			return
		}
		seen[item.Label] = true
		items = append(items, item)
	}
	owner := lspTypeName(expected)
	switch expected.Kind {
	case sema.EnumType:
		for _, member := range expected.EnumValues {
			add(completionItem{Label: owner + "." + member, Kind: 20, Detail: owner})
		}
	case sema.UnionType:
		if kind == subjectCompletionMatchArm {
			for _, variant := range expected.UnionVariants {
				add(completionItem{Label: variant.Name, Kind: 20, Detail: owner})
			}
		}
	case sema.ResultType:
		if kind == subjectCompletionMatchArm {
			add(completionItem{Label: "Ok", Kind: 20, Detail: owner})
			add(completionItem{Label: "Err", Kind: 20, Detail: owner})
		}
	}
	if kind == subjectCompletionMatchArm {
		if len(items) == 0 {
			return nil, false
		}
		add(completionItem{Label: "_", Kind: 14, Detail: "any remaining " + owner})
		sortCompletionItems(items)
		return items, true
	}
	returnContext := context
	returnContext.ExpectedType = &expected
	addReturnValueCompletionItems(text, analyzer, returnContext, add)
	sortCompletionItems(items)
	return items, true
}

// innermostSwitchAt returns the innermost switch whose body holds offset.
func innermostSwitchAt(program *ast.Program, text string, offset int) *ast.SwitchStatement {
	var best *ast.SwitchStatement
	bestStart := -1
	walkLSPNodes(program, func(node any) {
		statement, ok := node.(*ast.SwitchStatement)
		if !ok || statement == nil {
			return
		}
		start, end := bodyRangeAfter(text, statement.Token)
		if start >= 0 && start < offset && offset <= end && start > bestStart {
			best, bestStart = statement, start
		}
	})
	return best
}

// innermostMatchArmPositionAt returns the innermost match whose body holds
// offset outside every arm's own block body, so the cursor starts a pattern.
func innermostMatchArmPositionAt(program *ast.Program, text string, offset int) *ast.MatchExpression {
	var best *ast.MatchExpression
	bestStart := -1
	walkLSPNodes(program, func(node any) {
		match, ok := node.(*ast.MatchExpression)
		if !ok || match == nil || match.OptionAbsenceTest {
			return
		}
		start, end := bodyRangeAfter(text, match.Token)
		if start < 0 || start >= offset || offset > end || start <= bestStart {
			return
		}
		for _, arm := range match.Arms {
			if arm == nil || arm.BlockBody == nil {
				continue
			}
			armStart := textPositionOffset(text, arm.BlockBody.Token.Line, arm.BlockBody.Token.Column)
			if armEnd := matchingBraceOffset(text, armStart); armStart >= 0 && armStart < offset && offset <= armEnd {
				return
			}
		}
		best, bestStart = match, start
	})
	return best
}

// comparisonLeftOperandAt returns the left operand of the `==` or `!=`
// whose right operand begins at rightStart.
func comparisonLeftOperandAt(program *ast.Program, text string, rightStart int) ast.Expression {
	var left ast.Expression
	walkLSPNodes(program, func(node any) {
		infix, ok := node.(*ast.InfixExpression)
		if !ok || infix == nil || (infix.Operator != "==" && infix.Operator != "!=") || infix.Right == nil {
			return
		}
		token := leftmostExpressionToken(infix.Right)
		if textPositionOffset(text, token.Line, token.Column) == rightStart {
			left = infix.Left
		}
	})
	return left
}

// bodyRangeAfter returns the offsets of the first `{` after token and its
// matching `}`.
func bodyRangeAfter(text string, token lexer.Token) (int, int) {
	from := textPositionOffset(text, token.Line, token.Column)
	if from < 0 {
		return -1, -1
	}
	open := strings.IndexByte(text[from:], '{')
	if open < 0 {
		return -1, -1
	}
	open += from
	return open, matchingBraceOffset(text, open)
}

// leftmostExpressionToken returns the first source token of expr.
func leftmostExpressionToken(expr ast.Expression) lexer.Token {
	switch node := expr.(type) {
	case *ast.MemberExpression:
		if node.Object != nil {
			return leftmostExpressionToken(node.Object)
		}
		return node.Token
	case *ast.CallExpression:
		if node.Function != nil {
			return leftmostExpressionToken(node.Function)
		}
		return node.Token
	case *ast.IndexExpression:
		if node.Left != nil {
			return leftmostExpressionToken(node.Left)
		}
		return node.Token
	case *ast.InfixExpression:
		if node.Left != nil {
			return leftmostExpressionToken(node.Left)
		}
		return node.Token
	case *ast.Identifier:
		return node.Token
	}
	if value := reflect.ValueOf(expr); value.IsValid() && value.Kind() == reflect.Pointer && !value.IsNil() {
		if field := value.Elem().FieldByName("Token"); field.IsValid() {
			if token, ok := field.Interface().(lexer.Token); ok {
				return token
			}
		}
	}
	return lexer.Token{}
}

// expressionSourceName renders a covered case value as its qualified name.
func expressionSourceName(expr ast.Expression) string {
	switch node := expr.(type) {
	case *ast.Identifier:
		return node.Value
	case *ast.MemberExpression:
		if node.Property != nil {
			return expressionSourceName(node.Object) + "." + node.Property.Value
		}
	}
	return ""
}

// walkLSPNodes visits every AST node reachable from root.
func walkLSPNodes(root any, visit func(node any)) {
	var walk func(reflect.Value)
	walk = func(value reflect.Value) {
		if !value.IsValid() {
			return
		}
		switch value.Kind() {
		case reflect.Interface, reflect.Pointer:
			if value.IsNil() {
				return
			}
			if value.Kind() == reflect.Pointer && value.CanInterface() {
				visit(value.Interface())
			}
			walk(value.Elem())
		case reflect.Struct:
			if value.Type().PkgPath() != "sec/internal/ast" {
				return
			}
			for i := 0; i < value.NumField(); i++ {
				walk(value.Field(i))
			}
		case reflect.Slice, reflect.Array:
			for i := 0; i < value.Len(); i++ {
				walk(value.Index(i))
			}
		}
	}
	walk(reflect.ValueOf(root))
}

// subjectExpressionType is the Sema type of a subject or operand, falling back
// to the resolved binding of a plain identifier, whose expression type is not
// retained outside its analyzed scope.
func subjectExpressionType(analyzer *sema.Analyzer, expr ast.Expression) (sema.Type, bool) {
	if typ, ok := analyzer.TypeOf(expr); ok {
		return typ, true
	}
	if identifier, ok := expr.(*ast.Identifier); ok && identifier != nil {
		token := identifier.Token
		if binding, ok := analyzer.ResolvedBindingAt(token.File, token.Line, token.Column); ok && binding.Type.Kind != sema.InvalidType && binding.Type.Kind != "" {
			return binding.Type, true
		}
	}
	return sema.Type{}, false
}
