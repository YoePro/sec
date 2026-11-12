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
		value := variantValue(typ, "Ok", payload)
		return value, ok && x.chargeAggregate(value, expr.Token), true
	case *ast.ErrExpression:
		payload, ok := payloadOf(expr.Value)
		value := variantValue(typ, "Err", payload)
		return value, ok && x.chargeAggregate(value, expr.Token), true
	case *ast.MemberExpression:
		// `Union.Variant` of a payload-free variant that is not an ordinary
		// nominal constant (for example a variant of an error union).
		if typ.Kind == UnionType && !isOptionType(typ) && expr.Property != nil && unionVariantDeclared(typ, expr.Property.Value) && !unionVariantHasPayload(typ, expr.Property.Value) {
			if _, nominal := x.a.nominalMemberConstant(expr, Type{}); !nominal {
				return variantValue(typ, expr.Property.Value, nil), true, true
			}
		}
	case *ast.StructLiteral:
		// `Union.Variant { field: value }` constructs a field-payload variant.
		if typ.Kind == UnionType && !isOptionType(typ) && expr.Type != nil {
			value, ok := x.unionFieldVariant(expr, typ)
			return value, ok, true
		}
	case *ast.CallExpression:
		if member, ok := expr.Callee.(*ast.MemberExpression); ok && member.Property != nil && typ.Kind == UnionType && !isOptionType(typ) && len(expr.Arguments) == 1 {
			if _, resolved := x.a.resolvedCalls[expr]; !resolved && unionVariantHasPayload(typ, member.Property.Value) {
				payload, ok := payloadOf(expr.Arguments[0])
				value := variantValue(typ, member.Property.Value, payload)
				return value, ok && x.chargeAggregate(value, expr.Token), true
			}
		}
		if callee, ok := expr.Callee.(*ast.Identifier); ok && callee.Value == "Some" && len(expr.Arguments) == 1 && isOptionType(typ) {
			if _, resolved := x.a.resolvedCalls[expr]; !resolved {
				payload, ok := payloadOf(expr.Arguments[0])
				value := variantValue(typ, "Some", payload)
				return value, ok && x.chargeAggregate(value, expr.Token), true
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
		return x.tryWithHandlers(expr)
	}
	value, ok := x.tryOperand(expr.Expression)
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
	control, value, ok := x.matchArms(match, subject, statement)
	// A temporary subject ends with the match; payloads its arm bound by
	// value are owned by those bindings (destruction.md § 8).
	if ok && !isPlaceExpression(match.Subject) {
		x.destroyValue(subject, false, match.Token)
		ok = x.failure == nil
	}
	return control, value, ok
}

// matchArms selects and executes the first accepted arm. The arm's pattern
// bindings belong to the arm's scope; a binding that receives an owned
// value takes over its destruction.
func (x *cteExecutor) matchArms(match *ast.MatchExpression, subject DefaultConstant, statement bool) (cteControl, DefaultConstant, bool) {
	for _, arm := range match.Arms {
		if arm == nil || arm.Pattern == nil || arm.Invalid {
			continue
		}
		x.patternBindings = nil
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
		x.enterScope()
		x.takeOwnedPatternBindings()
		control, value, ok := cteNormal, DefaultConstant{}, true
		switch {
		case arm.ReturnBody != nil:
			control, value = x.statement(arm.ReturnBody)
			ok = x.failure == nil
		case arm.BlockBody != nil:
			if !statement {
				x.fail(cteImplementationGap, arm.Token, "a block-bodied match arm in an expression is not executed by the compile-time evaluator yet")
				ok = false
				break
			}
			control, value = x.block(arm.BlockBody)
			ok = x.failure == nil
		case arm.Body != nil:
			value, ok = x.eval(arm.Body)
		}
		x.leaveScope()
		return control, value, ok && x.failure == nil
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
			x.bindPattern(sourceTokenLocation(pattern.Binding.Name.Token), subject)
		}
		return true, true
	case ast.MatchPatternVariant, ast.MatchPatternFields:
	default:
		x.fail(cteImplementationGap, pattern.Token, "%s match patterns are not executed by the compile-time evaluator yet", pattern.Kind)
		return false, false
	}
	if pattern.ErrorVariant != nil || pattern.ContextualErrorVariant != nil || pattern.ErrorPayload != nil {
		return x.errorVariantMatches(pattern, subject, at)
	}
	if pattern.Kind == ast.MatchPatternFields {
		return x.fieldPatternMatches(pattern, subject)
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
		x.bindPattern(sourceTokenLocation(pattern.Binding.Name.Token), *subject.Aggregate.Payload)
	}
	return true, true
}

// unionVariantHasPayload reports a declared union variant with a payload.
func unionVariantHasPayload(typ Type, name string) bool {
	for _, variant := range typ.UnionVariants {
		if variant.Name == name {
			return variant.Payload != nil || len(variant.PayloadFields) > 0
		}
	}
	return false
}

// unionFieldVariant constructs `Union.Variant { field: value, ... }`, the
// payload being a struct-like value of the variant's declared fields.
func (x *cteExecutor) unionFieldVariant(literal *ast.StructLiteral, typ Type) (DefaultConstant, bool) {
	name := literal.Type.Name
	if index := strings.LastIndex(name, "."); index >= 0 {
		name = name[index+1:]
	}
	var variant *UnionVariant
	for index := range typ.UnionVariants {
		if typ.UnionVariants[index].Name == name {
			variant = &typ.UnionVariants[index]
		}
	}
	if variant == nil {
		x.fail(cteImplementationGap, literal.Token, "union variant literal %s is not executed by the compile-time evaluator yet", literal.String())
		return DefaultConstant{}, false
	}
	values := map[string]DefaultConstant{}
	for _, field := range literal.Fields {
		if field == nil || field.Spread || field.Name == nil {
			x.fail(cteImplementationGap, literal.Token, "this union variant literal form is not executed by the compile-time evaluator yet")
			return DefaultConstant{}, false
		}
		value, ok := x.eval(field.Value)
		if !ok {
			return DefaultConstant{}, false
		}
		values[field.Name.Value] = value
	}
	payload := &CompileTimeAggregate{Fields: map[string]DefaultConstant{}}
	for _, field := range variant.PayloadFields {
		value, ok := values[field.Name]
		if !ok {
			x.fail(cteImplementationGap, literal.Token, "omitted field %s of union variant %s is not executed by the compile-time evaluator yet", field.Name, name)
			return DefaultConstant{}, false
		}
		payload.Names = append(payload.Names, field.Name)
		payload.Fields[field.Name] = value
	}
	payloadValue := DefaultConstant{Kind: StructType, Aggregate: payload}
	value := variantValue(typ, name, &payloadValue)
	return value, x.chargeAggregate(value, literal.Token)
}

// fieldPatternMatches tests `Variant { field, field: binding }` and binds
// each named field of the active variant's payload.
func (x *cteExecutor) fieldPatternMatches(pattern *ast.MatchPattern, subject DefaultConstant) (bool, bool) {
	name := pattern.Name
	if index := strings.LastIndex(name, "."); index >= 0 {
		name = name[index+1:]
	}
	if subject.Aggregate == nil || subject.Aggregate.Variant != name {
		return false, true
	}
	payload := subject.Aggregate.Payload
	if payload == nil || payload.Aggregate == nil {
		x.fail(cteImplementationGap, pattern.Token, "variant %s has no field payload to bind", name)
		return false, false
	}
	for _, field := range pattern.Fields {
		if field == nil || field.Field == nil {
			continue
		}
		value, ok := payload.Aggregate.Fields[field.Field.Value]
		if !ok {
			x.fail(cteImplementationGap, field.Token, "field %s is not part of variant %s", field.Field.Value, name)
			return false, false
		}
		token := field.Field.Token
		if field.Binding != nil && field.Binding.Name != nil {
			if field.Binding.Mode != ast.MatchBindingValue && field.Binding.Mode != "" {
				x.fail(cteImplementationGap, field.Token, "reference field bindings are not executed by the compile-time evaluator yet")
				return false, false
			}
			token = field.Binding.Name.Token
		}
		x.bindPattern(sourceTokenLocation(token), value)
	}
	return true, true
}

// errorVariantMatches tests `Err(ErrorType.Variant)` and `Err(.Variant)`
// narrowing against an Err value's payload (flowcontrol_match.md "Open error
// narrowing"; MD-029).
func (x *cteExecutor) errorVariantMatches(pattern *ast.MatchPattern, subject DefaultConstant, at lexer.Token) (bool, bool) {
	if subject.Aggregate == nil || subject.Aggregate.Variant != "Err" || subject.Aggregate.Payload == nil {
		return false, true
	}
	payload := *subject.Aggregate.Payload
	// `Err(E.Variant(binding))` / `Err(E.Variant { fields })`: the payload
	// pattern selects the error variant and binds its payload.
	if pattern.ErrorPayload != nil {
		return x.patternMatches(pattern.ErrorPayload, payload, at)
	}
	if pattern.ContextualErrorVariant != nil {
		return payload.Nominal.Member == pattern.ContextualErrorVariant.Value, true
	}
	// A payload-free variant of a union that also has payload variants is
	// an aggregate value; `E.Variant` selects it by variant name.
	if member := pattern.ErrorVariant; member != nil && member.Property != nil && payload.Aggregate != nil && payload.Aggregate.Variant != "" {
		return payload.Aggregate.Variant == member.Property.Value && payload.Aggregate.Payload == nil, true
	}
	expected, ok := x.eval(pattern.ErrorVariant)
	if !ok {
		return false, false
	}
	return x.valuesEqual(payload, expected, at, pattern.ErrorVariant.String())
}

// tryWithHandlers evaluates `try operand { handlers }`: Ok/Some unwrap, and an
// Err selects the first handler whose pattern and guard accept the error; the
// handler's value replaces the expression, or its return leaves the function.
func (x *cteExecutor) tryWithHandlers(expr *ast.TryExpression) (DefaultConstant, bool) {
	value, ok := x.tryOperand(expr.Expression)
	if !ok {
		return DefaultConstant{}, false
	}
	if value.Aggregate == nil || value.Aggregate.Variant == "" {
		x.fail(cteImplementationGap, expr.Token, "try on %s is not executed by the compile-time evaluator yet", expr.Expression.String())
		return DefaultConstant{}, false
	}
	if value.Aggregate.Variant == "Ok" || value.Aggregate.Variant == "Some" {
		if value.Aggregate.Payload == nil {
			return DefaultConstant{Kind: VoidType}, true
		}
		return *value.Aggregate.Payload, true
	}
	for _, handler := range expr.Handlers {
		if handler == nil || handler.Invalid {
			continue
		}
		matched, ok := x.handlerMatches(handler, value)
		if !ok {
			return DefaultConstant{}, false
		}
		if !matched {
			continue
		}
		if handler.Guard != nil {
			accepted, ok := x.condition(handler.Guard)
			if !ok {
				return DefaultConstant{}, false
			}
			if !accepted {
				continue
			}
		}
		switch {
		case handler.ReturnBody != nil:
			control, returned := x.statement(handler.ReturnBody)
			if x.failure == nil && control == cteReturn {
				x.propagate(returned)
			}
			return DefaultConstant{}, false
		case handler.BlockBody != nil:
			return x.handlerBlockValue(handler)
		case handler.Body != nil:
			return x.eval(handler.Body)
		}
	}
	x.fail(cteEvaluationFailed, expr.Token, "no try handler accepted the error")
	return DefaultConstant{}, false
}

// handlerMatches tests a handler pattern against an Err or None value.
func (x *cteExecutor) handlerMatches(handler *ast.TryHandler, value DefaultConstant) (bool, bool) {
	if handler.ErrPattern != nil {
		return x.patternMatches(handler.ErrPattern, value, handler.Token)
	}
	switch pattern := handler.Pattern.(type) {
	case nil:
		return true, true
	case *ast.Identifier:
		if pattern.Value == "_" {
			return true, true
		}
		x.frame().values[sourceTokenLocation(pattern.Token)] = value
		return true, true
	case *ast.ErrExpression:
		if value.Aggregate.Variant != "Err" {
			return false, true
		}
		payload := DefaultConstant{}
		if value.Aggregate.Payload != nil {
			payload = *value.Aggregate.Payload
		}
		switch inner := pattern.Value.(type) {
		case nil:
			return true, true
		case *ast.Identifier:
			if inner.Value != "_" {
				x.frame().values[sourceTokenLocation(inner.Token)] = payload
			}
			return true, true
		case *ast.MemberExpression:
			expected, ok := x.eval(inner)
			if !ok {
				return false, false
			}
			return x.valuesEqual(payload, expected, handler.Token, inner.String())
		}
	}
	x.fail(cteImplementationGap, handler.Token, "this try handler pattern is not executed by the compile-time evaluator yet")
	return false, false
}

// handlerBlockValue runs a block handler: a return leaves the function, and
// otherwise the block's final expression statement is the handler's value
// (errorhandling.md § 21.1 "Block handler result position").
func (x *cteExecutor) handlerBlockValue(handler *ast.TryHandler) (DefaultConstant, bool) {
	statements := handler.BlockBody.Statements
	last := len(statements) - 1
	for last >= 0 {
		if _, comment := statements[last].(*ast.CommentStatement); !comment {
			break
		}
		last--
	}
	final, hasValue := (*ast.ExpressionStatement)(nil), false
	if last >= 0 {
		final, hasValue = statements[last].(*ast.ExpressionStatement)
	}
	if !hasValue {
		control, returned := x.block(handler.BlockBody)
		if x.failure == nil && control == cteReturn {
			x.propagate(returned)
		} else if x.failure == nil {
			x.fail(cteImplementationGap, handler.Token, "a block handler without a final value or return produces no value")
		}
		return DefaultConstant{}, false
	}
	prefix := &ast.BlockStatement{Token: handler.BlockBody.Token, Statements: statements[:last]}
	control, returned := x.block(prefix)
	if x.failure != nil {
		return DefaultConstant{}, false
	}
	if control == cteReturn {
		x.propagate(returned)
		return DefaultConstant{}, false
	}
	return x.eval(final.Expression)
}

// unionVariantDeclared reports a declared union variant name.
func unionVariantDeclared(typ Type, name string) bool {
	for _, variant := range typ.UnionVariants {
		if variant.Name == name {
			return true
		}
	}
	return false
}

// valuesEqual compares two evaluator values: variants by active variant and
// payload, structs field by field, scalars and nominal members through the
// shared operator semantics.
func (x *cteExecutor) valuesEqual(left, right DefaultConstant, at lexer.Token, what string) (bool, bool) {
	if left.Aggregate != nil || right.Aggregate != nil {
		if left.Aggregate == nil || right.Aggregate == nil {
			// A nominal payload-free variant against its variant value.
			variant := func(value DefaultConstant) string {
				if value.Aggregate != nil {
					return value.Aggregate.Variant
				}
				return value.Nominal.Member
			}
			return variant(left) != "" && variant(left) == variant(right), true
		}
		if left.Aggregate.Variant != right.Aggregate.Variant {
			return false, true
		}
		if (left.Aggregate.Payload == nil) != (right.Aggregate.Payload == nil) {
			return false, true
		}
		if left.Aggregate.Payload != nil {
			if equal, ok := x.valuesEqual(*left.Aggregate.Payload, *right.Aggregate.Payload, at, what); !ok || !equal {
				return equal, ok
			}
		}
		if len(left.Aggregate.Fields) != len(right.Aggregate.Fields) {
			return false, true
		}
		for name, value := range left.Aggregate.Fields {
			other, exists := right.Aggregate.Fields[name]
			if !exists {
				return false, true
			}
			if equal, ok := x.valuesEqual(value, other, at, what); !ok || !equal {
				return equal, ok
			}
		}
		return true, true
	}
	equal, outcome := x.a.compileTimeBinary("==", left, right)
	if outcome != compileTimeEvaluated || equal.Kind != BoolType {
		x.fail(cteImplementationGap, at, "comparing the value with %s is not executed by the compile-time evaluator yet", what)
		return false, false
	}
	return equal.Bool, true
}
