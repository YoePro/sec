package sema

import (
	"sort"

	"sec/internal/lexer"
)

// SummaryEffect is a may-effect capability, separate from ordered arena events.
// Rules: rules/analysis/effect_analysis.md — "Summary may-effects".
type SummaryEffect string

const (
	EffectMayAllocate                  EffectKind    = "may-allocate"
	EffectMayBlock                     EffectKind    = "may-block"
	EffectMayAccessVolatile            EffectKind    = "may-access-volatile"
	SummaryMayPanic                    SummaryEffect = "MayPanic"
	SummaryMayAllocate                 SummaryEffect = "MayAllocate"
	SummaryMayBlock                    SummaryEffect = "MayBlock"
	SummaryMaySuspend                  SummaryEffect = "MaySuspend"
	SummaryMaySpawn                    SummaryEffect = "MaySpawn"
	SummaryMayIO                       SummaryEffect = "MayIO"
	SummaryMayAccessVolatile           SummaryEffect = "MayAccessVolatile"
	SummaryMayMutateExternalState      SummaryEffect = "MayMutateExternalState"
	SummaryMayUseNondeterministicInput SummaryEffect = "MayUseNondeterministicInput"
)

// These direct facts are published by the owning semantic/runtime producers.
// They do not define new source attributes, imports or backend behavior.
// Rules: rules/analysis/effect_analysis.md — "Direct effects".
const (
	EffectMaySuspend             EffectKind = "may-suspend"
	EffectMaySpawn               EffectKind = "may-spawn"
	EffectMayIO                  EffectKind = "may-io"
	EffectMayMutateExternalState EffectKind = "may-mutate-external-state"
)

var summaryEffects = []SummaryEffect{SummaryMayPanic, SummaryMayAllocate, SummaryMayBlock, SummaryMaySuspend, SummaryMaySpawn, SummaryMayIO, SummaryMayAccessVolatile, SummaryMayMutateExternalState, SummaryMayUseNondeterministicInput}

// EffectDomainCause retains one introducing fact and a representative shortest
// same-stack path, including every call site's dispatch and execution metadata.
// Unknown identifies missing proof, not a proven effect or effect freedom.
// Rules: rules/analysis/call_graph.md — "Diagnostics", "Conservative unknown facts";
// rules/analysis/effect_analysis.md — "Summary may-effects".
type EffectDomainCause struct {
	Domain   SummaryEffect
	Callable CallableID
	Source   lexer.Token
	Fact     string
	Unknown  bool
	Path     []CallSite
}

// CallableDomainSummary exposes an idempotent may-set and independent unknown
// domains, with all distinct direct/transitive introducing causes. It consumes
// only facts already in this graph; absence never certifies a guarantee when
// the owning producer or graph coverage is incomplete. In particular this API
// does not establish implicit cleanup coverage or import trust/plan compatibility.
// Rules: rules/analysis/effect_analysis.md — "Inferred summaries", "Transitive effects";
// rules/analysis/call_graph.md — "Effect-analysis integration".
type CallableDomainSummary struct {
	Effects        []SummaryEffect
	UnknownEffects []SummaryEffect
	DirectCauses   []EffectDomainCause
	Causes         []EffectDomainCause
}

// DomainSummary unions canonical effects over same-stack dependencies. Spawn
// creation belongs to the caller; spawned-body effects remain in their context.
// A visited-node worklist terminates even for recursive SCCs and retains one
// shortest deterministic call-site witness per introducing callable. Open or
// foreign boundaries cannot become empty-effect proofs. Facts are detached.
// Rules: rules/analysis/call_graph.md — "Same-stack effect transfer", "Spawn effect transfer",
// "Closed target set", "Conservative unknown facts", "Diagnostics";
// rules/analysis/effect_analysis.md — "Summary may-effects", "Fixed-point analysis".
func (g *CallGraph) DomainSummary(id CallableID) CallableDomainSummary {
	var result CallableDomainSummary
	if g == nil {
		return result
	}
	if _, exists := g.nodes[id]; !exists {
		return result
	}
	type entry struct {
		callable CallableID
		path     []CallSite
	}
	queue := []entry{{callable: id}}
	seen := map[CallableID]bool{id: true}
	may, unknown := map[SummaryEffect]bool{}, map[SummaryEffect]bool{}
	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]
		for _, cause := range g.directDomainCauses(current.callable) {
			cause.Path = cloneDomainPath(current.path)
			result.Causes = append(result.Causes, cause)
			if current.callable == id {
				result.DirectCauses = append(result.DirectCauses, cloneDomainCause(cause))
			}
			if cause.Unknown {
				unknown[cause.Domain] = true
			} else {
				may[cause.Domain] = true
			}
		}
		sites := g.Outgoing(current.callable)
		sort.Slice(sites, func(i, j int) bool { return sites[i].ID < sites[j].ID })
		for _, site := range sites {
			if !sameStackExecution(site.Execution) {
				continue
			}
			targets := append([]CallableID(nil), site.Targets...)
			sort.Slice(targets, func(i, j int) bool { return targets[i] < targets[j] })
			for _, target := range targets {
				if seen[target] {
					continue
				}
				if _, exists := g.nodes[target]; !exists {
					continue
				}
				seen[target] = true
				path := append(cloneDomainPath(current.path), cloneCallSite(site))
				// Narrow only the path's chosen target, retaining the original target-set
				// facts so diagnostics can still explain an indirect alternative.
				path[len(path)-1].Targets = []CallableID{target}
				queue = append(queue, entry{target, path})
			}
		}
	}
	for _, effect := range summaryEffects {
		if may[effect] {
			result.Effects = append(result.Effects, effect)
		}
		if unknown[effect] {
			result.UnknownEffects = append(result.UnknownEffects, effect)
		}
	}
	sortDomainCauses(result.DirectCauses)
	sortDomainCauses(result.Causes)
	return result
}

// directDomainCauses consumes existing panic, allocation, blocking, volatile,
// clock and producer-supplied runtime facts without reclassifying arena reset
// or release as allocation. Unknown call contracts remain independent causes.
// Rules: rules/analysis/effect_analysis.md — "Direct effects", "Ordered arena-lifetime effects";
// rules/analysis/call_graph.md — "Unknown callable contract", "Spawn effects".
func (g *CallGraph) directDomainCauses(id CallableID) []EffectDomainCause {
	var causes []EffectDomainCause
	add := func(domain SummaryEffect, source lexer.Token, fact string, unknown bool) {
		candidate := EffectDomainCause{Domain: domain, Callable: id, Source: source, Fact: fact, Unknown: unknown}
		for _, existing := range causes {
			if existing.Domain == domain && existing.Source == source && existing.Fact == fact && existing.Unknown == unknown {
				return
			}
		}
		causes = append(causes, candidate)
	}
	if node := g.nodes[id]; node.Extern {
		for _, domain := range summaryEffects {
			add(domain, node.Declaration, "foreign body effect contract unavailable", true)
		}
	}
	for _, effect := range g.effects[id] {
		if domain, known := summaryDomainForEffect(effect.Kind); known {
			uncertain := effect.Kind == EffectMayPanicForeign || effect.Kind == EffectMayPanicUnknownCallee
			add(domain, effect.Source, string(effect.Kind), uncertain)
		} else {
			for _, domain := range summaryEffects {
				add(domain, effect.Source, "unclassified semantic effect: "+string(effect.Kind), true)
			}
		}
		if effect.Kind == EffectMayPanicUnknownCallee {
			for _, domain := range summaryEffects {
				add(domain, effect.Source, "unknown callable effects", true)
			}
		}
	}
	for _, effect := range g.arenaEffects[id] {
		if effect.UnknownAllocation {
			add(SummaryMayAllocate, effect.Source, string(effect.Kind), true)
		} else if effect.MayAllocate {
			add(SummaryMayAllocate, effect.Source, string(effect.Kind), false)
		}
	}
	for _, effect := range g.blockEffects[id] {
		add(SummaryMayBlock, effect.Source, string(effect.Kind), effect.Kind != BlockEffectOperation)
	}
	for _, site := range g.Outgoing(id) {
		switch site.Execution {
		case CallExecutionSpawnTask, CallExecutionSpawnThread, CallExecutionSpawnProcess:
			add(SummaryMaySpawn, site.Source, string(site.Execution), false)
		case CallExecutionSynchronous, CallExecutionDeferred:
			unresolved := !site.TargetSet.IsClosed || site.TargetSet.HasOpenContract || site.TargetSet.OpenContract != "" || len(site.Targets) == 0 || len(site.Targets) != len(site.TargetSet.KnownTargets)
			for _, target := range site.Targets {
				node, exists := g.nodes[target]
				unresolved = unresolved || !exists || node.Extern
			}
			for _, body := range site.TargetSet.KnownTargets {
				target, exists := g.bodyNodes[body]
				covered := false
				for _, candidate := range site.Targets {
					covered = covered || candidate == target
				}
				unresolved = unresolved || !exists || !covered
			}
			if unresolved {
				for _, domain := range summaryEffects {
					add(domain, site.Source, "call effect contract unavailable", true)
				}
			}
		default:
			for _, domain := range summaryEffects {
				add(domain, site.Source, "unsupported execution relation", true)
			}
		}
	}
	return causes
}

// summaryDomainForEffect maps canonical producer facts to idempotent may-sets.
// Unknown future facts require conservative fallback, never silent omission.
// Rules: rules/analysis/effect_analysis.md — "Summary may-effects", "Direct effects".
func summaryDomainForEffect(kind EffectKind) (SummaryEffect, bool) {
	if isPanicEffectKind(kind) {
		return SummaryMayPanic, true
	}
	switch kind {
	case EffectMayAllocate:
		return SummaryMayAllocate, true
	case EffectMayBlock:
		return SummaryMayBlock, true
	case EffectMayAccessVolatile:
		return SummaryMayAccessVolatile, true
	case EffectMaySuspend:
		return SummaryMaySuspend, true
	case EffectMaySpawn:
		return SummaryMaySpawn, true
	case EffectMayIO:
		return SummaryMayIO, true
	case EffectMayMutateExternalState:
		return SummaryMayMutateExternalState, true
	case EffectVolatileRead, EffectVolatileWrite:
		return SummaryMayAccessVolatile, true
	case EffectMayUseNondeterministicInput:
		return SummaryMayUseNondeterministicInput, true
	}
	return "", false
}

// cloneDomainPath detaches target slices and contract coverage at every step.
// Rules: rules/analysis/call_graph.md — "Call-site record", "Diagnostics".
func cloneDomainPath(path []CallSite) []CallSite {
	if path == nil {
		return nil
	}
	result := make([]CallSite, len(path))
	for i, site := range path {
		result[i] = cloneCallSite(site)
	}
	return result
}

// cloneDomainCause gives direct and transitive publication independent ownership.
// Rules: rules/analysis/call_graph.md — "Graph data model".
func cloneDomainCause(cause EffectDomainCause) EffectDomainCause {
	cause.Path = cloneDomainPath(cause.Path)
	return cause
}

// sortDomainCauses selects stable display order, not runtime event ordering.
// Rules: rules/analysis/effect_analysis.md — "Summary may-effects", "Ordered arena-lifetime effects".
func sortDomainCauses(causes []EffectDomainCause) {
	sort.Slice(causes, func(i, j int) bool {
		a, b := causes[i], causes[j]
		if a.Domain != b.Domain {
			return a.Domain < b.Domain
		}
		if a.Callable != b.Callable {
			return a.Callable < b.Callable
		}
		if a.Source.File != b.Source.File {
			return a.Source.File < b.Source.File
		}
		if a.Source.Line != b.Source.Line {
			return a.Source.Line < b.Source.Line
		}
		if a.Source.Column != b.Source.Column {
			return a.Source.Column < b.Source.Column
		}
		if a.Unknown != b.Unknown {
			return !a.Unknown
		}
		return a.Fact < b.Fact
	})
}
