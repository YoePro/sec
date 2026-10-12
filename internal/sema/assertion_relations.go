package sema

import (
	"math/big"
	"sec/internal/ast"
	"strings"
)

// relationProofOperand identifies a pure value or an immutable integer constant.
// Constant spellings are normalized by value, so literals, unary negatives and
// named constants can share a relation node without a syntax-level equality.
// Rules: rules/errors/panic.md — §§15.6,15.8(1); rules/foundations/operators.md — comparisons.
func (a *Analyzer) relationProofOperand(expr ast.Expression) (string, bool) {
	// Numeric coordinates in different units are not interchangeable constants.
	if a.expressionTypes[expr].Unit != "" {
		return relationOperandSpelling(expr)
	}
	if value, constant := a.constantConditionIntegerValue(expr); constant {
		return "$constant:" + value.String(), true
	}
	return relationOperandSpelling(expr)
}

// addConstantRelationEdges bridges relation chains through ordered constants.
// Only integer values proven immutable by the canonical constant evaluator
// contribute edges; current values of mutable bindings are never constants.
// Rules: rules/errors/panic.md — §15.8(1); rules/foundations/operators.md — integer comparisons.
func (a *Analyzer) addConstantRelationEdges(edges map[string][]relationEdge, left, right string) {
	constants := map[string]*big.Int{}
	add := func(key string) {
		if strings.HasPrefix(key, "$constant:") {
			if value, ok := new(big.Int).SetString(strings.TrimPrefix(key, "$constant:"), 10); ok {
				constants[key] = value
			}
		}
	}
	add(left)
	add(right)
	for from, targets := range edges {
		add(from)
		for _, target := range targets {
			add(target.to)
		}
	}
	for from, value := range constants {
		for to, other := range constants {
			if value.Cmp(other) < 0 {
				edges[from] = append(edges[from], relationEdge{to: to, strict: true})
			}
		}
	}
}

// relationEdge is one dominating fact `from <= to` (strict: `from < to`)
// between two pure values.
type relationEdge struct {
	to     string
	strict bool
}

// transitiveRelationImplies reports that a chain of dominating comparisons
// through other pure values implies the asserted comparison. Every fact is
// normalized to `x < y` or `x <= y` edges (`==` gives both directions, `!=`
// gives none); the claim holds when a path of sufficient strictness connects
// its operands: `l < r` needs a path from l to r with a strict edge, `l <= r`
// any path, `l == r` paths in both directions, and `l != r` a strict path in
// either direction. Only facts no mutation has invalidated participate.
//
// Rules:
//   - rules/errors/panic.md — § 15.6(1), (3) "Assertion refinement", § 15.8(1) proof on every path
func (a *Analyzer) transitiveRelationImplies(claim *ast.InfixExpression) bool {
	left, leftPure := a.relationProofOperand(claim.Left)
	right, rightPure := a.relationProofOperand(claim.Right)
	if !leftPure || !rightPure || left == right {
		return false
	}
	edges := map[string][]relationEdge{}
	for _, fact := range a.activeComparisonFacts() {
		from, okFrom := a.relationProofOperand(fact.Left)
		to, okTo := a.relationProofOperand(fact.Right)
		if !okFrom || !okTo || from == to {
			continue
		}
		switch fact.Operator {
		case "<":
			edges[from] = append(edges[from], relationEdge{to: to, strict: true})
		case "<=":
			edges[from] = append(edges[from], relationEdge{to: to})
		case ">":
			edges[to] = append(edges[to], relationEdge{to: from, strict: true})
		case ">=":
			edges[to] = append(edges[to], relationEdge{to: from})
		case "==":
			edges[from] = append(edges[from], relationEdge{to: to})
			edges[to] = append(edges[to], relationEdge{to: from})
		}
	}
	a.addConstantRelationEdges(edges, left, right)
	if len(edges) == 0 {
		return false
	}
	switch claim.Operator {
	case "<":
		_, strict := relationPath(edges, left, right)
		return strict
	case "<=":
		reachable, _ := relationPath(edges, left, right)
		return reachable
	case ">":
		_, strict := relationPath(edges, right, left)
		return strict
	case ">=":
		reachable, _ := relationPath(edges, right, left)
		return reachable
	case "==":
		forward, _ := relationPath(edges, left, right)
		backward, _ := relationPath(edges, right, left)
		return forward && backward
	case "!=":
		_, forward := relationPath(edges, left, right)
		_, backward := relationPath(edges, right, left)
		return forward || backward
	}
	return false
}

// relationPath searches the relation graph from one value to another and
// reports whether any path exists and whether some path contains a strict
// edge.
func relationPath(edges map[string][]relationEdge, from string, to string) (reachable bool, strict bool) {
	type state struct {
		node   string
		strict bool
	}
	seen := map[state]bool{{node: from}: true}
	queue := []state{{node: from}}
	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]
		for _, edge := range edges[current.node] {
			next := state{node: edge.to, strict: current.strict || edge.strict}
			if next.node == to {
				reachable = true
				if next.strict {
					return true, true
				}
			}
			if !seen[next] {
				seen[next] = true
				queue = append(queue, next)
			}
		}
	}
	return reachable, false
}
