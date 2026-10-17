package sema

import (
	"sec/internal/ast"
	"sec/internal/lexer"
)

// CompareExchangePath preserves one normal outcome, its independent order and
// its memory-model classification. Only NotExchanged has an observed payload;
// ObservedBy names the exact operation that supplies it, never a later Load.
// Rules: rules/concurrency/atomics.md — §§6,17(1–10),24–25;
// rules/concurrency/concurrency_memory_model.md — §§16–17,24–25.
type CompareExchangePath struct {
	Variant                  string
	Order                    string
	ReadsStorage             bool
	ReadModifyWrite          bool
	ModifiesStorage          bool
	MayExtendReleaseSequence bool
	PayloadType              *Type
	ObservedBy               *ast.CallExpression
}

// ResolvedCompareExchange retains the two possible paths of one strong atomic
// operation. Storage, Expected and Desired are resolved source-node identities,
// not expressions to re-evaluate separately on either outcome. A path being
// present does not prove its runtime selection or target availability.
// Rules: rules/concurrency/atomics.md — §§17,27,45–46;
// rules/concurrency/concurrency_memory_model.md — §52(4).
type ResolvedCompareExchange struct {
	Storage     ast.Expression
	Expected    ast.Expression
	Desired     ast.Expression
	ElementType Type
	ResultType  Type
	Source      lexer.Token
	Success     CompareExchangePath
	Failure     CompareExchangePath
}

// recordCompareExchange freezes successful and failed paths only after value
// and order validation. Failure is read-only and carries the same operation's T.
// Rules: rules/concurrency/atomics.md — §§17(2–10),24–25,46(3–4).
func (a *Analyzer) recordCompareExchange(call *ast.CallExpression, storage ast.Expression, element, result Type, successOrder, failureOrder string) {
	payload := semanticSnapshotType(element)
	a.resolvedCompareExchanges[call] = ResolvedCompareExchange{
		Storage: storage, Expected: call.Arguments[0], Desired: call.Arguments[1],
		ElementType: semanticSnapshotType(element), ResultType: semanticSnapshotType(result),
		Source: call.Token,
		Success: CompareExchangePath{
			Variant: "Exchanged", Order: successOrder, ReadsStorage: true,
			ReadModifyWrite: true, ModifiesStorage: true, MayExtendReleaseSequence: true,
		},
		Failure: CompareExchangePath{
			Variant: "NotExchanged", Order: failureOrder, ReadsStorage: true,
			PayloadType: &payload, ObservedBy: call,
		},
	}
}

// ResolvedCompareExchangeOf returns detached type facts for an analyzed atomic
// call. AST pointers are immutable source identities; querying never re-runs
// inference, CTE or a load and does not create a target capability claim.
// Rules: rules/compiler/compiler_analysis.md — immutable analysis results;
// rules/concurrency/atomics.md — §§17(5–10),37(1),46(3–4).
func (a *Analyzer) ResolvedCompareExchangeOf(call *ast.CallExpression) (ResolvedCompareExchange, bool) {
	if a == nil || call == nil {
		return ResolvedCompareExchange{}, false
	}
	plan, ok := a.resolvedCompareExchanges[call]
	if !ok {
		return ResolvedCompareExchange{}, false
	}
	plan.ElementType = semanticSnapshotType(plan.ElementType)
	plan.ResultType = semanticSnapshotType(plan.ResultType)
	if plan.Failure.PayloadType != nil {
		payload := semanticSnapshotType(*plan.Failure.PayloadType)
		plan.Failure.PayloadType = &payload
	}
	return plan, true
}
