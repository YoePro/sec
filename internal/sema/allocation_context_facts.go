package sema

import (
	"fmt"
	"sort"
	"strings"

	"sec/internal/ast"
	"sec/internal/lexer"
)

// AllocationContextFact exposes selection evidence for one canonical allocation
// site. CreatedDomain is the resulting Arena's abstract creation-site identity, not the domain backing
// its construction. Empty domain IDs and ContextKnown=false remain unresolved.
// ContextKnown describes semantic context selection/availability, not a resolved
// physical provider, reclamation plan or lowering-readiness guarantee.
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
	// Origins belong to the selected/created domain's canonical lexical
	// provenance, never the current receiver spelling or the allocation site.
	SelectedDomainOrigin lexer.Token
	CreatedDomainOrigin  lexer.Token
	Path                 []CallableID
	LayoutFailure        string
}

// AllocationContextFacts correlates allocation sites with already-resolved
// string plans and Arena result types. It never selects a domain, derives one
// from a lexical name, or assumes that every operation uses the ambient context.
// Facts are deterministic detached values; folded text has no allocation site.
// Domain origins project the generated-identity registry's current source
// tokens, never coordinates decoded from an identity or the current receiver.
// Rules: rules/memory/allocation.md — §§5(4),(8),6(6),17,24,29(1),(3);
// rules/compiler/compiler.md — §71;
// rules/corrections/applied/missing-decisions-md001-md009-correction-20261003.md — §§5.8,5.10–5.14.
func (a *Analyzer) AllocationContextFacts() []AllocationContextFact {
	origins := map[string]lexer.Token{}
	for _, identity := range a.GeneratedIdentities() {
		if identity.Kind == "arena-domain" {
			origins[identity.ID] = identity.Source
		}
	}
	contexts := map[sourceTokenKey]AllocationContext{}
	layoutFailures := map[sourceTokenKey]map[string]bool{}
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
	// String appends retain the same selected context as ordinary concat.
	for value, site := range a.stringAppendSites {
		if plan, known := a.stringAppends[value]; known && site.reachable {
			key := sourceTokenLocation(site.token)
			if previous, exists := contexts[key]; exists && previous != plan.Plan.Allocation.Context {
				ambiguousContexts[key] = true
			}
			contexts[key] = plan.Plan.Allocation.Context
		}
	}
	// A dynamic-array allocation plan names the context its growth consumes
	// (package18 §§ 56–57).
	for _, plan := range a.allocationOperationPlans {
		if !plan.MayAllocate {
			continue
		}
		key := sourceTokenLocation(plan.Source)
		if previous, exists := contexts[key]; exists && previous != plan.Context {
			ambiguousContexts[key] = true
		}
		contexts[key] = plan.Context
	}
	// P19 records explicit Arena selection and constructor backing context at
	// successful source analysis. Publish those existing decisions; do not
	// select a context from currently visible bindings or recreate a provider.
	// Rules: rules/mlir/packages/sec-mlir-dialect_package19.md — §§52–54,72–76.
	for _, plan := range a.arenaOperationPlans {
		if plan.Operation != ArenaOperationNew && plan.Operation != ArenaOperationAlloc &&
			!(plan.Operation == ArenaOperationCreate && plan.Fallible) {
			continue
		}
		key := sourceTokenLocation(plan.Source)
		if plan.LayoutFailure != "" {
			if layoutFailures[key] == nil {
				layoutFailures[key] = map[string]bool{}
			}
			layoutFailures[key][plan.LayoutFailure] = true
		}
		if previous, exists := contexts[key]; exists && previous != plan.Context {
			ambiguousContexts[key] = true
		}
		contexts[key] = plan.Context
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
			// The effect site carries the stable domain it selected or
			// created; Result-type correlation remains the fallback.
			// Conflicting or ambiguous result evidence still selects nothing.
			value, correlated := results[key]
			conflict := ambiguousResults[key] ||
				correlated && site.Kind == ArenaEffectAllocate && site.Domain != "" && value.selected != site.Domain
			if conflict {
				fact.ContextKnown = false
				fact.Context = AllocationContext{}
			}
			switch {
			case conflict:
			case site.Domain != "" && site.Kind == ArenaEffectAllocate:
				fact.SelectedDomain = site.Domain
			case site.Domain != "" && (site.Kind == ArenaEffectCreateOwned || site.Kind == ArenaEffectCreateGrowable):
				fact.CreatedDomain = site.Domain
			}
			if correlated && !conflict && site.Domain == "" {
				if site.Kind == ArenaEffectAllocate {
					fact.SelectedDomain = value.selected
				}
				if site.Kind == ArenaEffectCreateOwned || site.Kind == ArenaEffectCreateGrowable {
					fact.CreatedDomain = value.created
				}
			}
			fact.SelectedDomainOrigin = origins[fact.SelectedDomain]
			var failures []string
			for failure := range layoutFailures[sourceTokenLocation(fact.Source)] {
				failures = append(failures, failure)
			}
			sort.Strings(failures)
			fact.LayoutFailure = strings.Join(failures, "; ")
			fact.CreatedDomainOrigin = origins[fact.CreatedDomain]
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

// AllocationContextDescription renders semantic selection evidence consistently
// for CLI and LSP. Internal domain tokens are analysis-local identities; a newly
// created Arena is kept distinct from its backing allocation context. Known
// selection never hides an explicit Arena domain or invents ambient provenance.
// Rules: rules/memory/allocation.md — §§5(8),29(1),(3); rules/memory/arena.md — §§4.2,4.3,67(4).
func AllocationContextDescription(fact AllocationContextFact) (description string) {
	defer func() {
		if fact.LayoutFailure != "" {
			description += "; " + fact.LayoutFailure
		}
	}()
	if fact.SelectedDomain != "" {
		return fmt.Sprintf("explicit Arena allocation domain: %s (receiver %s); %s", fact.SelectedDomain, fact.Arena, allocationOriginDescription("allocation context origin", fact.SelectedDomainOrigin))
	}
	if fact.CreatedDomain != "" {
		if fact.ContextKnown {
			return fmt.Sprintf("created Arena domain: %s; backing allocation context: %s; %s", fact.CreatedDomain, allocationContextAvailabilityDescription(fact.Context), allocationOriginDescription("created domain origin", fact.CreatedDomainOrigin))
		}
		return fmt.Sprintf("created Arena domain: %s; backing allocation context unresolved; %s", fact.CreatedDomain, allocationOriginDescription("created domain origin", fact.CreatedDomainOrigin))
	}
	if fact.ContextKnown {
		return "implicit allocation context: " + allocationContextAvailabilityDescription(fact.Context) + "; domain identity unresolved"
	}
	return "allocation context/domain unresolved"
}

// allocationContextAvailabilityDescription renders recorded semantic context
// availability without asserting physical provider support or source provenance.
// Rules: rules/memory/allocation.md — §§5(8),17,29(1),(3).
func allocationContextAvailabilityDescription(context AllocationContext) string {
	availability := "available"
	if !context.Available {
		availability = "unavailable"
	}
	profile := context.Profile
	if profile == "" {
		profile = "unresolved"
	}
	return fmt.Sprintf("%s; profile %s; origin %s", availability, profile, context.Origin)
}

// allocationOriginDescription renders only canonical source provenance. An
// absent origin stays unresolved; a created domain never identifies its backing
// context. Implicit contexts retain target/profile evidence without a fake source.
// Rules: rules/memory/allocation.md §§5(8),6(6),29(1),(3);
// rules/memory/arena.md §§4.2,4.3,67(4); rules/compiler/compiler.md §71.
func allocationOriginDescription(label string, source lexer.Token) string {
	if source.Line <= 0 || source.Column <= 0 {
		return label + " unresolved"
	}
	if source.File == "" {
		return fmt.Sprintf("%s: line %d:%d", label, source.Line, source.Column)
	}
	return fmt.Sprintf("%s: %s:%d:%d", label, source.File, source.Line, source.Column)
}
