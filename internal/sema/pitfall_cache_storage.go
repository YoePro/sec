package sema

// lookupOrBegin validates observed prerequisite revisions and creates a unique
// publication lease on misses. A changed fact evicts every dependent body,
// including pending work, while leaving unrelated scope/body units untouched.
// Rules: rules/compiler/incremental_compilation.md — §§7-8,29-34;
// rules/analysis/pitfall_analysis.md — "Incremental analysis".
func (cache *PitfallCache) lookupOrBegin(key pitfallCacheKey, fingerprint [32]byte, dependencies []PitfallDependency) (pitfallBodyPayload, bool, pitfallCacheLease) {
	cache.mu.Lock()
	defer cache.mu.Unlock()
	if cache.entries == nil {
		cache.entries = map[pitfallCacheKey]pitfallCacheEntry{}
		cache.pending = map[pitfallCacheKey]pitfallCacheLease{}
		cache.observed = map[pitfallFactKey]string{}
	}
	for _, dep := range dependencies {
		fact := pitfallFactKey{key.Scope, dep.Kind, dep.Identity}
		if old, exists := cache.observed[fact]; exists && old != dep.Fingerprint {
			cache.invalidateLocked(&key.Scope, dep.Kind, dep.Identity)
		}
		cache.observed[fact] = dep.Fingerprint
	}
	if entry, exists := cache.entries[key]; exists && entry.Fingerprint == fingerprint {
		return clonePitfallPayload(entry.Payload), true, pitfallCacheLease{}
	}
	delete(cache.entries, key)
	cache.generation++
	lease := pitfallCacheLease{Owner: cache, Key: key, Generation: cache.generation, Entry: pitfallCacheEntry{Fingerprint: fingerprint, Dependencies: append([]PitfallDependency(nil), dependencies...)}}
	cache.pending[key] = lease
	return pitfallBodyPayload{}, false, lease
}

// publish commits one complete body result only if its lease and dependency
// revisions are still current. The lease cannot publish a second time.
// Rules: rules/compiler/incremental_compilation.md — §§29-34.
func (cache *PitfallCache) publish(lease pitfallCacheLease) bool {
	if lease.Owner != cache {
		return false
	}
	for _, finding := range lease.Entry.Payload.Findings {
		if finding.State != PitfallStateFinding && finding.State != PitfallStateSuppressed {
			return false
		}
	}
	cache.mu.Lock()
	defer cache.mu.Unlock()
	pending, exists := cache.pending[lease.Key]
	if !exists || pending.Generation != lease.Generation {
		return false
	}
	for _, dep := range lease.Entry.Dependencies {
		if cache.observed[pitfallFactKey{lease.Key.Scope, dep.Kind, dep.Identity}] != dep.Fingerprint {
			return false
		}
	}
	lease.Entry.Payload = clonePitfallPayload(lease.Entry.Payload)
	lease.Entry.Dependencies = append([]PitfallDependency(nil), lease.Entry.Dependencies...)
	cache.entries[lease.Key] = lease.Entry
	delete(cache.pending, lease.Key)
	return true
}

// discard cancels only the current lease, preserving any newer computation.
// Rules: rules/compiler/incremental_compilation.md — §§29-34.
func (cache *PitfallCache) discard(lease pitfallCacheLease) {
	if lease.Owner != cache {
		return
	}
	cache.mu.Lock()
	defer cache.mu.Unlock()
	if pending, exists := cache.pending[lease.Key]; exists && pending.Generation == lease.Generation {
		delete(cache.pending, lease.Key)
	}
}

// Invalidate evicts every completed or pending body using a changed canonical
// fact identity, across plans/models. It returns a deterministic affected-unit
// count; unrelated bodies remain reusable. Revisions are reobserved on demand.
// Rules: rules/analysis/pitfall_analysis.md — "Incremental analysis";
// rules/compiler/incremental_compilation.md — §§8,19,29.
func (cache *PitfallCache) Invalidate(kind, identity string) int {
	if cache == nil {
		return 0
	}
	cache.mu.Lock()
	defer cache.mu.Unlock()
	return cache.invalidateLocked(nil, kind, identity)
}

// invalidateLocked removes precisely the cached/pending dependency consumers.
// Rules: rules/compiler/incremental_compilation.md — §§8,19,29.
func (cache *PitfallCache) invalidateLocked(scope *PitfallCacheScope, kind, identity string) int {
	affected := map[pitfallCacheKey]bool{}
	depends := func(key pitfallCacheKey, deps []PitfallDependency) bool {
		if scope != nil && key.Scope != *scope {
			return false
		}
		for _, dep := range deps {
			if dep.Kind == kind && dep.Identity == identity {
				return true
			}
		}
		return false
	}
	for key, entry := range cache.entries {
		if depends(key, entry.Dependencies) {
			delete(cache.entries, key)
			affected[key] = true
		}
	}
	for key, lease := range cache.pending {
		if depends(key, lease.Entry.Dependencies) {
			delete(cache.pending, key)
			affected[key] = true
		}
	}
	return len(affected)
}
