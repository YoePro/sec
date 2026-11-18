package sema

// AtomicReleaseSequenceEffect is the transfer rule for one validated atomic
// outcome in its object's modification order. It represents release-sequence
// constraints, not a source-ordered chain or an observed runtime read-from edge.
// A release RMW both continues ALL predecessor heads and introduces its own
// head: keeping only the most recent head would lose publication. Neither
// continuation nor head identity requires equal callables/execution contexts.
//
// A consumer must use the canonical storage/lifetime identity and the actual
// immediately preceding modification, with a complete contiguous RMW path;
// MayAlias/Unknown storage, source adjacency, equal values and later RMWs after
// a breaking Store cannot establish membership. Head operation, source,
// callable and outcome remain in ResolvedAtomicMemoryOperation and its Paths.
// Acquire synchronization additionally needs a qualifying observed value;
// these facts alone introduce no synchronizes-with or happens-before edge.
// Rules: rules/concurrency/concurrency_memory_model.md §§16–20,25–28,52.
type AtomicReleaseSequenceEffect struct {
	// StartsHead means THIS modification has release semantics. Its own head
	// coexists with inherited heads for a release/AcqRel/SeqCst RMW.
	StartsHead bool
	// ContinuesPredecessorHeads propagates every head carried by the exact
	// preceding modification of the SAME atomic object. Relaxed/Acquire RMWs
	// can continue a chain without starting a new release head.
	ContinuesPredecessorHeads bool
	// BreaksPredecessorHeads removes every preceding head. Every plain Store
	// does this, even one in the same thread or a release Store starting anew.
	BreaksPredecessorHeads bool
	// NoModification leaves modification order and its chains untouched:
	// Load and failed CAS neither extend nor break a release sequence. Their
	// observed value may still qualify separately for acquire synchronization.
	NoModification bool
	// SameObject and ContiguousRMW are required relations, never assertions
	// that two source sites actually access one live object consecutively.
	RequiresSameObject    bool
	RequiresContiguousRMW bool
}

// atomicReleaseSequenceEffect consumes already validated outcome/order facts.
// No second order evaluator, operation-name heuristic or target capability
// lookup participates in the language's release-sequence classification.
// Rules: rules/concurrency/concurrency_memory_model.md §§18(4),25(1–9),27,52.
func atomicReleaseSequenceEffect(path AtomicMemoryPath) AtomicReleaseSequenceEffect {
	if !path.ModifiesStorage {
		return AtomicReleaseSequenceEffect{NoModification: true}
	}
	release := path.Order == "Release" || path.Order == "AcqRel" || path.Order == "SeqCst"
	return AtomicReleaseSequenceEffect{
		StartsHead:                release,
		ContinuesPredecessorHeads: path.ReadModifyWrite,
		BreaksPredecessorHeads:    !path.ReadModifyWrite,
		RequiresSameObject:        true,
		RequiresContiguousRMW:     path.ReadModifyWrite,
	}
}
