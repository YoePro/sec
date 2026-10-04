package sema

import "sec/internal/ast"

// provesResultProjectionSafe reports that the alternate state forgotten by a
// consuming projection is unreachable on the current path: the immutable
// binding was constructed by the matching Ok(...) or Err(...), a dominating
// condition proves `result.ErrRef is None` or `result.OkRef is not None` for
// Ok() (and the mirrored tests for Err()), or a path-recorded state fact holds:
// the else branch of such a test, the code after a branch that exits, an
// `is Some(binding)` test, or a match arm on `result.OkRef`/`result.ErrRef`.
// The Result must be an immutable binding so no later operation can change
// its state.
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
		if active.resultBinding != "" {
			if active.resultBinding == identifier.Value && active.resultState == projection {
				return true
			}
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
// initialized directly by Ok(...) or Err(...), and the active variant of an
// immutable union binding initialized directly by a variant constructor; any
// other initializer, a mutable binding, or a shadowing declaration leaves the
// state unknown.
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
	symbol.ConstructedVariant = ""
	if !stmt.Mutable {
		switch stmt.Value.(type) {
		case *ast.OkExpression:
			symbol.ResultConstruction = "Ok"
		case *ast.ErrExpression:
			symbol.ResultConstruction = "Err"
		}
		symbol.ConstructedVariant = symbol.ResultConstruction
		if symbol.ConstructedVariant == "" {
			symbol.ConstructedVariant = a.unionConstructorVariant(stmt.Value, symbol.Type)
		}
	}
	a.symbols[stmt.Name.Value] = symbol
}

// recordResultStateFact activates a proven Result state for an immutable
// binding until the enclosing refinement scope ends.
//
// Rules:
//   - rules/errors/errorhandling.md — §6.1 "Consuming projections" (state proven unreachable by control flow)
func (a *Analyzer) recordResultStateFact(binding string, state string) {
	if symbol, exists := a.symbols[binding]; !exists || symbol.Mutable {
		return
	}
	a.activeConditionFacts = append(a.activeConditionFacts, activeConditionFact{epoch: a.arrayIndexMutationEpoch, resultBinding: binding, resultState: state})
}

func oppositeResultState(state string) string {
	if state == "Ok" {
		return "Err"
	}
	return "Ok"
}

// borrowedProjectionState names the binding of `name.OkRef` or `name.ErrRef`
// and the Result state in which that projection is present.
func borrowedProjectionState(subject ast.Expression) (string, string, bool) {
	member, ok := subject.(*ast.MemberExpression)
	if !ok || member == nil || member.Property == nil {
		return "", "", false
	}
	root, ok := member.Object.(*ast.Identifier)
	if !ok || root == nil {
		return "", "", false
	}
	switch member.Property.Value {
	case "OkRef":
		return root.Value, "Ok", true
	case "ErrRef":
		return root.Value, "Err", true
	}
	return "", "", false
}

// ifResultStateTest returns the Result state an if condition proves on its
// true path: `r.ErrRef is None`, `r.OkRef is not None`, their mirrors, and the
// `if r.OkRef is Some(value)` binding form.
func ifResultStateTest(stmt *ast.IfStatement) (string, string, bool) {
	if stmt == nil {
		return "", "", false
	}
	if stmt.OptionBinding != nil {
		return borrowedProjectionState(stmt.OptionBinding.Subject)
	}
	match, ok := stmt.Condition.(*ast.MatchExpression)
	if !ok || match == nil {
		return "", "", false
	}
	binding, presentState, ok := borrowedProjectionState(match.Subject)
	if !ok {
		return "", "", false
	}
	_, isNone, ok := optionStateTest(stmt.Condition, binding)
	if !ok {
		return "", "", false
	}
	if isNone {
		return binding, oppositeResultState(presentState), true
	}
	return binding, presentState, true
}

// matchArmResultState returns the Result state a `None` or `Some(...)` arm of
// a match on `r.OkRef` or `r.ErrRef` proves inside that arm.
func matchArmResultState(subject ast.Expression, arm *ast.MatchArm) (string, string, bool) {
	if arm == nil || arm.Pattern == nil {
		return "", "", false
	}
	binding, presentState, ok := borrowedProjectionState(subject)
	if !ok {
		return "", "", false
	}
	switch arm.Pattern.Name {
	case "None":
		return binding, oppositeResultState(presentState), true
	case "Some":
		return binding, presentState, true
	}
	return "", "", false
}
