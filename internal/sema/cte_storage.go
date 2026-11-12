package sema

import "sec/internal/lexer"

// These are deterministic compiler budgets in abstract storage units, not
// target bytes or allocation errors. A scalar cell and a retained UTF-8 byte
// each cost one unit; cumulative accounting includes copy-on-write storage.
// Rules: rules/compiler/compile_time_evaluation.md — §§24,48(3–6),49(5).
const (
	cteMaxAggregateValueSize = 1 << 16
	cteMaxStringValueSize    = 1 << 20
	cteMaxTransientMemory    = 1 << 22
)

// reserveTransientStorage bounds evaluator-owned allocation before constructing
// new backing storage. A failed reservation leaves the ledger unchanged; a
// later request starts with its own executor and budget. No target allocator,
// runtime generation, wall clock or host address participates in this model.
// Rules: rules/compiler/compile_time_evaluation.md — §§24–25,48–49.
func (x *cteExecutor) reserveTransientStorage(units int, at lexer.Token) bool {
	if x.failure != nil {
		return false
	}
	if units < 0 || units > cteMaxTransientMemory-x.transient {
		x.fail(cteResourceLimit, at, "the TransientMemory budget of %d evaluator storage units was reached", cteMaxTransientMemory)
		return false
	}
	x.transient += units
	return true
}

// reserveStringStorage checks each operand length before addition so neither
// host integer overflow nor an exponential concatenation can allocate first
// and report exhaustion later. This is a compiler ResourceLimit, never Err
// from the target allocator or a retryable Sec out-of-memory condition.
// Rules: rules/compiler/compile_time_evaluation.md — §§24(3),33(4),48–49.
func (x *cteExecutor) reserveStringStorage(left, right int, at lexer.Token) bool {
	if x.failure != nil {
		return false
	}
	if left < 0 || right < 0 || left > cteMaxStringValueSize || right > cteMaxStringValueSize-left {
		x.fail(cteResourceLimit, at, "the AggregateValueSize budget of %d UTF-8 bytes for a string was reached", cteMaxStringValueSize)
		return false
	}
	return x.reserveTransientStorage(left+right, at)
}

// aggregateSize is the flattened scalar count of a value.
// Rules: rules/compiler/compile_time_evaluation.md — §§24,32,48(4).
func aggregateSize(value DefaultConstant) int {
	if value.Aggregate == nil {
		return 1
	}
	size := 0
	for _, field := range value.Aggregate.Fields {
		size += aggregateSize(field)
	}
	if value.Aggregate.Payload != nil {
		size += aggregateSize(*value.Aggregate.Payload)
	}
	if size == 0 {
		size = 1
	}
	return size
}

// chargeAggregate applies the AggregateValueSize and TransientMemory budgets
// to a newly built or copied aggregate value.
// Rules: rules/compiler/compile_time_evaluation.md — §§24,26–27,32,48(4).
func (x *cteExecutor) chargeAggregate(value DefaultConstant, at lexer.Token) bool {
	if value.Aggregate != nil {
		x.dependOnType(value.Aggregate.Type)
	}
	x.assignOwner(value)
	size := aggregateSize(value)
	if size > cteMaxAggregateValueSize {
		x.fail(cteResourceLimit, at, "the AggregateValueSize budget of %d scalar values was reached", cteMaxAggregateValueSize)
		return false
	}
	return x.reserveTransientStorage(size, at)
}
