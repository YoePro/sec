package sema

import (
	"sec/internal/ast"
	"sec/internal/diagnostics"
)

// recordRawPointerNullBinding marks an immutable RawPtr[T] binding whose
// initializer is the null sentinel. The binding cannot be reassigned, so the
// fact holds for its whole scope.
func (a *Analyzer) recordRawPointerNullBinding(name string, value ast.Expression) {
	symbol, ok := a.symbols.lookup(name)
	if !ok || symbol.Mutable || symbol.Volatile || symbol.Type.Kind != RawPtrType || !a.isNullSentinel(value) {
		return
	}
	symbol.RawPointerNull = true
	a.symbols.set(name, symbol)
}

// rejectProvenNullDereference reports raw access or safe-reference formation
// whose receiver is proven null: an immutable binding initialized with null, or a
// stable binding (never assigned, mutably borrowed, or captured in the
// function; stableBindingsOf) that a dominating `is null` test or the failed
// edge of `is not null` fixes as null on the current path. Parameters are
// assignable, so only stability keeps a path fact valid. Unsafe
// accepts obligations the compiler cannot prove; it does not make a proven
// null dereference valid.
//
// Rules:
//   - rules/memory/raw_pointers.md — § 7(6)-(8), § 34(2)-(4) "null dereference known statically"
//   - rules/memory/unsafe.md — § 8(6), § 21(2), § 50(2) "unsafe.raw-pointer-null"
func (a *Analyzer) rejectProvenNullDereference(member *ast.MemberExpression, operation string) bool {
	ident, ok := member.Object.(*ast.Identifier)
	if !ok {
		return false
	}
	symbol, ok := a.symbols.lookup(ident.Value)
	if !ok || symbol.Volatile {
		return false
	}
	proven := symbol.RawPointerNull && !symbol.Mutable
	if !proven && a.currentStableBindings[ident.Value] {
		proven = a.activeFactsProveNull(ident.Value)
	}
	if !proven {
		return false
	}
	access := "read"
	if operation == "Write" || operation == "VolatileWrite" {
		access = "write"
	}
	if operation == "AsRef" || operation == "AsRefMut" {
		access = "reference formation"
	}
	a.reportRawPointerProvenInvalid(member.Property.Token, symbol.Token, diagnostics.RawPointerNullDereference, "raw-pointer binding",
		"unsafe can accept obligations that the compiler cannot prove, but it does not make a proven null dereference or reference formation valid. Use valid live storage, or take only a non-null control-flow path before accessing it; a check cannot make an always-null pointer usable.",
		"this raw-pointer %s through %s is known to use a null address (RawPtr.%s)", access, ident.Value, operation)
	return true
}

func (a *Analyzer) activeFactsProveNull(subject string) bool {
	for _, active := range a.activeConditionFacts {
		if active.fact.Condition == nil {
			continue
		}
		if conditionProvesNull(active.fact.Condition, subject, active.fact.Kind != ConditionFactLogicalRHSFalse) {
			return true
		}
	}
	return false
}

// conditionProvesNull reports whether condition evaluating to truth proves
// subject null: `subject is null` true, `subject is not null` false, through
// `!`, a true conjunction, and a false disjunction.
func conditionProvesNull(condition ast.Expression, subject string, truth bool) bool {
	switch expr := condition.(type) {
	case *ast.NullTestExpression:
		ident, ok := expr.Subject.(*ast.Identifier)
		return ok && ident.Value == subject && expr.Negated != truth
	case *ast.PrefixExpression:
		return expr.Operator == "!" && conditionProvesNull(expr.Right, subject, !truth)
	case *ast.InfixExpression:
		switch {
		case expr.Operator == "&&" && truth, expr.Operator == "||" && !truth:
			return conditionProvesNull(expr.Left, subject, truth) || conditionProvesNull(expr.Right, subject, truth)
		}
	}
	return false
}

// negatedNullTest flips a null test on a binding, which is pure, so its
// false edge can be recorded as a path fact.
func negatedNullTest(condition ast.Expression) (ast.Expression, bool) {
	test, ok := condition.(*ast.NullTestExpression)
	if !ok || test == nil {
		return nil, false
	}
	if _, ok := test.Subject.(*ast.Identifier); !ok {
		return nil, false
	}
	negated := *test
	negated.Negated = !test.Negated
	return &negated, true
}
