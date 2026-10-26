package main

import (
	"reflect"

	"sec/internal/ast"
	"sec/internal/sema"
)

// expectedValueTypeAt returns the type a member selector written directly as
// a value is expected to have, from the construct that receives it: a typed
// let initializer, an assignment, a call argument, a struct literal field or a
// match arm value. Like the return position, only a selector that is the
// whole value counts; a selector nested in a larger expression has no
// expected type of its own. Every type comes from Sema.
//
// Rules: rules/tooling/lsp.md — "Completion", expected-type member completion.
func expectedValueTypeAt(program *ast.Program, analyzer *sema.Analyzer, text string, dotOffset int) (sema.Type, bool) {
	member, parents := selectorWithParents(program, text, dotOffset)
	if member == nil || len(parents) == 0 {
		return sema.Type{}, false
	}
	value := ast.Expression(member)
	for index := len(parents) - 1; index >= 0; index-- {
		switch parent := parents[index].(type) {
		case *ast.LetStatement:
			if parent.Value != value || parent.Type == nil || parent.Name == nil {
				return sema.Type{}, false
			}
			if binding, ok := analyzer.ResolvedBindingOf(parent.Name); ok {
				return usableExpectedType(binding.Type)
			}
			return sema.Type{}, false
		case *ast.AssignmentStatement:
			if parent.Value != value || parent.Operator != "=" {
				return sema.Type{}, false
			}
			if identifier, ok := parent.Target.(*ast.Identifier); ok {
				if binding, found := analyzer.ResolvedBindingOf(identifier); found {
					return usableExpectedType(binding.Type)
				}
			}
			if target, ok := analyzer.TypeOf(parent.Target); ok {
				return usableExpectedType(target)
			}
			return sema.Type{}, false
		case *ast.CallExpression:
			return expectedArgumentType(analyzer, parent, value)
		case *ast.StructLiteralField:
			if parent.Value != value || parent.Name == nil || parent.Spread || index == 0 {
				return sema.Type{}, false
			}
			literal, ok := parents[index-1].(*ast.StructLiteral)
			if !ok {
				return sema.Type{}, false
			}
			owner, ok := analyzer.TypeOf(literal)
			if !ok {
				return sema.Type{}, false
			}
			for _, field := range owner.Fields {
				if field.Name == parent.Name.Value {
					return usableExpectedType(field.Type)
				}
			}
			return sema.Type{}, false
		case *ast.MatchArm:
			if parent.Body != value || index == 0 {
				return sema.Type{}, false
			}
			match, ok := parents[index-1].(*ast.MatchExpression)
			if !ok {
				return sema.Type{}, false
			}
			return expectedMatchArmType(analyzer, match, parent)
		case *ast.ExpressionStatement, *ast.ReturnStatement, *ast.BlockStatement:
			return sema.Type{}, false
		default:
			// Any other enclosing expression (operator, index, unary…)
			// makes the selector only part of the value.
			return sema.Type{}, false
		}
	}
	return sema.Type{}, false
}

// expectedArgumentType is the parameter type of the argument position the
// selector fills in a resolved call, or in the only overload of that arity.
func expectedArgumentType(analyzer *sema.Analyzer, call *ast.CallExpression, value ast.Expression) (sema.Type, bool) {
	position := -1
	for index, argument := range call.Arguments {
		if argument == value {
			position = index
		}
	}
	if position < 0 {
		return sema.Type{}, false
	}
	candidates := []sema.Function{}
	if resolved, ok := analyzer.ResolvedCallTarget(call); ok {
		candidates = append(candidates, resolved.Function)
	} else if callee, ok := call.Callee.(*ast.Identifier); ok {
		for _, overload := range analyzer.Functions()[callee.Value] {
			if len(overload.Parameters) == len(call.Arguments) {
				candidates = append(candidates, overload)
			}
		}
	}
	if len(candidates) != 1 || position >= len(candidates[0].Parameters) {
		return sema.Type{}, false
	}
	parameter := candidates[0].Parameters[position]
	if parameter.Ref || parameter.MutableRef || parameter.Variadic {
		return sema.Type{}, false
	}
	return usableExpectedType(parameter.Type)
}

// expectedMatchArmType is the value type of a match whose arms produce
// values: Sema's type of the match, or the agreeing type of its other arms.
func expectedMatchArmType(analyzer *sema.Analyzer, match *ast.MatchExpression, arm *ast.MatchArm) (sema.Type, bool) {
	if typ, ok := analyzer.TypeOf(match); ok {
		if expected, usable := usableExpectedType(typ); usable {
			return expected, true
		}
	}
	var found *sema.Type
	for _, other := range match.Arms {
		if other == nil || other == arm || other.Body == nil {
			continue
		}
		typ, ok := analyzer.TypeOf(other.Body)
		if !ok {
			continue
		}
		if found != nil && !completionTypeMatches(analyzer, *found, typ) {
			return sema.Type{}, false
		}
		found = &typ
	}
	if found == nil {
		return sema.Type{}, false
	}
	return usableExpectedType(*found)
}

func usableExpectedType(typ sema.Type) (sema.Type, bool) {
	if typ.Kind == sema.InvalidType || typ.Kind == "" {
		return sema.Type{}, false
	}
	return typ, true
}

// selectorWithParents finds the member expression whose dot is at dotOffset
// and the chain of AST nodes enclosing it, outermost first.
func selectorWithParents(program *ast.Program, text string, dotOffset int) (*ast.MemberExpression, []any) {
	if program == nil || dotOffset < 0 || dotOffset >= len(text) || text[dotOffset] != '.' {
		return nil, nil
	}
	var found *ast.MemberExpression
	var chain, result []any
	var visit func(reflect.Value)
	visit = func(value reflect.Value) {
		if found != nil || !value.IsValid() {
			return
		}
		switch value.Kind() {
		case reflect.Interface, reflect.Pointer:
			if value.IsNil() {
				return
			}
			if value.Kind() == reflect.Pointer && value.CanInterface() {
				node := value.Interface()
				if member, ok := node.(*ast.MemberExpression); ok && textPositionOffset(text, member.Token.Line, member.Token.Column) == dotOffset {
					found = member
					result = append([]any(nil), chain...)
					return
				}
				if value.Elem().Kind() == reflect.Struct && value.Elem().Type().PkgPath() == "sec/internal/ast" {
					chain = append(chain, node)
					visit(value.Elem())
					chain = chain[:len(chain)-1]
					return
				}
			}
			visit(value.Elem())
		case reflect.Struct:
			if value.Type().PkgPath() != "sec/internal/ast" {
				return
			}
			for i := 0; i < value.NumField(); i++ {
				visit(value.Field(i))
			}
		case reflect.Slice, reflect.Array:
			for i := 0; i < value.Len(); i++ {
				visit(value.Index(i))
			}
		}
	}
	visit(reflect.ValueOf(program))
	return found, result
}
