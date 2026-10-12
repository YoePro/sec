package sema

import (
	"fmt"
	"strings"

	"sec/internal/ast"
	"sec/internal/diagnostics"
	"sec/internal/lexer"
)

// Contextual call arguments (MD-027). A bare None, an empty array literal []
// and the raw-pointer null sentinel have no standalone type. When every
// candidate agrees on the argument's type it is bound before inference
// (bindNullArgumentContexts); otherwise the argument is deferred: overload
// resolution considers all supplied arguments together, a deferred position
// accepts only a parameter of the literal's family, and the argument is typed
// from the single selected overload. Remaining viable overloads with
// different types for a deferred argument make the call ambiguous; neither
// declaration order nor the expected result type selects one.
//
// Rules:
//   - rules/types/types.md — "Contextual None and empty-literal arguments"
//   - rules/declarations/functions.md — "Overload resolution"
//   - rules/corrections/applied/md027-md033-md036-md054-contextual-literals-formatter-configuration-correction-20261012.md — § 2

// deferContextualArguments marks the contextual arguments that no binding
// typed yet and returns the function that clears the marks.
func (a *Analyzer) deferContextualArguments(arguments []ast.Expression) func() {
	if a.deferredContextualArguments == nil {
		a.deferredContextualArguments = map[ast.Expression]bool{}
	}
	var deferred []ast.Expression
	for _, argument := range arguments {
		if _, bound := a.expectedExpressionTypes[argument]; bound || a.deferredContextualArguments[argument] {
			continue
		}
		if a.isBareNoneArgument(argument) || isEmptyArrayLiteral(argument) || a.isNullSentinel(argument) {
			a.deferredContextualArguments[argument] = true
			deferred = append(deferred, argument)
		}
	}
	return func() {
		for _, argument := range deferred {
			delete(a.deferredContextualArguments, argument)
		}
	}
}

func (a *Analyzer) hasDeferredContextualArgument(arguments []ast.Expression) bool {
	for _, argument := range arguments {
		if a.deferredContextualArguments[argument] {
			return true
		}
	}
	return false
}

// contextualArgumentAccepts reports whether parameter belongs to the family
// a contextual argument can be typed as.
func (a *Analyzer) contextualArgumentAccepts(argument ast.Expression, parameter Type) bool {
	switch {
	case a.isBareNoneArgument(argument):
		return isOptionType(parameter)
	case isEmptyArrayLiteral(argument):
		return acceptsEmptyArrayLiteral(parameter)
	case a.isNullSentinel(argument):
		return parameter.Kind == RawPtrType
	}
	return false
}

// typeDeferredContextualArguments types every deferred argument from the
// selected overload's parameter.
func (a *Analyzer) typeDeferredContextualArguments(function Function, arguments []ast.Expression, types []Type) bool {
	for index, argument := range arguments {
		if !a.deferredContextualArguments[argument] {
			continue
		}
		parameter, ok := functionParameterForArgument(function, index)
		if !ok {
			return false
		}
		delete(a.deferredContextualArguments, argument)
		typ, _ := a.inferExpressionWithExpected(argument, parameter.Type)
		if typ.Kind == InvalidType {
			return false
		}
		types[index] = typ
	}
	return true
}

// reportAmbiguousContextualArguments reports a call whose remaining viable
// overloads give a deferred argument different types, naming the candidate
// parameter types.
func (a *Analyzer) reportAmbiguousContextualArguments(token lexer.Token, name string, best []overloadMatch, arguments []ast.Expression) bool {
	for index, argument := range arguments {
		if !a.deferredContextualArguments[argument] {
			continue
		}
		var candidates []string
		for _, match := range best {
			parameter, ok := functionParameterForArgument(match.Function, index)
			if !ok {
				continue
			}
			display := typeDisplayName(parameter.Type)
			duplicate := false
			for _, existing := range candidates {
				duplicate = duplicate || existing == display
			}
			if !duplicate {
				candidates = append(candidates, display)
			}
		}
		if len(candidates) < 2 {
			continue
		}
		spelled := argument.String()
		a.addErrorAtTokenWithMetadata(expressionToken(argument), diagnostics.AmbiguousContextualArgument,
			fmt.Sprintf("Give the argument an explicit type, for example with a typed binding `let value: %s := %s`, so exactly one overload of %s applies.", candidates[0], spelled, name),
			"ambiguous call to %s: %s could be %s", name, spelled, strings.Join(candidates, " or "))
		return true
	}
	return false
}

func isEmptyArrayLiteral(expr ast.Expression) bool {
	literal, ok := expr.(*ast.ArrayLiteral)
	return ok && len(literal.Elements) == 0
}

// acceptsEmptyArrayLiteral reports an array type an empty literal can
// initialize: a dynamic array or a fixed array of length zero.
func acceptsEmptyArrayLiteral(typ Type) bool {
	if typ.Kind != ArrayType || typ.Element == nil {
		return false
	}
	if length, fixed := exactFixedArrayLength(typ); fixed {
		return length.Sign() == 0
	}
	return true
}
