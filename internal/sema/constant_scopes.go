package sema

import (
	"math/big"
	"reflect"

	"sec/internal/ast"
)

// restoreConstInts leaves a nested scope whose constants were analyzed in a
// copy. An outer binding keeps its known value only when the scope ended with
// the same value: an assignment on any path through the scope (a branch, arm,
// handler or loop body) may reach the join, so the outer constant no longer
// holds there. A binding the scope declared itself shadows the outer one and
// is dropped with the copy.
//
// Rules: rules/types/types.md — int and uint (only definite overflow is a
// compile-time error); rules/control-flow/flowcontrol_if.md — joins.
func (a *Analyzer) restoreConstInts(previous map[string]*big.Int) {
	inner := a.constInts
	a.constInts = previous
	for name, value := range previous {
		if current, ok := inner[name]; !ok || current.Cmp(value) != 0 {
			delete(previous, name)
		}
	}
}

// forgetLoopAssignedConstants removes, at loop body entry, the known values
// of bindings the body assigns: the body runs again with the values a
// previous iteration left, so the pre-loop constant holds only on the first
// entry and cannot prove a definite overflow (for example
// `while n > 0 { n -= 1 }` entered with n == 0, where the body never runs).
//
// The forgotten values are kept in a first-iteration domain used only to warn
// about an overflow certain on the first iteration (S1023 is a warning;
// operators.md "Compile-time overflow"). The returned function ends that
// domain and must run right after the first analysis of the body.
func (a *Analyzer) forgetLoopAssignedConstants(body *ast.BlockStatement, firstIterationReachable bool, condition ast.Expression) func() {
	previousFirst, previousDepth := a.firstIterationConsts, a.firstIterationDepth
	restore := func() { a.firstIterationConsts, a.firstIterationDepth = previousFirst, previousDepth }
	if body == nil || len(a.constInts) == 0 {
		a.firstIterationConsts = nil
		return restore
	}
	first := map[string]*big.Int{}
	for _, name := range a.loopAssignedNames(body) {
		if value, known := a.constInts[name]; known && firstIterationReachable && !a.summaryPass {
			first[name] = new(big.Int).Set(value)
		}
		delete(a.constInts, name)
	}
	// The body runs a first time only when the loop condition can hold with
	// these values; an unknown condition over a loop-assigned binding proves
	// nothing (`while n > 0 { n -= 1 }` entered with n == 0 never runs).
	if condition != nil && len(first) > 0 {
		if holds, known := a.firstIterationCondition(condition, first); known && !holds || !known && a.mentionsAny(condition, a.loopAssignedNames(body)) {
			first = map[string]*big.Int{}
		}
	}
	a.firstIterationConsts, a.firstIterationDepth = first, a.scopeDepth+1
	return restore
}

// firstIterationAssignmentValue evaluates an assignment to a loop-assigned
// binding with its first-iteration value, which is still exact while every
// earlier write in the body ran unconditionally at the body's top level.
func (a *Analyzer) firstIterationAssignmentValue(name string, stmt *ast.AssignmentStatement) (*big.Int, bool) {
	value, known := a.firstIterationConsts[name]
	if !known {
		return nil, false
	}
	if _, tracked := a.constInts[name]; tracked {
		return nil, false
	}
	a.constInts[name] = value
	result, ok := a.assignmentIntegerValue(name, stmt)
	delete(a.constInts, name)
	return result, ok
}

// advanceFirstIterationValue follows an assignment through the
// first-iteration domain: a top-level write keeps the exact next value, and a
// write inside a nested branch or loop may not run, so it ends the domain for
// that binding.
func (a *Analyzer) advanceFirstIterationValue(name string, stmt *ast.AssignmentStatement, overflowed bool) {
	if _, known := a.firstIterationConsts[name]; !known {
		return
	}
	if a.scopeDepth != a.firstIterationDepth || overflowed {
		delete(a.firstIterationConsts, name)
		return
	}
	if next, ok := a.firstIterationAssignmentValue(name, stmt); ok {
		a.firstIterationConsts[name] = next
		return
	}
	delete(a.firstIterationConsts, name)
}

// mergeBranchExitConstants restores constants a join can still prove: every
// continuing branch ended with the same value (restoreConstInts already keeps
// a value no branch changed).
func (a *Analyzer) mergeBranchExitConstants(exits []map[string]*big.Int) {
	if len(exits) == 0 {
		return
	}
	for name, value := range exits[0] {
		if _, known := a.constInts[name]; known {
			continue
		}
		if _, visible := a.symbols.lookup(name); !visible {
			continue
		}
		common := true
		for _, other := range exits[1:] {
			if candidate, ok := other[name]; !ok || candidate.Cmp(value) != 0 {
				common = false
				break
			}
		}
		if common {
			a.constInts[name] = new(big.Int).Set(value)
		}
	}
}

// wantBlockExitConstants asks analyzeBlockStatements to keep the constants a
// block ends with for the join that follows it.
func (a *Analyzer) wantBlockExitConstants(blocks ...*ast.BlockStatement) {
	if a.blockExitConstInts == nil {
		a.blockExitConstInts = map[*ast.BlockStatement]map[string]*big.Int{}
	}
	for _, block := range blocks {
		if block != nil {
			a.blockExitConstInts[block] = nil
		}
	}
}

// loopAssignedNames lists the plain binding names a loop body assigns,
// cached per body because loops are analyzed again for their fixed point.
func (a *Analyzer) loopAssignedNames(body *ast.BlockStatement) []string {
	if names, ok := a.loopAssignments[body]; ok {
		return names
	}
	seen := map[string]bool{}
	var names []string
	walkASTValue(reflect.ValueOf(body), func(node any) {
		assignment, ok := node.(*ast.AssignmentStatement)
		if !ok {
			return
		}
		if target, ok := assignment.Target.(*ast.Identifier); ok && target != nil && !seen[target.Value] {
			seen[target.Value] = true
			names = append(names, target.Value)
		}
	})
	if a.loopAssignments == nil {
		a.loopAssignments = map[*ast.BlockStatement][]string{}
	}
	a.loopAssignments[body] = names
	return names
}

// firstIterationCondition evaluates a loop condition built from `&&`, `||`,
// `!`, boolean literals and integer comparisons whose operands are constants
// or first-iteration values. Anything else is unknown.
func (a *Analyzer) firstIterationCondition(condition ast.Expression, first map[string]*big.Int) (bool, bool) {
	switch condition := condition.(type) {
	case *ast.BooleanLiteral:
		return condition.Value, true
	case *ast.PrefixExpression:
		if condition.Operator == "!" {
			value, known := a.firstIterationCondition(condition.Right, first)
			return !value, known
		}
	case *ast.InfixExpression:
		switch condition.Operator {
		case "&&", "||":
			left, leftKnown := a.firstIterationCondition(condition.Left, first)
			right, rightKnown := a.firstIterationCondition(condition.Right, first)
			if condition.Operator == "&&" {
				if leftKnown && !left || rightKnown && !right {
					return false, true
				}
				return true, leftKnown && rightKnown
			}
			if leftKnown && left || rightKnown && right {
				return true, true
			}
			return false, leftKnown && rightKnown
		case "<", "<=", ">", ">=", "==", "!=":
			left, leftKnown := a.firstIterationOperand(condition.Left, first)
			right, rightKnown := a.firstIterationOperand(condition.Right, first)
			if !leftKnown || !rightKnown {
				return false, false
			}
			comparison := left.Cmp(right)
			switch condition.Operator {
			case "<":
				return comparison < 0, true
			case "<=":
				return comparison <= 0, true
			case ">":
				return comparison > 0, true
			case ">=":
				return comparison >= 0, true
			case "==":
				return comparison == 0, true
			default:
				return comparison != 0, true
			}
		}
	}
	return false, false
}

func (a *Analyzer) firstIterationOperand(expr ast.Expression, first map[string]*big.Int) (*big.Int, bool) {
	if identifier, ok := expr.(*ast.Identifier); ok && identifier != nil {
		if value, known := first[identifier.Value]; known {
			return value, true
		}
	}
	return a.integerConstantValue(expr)
}

// mentionsAny reports an identifier in expr named by one of names.
func (a *Analyzer) mentionsAny(expr ast.Expression, names []string) bool {
	wanted := map[string]bool{}
	for _, name := range names {
		wanted[name] = true
	}
	found := false
	walkASTValue(reflect.ValueOf(expr), func(node any) {
		if identifier, ok := node.(*ast.Identifier); ok && identifier != nil && wanted[identifier.Value] {
			found = true
		}
	})
	return found
}

// constantBranches gives every sibling branch of one join (if/else arms,
// match arms, switch clauses) the constants from before the join, instead of
// the shared map an earlier sibling's restoreConstInts already pruned, and
// leaves the join with the intersection of what each branch preserved.
type constantBranches struct {
	before  map[string]*big.Int
	joined  map[string]*big.Int
	entered bool
}

func (a *Analyzer) startConstantBranches() *constantBranches {
	return &constantBranches{before: copyConstInts(a.constInts)}
}

// enter starts the next sibling from the pre-join constants.
func (b *constantBranches) enter(a *Analyzer) {
	b.collect(a)
	a.constInts = copyConstInts(b.before)
}

// keep starts the next sibling from the current constants, for an entry
// (such as a switch fallthrough) whose state differs from the pre-join one.
func (b *constantBranches) keep(a *Analyzer) {
	b.collect(a)
}

func (b *constantBranches) collect(a *Analyzer) {
	if !b.entered {
		b.entered = true
		b.joined = copyConstInts(a.constInts)
		return
	}
	for name, value := range b.joined {
		if current, ok := a.constInts[name]; !ok || current.Cmp(value) != 0 {
			delete(b.joined, name)
		}
	}
}

// finish leaves the join with the constants every sibling preserved.
func (b *constantBranches) finish(a *Analyzer) {
	b.collect(a)
	a.constInts = b.joined
}

// intersectConstInts returns the constants two incoming edges agree on. A nil
// edge state is unknown and contributes nothing.
func intersectConstInts(left, right map[string]*big.Int) map[string]*big.Int {
	out := map[string]*big.Int{}
	for name, value := range left {
		if other, ok := right[name]; ok && other.Cmp(value) == 0 {
			out[name] = new(big.Int).Set(value)
		}
	}
	return out
}
