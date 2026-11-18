package layout

// LayoutCache keeps resolved layouts across resolver snapshots (P24-15). An
// entry is reused only for the exact plan key, type identity and contract
// it was resolved under; a different width, endianness, alignment fact,
// reference policy, contract, generic substitution or layout revision is a
// miss, and a malformed or stale entry is a miss that is counted, never a
// fallback or an authority.
//
// Rules:
//   - rules/memory/layout.md — § 3(5)
//   - rules/mlir/packages/sec-mlir-dialect_package24.md — § 9
type LayoutCache struct {
	entries  map[layoutCacheKey]ResolvedLayout
	Rejected int
}

type layoutCacheKey struct {
	plan     string
	identity string
	contract LayoutContractID
}

// NewLayoutCache creates an empty cache.
func NewLayoutCache() *LayoutCache {
	return &LayoutCache{entries: map[layoutCacheKey]ResolvedLayout{}}
}

func cacheKeyOf(plan CompilationPlanLayoutKey, identity string, contract LayoutContractID) layoutCacheKey {
	if contract == "" {
		contract = NativeLayoutContract
	}
	return layoutCacheKey{plan: plan.String(), identity: identity, contract: contract}
}

// Store records a detached copy of a published layout under its own key.
func (c *LayoutCache) Store(layout ResolvedLayout) {
	c.entries[cacheKeyOf(layout.PlanKey, layout.TypeIdentity, layout.Contract)] = layout.clone()
}

// Lookup returns the cached layout for an exact key. An entry whose own
// plan key, identity or contract disagrees with its slot, or that has no
// identity or completeness, is stale: it is dropped and the lookup misses.
func (c *LayoutCache) Lookup(plan CompilationPlanLayoutKey, identity string, contract LayoutContractID) (ResolvedLayout, bool) {
	key := cacheKeyOf(plan, identity, contract)
	entry, found := c.entries[key]
	if !found {
		return ResolvedLayout{}, false
	}
	if entry.ID == 0 || entry.Completeness == "" || entry.PlanKey != plan || entry.TypeIdentity != identity || cacheKeyOf(entry.PlanKey, entry.TypeIdentity, entry.Contract) != key {
		delete(c.entries, key)
		c.Rejected++
		return ResolvedLayout{}, false
	}
	return entry.clone(), true
}
