package sema

import (
	"fmt"
	"sort"
)

// InstantiationDemands derives a detached, deterministic demand graph from
// selected source calls. Nested calls, even with fixed concrete arguments, only
// activate under a demanded enclosing template. Forwarded owner and callable
// bindings compose simultaneously. Queries never instantiate bodies or mutate
// caches. A work limit is reported, never mistaken for a complete graph.
// Rules: rules/compiler/monomorphization.md — §§6–8,18.1,18.7.
func (a *Analyzer) InstantiationDemands() ([]InstantiationDemand, error) {
	if a == nil {
		return nil, nil
	}
	entries := make([]genericInstantiation, 0, len(a.genericInstantiations))
	for _, entry := range a.genericInstantiations {
		entries = append(entries, entry)
	}
	entries = sortedInstantiations(entries)
	demands := map[string]*InstantiationDemand{}
	pending := []genericInstantiation{}
	add := func(entry genericInstantiation, caller string) error {
		identity, known := a.InstantiationIdentityOf(entry.function)
		if !known {
			return nil
		}
		demand, found := demands[identity.Key]
		if !found {
			if len(demands) >= 4096 {
				return fmt.Errorf("generic instantiation demand resource limit of 4096 specializations")
			}
			demand = &InstantiationDemand{Identity: identity}
			demands[identity.Key] = demand
			pending = append(pending, entry)
		}
		for _, cause := range demand.Causes {
			if cause.Caller == caller && sameSourceToken(cause.Source, entry.call) {
				return nil
			}
		}
		demand.Causes = append(demand.Causes, InstantiationCause{Caller: caller, Source: entry.call})
		return nil
	}
	for _, entry := range entries {
		if entry.caller == (sourceTokenKey{}) && !entry.forwarded {
			if err := add(entry, ""); err != nil {
				return nil, err
			}
		}
	}
	for index := 0; index < len(pending); index++ {
		outer := pending[index]
		identity, _ := a.InstantiationIdentityOf(outer.function)
		for _, nested := range entries {
			if nested.caller != outer.template {
				continue
			}
			substitution := make(map[string]Type, len(nested.substitution))
			for name, typ := range nested.substitution {
				substitution[name] = substituteGenericType(typ, outer.substitution)
			}
			nested.substitution = substitution
			nested.function.GenericSubstitution = substitution
			if err := add(nested, identity.Key); err != nil {
				return nil, err
			}
		}
	}
	result := make([]InstantiationDemand, 0, len(demands))
	for _, demand := range demands {
		sort.Slice(demand.Causes, func(i, j int) bool {
			left, right := demand.Causes[i], demand.Causes[j]
			if left.Caller != right.Caller {
				return left.Caller < right.Caller
			}
			return tokenBefore(left.Source, right.Source)
		})
		result = append(result, *demand)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Identity.Key < result[j].Identity.Key })
	return result, nil
}
