package sema

import (
	"fmt"
	"sort"

	"sec/internal/ast"
)

// AtomicExecutionEvent binds one dynamic occurrence to an already validated
// source operation/outcome. Context and Sequence are producer-proven execution
// instance and sequenced-before facts, NOT callable IDs or source offsets.
// ReadsFrom identifies the exact observed modification, never an equal value.
// For RMW it is also the immediately preceding modification (§17). Empty means
// unavailable; the query cannot infer communication. The producer must normalize
// lifetime/replacement/transfer into the supplied resource snapshot and prove
// these occurrences coexist. This is a conditional execution witness, not a
// source-program reachability proof or a complete permitted-execution checker.
// Rules: rules/concurrency/concurrency_memory_model.md §§7–9,16–18,24–33,52.
type AtomicExecutionEvent struct {
	ID, Context string
	Sequence    uint64
	Call        *ast.CallExpression
	Path        int
	ReadsFrom   string
}

// AtomicFenceSynchronization retains both synchronization endpoints and the
// actual atomic carrier. Modification can be a release head whose value is
// carried to ObservedModification through a contiguous RMW chain; Read identifies
// the consumer outcome. IDs refer exclusively to the supplied execution witness.
// Rules: rules/concurrency/concurrency_memory_model.md §§25–33.
type AtomicFenceSynchronization struct {
	From, To                                 string
	Modification, ObservedModification, Read string
}

// AtomicFenceSynchronizationResult lists proven conditional edges. UnresolvedReads
// explicitly retains missing read-from/chain or uncertain object identity; absence of
// edges never proves race freedom, absence of publication, or whole-program safety.
// Rules: rules/concurrency/concurrency_memory_model.md §§28–33,52,56.
type AtomicFenceSynchronizationResult struct {
	Edges           []AtomicFenceSynchronization
	UnresolvedReads []string
}

// atomicWitnessEvent holds canonical Sema facts for a caller-supplied occurrence.
// Rules: rules/concurrency/concurrency_memory_model.md §§24–33,52.
type atomicWitnessEvent struct {
	AtomicExecutionEvent
	operation ResolvedAtomicMemoryOperation
	path      AtomicMemoryPath
	fence     string
}

// ResolveAtomicFenceSynchronization implements all three fence communication
// patterns using canonical source operations plus explicit execution evidence.
// It preserves overlapping release heads, excludes failed CAS from modification
// chains, and stops at a plain Store. Malformed witnesses fail atomically.
// Source order, call names, equal values and alias uncertainty never prove an
// edge. Runtime witness production, data-race solving and global SeqCst
// consistency are independent consumers/contracts, not supplied by this query.
// Rules: rules/concurrency/concurrency_memory_model.md §§16–18,24–33,52,55(13).
func (a *Analyzer) ResolveAtomicFenceSynchronization(events []AtomicExecutionEvent, state *SynchronizationResourceSnapshot) (AtomicFenceSynchronizationResult, error) {
	var result AtomicFenceSynchronizationResult
	if a == nil {
		return result, fmt.Errorf("atomic synchronization requires an analysis snapshot")
	}
	nodes := make(map[string]atomicWitnessEvent, len(events))
	positions := make(map[string]map[uint64]bool)
	ids := make([]string, 0, len(events))
	for _, event := range events {
		if event.ID == "" || event.Context == "" || event.Sequence == 0 {
			return result, fmt.Errorf("atomic event requires identity, execution context and nonzero sequence")
		}
		if _, duplicate := nodes[event.ID]; duplicate {
			return result, fmt.Errorf("duplicate atomic event %q", event.ID)
		}
		if positions[event.Context] == nil {
			positions[event.Context] = map[uint64]bool{}
		}
		if positions[event.Context][event.Sequence] {
			return result, fmt.Errorf("duplicate execution position for %q", event.ID)
		}
		positions[event.Context][event.Sequence] = true
		node := atomicWitnessEvent{AtomicExecutionEvent: event}
		if order, ok := a.resolvedAtomicFences[event.Call]; ok {
			if event.Path != 0 || event.ReadsFrom != "" {
				return result, fmt.Errorf("fence %q has no outcome or read-from", event.ID)
			}
			node.fence = order
		} else {
			operation, ok := a.ResolvedAtomicMemoryOperationOf(event.Call)
			if !ok || event.Path < 0 || event.Path >= len(operation.Paths) {
				return result, fmt.Errorf("event %q has no canonical atomic outcome", event.ID)
			}
			node.operation, node.path = operation, operation.Paths[event.Path]
			if event.ReadsFrom != "" && !node.path.ReadsStorage {
				return result, fmt.Errorf("non-reading event %q has read-from", event.ID)
			}
		}
		nodes[event.ID] = node
		ids = append(ids, event.ID)
	}
	sort.Strings(ids)
	// Validate all carrier references before returning any edge. Two RMWs cannot
	// both immediately follow the same modification in one execution witness.
	successors := map[string]string{}
	for _, id := range ids {
		node := nodes[id]
		if node.ReadsFrom == "" {
			continue
		}
		write, exists := nodes[node.ReadsFrom]
		if !exists || !write.path.ModifiesStorage || write.ID == id {
			return result, fmt.Errorf("event %q observes an invalid modification", id)
		}
		if write.Context == node.Context && write.Sequence >= node.Sequence {
			return result, fmt.Errorf("event %q observes a later same-context modification", id)
		}
		if node.path.ReadModifyWrite {
			if prior := successors[write.ID]; prior != "" {
				return result, fmt.Errorf("RMW events %q and %q claim the same immediate predecessor", prior, id)
			}
			successors[write.ID] = id
		}
	}
	// Iterative validation avoids recursion on long contiguous RMW chains.
	done := map[string]bool{}
	for _, id := range ids {
		path := []string{}
		active := map[string]bool{}
		for at := id; at != "" && !done[at]; {
			if active[at] {
				return result, fmt.Errorf("cyclic atomic modification predecessors at %q", at)
			}
			active[at] = true
			path = append(path, at)
			node := nodes[at]
			if !node.path.ReadModifyWrite {
				break
			}
			at = node.ReadsFrom
		}
		for _, at := range path {
			done[at] = true
		}
	}
	// Fence lists are indexed by the producer's execution context. Sequence is
	// compared only within that context, never across unrelated executions.
	releases, acquires := map[string][]atomicWitnessEvent{}, map[string][]atomicWitnessEvent{}
	for _, id := range ids {
		node := nodes[id]
		if atomicOrderHasRelease(node.fence) {
			releases[node.Context] = append(releases[node.Context], node)
		}
		if atomicOrderHasAcquire(node.fence) {
			acquires[node.Context] = append(acquires[node.Context], node)
		}
	}
	for _, id := range ids {
		read := nodes[id]
		if !read.path.ReadsStorage {
			continue
		}
		if read.ReadsFrom == "" {
			result.UnresolvedReads = append(result.UnresolvedReads, id)
			continue
		}
		unresolved := false
		for at := read.ReadsFrom; at != ""; {
			write := nodes[at]
			relation := AtomicModificationOrderRelationship(read.operation, write.operation, state)
			if relation != ResourceSame {
				if relation == ResourceDisjoint {
					return AtomicFenceSynchronizationResult{}, fmt.Errorf("event %q observes a disjoint atomic object", id)
				}
				unresolved = true
				break
			}
			add := func(from, to string) {
				result.Edges = append(result.Edges, AtomicFenceSynchronization{From: from, To: to, Modification: write.ID, ObservedModification: read.ReadsFrom, Read: id})
			}
			// §29 and §31: the fence must precede the communicating modification.
			for _, fence := range releases[write.Context] {
				if fence.Sequence >= write.Sequence {
					continue
				}
				if atomicOrderHasAcquire(read.path.Order) {
					add(fence.ID, read.ID)
				}
				for _, acquire := range acquires[read.Context] {
					if read.Sequence < acquire.Sequence {
						add(fence.ID, acquire.ID)
					}
				}
			}
			// §30: release head (including overlapping release RMW heads) to fence.
			if atomicOrderHasRelease(write.path.Order) {
				for _, fence := range acquires[read.Context] {
					if read.Sequence < fence.Sequence {
						add(write.ID, fence.ID)
					}
				}
			}
			if !write.path.ReadModifyWrite {
				break
			} // Store breaks every older chain.
			if write.ReadsFrom == "" {
				unresolved = true
				break
			}
			at = write.ReadsFrom
		}
		if unresolved {
			result.UnresolvedReads = append(result.UnresolvedReads, id)
		}
	}
	sort.Slice(result.Edges, func(i, j int) bool {
		l, r := result.Edges[i], result.Edges[j]
		if l.From != r.From {
			return l.From < r.From
		}
		if l.To != r.To {
			return l.To < r.To
		}
		if l.Modification != r.Modification {
			return l.Modification < r.Modification
		}
		return l.Read < r.Read
	})
	return result, nil
}

// atomicOrderHasRelease recognizes the release half of established orders.
// Rules: rules/concurrency/concurrency_memory_model.md §§20–22,32–33.
func atomicOrderHasRelease(order string) bool {
	return order == "Release" || order == "AcqRel" || order == "SeqCst"
}

// atomicOrderHasAcquire recognizes the acquire half of established orders.
// Rules: rules/concurrency/concurrency_memory_model.md §§19,21–22,32–33.
func atomicOrderHasAcquire(order string) bool {
	return order == "Acquire" || order == "AcqRel" || order == "SeqCst"
}
