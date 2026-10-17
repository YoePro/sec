package sema

import (
	"strings"

	"sec/internal/ast"
	"sec/internal/diagnostics"
)

// atomicFenceName is the canonical spelling of the fence function in the
// core `atomic` namespace (atomics.md § 3(3), § 26).
const atomicFenceName = "atomic.Fence"

// atomicFenceMemberID is the stable compiler-known identity of atomic.Fence
// for hover and synthetic navigation (atomics.md § 26, tooling.atomics-v2).
const atomicFenceMemberID = "CKF-ATOMIC-FENCE"

// AtomicFenceMember is the compiler-known presentation of atomic.Fence; its
// signature, documentation and rule come from the synthetic catalog.
func AtomicFenceMember() CompilerKnownMember {
	return withSyntheticMetadata(CompilerKnownMember{ID: atomicFenceMemberID, Name: "Fence", Kind: CompilerKnownAssociatedFunction, Result: Type{Name: "void", Kind: VoidType}})
}

// recordAtomicFenceSymbol attaches the compiler-known fence identity to the
// `Fence` name of a resolved call so LSP hover and go-to-definition reach
// its canonical synthetic definition; a fence with an established order
// names that order in its presentation.
func (a *Analyzer) recordAtomicFenceSymbol(expr *ast.CallExpression, order string) {
	member, ok := expr.Callee.(*ast.MemberExpression)
	if !ok || member.Property == nil {
		return
	}
	known := AtomicFenceMember()
	if order != "" {
		known.BoundReceiver = "MemoryOrder." + order
		known.Documentation += " This call establishes MemoryOrder." + order + "."
	}
	a.compilerKnownMemberFacts[sourceTokenLocation(member.Property.Token)] = known
}

// isAtomicFenceCall reports whether a call names the core fence function and
// is not shadowed by a local value named `atomic`.
func (a *Analyzer) isAtomicFenceCall(expr *ast.CallExpression) bool {
	if callExpressionName(expr) != atomicFenceName {
		return false
	}
	_, shadowed := a.symbols["atomic"]
	return !shadowed
}

// inferAtomicFence types `atomic.Fence(order)`: exactly one explicit
// MemoryOrder argument (there is no default), established at compile time,
// and one of Acquire, Release, AcqRel or SeqCst — Relaxed is rejected. The
// call is void and accesses no Atomic[T] value; the established order is
// retained for later semantic consumers.
//
// Rules:
//   - rules/concurrency/atomics.md — §§ 3(3), 24, 25(5), 26, 40, 45(9), 48
//   - rules/compiler/compile_time_evaluation.md — SemanticCompileTimeRequiredContext
func (a *Analyzer) inferAtomicFence(expr *ast.CallExpression) (Type, expressionValue, bool) {
	void := Type{Name: "void", Kind: VoidType}
	result := expressionValue{Display: expr.String()}
	a.recordAtomicFenceSymbol(expr, "")
	if len(expr.GenericArguments) > 0 {
		a.addErrorAtToken(expr.Token, "atomic.Fence does not take generic arguments")
		return Type{Kind: InvalidType}, result, true
	}
	if len(expr.Arguments) != 1 {
		a.addErrorAtTokenWithMetadata(expr.Token, diagnostics.AtomicInvalidMemoryOrder,
			"Pass the order explicitly, for example atomic.Fence(MemoryOrder.Acquire); a fence has no default order.",
			"atomic.Fence requires exactly one MemoryOrder argument, got %d", len(expr.Arguments))
		return Type{Kind: InvalidType}, result, true
	}
	argument := expr.Arguments[0]
	expected := a.types["MemoryOrder"]
	typ, _ := a.inferExpressionWithExpected(argument, expected)
	if typ.Kind == InvalidType {
		return Type{Kind: InvalidType}, result, true
	}
	if !a.canInitialize(expected, typ, argument) {
		a.addErrorAtTokenWithMetadata(expressionToken(argument), diagnostics.AtomicInvalidMemoryOrder,
			"Use a compile-time-established value of the canonical MemoryOrder enum.",
			"atomic.Fence order must be MemoryOrder, got %s", typeDisplayName(typ))
		return Type{Kind: InvalidType}, result, true
	}
	order, outcome := a.atomicMemoryOrderValue(argument)
	if outcome != compileTimeEvaluated {
		a.reportCompileTimeRequirement(argument, outcome, "atomic.Fence order argument", diagnostics.AtomicOrderNotCompileTime,
			"Use a MemoryOrder enum variant or an immutable compile-time-established MemoryOrder binding.")
		return Type{Kind: InvalidType}, result, true
	}
	allowed := atomicOrderNames("Fence", false)
	for _, candidate := range allowed {
		if order == candidate {
			a.atomicFenceOrders[expr] = order
			a.recordAtomicFenceSymbol(expr, order)
			return void, result, true
		}
	}
	a.addErrorAtTokenWithMetadata(expressionToken(argument), diagnostics.AtomicInvalidMemoryOrder,
		"Choose one of the allowed fence orders: "+strings.Join(allowed, ", ")+". A Relaxed fence orders nothing.",
		"MemoryOrder.%s is not valid for atomic.Fence; allowed orders: %s", order, strings.Join(allowed, ", "))
	return Type{Kind: InvalidType}, result, true
}

// AtomicFenceOrderOf returns the compile-time-established order of a valid
// `atomic.Fence` call for Semantic IR and tooling consumers.
func (a *Analyzer) AtomicFenceOrderOf(call *ast.CallExpression) (string, bool) {
	order, ok := a.atomicFenceOrders[call]
	return order, ok
}
