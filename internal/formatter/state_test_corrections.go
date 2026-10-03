package formatter

import (
	"reflect"
	"sort"
	"strings"

	"sec/internal/ast"
	"sec/internal/lexer"
	"sec/internal/parser"
)

// fixStateTestBadPractice applies the explicit sloppy corrections of MD-006:
// `(state is Idle) == true` becomes `state is Idle`, and `!(state is Idle)`
// becomes `state is not Idle`. A rewrite happens only on a parse without
// errors and only where the result keeps the same grouping: the rewritten
// test sits at the non-chainable equality level, so its parent must bind more
// loosely (a logical operator, a statement, an argument, or a grouping).
// Option binding tests and the null test are never rewritten, because the
// binding records explicit intent and null has no negated form.
//
// This is an opt-in Language Correction and must never run during ordinary
// formatting.
//
// Rules:
//   - rules/corrections/applied/missing-decisions-md001-md009-correction-20261003.md — §§ 7.8–7.9, 7.13–7.16, 7.25
//   - rules/tooling/formatter.md — § 26 "Language Corrections model", § 27 "Correction catalogue"
func fixStateTestBadPractice(text string) string {
	p := parser.New(lexer.New(text))
	program := p.ParseProgram()
	if len(p.Errors()) > 0 || program == nil {
		return text
	}
	replacements := []formatterReplacement{}
	var visit func(value reflect.Value, parent ast.Node)
	visit = func(value reflect.Value, parent ast.Node) {
		if !value.IsValid() {
			return
		}
		switch value.Kind() {
		case reflect.Interface:
			if !value.IsNil() {
				visit(value.Elem(), parent)
			}
			return
		case reflect.Pointer:
			if value.IsNil() || value.Type().Elem().PkgPath() != "sec/internal/ast" {
				return
			}
			node, _ := value.Interface().(ast.Node)
			if node != nil {
				if replacement, ok := stateTestCorrection(text, node, parent); ok {
					replacements = append(replacements, replacement)
					return
				}
				parent = node
			}
			visit(value.Elem(), parent)
			return
		case reflect.Struct:
			for index := 0; index < value.NumField(); index++ {
				if value.Type().Field(index).IsExported() {
					visit(value.Field(index), parent)
				}
			}
		case reflect.Slice, reflect.Array:
			for index := 0; index < value.Len(); index++ {
				visit(value.Index(index), parent)
			}
		}
	}
	for _, statement := range program.Statements {
		visit(reflect.ValueOf(statement), nil)
	}
	sort.Slice(replacements, func(i, j int) bool { return replacements[i].start > replacements[j].start })
	for _, replacement := range replacements {
		text = text[:replacement.start] + replacement.text + text[replacement.end:]
	}
	return text
}

// stateTestCorrection returns the rewrite of one bad-practice state-test form
// rooted at node, when its parent preserves the grouping.
func stateTestCorrection(text string, node ast.Node, parent ast.Node) (formatterReplacement, bool) {
	if !looserThanEquality(parent) {
		return formatterReplacement{}, false
	}
	switch node := node.(type) {
	case *ast.PrefixExpression:
		if node.Operator != "!" {
			return formatterReplacement{}, false
		}
		body, ok := groupedStateTestBody(text, node.Right, false)
		if !ok {
			return formatterReplacement{}, false
		}
		isToken, _, group, _ := ast.StateTestOf(node.Right)
		subject := strings.TrimSpace(text[group.Open.ByteEnd:isToken.ByteStart])
		designator := strings.TrimSpace(text[isToken.ByteEnd:group.Close.ByteStart])
		if subject == "" || designator == "" || body == "" {
			return formatterReplacement{}, false
		}
		return formatterReplacement{start: node.Token.ByteStart, end: group.Close.ByteEnd, text: subject + " is not " + designator}, true
	case *ast.InfixExpression:
		if node.Operator != "==" {
			return formatterReplacement{}, false
		}
		test, literal := node.Left, node.Right
		if _, _, _, isTest := ast.StateTestOf(test); !isTest {
			test, literal = node.Right, node.Left
		}
		boolean, isTrue := literal.(*ast.BooleanLiteral)
		if !isTrue || !boolean.Value {
			return formatterReplacement{}, false
		}
		body, ok := groupedStateTestBody(text, test, true)
		if !ok {
			return formatterReplacement{}, false
		}
		_, _, group, _ := ast.StateTestOf(test)
		start, end := group.Open.ByteStart, boolean.Token.ByteEnd
		if boolean.Token.ByteStart < group.Open.ByteStart {
			start, end = boolean.Token.ByteStart, group.Close.ByteEnd
		}
		return formatterReplacement{start: start, end: end, text: body}, true
	}
	return formatterReplacement{}, false
}

// groupedStateTestBody returns the source between the parentheses of a
// grouped, rewritable state test.
func groupedStateTestBody(text string, expr ast.Expression, allowNegated bool) (string, bool) {
	switch expr.(type) {
	case *ast.NullTestExpression, *ast.OptionBindingTestExpression:
		return "", false
	}
	_, negated, group, isTest := ast.StateTestOf(expr)
	if !isTest || group == nil || negated && !allowNegated {
		return "", false
	}
	if group.Open.ByteEnd > group.Close.ByteStart || group.Close.ByteEnd > len(text) {
		return "", false
	}
	body := text[group.Open.ByteEnd:group.Close.ByteStart]
	if strings.Contains(body, "//") || strings.Contains(body, "/*") || strings.Contains(body, "\n") {
		return "", false
	}
	return strings.TrimSpace(body), true
}

// looserThanEquality reports a parent position in which an ungrouped state
// test keeps its meaning: anything except an operand of another binary,
// prefix, member, index, or conversion form.
func looserThanEquality(parent ast.Node) bool {
	switch parent := parent.(type) {
	case *ast.InfixExpression:
		return parent.Operator == "&&" || parent.Operator == "||"
	case *ast.PrefixExpression, *ast.MemberExpression, *ast.IndexExpression, *ast.ConversionExpression,
		*ast.StateTestExpression, *ast.AvailabilityExpression, *ast.NullTestExpression, *ast.OptionBindingTestExpression:
		return false
	case *ast.MatchExpression:
		return !parent.OptionAbsenceTest
	}
	return true
}
