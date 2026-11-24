package sema

import (
	"math/big"
	"reflect"
	"sort"
	"strings"

	"sec/internal/ast"
)

// ArenaDemandSummary is the compile-time byte demand of one Arena domain
// created in a callable (P19 §§ 79–81, 130): the peak over its Reset windows
// of the bytes its New/Alloc reserve, including worst-case alignment padding.
// Kind is "constant" with Bytes as a proven upper bound, "unknown" when a
// loop, lambda, layout, count or a callee receiving the Arena leaves the
// demand open, or "unbounded" when the Arena reaches an allocating recursive
// component (§ 81). Expression is the symbolic derivation (constant, sum,
// maximum, constant-multiply, range-upper-bound, unknown, unbounded). It is
// analysis metadata, never a runtime value.
// Rules: rules/mlir/packages/sec-mlir-dialect_package19.md — §§ 79–81, 130;
// rules/memory/arena.md — §§ 15–16, 22, 38.
type ArenaDemandSummary struct {
	Domain        InvalidationDomainID
	Kind          string
	Bytes         *big.Int
	Expression    string
	CapacityBytes *big.Int
}

// arenaDemand is one upper-bound demand expression.
type arenaDemand struct {
	known, unbounded bool
	bytes            *big.Int
	expr             string
}

// demandConstant is an exact constant byte demand.
// Rules: rules/mlir/packages/sec-mlir-dialect_package19.md — §§ 79–80.
func demandConstant(value *big.Int) arenaDemand {
	return arenaDemand{known: true, bytes: new(big.Int).Set(value), expr: "constant(" + value.String() + ")"}
}

// demandZero is the empty demand.
// Rules: rules/mlir/packages/sec-mlir-dialect_package19.md — §§ 79–80.
func demandZero() arenaDemand { return demandConstant(big.NewInt(0)) }

// demandUnknown is an open demand with its reason.
// Rules: rules/mlir/packages/sec-mlir-dialect_package19.md — §§ 79–80.
func demandUnknown(reason string) arenaDemand {
	return arenaDemand{expr: "unknown(" + reason + ")"}
}

// demandUnbounded is a demand without any finite bound (§ 81: unbounded
// recursion), with its reason.
// Rules: rules/mlir/packages/sec-mlir-dialect_package19.md — §§ 79–81.
func demandUnbounded(reason string) arenaDemand {
	return arenaDemand{unbounded: true, expr: "unbounded(" + reason + ")"}
}

// isZero reports a constant zero demand.
// Rules: rules/mlir/packages/sec-mlir-dialect_package19.md — §§ 79–80.
func (d arenaDemand) isZero() bool { return d.known && d.bytes.Sign() == 0 }

// demandSum adds two upper bounds; an unknown part makes the sum unknown.
// Rules: rules/mlir/packages/sec-mlir-dialect_package19.md — §§ 79–80.
func demandSum(left, right arenaDemand) arenaDemand {
	switch {
	case left.isZero():
		return right
	case right.isZero():
		return left
	case !left.known || !right.known:
		return arenaDemand{unbounded: left.unbounded || right.unbounded, expr: "sum(" + left.expr + "," + right.expr + ")"}
	}
	return arenaDemand{known: true, bytes: new(big.Int).Add(left.bytes, right.bytes), expr: "sum(" + left.expr + "," + right.expr + ")"}
}

// demandMax is the maximum of exclusive alternatives.
// Rules: rules/mlir/packages/sec-mlir-dialect_package19.md — §§ 79–80.
func demandMax(left, right arenaDemand) arenaDemand {
	switch {
	case left.isZero():
		return right
	case right.isZero():
		return left
	case left.expr == right.expr:
		return left
	case !left.known || !right.known:
		return arenaDemand{unbounded: left.unbounded || right.unbounded, expr: "maximum(" + left.expr + "," + right.expr + ")"}
	}
	value := left.bytes
	if right.bytes.Cmp(value) > 0 {
		value = right.bytes
	}
	return arenaDemand{known: true, bytes: new(big.Int).Set(value), expr: "maximum(" + left.expr + "," + right.expr + ")"}
}

// demandTimes repeats a demand a known number of times.
// Rules: rules/mlir/packages/sec-mlir-dialect_package19.md — §§ 79–80.
func demandTimes(count *big.Int, demand arenaDemand) arenaDemand {
	if count.Sign() == 0 || demand.isZero() {
		return demandZero()
	}
	if !demand.known {
		return arenaDemand{unbounded: demand.unbounded, expr: "constant-multiply(" + count.String() + "," + demand.expr + ")"}
	}
	return arenaDemand{known: true, bytes: new(big.Int).Mul(count, demand.bytes), expr: "constant-multiply(" + count.String() + "," + demand.expr + ")"}
}

// arenaDemandState is the demand of one domain along a statement sequence:
// pre is the demand before the first Reset, cur the open window, peak the
// largest closed window, reset whether a Reset occurred.
type arenaDemandState struct {
	pre, cur, peak arenaDemand
	reset          bool
}

// zeroDemandState is the state before any allocation.
// Rules: rules/mlir/packages/sec-mlir-dialect_package19.md — §§ 79–80.
func zeroDemandState() arenaDemandState {
	return arenaDemandState{pre: demandZero(), cur: demandZero(), peak: demandZero()}
}

// allocate adds one allocation to the open window.
// Rules: rules/mlir/packages/sec-mlir-dialect_package19.md — §§ 79–80.
func (s arenaDemandState) allocate(demand arenaDemand) arenaDemandState {
	s.cur = demandSum(s.cur, demand)
	if !s.reset {
		s.pre = s.cur
	}
	return s
}

// resetWindow closes the window at Reset (§ 80: a new demand window).
// Rules: rules/mlir/packages/sec-mlir-dialect_package19.md — §§ 79–80.
func (s arenaDemandState) resetWindow() arenaDemandState {
	s.peak = demandMax(s.peak, s.cur)
	s.cur, s.reset = demandZero(), true
	return s
}

// joinDemandStates merges exclusive branches that each started from the
// same incoming state: every component is the branch maximum.
// Rules: rules/mlir/packages/sec-mlir-dialect_package19.md — §§ 79–80.
func joinDemandStates(states []arenaDemandState) arenaDemandState {
	joined := states[0]
	for _, state := range states[1:] {
		joined.pre = demandMax(joined.pre, state.pre)
		joined.cur = demandMax(joined.cur, state.cur)
		joined.peak = demandMax(joined.peak, state.peak)
		joined.reset = joined.reset || state.reset
	}
	return joined
}

// repeat applies a loop body summarized from the zero state: count is the
// known trip count or nil. Without a Reset the body accumulates per
// iteration; with one, every window is bounded by the body's own pieces.
// Rules: rules/mlir/packages/sec-mlir-dialect_package19.md — §§ 79–80.
func (s arenaDemandState) repeat(body arenaDemandState, count *big.Int) arenaDemandState {
	if count != nil && count.Sign() == 0 {
		return s
	}
	if !body.reset {
		if body.cur.isZero() {
			return s
		}
		added := demandUnknown("loop without a constant trip count")
		if body.cur.unbounded {
			added = body.cur
		}
		if count != nil {
			added = demandTimes(count, body.cur)
		}
		return s.allocate(added)
	}
	s.peak = demandMax(demandMax(s.peak, demandSum(s.cur, body.pre)), demandMax(body.peak, demandSum(body.cur, body.pre)))
	if !s.reset {
		s.pre = demandSum(s.cur, body.pre)
	}
	s.cur, s.reset = body.cur, true
	return s
}

// arenaDemandWalker computes the demand of the domains created in one
// callable from the published Arena plans; it never re-resolves a call.
type arenaDemandWalker struct {
	analyzer *Analyzer
	domains  map[InvalidationDomainID]bool
	// names maps each summarized domain to its Sema ArenaDomainID, which
	// argument types carry when the Arena is passed to a call.
	names    map[InvalidationDomainID]string
	inLambda map[*ast.CallExpression]bool
}

// ArenaDemandSummariesOf summarizes every Arena domain created by
// Arena.FromBuffer in body, ordered by domain identity.
// Rules: rules/mlir/packages/sec-mlir-dialect_package19.md — §§ 79–81, 130.
func (a *Analyzer) ArenaDemandSummariesOf(body *ast.BlockStatement) []ArenaDemandSummary {
	if a == nil || body == nil || len(a.arenaOperationPlans) == 0 {
		return nil
	}
	walker := arenaDemandWalker{analyzer: a, domains: map[InvalidationDomainID]bool{}, names: map[InvalidationDomainID]string{}, inLambda: map[*ast.CallExpression]bool{}}
	capacity := map[InvalidationDomainID]*big.Int{}
	walkASTValue(reflect.ValueOf(body), func(value any) {
		if lambda, ok := value.(*ast.LambdaExpression); ok {
			walkASTValue(reflect.ValueOf(lambda), func(inner any) {
				if call, ok := inner.(*ast.CallExpression); ok {
					walker.inLambda[call] = true
				}
			})
		}
		call, ok := value.(*ast.CallExpression)
		if !ok {
			return
		}
		if plan, planned := a.arenaOperationPlans[call]; planned && plan.Operation == ArenaOperationCreate && plan.Constructor == "FromBuffer" && plan.InvalidationDomain != 0 {
			walker.domains[plan.InvalidationDomain] = true
			walker.names[plan.InvalidationDomain] = plan.Domain
			if plan.Capacity.BackingBytes != nil {
				capacity[plan.InvalidationDomain] = new(big.Int).Set(plan.Capacity.BackingBytes)
			}
		}
	})
	var summaries []ArenaDemandSummary
	for domain := range walker.domains {
		state := walker.block(body, domain, zeroDemandState())
		peak := demandMax(state.peak, state.cur)
		summary := ArenaDemandSummary{Domain: domain, Kind: "unknown", Expression: peak.expr, CapacityBytes: capacity[domain]}
		switch {
		case peak.known:
			summary.Kind, summary.Bytes = "constant", new(big.Int).Set(peak.bytes)
		case peak.unbounded:
			summary.Kind = "unbounded"
		}
		summaries = append(summaries, summary)
	}
	sort.Slice(summaries, func(i, j int) bool { return summaries[i].Domain < summaries[j].Domain })
	return summaries
}

// block walks a statement sequence in order.
// Rules: rules/mlir/packages/sec-mlir-dialect_package19.md — §§ 79–80.
func (w arenaDemandWalker) block(block *ast.BlockStatement, domain InvalidationDomainID, state arenaDemandState) arenaDemandState {
	if block == nil {
		return state
	}
	for _, statement := range block.Statements {
		state = w.statement(statement, domain, state)
	}
	return state
}

// statement applies if/for/while structure and walks the rest flat.
// Rules: rules/mlir/packages/sec-mlir-dialect_package19.md — §§ 79–80.
func (w arenaDemandWalker) statement(statement ast.Statement, domain InvalidationDomainID, state arenaDemandState) arenaDemandState {
	switch node := statement.(type) {
	case *ast.IfStatement:
		state = w.flat(node.Condition, domain, state)
		return joinDemandStates([]arenaDemandState{w.block(node.Consequence, domain, state), w.block(node.Alternative, domain, state)})
	case *ast.ForStatement:
		state = w.flat(node.Iterable, domain, state)
		body := w.block(node.Body, domain, zeroDemandState())
		return state.repeat(body, constantTripCount(node))
	case *ast.WhileStatement:
		body := w.flat(node.Condition, domain, zeroDemandState())
		body = w.block(node.Body, domain, body)
		return state.repeat(body, nil)
	}
	return w.flat(statement, domain, state)
}

// flat walks a statement or expression without its own structure: every
// allocation is sequential, which is an upper bound for conditional or
// exclusive evaluation. A loop nested inside such a node, or a lambda, makes
// the domain's allocations in it unknown, since they may repeat.
// Rules: rules/mlir/packages/sec-mlir-dialect_package19.md — §§ 79–80.
func (w arenaDemandWalker) flat(node any, domain InvalidationDomainID, state arenaDemandState) arenaDemandState {
	if node == nil || reflect.ValueOf(node).IsZero() {
		return state
	}
	loopCalls := map[*ast.CallExpression]bool{}
	walkASTValue(reflect.ValueOf(node), func(value any) {
		switch loop := value.(type) {
		case *ast.ForStatement, *ast.WhileStatement:
			walkASTValue(reflect.ValueOf(loop), func(inner any) {
				if call, ok := inner.(*ast.CallExpression); ok {
					loopCalls[call] = true
				}
			})
		}
	})
	walkASTValue(reflect.ValueOf(node), func(value any) {
		call, ok := value.(*ast.CallExpression)
		if !ok {
			return
		}
		plan, planned := w.analyzer.arenaOperationPlans[call]
		if !planned {
			if demand, passed := w.callDemand(call, w.names[domain]); passed {
				if (w.inLambda[call] || loopCalls[call]) && !demand.isZero() && !demand.unbounded {
					demand = demandUnknown("call receiving the Arena in a loop or lambda")
				}
				state = state.allocate(demand)
			}
			return
		}
		if plan.InvalidationDomain != domain {
			return
		}
		switch plan.Operation {
		case ArenaOperationReset:
			state = state.resetWindow()
		case ArenaOperationNew, ArenaOperationAlloc:
			demand := arenaAllocationDemand(plan)
			if w.inLambda[call] {
				demand = demandUnknown("allocation in a lambda")
			} else if loopCalls[call] {
				demand = demandUnknown("allocation in a nested loop")
			}
			state = state.allocate(demand)
		}
	})
	return state
}

// callDemand is the demand a call adds when it receives the domain's Arena
// (by value, move or reference): a closed callee without allocation effects
// adds nothing; an allocating callee in a recursive component is unbounded;
// any other allocating callee, and an open (indirect, interface or foreign)
// target, is unknown, since callee summaries do not bound demand on an Arena
// parameter (§ 81). passed is false when no argument carries the domain.
// Rules: rules/mlir/packages/sec-mlir-dialect_package19.md — §§ 80–81.
func (w arenaDemandWalker) callDemand(call *ast.CallExpression, domain string) (demand arenaDemand, passed bool) {
	if domain == "" {
		return arenaDemand{}, false
	}
	for _, argument := range call.Arguments {
		if arenaArgumentDomain(w.analyzer.expressionTypes[argument]) == domain {
			passed = true
			break
		}
	}
	if !passed {
		return arenaDemand{}, false
	}
	resolved, ok := w.analyzer.ResolvedCallTarget(call)
	if !ok || (resolved.Kind != ResolvedDirectCall && resolved.Kind != ResolvedStaticMethodCall) {
		return demandUnknown("open callable receiving the Arena"), true
	}
	name := resolved.Function.Name
	graph := w.analyzer.callGraph
	nodes := graph.NodesForDeclaration(resolved.Function.Token)
	if len(nodes) == 0 {
		return demandUnknown("callee " + name + " without a summary"), true
	}
	allocating, recursive := false, false
	for _, node := range nodes {
		summary := graph.ArenaSummary(node.ID)
		allocating = allocating || summary.MayAllocate || summary.AllocationUnknown
		recursive = recursive || graph.IsSameStackRecursive(node.ID)
	}
	switch {
	case !allocating:
		return demandZero(), true
	case recursive:
		return demandUnbounded("recursion through " + name), true
	}
	return demandUnknown("Arena passed to " + name), true
}

// arenaArgumentDomain is the ArenaDomainID an argument of Arena or
// reference-to-Arena type carries, or "".
// Rules: rules/memory/arena.md — §§ 4.2, 68.
func arenaArgumentDomain(typ Type) string {
	if typ.Name == "Arena" {
		return typ.ArenaDomainID
	}
	if typ.Kind == ReferenceType && typ.Element != nil && typ.Element.Name == "Arena" {
		return typ.Element.ArenaDomainID
	}
	return ""
}

// arenaAllocationDemand is the bytes one New/Alloc may reserve: count *
// size plus worst-case alignment padding (align - 1, the cursor alignment
// being unproven); a zero-element allocation reserves nothing (arena.md § 22).
// Rules: rules/memory/arena.md — §§ 16, 22–23.
func arenaAllocationDemand(plan ResolvedArenaOperationPlan) arenaDemand {
	if plan.ElementLayout == nil {
		return demandUnknown("unresolved layout of T")
	}
	count, bound := plan.Count, ""
	if count == nil && plan.CountUpperBound != nil {
		count, bound = plan.CountUpperBound, "range-upper-bound"
	}
	if count == nil {
		return demandUnknown("non-constant count")
	}
	if count.Sign() == 0 || plan.ElementLayout.SizeBytes.Sign() == 0 {
		return demandZero()
	}
	payload := new(big.Int).Mul(count, plan.ElementLayout.SizeBytes)
	padding := new(big.Int).Sub(plan.ElementLayout.AlignBytes, big.NewInt(1))
	bytes := new(big.Int).Add(payload, padding)
	expr := "constant(" + bytes.String() + ")"
	if bound != "" {
		expr = bound + "(" + count.String() + "," + strings.TrimSuffix(strings.TrimPrefix(expr, "constant("), ")") + ")"
	}
	return arenaDemand{known: true, bytes: bytes, expr: expr}
}

// constantTripCount is the iteration count of `for x in a..<b` / `a..b`
// with integer literal bounds and the default step; nil otherwise.
// Rules: rules/control-flow/flowcontrol_for.md — ranges.
func constantTripCount(loop *ast.ForStatement) *big.Int {
	if loop.Step != nil {
		return nil
	}
	rangeExpr, ok := loop.Iterable.(*ast.RangeExpression)
	if !ok || rangeExpr.Start == nil || rangeExpr.End == nil {
		return nil
	}
	start, startKnown := constantIntegerValue(rangeExpr.Start)
	end, endKnown := constantIntegerValue(rangeExpr.End)
	if !startKnown || !endKnown {
		return nil
	}
	count := new(big.Int).Sub(end, start)
	if !rangeExpr.Exclusive {
		count.Add(count, big.NewInt(1))
	}
	if count.Sign() < 0 {
		return big.NewInt(0)
	}
	return count
}
