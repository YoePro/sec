package sema

import (
	"sort"
)

// staticDestructionOrder orders the deterministic destruction of the
// statics that need it from canonical dependency and use relations
// (static.md § 20; initialization.md § 33): when destroying static X runs
// code (the custom free of X's type or of any owned part of it, and every
// callable that free reaches) that uses static Y, Y must still be alive, so
// X is destroyed first. Independent statics use the canonical identity
// order. A cycle is returned instead of an order (initialization.md § 34).
func (a *Analyzer) staticDestructionOrder() ([]string, []string) {
	statics := []ResolvedStatic{}
	for _, static := range a.ResolvedStaticDeclarations() {
		if !TriviallyDestructible(static.Type) {
			statics = append(statics, static)
		}
	}
	if len(statics) == 0 {
		return nil, nil
	}
	name := func(static ResolvedStatic) string {
		if static.Function != "" {
			return static.Module + "." + static.Function + "::" + static.Name
		}
		return static.Module + "." + static.Name
	}
	declarationOf := map[sourceTokenKey]string{}
	for _, static := range statics {
		if static.Declaration != nil && static.Declaration.Name != nil {
			declarationOf[sourceTokenLocation(static.Declaration.Name.Token)] = name(static)
		}
	}
	// uses lists the source locations that use each static.
	uses := map[string][]sourceTokenKey{}
	for use, declarations := range a.definitionTokens {
		for _, declaration := range declarations {
			key := sourceTokenLocation(declaration)
			if owner, ok := declarationOf[key]; ok && key != use {
				uses[owner] = append(uses[owner], use)
			}
		}
	}
	dependsOn := map[string]map[string]bool{}
	for _, static := range statics {
		self := name(static)
		dependsOn[self] = map[string]bool{}
		for _, body := range a.destructionBodies(static.Type) {
			for other, locations := range uses {
				if other == self {
					continue
				}
				for _, location := range locations {
					if withinCallable(location, body) {
						dependsOn[self][other] = true
					}
				}
			}
		}
	}
	// X before Y when X's destruction uses Y: Y waits for X.
	waits := map[string][]string{}
	for static, dependencies := range dependsOn {
		for dependency := range dependencies {
			waits[dependency] = append(waits[dependency], static)
		}
	}
	graph := map[string][]string{}
	for static := range dependsOn {
		graph[static] = waits[static]
	}
	if cycle := dependencyCycle(graph); len(cycle) > 0 {
		return nil, cycle
	}
	actions := map[string]StartupAction{}
	for static := range dependsOn {
		actions[static] = StartupAction{ID: static, Requires: waits[static]}
	}
	order := []string{}
	for _, action := range topologicalStartup(actions) {
		order = append(order, action.ID)
	}
	return order, nil
}

// destructionBodies returns the callables that destroying a value of typ may
// execute: the custom free of typ and of every owned field or payload type,
// and every callable reachable from them.
func (a *Analyzer) destructionBodies(typ Type) []CallableNode {
	if a.callGraph == nil {
		return nil
	}
	starts := []CallableID{}
	seen := map[string]bool{}
	var visit func(Type)
	visit = func(typ Type) {
		key := canonicalTypeIdentity(typ)
		if seen[key] {
			return
		}
		seen[key] = true
		if typ.CustomFree {
			for _, function := range a.functions[freeFunctionName(typ.Name)] {
				if function.Module == typ.Module || typ.Module == "" {
					starts = append(starts, callableID(function))
				}
			}
		}
		for _, field := range typ.Fields {
			visit(field.Type)
		}
		for _, argument := range typ.TypeArgs {
			visit(argument)
		}
		for _, variant := range typ.UnionVariants {
			if variant.Payload != nil {
				visit(*variant.Payload)
			}
			for _, field := range variant.PayloadFields {
				visit(field.Type)
			}
		}
	}
	visit(typ)
	reachable := map[CallableID]bool{}
	for _, start := range starts {
		reachable[start] = true
		for id := range a.callGraph.reachableIDsFrom(start) {
			reachable[id] = true
		}
	}
	ids := make([]CallableID, 0, len(reachable))
	for id := range reachable {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	bodies := []CallableNode{}
	for _, id := range ids {
		if node, ok := a.callGraph.Node(id); ok && node.DeclarationEnd.Line > 0 {
			bodies = append(bodies, node)
		}
	}
	return bodies
}

// withinCallable reports whether a use lies inside a callable's source range.
func withinCallable(use sourceTokenKey, node CallableNode) bool {
	start, end := node.Declaration, node.DeclarationEnd
	if use.File != start.File {
		return false
	}
	after := use.Line > start.Line || use.Line == start.Line && use.Column >= start.Column
	before := use.Line < end.Line || use.Line == end.Line && use.Column <= end.Column
	return after && before
}
