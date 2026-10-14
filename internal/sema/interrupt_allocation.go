package sema

// interruptAllocationHelp keeps repairs compatible with ISR's effective
// allocation guarantee, including when @noAlloc is redundantly explicit.
// Rules: rules/foundations/attributes.md — "ISR verification", "Interrupt-safe
// guarantee", "Attribute implications", "@noAlloc and fallible allocation";
// rules/memory/allocation.md — §28 "Diagnostics".
func interruptAllocationHelp(kind ArenaEffectKind, attribute string) string {
	prefix := "@" + attribute + " requires noAlloc on every reachable synchronous and defer path; unsafe does not waive this requirement. "
	switch kind {
	case ArenaEffectForeign:
		return prefix + "Foreign allocation behavior is unknown. Supply a documented trusted @noAlloc foreign contract, or move the call outside ISR execution; other ISR/FFI requirements still apply."
	case ArenaEffectUnknownCallee:
		return prefix + "This call's target is unknown. Use a compiler-resolved target whose allocation-free behavior can be proven, or move the call outside ISR execution."
	default:
		return prefix + "This operation acquires new storage. try only handles its failure; it still allocates. Use caller-provided or preallocated storage, or move the allocating work outside ISR execution."
	}
}
