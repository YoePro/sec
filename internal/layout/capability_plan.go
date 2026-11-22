package layout

// capabilityPlanBase binds independent capability tables to the same complete
// scalar/profile contract without nesting tables into one another. Each table
// has its own semantic facts; neither ordinary atomic support nor IPC support
// establishes the other. Address-domain slices are detached at the boundary.
// Rules: rules/concurrency/ipc.md §118; rules/concurrency/atomics.md §37;
// rules/platform/target_profiles.md §§29–31.
// rules/concurrency/mutex.md §60 (independent ordinary lock contracts).
// rules/concurrency/cancellation.md §24 (independent physical-thread identity).
func capabilityPlanBase(p ResolvedScalarPlan) ResolvedScalarPlan {
	p.Atomics = AtomicCapabilities{}
	p.IPC = IPCCapabilities{}
	p.Mutex = MutexCapabilities{}
	p.PhysicalThreads = PhysicalThreadCapabilities{}
	p.AddressDomains = append([]AddressDomain(nil), p.AddressDomains...)
	return p
}
