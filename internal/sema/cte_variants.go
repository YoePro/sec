package sema

import (
	"strings"

	"sec/internal/ast"
	"sec/internal/lexer"
)

// variantValue builds an Option, Result or union value.
func variantValue(typ Type, variant string, payload *DefaultConstant) DefaultConstant {
	return DefaultConstant{Kind: typ.Kind, Aggregate: &CompileTimeAggregate{Type: typ, Variant: variant, Payload: payload}}
}

// variantExpression constructs Some/Ok/Err values and the None value
// (compile_time_evaluation.md § 46: Result and Option are ordinary data).
func (x *cteExecutor) variantExpression(expr ast.Expression) (DefaultConstant, bool, bool) {
	typ, _ := x.a.ResolvedTypeOf(expr)
	payloadOf := func(value ast.Expression) (*DefaultConstant, bool) {
		if value == nil {
			return nil, true
		}
		evaluated, ok := x.eval(value)
		if !ok {
			return nil, false
		}
		return &evaluated, true
	}
	switch expr := expr.(type) {
	case *ast.OkExpression:
		payload, ok := payloadOf(expr.Value)
		return variantValue(typ, "Ok", payload), ok, true
	case *ast.ErrExpression:
		payload, ok := payloadOf(expr.Value)
		return variantValue(typ, "Err", payload), ok, true
	case *ast.CallExpression:
		if callee, ok := expr.Callee.(*ast.Identifier); ok && callee.Value == "Some" && len(expr.Arguments) == 1 && isOptionType(typ) {
			if _, resolved := x.a.resolvedCalls[expr]; !resolved {
				payload, ok := payloadOf(expr.Arguments[0])
				return variantValue(typ, "Some", payload), ok, true
			}
		}
	case *ast.Identifier:
		if expr.Value == "None" && isOptionType(typ) {
			return variantValue(typ, "None", nil), true, true
		}
	}
	return DefaultConstant{}, false, false
}

// propagate ends the current function with an early return of value, the
// failure edge of `try` (errorhandling.md; compile_time_evaluation.md § 46(4)).
func (x *cteExecutor) propagate(value DefaultConstant) {
	x.pendingReturn = &value
}

// tryExpression evaluates `try operand` without handlers: Ok/Some unwrap,
// Err/None return from the enclosing function.
func (x *cteExecutor) tryExpression(expr *ast.TryExpression) (DefaultConstant, bool) {
	if len(expr.Handlers) > 0 {
		x.fail(cteImplementationGap, expr.Token, "try with local handlers is not executed by the compile-time evaluator yet")
		return DefaultConstant{}, false
	}
	value, ok := x.eval(expr.Expression)
	if !ok {
		return DefaultConstant{}, false
	}
	if value.Aggregate == nil || value.Aggregate.Variant == "" {
		x.fail(cteImplementationGap, expr.Token, "try on %s is not executed by the compile-time evaluator yet", expr.Expression.String())
		return DefaultConstant{}, false
	}
	switch value.Aggregate.Variant {
	case "Ok", "Some":
		if value.Aggregate.Payload == nil {
			return DefaultConstant{Kind: VoidType}, true
		}
		return *value.Aggregate.Payload, true
	case "Err", "None":
		if len(x.frames) <= 1 {
			x.fail(cteEvaluationFailed, expr.Token, "try propagated %s out of the evaluated expression", value.Aggregate.Variant)
			return DefaultConstant{}, false
		}
		returned := value
		if function := x.currentFunctionReturn(); function.Kind != "" {
			returned.Aggregate = &CompileTimeAggregate{Type: function, Variant: value.Aggregate.Variant, Payload: value.Aggregate.Payload}
			returned.Kind = function.Kind
		}
		x.propagate(returned)
		return DefaultConstant{}, false
	}
	x.fail(cteImplementationGap, expr.Token, "try on variant %s is not executed by the compile-time evaluator yet", value.Aggregate.Variant)
	return DefaultConstant{}, false
}

// matchValue selects the first arm whose pattern and guard accept the
// subject, binding a payload, and runs its body. In statement position a
// block body runs as statements; in expression position the arm's value is
// its expression body.
func (x *cteExecutor) matchValue(match *ast.MatchExpression, statement bool) (cteControl, DefaultConstant, bool) {
	subject, ok := x.eval(match.Subject)
	if !ok {
		return cteNormal, DefaultConstant{}, false
	}
	for _, arm := range match.Arms {
		if arm == nil || arm.Pattern == nil || arm.Invalid {
			continue
		}
		matched, ok := x.patternMatches(arm.Pattern, subject, arm.Token)
		if !ok {
			return cteNormal, DefaultConstant{}, false
		}
		if !matched {
			continue
		}
		if arm.Guard != nil {
			accepted, ok := x.condition(arm.Guard)
			if !ok {
				return cteNormal, DefaultConstant{}, false
			}
			if !accepted {
				continue
			}
		}
		switch {
		case arm.ReturnBody != nil:
			control, value := x.statement(arm.ReturnBody)
			return control, value, x.failure == nil
		case arm.BlockBody != nil:
			if !statement {
				x.fail(cteImplementationGap, arm.Token, "a block-bodied match arm in an expression is not executed by the compile-time evaluator yet")
				return cteNormal, DefaultConstant{}, false
			}
			control, value := x.block(arm.BlockBody)
			return control, value, x.failure == nil
		case arm.Body != nil:
			value, ok := x.eval(arm.Body)
			return cteNormal, value, ok
		}
		return cteNormal, DefaultConstant{}, true
	}
	x.fail(cteEvaluationFailed, match.Token, "no match arm accepted the value")
	return cteNormal, DefaultConstant{}, false
}

// patternMatches tests one pattern against the subject and binds its
// payload to the binding's declaration.
func (x *cteExecutor) patternMatches(pattern *ast.MatchPattern, subject DefaultConstant, at lexer.Token) (bool, bool) {
	switch pattern.Kind {
	case ast.MatchPatternCatchAll:
		if pattern.Binding != nil && pattern.Binding.Name != nil {
			x.frame().values[sourceTokenLocation(pattern.Binding.Name.Token)] = subject
		}
		return true, true
	case ast.MatchPatternVariant:
	default:
		x.fail(cteImplementationGap, pattern.Token, "%s match patterns are not executed by the compile-time evaluator yet", pattern.Kind)
		return false, false
	}
	if pattern.ErrorVariant != nil || pattern.ContextualErrorVariant != nil || pattern.ErrorPayload != nil {
		x.fail(cteImplementationGap, pattern.Token, "error-variant narrowing patterns are not executed by the compile-time evaluator yet")
		return false, false
	}
	name := pattern.Name
	if index := strings.LastIndex(name, "."); index >= 0 {
		name = name[index+1:]
	}
	if subject.Aggregate == nil {
		if subject.Nominal.Owner != "" {
			return subject.Nominal.Member == name, true
		}
		x.fail(cteImplementationGap, at, "matching %s against this value is not executed by the compile-time evaluator yet", pattern.Name)
		return false, false
	}
	if subject.Aggregate.Variant != name {
		return false, true
	}
	if pattern.Binding != nil && pattern.Binding.Name != nil {
		if pattern.Binding.Mode != ast.MatchBindingValue && pattern.Binding.Mode != "" {
			x.fail(cteImplementationGap, pattern.Token, "reference match bindings are not executed by the compile-time evaluator yet")
			return false, false
		}
		if subject.Aggregate.Payload == nil {
			x.fail(cteImplementationGap, pattern.Token, "variant %s has no payload to bind", name)
			return false, false
		}
		x.frame().values[sourceTokenLocation(pattern.Binding.Name.Token)] = *subject.Aggregate.Payload
	}
	return true, true
}
