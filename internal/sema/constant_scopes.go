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
func (a *Analyzer) forgetLoopAssignedConstants(body *ast.BlockStatement) {
	if body == nil || len(a.constInts) == 0 {
		return
	}
	for _, name := range a.loopAssignedNames(body) {
		delete(a.constInts, name)
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
