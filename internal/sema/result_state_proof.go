package sema

import "sec/internal/ast"

// provesResultProjectionSafe reports that the alternate state forgotten by a
// consuming projection is unreachable on the current path: the immutable
// binding was constructed by the matching Ok(...) or Err(...), or a dominating
// condition proves `result.ErrRef is None` or `result.OkRef is not None` for
// Ok(), and the mirrored tests for Err(). The Result must be an immutable
// binding so no later operation can change its state.
//
// Rules:
//   - rules/errors/errorhandling.md — §6.1 "Consuming projections", §6.2 borrowed projections, §28
//   - rules/control-flow/discard.md — "Recursive discardability": a union is discardable when the active variant is proven
func (a *Analyzer) provesResultProjectionSafe(object ast.Expression, projection string) bool {
	identifier, ok := object.(*ast.Identifier)
	if !ok {
		return false
	}
	symbol, exists := a.symbols[identifier.Value]
	if !exists || symbol.Mutable {
		return false
	}
	// An immutable binding initialized by Ok(...) or Err(...) keeps that
	// state for its whole lifetime.
	if symbol.ResultConstruction == projection {
		return true
	}
	for _, active := range a.activeConditionFacts {
		if active.epoch != a.arrayIndexMutationEpoch {
			continue
		}
		property, isNone, ok := optionStateTest(active.fact.Condition, identifier.Value)
		if !ok {
			continue
		}
		switch projection {
		case "Ok":
			if property == "ErrRef" && isNone || property == "OkRef" && !isNone {
				return true
			}
		case "Err":
			if property == "OkRef" && isNone || property == "ErrRef" && !isNone {
				return true
			}
		}
	}
	return false
}

// optionStateTest recognizes the parser's lowering of `name.Property is None`
// and `name.Property is not None` into a two-arm match on the borrowed
// projection, returning the projection property and whether the fact means
// the projection is None.
func optionStateTest(condition ast.Expression, name string) (string, bool, bool) {
	match, ok := condition.(*ast.MatchExpression)
	if !ok || len(match.Arms) != 2 {
		return "", false, false
	}
	subject, ok := match.Subject.(*ast.MemberExpression)
	if !ok || subject.Property == nil {
		return "", false, false
	}
	root, ok := subject.Object.(*ast.Identifier)
	if !ok || root.Value != name {
		return "", false, false
	}
	first, second := match.Arms[0], match.Arms[1]
	if first.Pattern == nil || second.Pattern == nil || first.Pattern.Name != "None" || second.Pattern.Name != "_" {
		return "", false, false
	}
	noneBody, okNone := first.Body.(*ast.BooleanLiteral)
	otherBody, okOther := second.Body.(*ast.BooleanLiteral)
	if !okNone || !okOther || noneBody.Value == otherBody.Value {
		return "", false, false
	}
	return subject.Property.Value, noneBody.Value, true
}

// recordResultConstruction proves the state of an immutable Result binding
// initialized directly by Ok(...) or Err(...); any other initializer, a
// mutable binding, or a shadowing declaration leaves the state unknown.
//
// Rules:
//   - rules/errors/errorhandling.md — § 6 "Result success/error projections"
func (a *Analyzer) recordResultConstruction(stmt *ast.LetStatement) {
	if stmt == nil || stmt.Name == nil {
		return
	}
	symbol, exists := a.symbols[stmt.Name.Value]
	if !exists {
		return
	}
	symbol.ResultConstruction = ""
	if !stmt.Mutable {
		switch stmt.Value.(type) {
		case *ast.OkExpression:
			symbol.ResultConstruction = "Ok"
		case *ast.ErrExpression:
			symbol.ResultConstruction = "Err"
		}
	}
	a.symbols[stmt.Name.Value] = symbol
}
