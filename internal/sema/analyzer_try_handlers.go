package sema

import (
	"sec/internal/ast"
	"sec/internal/lexer"
)

// analyzeTryHandlerGuard checks a handler's `where` condition in the arm scope
// where the pattern binding is already visible. The guard runs before the
// handler is selected, so it may inspect but must not consume the binding.
//
// Rules:
//   - rules/errors/errorhandling.md — §19 "Guards", §20 "Handler ownership and guards"
//   - rules/errors/errorhandling.md — §22 "Handler failures are outside the protected set"
func (a *Analyzer) analyzeTryHandlerGuard(handler *ast.TryHandler, bindingName string) {
	guardType, _ := a.inferExpression(handler.Guard)
	if guardType.Kind != InvalidType && guardType.Kind != BoolType {
		a.addErrorAtToken(expressionToken(handler.Guard), "try handler guard must be bool, got %s", typeDisplayName(guardType))
	}
	if bindingName == "" {
		return
	}
	if token, moved := a.moved[bindingName]; moved {
		a.addErrorAtToken(token, "try handler guard must not consume %s before the handler is selected; inspect or borrow it in the guard and move it in the handler body", bindingName)
	}
}

// tryHandlerBlockValue returns the final expression statement of a handler
// block, which provides the recovery value in result position.
func tryHandlerBlockValue(block *ast.BlockStatement) (ast.Expression, bool) {
	if block == nil || len(block.Statements) == 0 {
		return nil, false
	}
	last := block.Statements[len(block.Statements)-1]
	statement, ok := last.(*ast.ExpressionStatement)
	if !ok || statement.Expression == nil {
		return nil, false
	}
	return statement.Expression, true
}

// analyzeTryHandlerBlockValue analyzes a handler block whose final expression
// is its recovery value: the preceding statements run in the handler scope and
// the final expression must be assignable to the try success type.
//
// Rules:
//   - rules/errors/errorhandling.md — §21 "Recovery values", §21.1 "Block handler result position"
func (a *Analyzer) analyzeTryHandlerBlockValue(handler *ast.TryHandler, value ast.Expression, successType Type) ResolvedTryHandlerFlow {
	block := handler.BlockBody
	prefix := &ast.BlockStatement{Token: block.Token, Tokens: block.Tokens, Statements: block.Statements[:len(block.Statements)-1]}
	a.analyzeBlockStatements(prefix)
	if len(prefix.Statements) > 0 && !blockCanFallThrough(prefix) {
		a.addErrorAtToken(expressionToken(value), "unreachable recovery value; the handler block already leaves control flow")
		if blockDefinitelyReturns(prefix) {
			return TryHandlerReturns
		}
		return TryHandlerTerminates
	}
	valueType, _ := a.inferExpressionWithExpected(value, successType)
	if valueType.Kind == InvalidType {
		return TryHandlerInvalidFlow
	}
	if !canInitialize(successType, valueType, value) {
		a.addErrorAtToken(expressionToken(value), "try handler must produce %s, got %s", typeDisplayName(successType), typeDisplayName(valueType))
		return TryHandlerInvalidFlow
	}
	return TryHandlerProducesValue
}

// rejectCarrierFamilyMismatch explains a match pattern from the wrong carrier
// family: Ok/Err against an Option subject or Some/None against a Result
// subject. Without it, None would bind as an ordinary catch-all name.
//
// Rules:
//   - rules/errors/errorhandling.md — §27.1 "Patterns are checked against the resolved subject type"
//   - rules/errors/errorhandling.md — §30 "Diagnostics must act as a mentor"
func (a *Analyzer) rejectCarrierFamilyMismatch(pattern ast.Expression, subjectType Type) bool {
	family, token := carrierPatternFamily(pattern)
	switch {
	case family == "Result" && subjectType.Kind == UnionType && subjectType.Name == "Option":
		a.addErrorAtToken(token, "Ok and Err patterns match Result values; %s uses Some(value) and None", typeDisplayName(subjectType))
		return true
	case family == "Option" && subjectType.Kind == ResultType:
		if _, shadowed := a.symbols["None"]; shadowed {
			return false
		}
		a.addErrorAtToken(token, "Some and None patterns match Option values; %s uses Ok(value) and Err(error)", typeDisplayName(subjectType))
		return true
	}
	return false
}

// carrierPatternFamily names the carrier whose variant a pattern spells, if any.
func carrierPatternFamily(pattern ast.Expression) (string, lexer.Token) {
	switch pattern := pattern.(type) {
	case *ast.OkExpression:
		return "Result", pattern.Token
	case *ast.ErrExpression:
		return "Result", pattern.Token
	case *ast.Identifier:
		if pattern.Value == "None" {
			return "Option", pattern.Token
		}
	case *ast.CallExpression:
		if callee, ok := pattern.Callee.(*ast.Identifier); ok && callee.Value == "Some" {
			return "Option", callee.Token
		}
		if pattern.Function != nil && pattern.Function.Value == "Some" {
			return "Option", pattern.Function.Token
		}
	}
	return "", lexer.Token{}
}

// openErrorNarrowingPrefix marks a match-pattern variant key that names a
// concrete error variant narrowed from an open error channel.
const openErrorNarrowingPrefix = "error:"

// analyzeMatchErrorNarrowing validates `Err(ErrorType.Variant)` in a match.
// The concrete-variant form narrows only an open Result[T, error] channel; it
// never covers the open error domain, so an Err fallback is still required.
// For a concrete channel the payload must be bound instead.
//
// Rules:
//   - rules/control-flow/flowcontrol_match.md — "Open error narrowing"
//   - rules/errors/errorhandling.md — §27.2 "Matching Result[T, error]"
//   - rules/corrections/applied/match-errorhandling-correction-20260824.md — "Open-domain exhaustiveness"
func (a *Analyzer) analyzeMatchErrorNarrowing(member *ast.MemberExpression, errorType Type) (matchPatternInfo, bool) {
	if errorType.Kind != ErrorRootType {
		a.addErrorAtToken(member.Token, "Err(%s) narrows only an open Result[T, error] channel; for %s bind the payload and use a where guard or another match", member.String(), typeDisplayName(errorType))
		return matchPatternInfo{}, false
	}
	variantType, ok := a.inferMemberExpression(member)
	if !ok || variantType.Kind == InvalidType {
		return matchPatternInfo{}, false
	}
	if !variantType.ErrorAssignable {
		a.addErrorAtToken(member.Token, "Err(%s) must name a concrete error variant, got %s", member.String(), typeDisplayName(variantType))
		return matchPatternInfo{}, false
	}
	return matchPatternInfo{Variant: openErrorNarrowingPrefix + typeDisplayName(variantType) + "." + member.Property.Value}, true
}
