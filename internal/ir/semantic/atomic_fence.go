package semantic

import "fmt"

// OpAtomicFence is an explicit memory-ordering fence for the current
// execution context (atomics.md §§ 26, 40). It has no operands, results or
// storage: it neither loads nor stores an Atomic[T] and adds no modification.
// Its compile-time-established MemoryOrder is AtomicOrder.
//
// Rules:
//   - rules/compiler/semantic_ir.md — § 80(8), § 122 "Concurrency v2 synchronization facts"
//   - rules/concurrency/atomics.md — §§ 24, 26, 40, 41(3)
const OpAtomicFence OpKind = "atomic.fence"

// validAtomicFenceOrder is the fence row of the atomics order matrix:
// Relaxed is never a fence order.
func validAtomicFenceOrder(order string) bool {
	switch order {
	case "Acquire", "Release", "AcqRel", "SeqCst":
		return true
	}
	return false
}

func verifyAtomicFence(op Operation) error {
	if len(op.Operands) != 0 || len(op.Results) != 0 || len(op.Successors) != 0 {
		return fmt.Errorf("invalid atomic.fence: a fence has no operands, results or successors")
	}
	if !validAtomicFenceOrder(op.AtomicOrder) {
		return fmt.Errorf("invalid atomic.fence order %q: a fence orders with Acquire, Release, AcqRel or SeqCst", op.AtomicOrder)
	}
	return nil
}
