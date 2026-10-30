package sema

import (
	"math/big"

	"sec/internal/ast"
)

// ResolvedTypePackConstantOf evaluates a typed template expression for one
// selected shared-pack call. Only the actual parameter's Len is substituted;
// ordinary argument values, same-spelled foreign bindings and function/getter
// execution never acquire compile-time values from this query. The template
// remains unchanged, so different calls can have independent constant results.
// Rules: rules/declarations/functions.md §38A(5)(2),(9);
// rules/compiler/compile_time_evaluation.md §§2(3),4(4),17,47;
// rules/compiler/semantic_ir.md §27A.
func (a *Analyzer) ResolvedTypePackConstantOf(call *ast.CallExpression, expression ast.Expression) (DefaultConstant, bool) {
	if a == nil || expression == nil {
		return DefaultConstant{}, false
	}
	fact, known := a.ResolvedTypePackCallOf(call)
	if !known {
		return DefaultConstant{}, false
	}
	target, known := a.ResolvedCallTarget(call)
	if !known {
		return DefaultConstant{}, false
	}
	parameter, shared := sharedTypePackParameter(target.Function)
	if !shared {
		return DefaultConstant{}, false
	}
	binding, known := a.ResolvedBindingDeclaredAt(parameter.Token.File, parameter.Token.Line, parameter.Token.Column)
	if !known {
		return DefaultConstant{}, false
	}
	typ, typed := a.ResolvedTypeOf(expression)
	if !typed {
		return DefaultConstant{}, false
	}
	resolve := func(expr ast.Expression) (DefaultConstant, compileTimeOutcome, bool) {
		member, ok := expr.(*ast.MemberExpression)
		if !ok || member.Property == nil || member.Property.Value != "Len" {
			return DefaultConstant{}, compileTimeNotConstant, false
		}
		receiver, ok := member.Object.(*ast.Identifier)
		if !ok {
			return DefaultConstant{}, compileTimeNotConstant, false
		}
		actual, known := a.ResolvedBindingOf(receiver)
		if !known || actual.ID != binding.ID {
			return DefaultConstant{}, compileTimeNotConstant, false
		}
		value := new(big.Int).SetUint64(fact.Length)
		return DefaultConstant{Kind: UintType, Integer: value, Lexeme: value.String()}, compileTimeEvaluated, true
	}
	// Runtime bindings are intentionally unavailable. This also bypasses the
	// module-name shortcut, which cannot identify a specialized local binding.
	identifier := func(*ast.Identifier) (DefaultConstant, compileTimeOutcome) {
		return DefaultConstant{}, compileTimeNotConstant
	}
	value, outcome := a.semanticCompileTimeConstantResolving(expression, map[string]bool{}, typ, identifier, resolve)
	if outcome != compileTimeEvaluated {
		return DefaultConstant{}, false
	}
	if value.Integer != nil {
		value.Integer = new(big.Int).Set(value.Integer)
		value.Kind = typ.Kind
	}
	return value, true
}
