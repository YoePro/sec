package layout

import (
	"crypto/sha256"
	"encoding/json"
)

type layoutGraphKey struct {
	layoutCacheKey
	requirement [32]byte
}

type cachedLayoutGraph struct {
	root    LayoutID
	layouts map[LayoutID]ResolvedLayout
	digest  [32]byte
}

// NewResolverWithCache binds the canonical resolver to an optional reusable
// cache. Concrete type identity, all resolved plan facts and the complete
// requirement participate in reuse; declaration changes cannot hit old entries.
// Rules: rules/memory/layout.md §§3(5),26(1–5),43(5);
// rules/mlir/packages/sec-mlir-dialect_package24.md §9.
func NewResolverWithCache(plan ResolvedScalarPlan, alignments ScalarAlignments, cache *LayoutCache) (*Resolver, error) {
	r, err := NewResolver(plan, alignments)
	if err != nil {
		return nil, err
	}
	r.cache = cache
	return r, nil
}

// graphKey fingerprints the canonical requirement graph, including field order,
// concrete substitutions, counts and validity, independently of resolver IDs.
// An unserializable cyclic requirement is resolved normally, never cached.
// Rules: rules/memory/layout.md §§25–26,43(5).
func (r *Resolver) graphKey(requirement LayoutRequirement) (layoutGraphKey, bool) {
	data, err := json.Marshal(requirement)
	if err != nil {
		return layoutGraphKey{}, false
	}
	return layoutGraphKey{cacheKeyOf(r.key, requirement.TypeIdentity, requirement.Contract), sha256.Sum256(data)}, true
}

// layoutDependencies enumerates every layout-ID edge that must stay inside a
// detached cached graph and be rebased into the receiving resolver snapshot.
// Rules: rules/memory/layout.md §§19,23,26,39.
func layoutDependencies(l ResolvedLayout) []LayoutID {
	ids := []LayoutID{l.Carrier, l.Element}
	for _, f := range l.Fields {
		ids = append(ids, f.Layout)
	}
	for _, v := range l.Variants {
		ids = append(ids, v.Payload)
	}
	return ids
}

// cacheResolvedLayout stores only complete, detached dependency closures. A
// partial or unsupported child never becomes a reusable positive layout fact.
// Rules: rules/memory/layout.md §§5(5),26(5),39.
func (r *Resolver) cacheResolvedLayout(requirement LayoutRequirement, root LayoutID) {
	key, ok := r.graphKey(requirement)
	if !ok {
		return
	}
	graph := cachedLayoutGraph{root: root, layouts: map[LayoutID]ResolvedLayout{}}
	pending := []LayoutID{root}
	for len(pending) > 0 {
		id := pending[len(pending)-1]
		pending = pending[:len(pending)-1]
		if id == 0 {
			continue
		}
		if _, found := graph.layouts[id]; found {
			continue
		}
		l, found := r.Layout(id)
		if !found || !l.Complete() {
			return
		}
		graph.layouts[id] = l
		pending = append(pending, layoutDependencies(l)...)
	}
	data, err := json.Marshal(graph.layouts)
	if err != nil {
		return
	}
	graph.digest = sha256.Sum256(data)
	r.cache.mu.Lock()
	defer r.cache.mu.Unlock()
	if r.cache.graphs == nil {
		r.cache.graphs = map[layoutGraphKey]cachedLayoutGraph{}
	}
	r.cache.graphs[key] = graph
}

// restoreCachedLayout validates the entire graph before publishing any node,
// then rebases snapshot-local IDs in deterministic dependency order. Malformed,
// stale or incompatible entries are misses; cold resolution remains authority.
// Rules: rules/memory/layout.md §§3(5),26(5),39;
// rules/mlir/packages/sec-mlir-dialect_package24.md §§9,40.
func (r *Resolver) restoreCachedLayout(requirement LayoutRequirement) (LayoutID, bool) {
	key, ok := r.graphKey(requirement)
	if !ok {
		return 0, false
	}
	c := r.cache
	c.mu.Lock()
	defer c.mu.Unlock()
	graph, found := c.graphs[key]
	if !found {
		return 0, false
	}
	states := map[LayoutID]uint8{}
	var order []LayoutID
	var visit func(LayoutID) bool
	visit = func(id LayoutID) bool {
		if id == 0 {
			return true
		}
		if states[id] == 1 {
			return false
		}
		if states[id] == 2 {
			return true
		}
		l, found := graph.layouts[id]
		if !found || l.ID != id || !l.Complete() || l.TypeIdentity == "" || l.PlanKey != r.key {
			return false
		}
		states[id] = 1
		for _, child := range layoutDependencies(l) {
			if !visit(child) {
				return false
			}
		}
		states[id] = 2
		order = append(order, id)
		return true
	}
	root, found := graph.layouts[graph.root]
	data, err := json.Marshal(graph.layouts)
	if err != nil || sha256.Sum256(data) != graph.digest || !found || cacheKeyOf(root.PlanKey, root.TypeIdentity, root.Contract) != key.layoutCacheKey || !visit(graph.root) {
		delete(c.graphs, key)
		c.Rejected++
		return 0, false
	}
	rebased := map[LayoutID]LayoutID{0: 0}
	for _, id := range order {
		l := graph.layouts[id].clone()
		memo := memoKey(LayoutRequirement{TypeIdentity: l.TypeIdentity, Contract: l.Contract})
		if existing, found := r.memo[memo]; found {
			rebased[id] = existing
			continue
		}
		l.Carrier, l.Element = rebased[l.Carrier], rebased[l.Element]
		for i := range l.Fields {
			l.Fields[i].Layout = rebased[l.Fields[i].Layout]
		}
		for i := range l.Variants {
			l.Variants[i].Payload = rebased[l.Variants[i].Payload]
		}
		rebased[id] = r.publish(l, memo)
	}
	c.hits++
	return rebased[graph.root], true
}
