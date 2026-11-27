package sema

import "sec/internal/lexer"

// arrayDefault materializes the canonical element default, never the bounded
// AST preview. A zero-length array constructs no element. The evaluator's
// existing compiler budgets bound expansion before allocation, independently
// of the target's storage size; exhaustion is not a language restriction.
// Rules: rules/types/default_values.md — §31, §§40-41;
// rules/compiler/compile_time_evaluation.md — §§24,32,48-49.
func (x *cteExecutor) arrayDefault(resolution DefaultResolution, typ Type, at lexer.Token) (DefaultConstant, bool) {
	length, fixed := FixedArrayLength(typ)
	if !fixed || typ.Element == nil || resolution.ArrayLengthDecimal != length.String() || length.Sign() > 0 && resolution.ArrayElementDefault == nil {
		x.fail(cteCompilerInvariantFailure, at, "fixed-array default has no complete canonical length/element fact")
		return DefaultConstant{}, false
	}
	if !length.IsInt64() || length.Int64() >= cteMaxAggregateValueSize {
		x.fail(cteResourceLimit, at, "the AggregateValueSize budget of %d scalar values was reached while materializing a fixed-array default", cteMaxAggregateValueSize)
		return DefaultConstant{}, false
	}
	count := int(length.Int64())
	if !x.reserveTransientStorage(count, at) {
		return DefaultConstant{}, false
	}
	aggregate := &CompileTimeAggregate{Type: typ, Elements: make([]DefaultConstant, count)}
	for i := range aggregate.Elements {
		if !x.step(at) {
			return DefaultConstant{}, false
		}
		value, ok := x.defaultValue(*resolution.ArrayElementDefault, *typ.Element, at)
		if !ok {
			return DefaultConstant{}, false
		}
		aggregate.Elements[i] = value
	}
	return DefaultConstant{Kind: ArrayType, Aggregate: aggregate}, true
}
