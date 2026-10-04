package sema

import (
	"fmt"
	"reflect"

	"sec/internal/ast"
	"sec/internal/diagnostics"
)

// pureIntegerCondition reports a condition built only from integer
// comparisons between immutable bindings and integer constants, combined with
// && and ||. Only such conditions
// have a sound negation (`a < b` is false exactly when `a >= b`), which the
// relational reachability proof relies on.
func (a *Analyzer) pureIntegerCondition(condition ast.Expression) bool {
	infix, ok := condition.(*ast.InfixExpression)
	if !ok || infix == nil {
		return false
	}
	switch infix.Operator {
	case "&&", "||":
		return a.pureIntegerCondition(infix.Left) && a.pureIntegerCondition(infix.Right)
	case "<", "<=", ">", ">=", "==", "!=":
		return a.pureIntegerOperand(infix.Left) && a.pureIntegerOperand(infix.Right)
	}
	return false
}

func (a *Analyzer) pureIntegerOperand(operand ast.Expression) bool {
	if _, constant := a.constantConditionIntegerValue(operand); constant {
		return true
	}
	// Only a binding that keeps its value for its whole lifetime qualifies:
	// an immutable binding, or a mutable working binding (such as an owned
	// parameter) that the function never assigns, mutably borrows, or
	// captures. A member read can change through a method call that the
	// comparison facts do not see.
	identifier, ok := operand.(*ast.Identifier)
	if !ok || identifier == nil {
		return false
	}
	symbol, exists := a.symbols[identifier.Value]
	if !exists || symbol.TransientConstant || symbol.Mutable && !a.currentStableBindings[identifier.Value] {
		return false
	}
	_, _, integer := a.integerInterval(operand)
	return integer
}

// negatedIntegerCondition returns the condition that holds exactly when a
// pure integer condition is false, by De Morgan over && and || and the
// complementary comparison.
func negatedIntegerCondition(condition ast.Expression) ast.Expression {
	infix := condition.(*ast.InfixExpression)
	switch infix.Operator {
	case "&&":
		return &ast.InfixExpression{Token: infix.Token, Left: negatedIntegerCondition(infix.Left), Operator: "||", Right: negatedIntegerCondition(infix.Right)}
	case "||":
		return &ast.InfixExpression{Token: infix.Token, Left: negatedIntegerCondition(infix.Left), Operator: "&&", Right: negatedIntegerCondition(infix.Right)}
	}
	complement := map[string]string{"<": ">=", "<=": ">", ">": "<=", ">=": "<", "==": "!=", "!=": "=="}[infix.Operator]
	return &ast.InfixExpression{Token: infix.Token, Left: infix.Left, Operator: complement, Right: infix.Right}
}

// relationalConditionValue proves a pure integer condition always true or
// always false from the dominating, unmutated condition facts of the path
// (enclosing branches, else branches, earlier exits, and assertions).
//
// Rules:
//   - rules/analysis/call_graph.md — "Unreachable code" (proven unreachable code is an error)
//   - rules/tooling/diagnostics.md — § 21(1), (3)–(6) "Proven unreachable and dead code"
//   - rules/control-flow/flowcontrol_if.md — § 20 unreachable-code policy
func (a *Analyzer) relationalConditionValue(condition ast.Expression) (value bool, known bool) {
	if condition == nil || !a.pureIntegerCondition(condition) || len(a.activeComparisonFacts()) == 0 {
		return false, false
	}
	value, known = a.decidePureIntegerCondition(condition)
	if !known {
		return false, false
	}
	// A condition that type ranges and contracts decide without any path
	// fact is a meaningless comparison (pitfall.range.meaningless-comparison),
	// not a path-proven unreachable branch; deciding it here only when some
	// unrelated fact happens to be active would be inconsistent.
	facts := a.activeConditionFacts
	a.activeConditionFacts = nil
	_, typeDecided := a.decidePureIntegerCondition(condition)
	a.activeConditionFacts = facts
	if typeDecided {
		return false, false
	}
	return value, true
}

func (a *Analyzer) decidePureIntegerCondition(condition ast.Expression) (bool, bool) {
	if a.proveCondition(condition) {
		return true, true
	}
	if a.proveCondition(negatedIntegerCondition(condition)) {
		return false, true
	}
	return false, false
}

// recordPathConditionFact activates a condition, or the negation of a pure
// integer condition, as a fact that holds on the current path: an else
// branch, the code after an if whose only continuing branch fixes the
// condition's value, or a loop body entry.
func (a *Analyzer) recordPathConditionFact(condition ast.Expression, holds bool) {
	if condition == nil {
		return
	}
	if !holds {
		// Only a pure integer condition or a variant test on a binding has
		// a sound negation.
		if negated, ok := negatedVariantStateTest(condition); ok {
			condition = negated
		} else if a.pureIntegerCondition(condition) {
			condition = negatedIntegerCondition(condition)
		} else {
			return
		}
	}
	a.activeConditionFacts = append(a.activeConditionFacts, activeConditionFact{
		fact:  ResolvedConditionFact{Kind: ConditionFactBranchTrue, Condition: condition},
		epoch: a.arrayIndexMutationEpoch,
	})
}

// diagnoseRelationallyUnreachableBlock reports the first statement of a
// branch whose condition earlier conditions on the path decide.
//
// Rules:
//   - rules/tooling/diagnostics.md — § 21(5) S3001 explains the control-flow fact
func (a *Analyzer) diagnoseRelationallyUnreachableBlock(block *ast.BlockStatement, conditionValue bool, region string) {
	if block == nil || len(block.Statements) == 0 {
		return
	}
	a.addErrorAtTokenWithMetadata(
		statementToken(block.Statements[0]),
		diagnostics.UnreachableStatement,
		fmt.Sprintf("Earlier conditions on this path prove the condition always %t, so this %s is impossible. Remove it or change the condition.", conditionValue, region),
		"unreachable statement",
	)
}

// enterLoopBodyFacts invalidates every comparison fact established before a
// loop body: the body is analyzed once, but its later iterations see values
// that the body itself has changed, so a fact from before the loop is not a
// fact at the start of every iteration.
func (a *Analyzer) enterLoopBodyFacts() {
	a.arrayIndexMutationEpoch++
}

// stableBindingsOf names the parameters and locals of a function whose value
// no statement of the function can change: the name is never an assignment
// target root, never mutably borrowed (`ref mut name`), and never mentioned
// inside a lambda or spawn, whose captures may interleave with the enclosing
// path. Local shadowing is not permitted, so a name identifies one binding of
// the function.
func stableBindingsOf(function *ast.FunctionDeclaration) map[string]bool {
	stable := map[string]bool{}
	if function == nil || function.Body == nil {
		return stable
	}
	changed := map[string]bool{}
	declared := map[string]bool{}
	for _, parameter := range function.Parameters {
		if parameter != nil && parameter.Name != nil {
			declared[parameter.Name.Value] = true
		}
	}
	markAll := func(value reflect.Value) {
		walkASTValue(value, func(node any) {
			if identifier, ok := node.(*ast.Identifier); ok && identifier != nil {
				changed[identifier.Value] = true
			}
		})
	}
	walkASTValue(reflect.ValueOf(function.Body), func(node any) {
		switch node := node.(type) {
		case *ast.LetStatement:
			if node != nil && node.Name != nil {
				declared[node.Name.Value] = true
			}
		case *ast.AssignmentStatement:
			if node != nil {
				if root, ok := expressionRootName(node.Target); ok {
					changed[root] = true
				}
			}
		case *ast.RefExpression:
			if node != nil && node.Mutable {
				if root, ok := expressionRootName(node.Value); ok {
					changed[root] = true
				}
			}
		case *ast.LambdaExpression:
			markAll(reflect.ValueOf(node))
		case *ast.SpawnExpression:
			markAll(reflect.ValueOf(node))
		}
	})
	for name := range declared {
		if !changed[name] {
			stable[name] = true
		}
	}
	return stable
}

// expressionRootName returns the binding at the root of a place expression.
func expressionRootName(expression ast.Expression) (string, bool) {
	switch expression := expression.(type) {
	case *ast.Identifier:
		if expression != nil {
			return expression.Value, true
		}
	case *ast.MemberExpression:
		if expression != nil {
			return expressionRootName(expression.Object)
		}
	case *ast.IndexExpression:
		if expression != nil {
			return expressionRootName(expression.Left)
		}
	case *ast.PrefixExpression:
		if expression != nil {
			return expressionRootName(expression.Right)
		}
	}
	return "", false
}

// walkASTValue visits every AST node pointer reachable from value.
func walkASTValue(value reflect.Value, visit func(any)) {
	seen := map[uintptr]bool{}
	var walk func(reflect.Value)
	walk = func(value reflect.Value) {
		if !value.IsValid() {
			return
		}
		switch value.Kind() {
		case reflect.Interface:
			if !value.IsNil() {
				walk(value.Elem())
			}
		case reflect.Pointer:
			if value.IsNil() || seen[value.Pointer()] {
				return
			}
			seen[value.Pointer()] = true
			if value.CanInterface() {
				visit(value.Interface())
			}
			walk(value.Elem())
		case reflect.Struct:
			for index := 0; index < value.NumField(); index++ {
				if value.Type().Field(index).IsExported() {
					walk(value.Field(index))
				}
			}
		case reflect.Slice, reflect.Array:
			for index := 0; index < value.Len(); index++ {
				walk(value.Index(index))
			}
		}
	}
	walk(value)
}
