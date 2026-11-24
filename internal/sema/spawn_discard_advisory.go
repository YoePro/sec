package sema

import (
	"fmt"
	"sec/internal/ast"
	"strings"
)

// explainVoidSpawnDiscard enriches the existing final-call occurrence only
// after a valid void child has been checked. It preserves the occurrence key,
// source/provenance and deferred policy application, and offers no return edit
// whose transferability would require a different child contract.
// Rules: rules/concurrency/spawn.md §3A(8); rules/tooling/diagnostics.md §25(8).
func (a *Analyzer) explainVoidSpawnDiscard(expr *ast.SpawnExpression, result Type, errorsBefore int) {
	if a.summaryPass || a.iterationDependencyProbe || len(a.errors) != errorsBefore || result.Kind != VoidType || expr.Body == nil || len(expr.Body.Statements) == 0 || spawnBlockHasValueReturn(expr.Body) {
		return
	}
	last, ok := expr.Body.Statements[len(expr.Body.Statements)-1].(*ast.ExpressionStatement)
	if !ok {
		return
	}
	if _, ordinary := last.Expression.(*ast.CallExpression); !ordinary {
		return
	}
	fact, recorded := a.implicitDiscards[last.Expression]
	if !recorded {
		return
	}
	owner := ""
	switch expr.Kind {
	case "", "task":
		owner = "Task"
	case "thread":
		owner = "Thread"
	case "process":
		owner = "Process"
	default:
		return
	}
	suffix := fmt.Sprintf(". This spawn block has no explicit value return, so its successful handle is %s[void]", owner)
	if !strings.HasSuffix(fact.Message, suffix) {
		fact.Message += suffix
	}
	fact.Help = "A final call is not an implicit return from a spawn block. Write `discard` before the call if ignoring its result is intentional."
	a.implicitDiscards[last.Expression] = fact
}
