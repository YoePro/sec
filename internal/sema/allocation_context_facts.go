package sema

import (
	"sort"

	"sec/internal/ast"
	"sec/internal/lexer"
)

// AllocationContextFact exposes selection evidence for one canonical allocation
// site. CreatedDomain is the resulting Arena's identity, not the domain backing
// its construction. Empty domain IDs and ContextKnown=false remain unresolved.
// Rules: rules/memory/allocation.md — §§5,6,8,17(6)-(9),29(1),(3);
// rules/memory/arena.md — §§4.2,4.3,67(4).
type AllocationContextFact struct {
	Callable       CallableID
	Source         lexer.Token
	Kind           ArenaEffectKind
	Arena          string
	ContextKnown   bool
	Context        AllocationContext
	SelectedDomain string
	CreatedDomain  string
	Path           []CallableID
}

// AllocationContextFacts correlates allocation sites with already-resolved
// string plans and Arena result types. It never selects a domain, derives one
// from a lexical name, or assumes that every operation uses the ambient context.
// Facts are deterministic detached values; folded text has no allocation site.
// Rules: rules/memory/allocation.md — §§5(4),(8),6(6),17,24,29(1),(3);
// rules/corrections/applied/missing-decisions-md001-md009-correction-20261003.md — §§5.8,5.10–5.14.
func (a *Analyzer) AllocationContextFacts() []AllocationContextFact {
	contexts := map[sourceTokenKey]AllocationContext{}
	ambiguousContexts := map[sourceTokenKey]bool{}
	for root, site := range a.stringMaterializationSites {
		if plan, known := a.stringConcatPlans[root]; known && plan.Runtime && site.reachable {
			key := sourceTokenLocation(site.token)
			if previous, exists := contexts[key]; exists && previous != plan.Allocation.Context {
				ambiguousContexts[key] = true
			}
			contexts[key] = plan.Allocation.Context
		}
	}
	// A Result's success type preserves the frontend's selected Arena domain.
	// Constructors instead preserve the domain of the Arena they create.
	type domains struct{ selected, created string }
	results := map[sourceTokenKey]domains{}
	ambiguousResults := map[sourceTokenKey]bool{}
	for expression, typ := range a.expressionTypes {
		if call, ok := expression.(*ast.CallExpression); ok {
			if typ.Kind == ResultType && len(typ.TypeArgs) > 0 {
				typ = typ.TypeArgs[0]
			}
			value := domains{}
			if typ.Kind == ReferenceType && typ.ReferenceOriginStorage == StorageOriginArena {
				value.selected = typ.ReferenceOriginName
			}
			if typ.Name == "Arena" {
				value.created = typ.ArenaDomainID
			}
			key := sourceTokenLocation(callCalleeDefinitionToken(call))
			if previous, exists := results[key]; exists && previous != value {
				ambiguousResults[key] = true
			}
			results[key] = value
		}
	}
	facts := []AllocationContextFact{}
	for _, node := range a.callGraph.Nodes() {
		for _, site := range a.callGraph.arenaEffects[node.ID] {
			if !site.MayAllocate && !site.UnknownAllocation {
				continue
			}
			fact := AllocationContextFact{Callable: node.ID, Source: site.Source, Kind: site.Kind, Arena: site.Arena}
			key := sourceTokenLocation(site.Source)
			if context, known := contexts[key]; known && !ambiguousContexts[key] {
				fact.ContextKnown = true
				fact.Context = context
			}
			if value, known := results[key]; known && !ambiguousResults[key] {
				if site.Kind == ArenaEffectAllocate {
					fact.SelectedDomain = value.selected
				}
				if site.Kind == ArenaEffectCreateOwned || site.Kind == ArenaEffectCreateGrowable {
					fact.CreatedDomain = value.created
				}
			}
			facts = append(facts, fact)
		}
	}
	sort.Slice(facts, func(i, j int) bool {
		l, r := facts[i], facts[j]
		if l.Callable != r.Callable {
			return l.Callable < r.Callable
		}
		if l.Source.File != r.Source.File {
			return l.Source.File < r.Source.File
		}
		if l.Source.Line != r.Source.Line {
			return l.Source.Line < r.Source.Line
		}
		if l.Source.Column != r.Source.Column {
			return l.Source.Column < r.Source.Column
		}
		return l.Kind < r.Kind
	})
	return facts
}

// AllocationContexts returns each synchronously reachable site's selection
// evidence once, with a shortest canonical cause path. Recursive cycles terminate;
// deferred execution participates, while spawned execution uses its own context.
// Unknown foreign/indirect contexts remain explicit instead of borrowing the
// caller's profile as an invented selection.
// Rules: rules/memory/allocation.md — §§6(3)-(7),24(6),29(1),(3);
// rules/analysis/call_graph.md — "Execution relations".
func (a *Analyzer) AllocationContexts(id CallableID) []AllocationContextFact {
	byCallable := map[CallableID][]AllocationContextFact{}
	for _, fact := range a.AllocationContextFacts() {
		byCallable[fact.Callable] = append(byCallable[fact.Callable], fact)
	}
	if _, known := a.callGraph.Node(id); !known {
		return nil
	}
	paths := [][]CallableID{{id}}
	visited := map[CallableID]bool{id: true}
	result := []AllocationContextFact{}
	for len(paths) > 0 {
		path := paths[0]
		paths = paths[1:]
		current := path[len(path)-1]
		for _, fact := range byCallable[current] {
			fact.Path = append([]CallableID(nil), path...)
			result = append(result, fact)
		}
		for _, target := range a.callGraph.sameStackTargets(current) {
			if !visited[target] {
				visited[target] = true
				paths = append(paths, append(append([]CallableID(nil), path...), target))
			}
		}
	}
	return result
}
