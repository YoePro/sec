package sema

import (
	"strings"

	"sec/internal/ast"
	"sec/internal/diagnostics"
)

// atomicOrderConstant retains compile-time order evidence on lexical bindings.
// Rules: rules/concurrency/atomics.md — §24(4–6).
type atomicOrderConstant struct {
	name    string
	outcome compileTimeOutcome
}

// atomicOrderNames is the single operation/order matrix used by validation
// and member documentation. CompareExchange failure is independent of success.
// Rules: rules/concurrency/atomics.md — §§14–16,18–20,23–26,45(5–9).
func atomicOrderNames(operation string, failure bool) []string {
	if failure || operation == "Load" {
		return []string{"Relaxed", "Acquire", "SeqCst"}
	}
	switch operation {
	case "Store":
		return []string{"Relaxed", "Release", "SeqCst"}
	case "Fence":
		return []string{"Acquire", "Release", "AcqRel", "SeqCst"}
	case "Swap", "CompareExchange", "FetchAdd", "FetchSub", "FetchAnd", "FetchOr", "FetchXor":
		return []string{"Relaxed", "Acquire", "Release", "AcqRel", "SeqCst"}
	}
	return nil
}

// atomicMemoryOrderValue resolves a canonical enum constant or an immutable
// binding snapshot. Mutable values and executable expressions are not guessed.
// Rules: rules/concurrency/atomics.md — §§5,24,45(5–7);
// rules/compiler/compile_time_evaluation.md — SemanticCompileTimeRequiredContext.
func (a *Analyzer) atomicMemoryOrderValue(expr ast.Expression) (string, compileTimeOutcome) {
	if id, ok := expr.(*ast.Identifier); ok {
		if symbol, found := a.symbols[id.Value]; found {
			if symbol.Mutable || symbol.Volatile || symbol.Parameter {
				return "", compileTimeNotConstant
			}
			if symbol.atomicMemoryOrder != nil {
				return symbol.atomicMemoryOrder.name, symbol.atomicMemoryOrder.outcome
			}
			if symbol.Local {
				return "", compileTimeNotConstant
			}
		}
	}
	constant, outcome := a.semanticCompileTimeConstantVisiting(expr, map[string]bool{}, a.types["MemoryOrder"])
	if outcome != compileTimeEvaluated {
		return "", outcome
	}
	if constant.Kind != EnumType || constant.Nominal.Owner != "MemoryOrder" {
		return "", compileTimeNotConstant
	}
	return constant.Nominal.Member, compileTimeEvaluated
}

// recordAtomicOrderBinding retains a proven order on the lexical symbol, so
// aliases, shadowing and scope restoration use the ordinary binding identity.
// Rules: rules/concurrency/atomics.md — §§5,24;
// rules/foundations/names_scopes_visibility.md — Lexical scopes.
func (a *Analyzer) recordAtomicOrderBinding(name string, expr ast.Expression) {
	symbol, ok := a.symbols[name]
	if !ok || symbol.Mutable || symbol.Volatile || !sameConcreteType(symbol.Type, a.types["MemoryOrder"]) {
		return
	}
	order, outcome := a.atomicMemoryOrderValue(expr)
	symbol.atomicMemoryOrder = &atomicOrderConstant{name: order, outcome: outcome}
	a.symbols[name] = symbol
}

// checkAtomicOrderArgument type-checks each explicit order and rejects a known
// invalid or non-compile-time operation/order pair before lowering. It never imposes a C-like
// relative-strength restriction on CompareExchange's separate order arguments.
// Rules: rules/concurrency/atomics.md — §§12–16,18–20,23–25,45(5–8);
// rules/compiler/compile_time_evaluation.md — §§4,7–8.
func (a *Analyzer) checkAtomicOrderArgument(call *ast.CallExpression, operation string, index int, failure bool) bool {
	_, valid := a.resolveAtomicOrderArgument(call, operation, index, failure)
	return valid
}

// resolveAtomicOrderArgument retains the established order for semantic path
// facts, using the same validation as ordinary atomic calls.
// Rules: rules/concurrency/atomics.md — §§24–25,45(5–8),46(2–4).
func (a *Analyzer) resolveAtomicOrderArgument(call *ast.CallExpression, operation string, index int, failure bool) (string, bool) {
	if index >= len(call.Arguments) {
		return "SeqCst", true // Ordinary omitted orders mean SeqCst (§25).
	}
	argument := call.Arguments[index]
	typ, _ := a.inferExpression(argument)
	if typ.Kind == InvalidType {
		return "", false
	}
	expected := a.types["MemoryOrder"]
	if !a.canInitialize(expected, typ, argument) {
		a.addErrorAtTokenWithMetadata(expressionToken(argument), diagnostics.AtomicInvalidMemoryOrder,
			"Use a compile-time-established value of the canonical MemoryOrder enum.",
			"Atomic.%s order argument %d must be MemoryOrder, got %s", operation, index+1, typeDisplayName(typ))
		return "", false
	}
	order, outcome := a.atomicMemoryOrderValue(argument)
	if outcome != compileTimeEvaluated {
		a.reportCompileTimeRequirement(argument, outcome, "Atomic."+operation+" order argument", diagnostics.AtomicOrderNotCompileTime,
			"Use a MemoryOrder enum variant or an immutable compile-time-established MemoryOrder binding.")
		return "", false
	}
	allowed := atomicOrderNames(operation, failure)
	for _, candidate := range allowed {
		if order == candidate {
			return order, true
		}
	}
	role := "Atomic." + operation
	if failure {
		role = "CompareExchange failure order"
	}
	a.addErrorAtTokenWithMetadata(expressionToken(argument), diagnostics.AtomicInvalidMemoryOrder,
		"Choose one of the allowed orders: "+strings.Join(allowed, ", ")+".",
		"MemoryOrder.%s is not valid for %s; allowed orders: %s", order, role, strings.Join(allowed, ", "))
	return "", false
}
