package sema

import (
	"encoding/json"
	"fmt"
)

// scopedGraphIdentity namespaces one semantic record by concrete plan/model.
// Rules: rules/analysis/call_graph.md — "One graph per `CompilationPlan`",
// "Callable node identity", "Call-site identity".
func scopedGraphIdentity(scope CallGraphScope, kind, identity string) string {
	key, _ := json.Marshal([]string{scope.Module, scope.CompilationPlan, scope.CompilerModel, kind, identity})
	return "plan-" + kind + "|" + graphIdentityDigest(key)
}

// CompilationScope returns the explicit producer-supplied scope. Zero denotes
// an unbound Sema snapshot, never a host-derived concrete compilation plan.
// Rules: rules/analysis/call_graph.md — "One graph per `CompilationPlan`".
func (g *CallGraph) CompilationScope() CallGraphScope {
	if g == nil {
		return CallGraphScope{}
	}
	return g.scope
}

// CallableInScope resolves an unbound declaration/body node ID into this graph.
// Already-bound IDs are accepted without applying the namespace twice.
// Rules: rules/analysis/call_graph.md — "Callable node identity".
func (g *CallGraph) CallableInScope(id CallableID) (CallableID, bool) {
	if g == nil {
		return "", false
	}
	if _, exists := g.nodes[id]; exists {
		return id, true
	}
	if g.scope == (CallGraphScope{}) {
		return "", false
	}
	qualified := CallableID(scopedGraphIdentity(g.scope, "callable", string(id)))
	_, exists := g.nodes[qualified]
	return qualified, exists
}

// ForCompilationPlan binds a detached graph to one concrete scope atomically.
// All node/body/site/root references and introducing effect facts move together;
// target coverage and reachability are unchanged. Binding does not perform
// source selection: the frontend must have analyzed the active source universe
// for this plan. A graph already bound to another plan cannot be relabelled or
// merged. This keeps cross-plan index queries separate from runtime graphs.
// Rules: rules/analysis/call_graph.md — "One graph per `CompilationPlan`",
// "Cross-plan comparison", "Inactive declarations", "Graph data model".
func (g *CallGraph) ForCompilationPlan(scope CallGraphScope) (*CallGraph, error) {
	if g == nil || scope.Module == "" || scope.CompilationPlan == "" || scope.CompilerModel == "" {
		return nil, fmt.Errorf("graph requires complete compilation scope")
	}
	if g.scope != (CallGraphScope{}) {
		if g.scope != scope {
			return nil, fmt.Errorf("graph belongs to another compilation plan")
		}
		return g.clone(), nil
	}
	if !validTestRootScope(g, scope) {
		return nil, fmt.Errorf("test roots belong to another compilation plan")
	}
	result := newCallGraph()
	result.scope = scope
	callable := func(id CallableID) CallableID { return CallableID(scopedGraphIdentity(scope, "callable", string(id))) }
	body := func(id CallableBodyID) CallableBodyID {
		return CallableBodyID(scopedGraphIdentity(scope, "body", string(id)))
	}
	siteID := func(id CallSiteID) CallSiteID {
		if id == "" {
			return ""
		}
		return CallSiteID(scopedGraphIdentity(scope, "site", string(id)))
	}
	for _, id := range g.nodeOrder {
		node := g.nodes[id]
		node.ID = callable(id)
		result.nodes[node.ID] = node
		result.nodeOrder = append(result.nodeOrder, node.ID)
	}
	for id, target := range g.bodyNodes {
		result.bodyNodes[body(id)] = callable(target)
	}
	for _, original := range g.sites {
		site := cloneCallSite(original)
		site.ID, site.Caller = siteID(site.ID), callable(site.Caller)
		for i, target := range site.Targets {
			site.Targets[i] = callable(target)
		}
		for i, target := range site.TargetSet.KnownTargets {
			site.TargetSet.KnownTargets[i] = body(target)
		}
		if site.TargetSet.OpenContract != "" {
			site.TargetSet.OpenContract = CallableContractID(scopedGraphIdentity(scope, "contract", string(site.TargetSet.OpenContract)))
		}
		result.sites = append(result.sites, site)
		result.siteIDs[site.ID] = true
	}
	for _, id := range g.rootOrder {
		root := g.roots[id]
		root.ID = CallRootID(scopedGraphIdentity(scope, "root", string(root.ID)))
		root.Node, root.ParentSite, root.Scope = callable(root.Node), siteID(root.ParentSite), scope
		result.roots[root.ID] = root
		result.rootOrder = append(result.rootOrder, root.ID)
	}
	for id, facts := range g.effects {
		result.effects[callable(id)] = cloneEffectSites(facts)
	}
	for id, facts := range g.arenaEffects {
		result.arenaEffects[callable(id)] = append([]ArenaEffectSite(nil), facts...)
	}
	for id, facts := range g.blockEffects {
		result.blockEffects[callable(id)] = append([]BlockEffectSite(nil), facts...)
	}
	return result, nil
}
