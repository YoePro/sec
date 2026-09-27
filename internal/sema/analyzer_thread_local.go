package sema

import "sec/internal/ast"

// inferThreadLocalCall validates the exact source-level ThreadLocal[T] v2
// access surface. It deliberately does not invent the later runtime model:
// first-access effects, physical-thread provenance, and slot alias conflicts
// are retained as distinct implementation steps.
//
// Rules:
//   - rules/concurrency/thread_local.md — §4 "Exact ThreadLocal[T] declaration"
//   - rules/concurrency/thread_local.md — §§15–18 "Access and aliasing"
//   - rules/memory/copy_move.md — §8.2 "Consuming parameters"
func (a *Analyzer) inferThreadLocalCall(
	expr *ast.CallExpression,
	threadLocal Type,
	member CompilerKnownMember,
) (Type, expressionValue, bool) {
	result := expressionValue{Display: expr.String()}
	if threadLocal.Name != "ThreadLocal" || len(threadLocal.TypeArgs) != 1 {
		return Type{}, expressionValue{}, false
	}
	operation := typeDisplayName(threadLocal) + "." + member.Name
	switch member.Name {
	case "Borrow", "BorrowMut":
		if !a.checkCompilerKnownCallArity(expr, operation, 0, 0) {
			return Type{Kind: InvalidType}, result, true
		}
		return member.Result, result, true
	case "Replace":
		if !a.checkCompilerKnownCallArity(expr, operation, 1, 1) {
			return Type{Kind: InvalidType}, result, true
		}
		payload := threadLocal.TypeArgs[0]
		argumentType, _ := a.inferCallArgumentExpression(expr.Arguments[0])
		argumentType = a.contextualCallArgumentType(expr.Arguments[0], argumentType, payload)
		if argumentType.Kind == InvalidType {
			return Type{Kind: InvalidType}, result, true
		}
		if !canInitialize(payload, argumentType, expr.Arguments[0]) {
			a.addErrorAtToken(
				expressionToken(expr.Arguments[0]),
				"ThreadLocal.Replace value must be %s, got %s",
				typeDisplayName(payload),
				typeDisplayName(argumentType),
			)
			return Type{Kind: InvalidType}, result, true
		}
		function := Function{
			Name: "ThreadLocal.Replace",
			Parameters: []FunctionParameter{{
				Name: "value",
				Type: payload,
			}},
			ReturnType: payload,
		}
		if !a.validateCallArgumentOwnership(function, expr.Arguments, nil) {
			return Type{Kind: InvalidType}, result, true
		}
		a.markMovedCallArguments(function, expr.Arguments, nil, false)
		return payload, result, true
	default:
		return Type{}, expressionValue{}, false
	}
}
