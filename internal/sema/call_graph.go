package sema

import (
	"fmt"
	"sort"

	"sec/internal/lexer"
)

// CallableID identifies one callable declaration within an analyzed source
// snapshot. Compilation-plan and specialization identity will extend this key
// when those compiler services become available.
type CallableID string

// CallSiteID identifies one semantic invocation within its containing callable.
type CallSiteID string

type CallRootID string

type CallRootKind string

const (
	CallRootProgramEntry CallRootKind = "program-entry"
	CallRootTaskEntry    CallRootKind = "task-entry"
	CallRootThreadEntry  CallRootKind = "thread-entry"
)

type CallDispatchKind string

const (
	CallDispatchDirect       CallDispatchKind = "direct"
	CallDispatchStaticMethod CallDispatchKind = "static-method"
	CallDispatchForeign      CallDispatchKind = "foreign-direct"
)

type CallExecutionRelation string

const (
	CallExecutionSynchronous  CallExecutionRelation = "synchronous"
	CallExecutionSpawnTask    CallExecutionRelation = "spawn-task"
	CallExecutionSpawnThread  CallExecutionRelation = "spawn-thread"
	CallExecutionSpawnProcess CallExecutionRelation = "spawn-process"
)

type ArenaEffectKind string

const (
	ArenaEffectCreateBorrowed ArenaEffectKind = "create-borrowed"
	ArenaEffectCreateOwned    ArenaEffectKind = "create-owned"
	ArenaEffectCreateGrowable ArenaEffectKind = "create-growable"
	ArenaEffectAllocate       ArenaEffectKind = "allocate"
	ArenaEffectReset          ArenaEffectKind = "reset"
	ArenaEffectRelease        ArenaEffectKind = "release"
)

type ArenaEffectSite struct {
	Kind        ArenaEffectKind
	Arena       string
	Source      lexer.Token
	MayAllocate bool
}

type ArenaCallableSummary struct {
	DirectEffects  []ArenaEffectSite
	MayAllocate    bool
	AllocationPath []CallableID
}

type CallableNode struct {
	ID          CallableID
	Name        string
	Module      string
	ImplTarget  string
	Declaration lexer.Token
	Extern      bool
}

type CallSite struct {
	ID        CallSiteID
	Caller    CallableID
	Targets   []CallableID
	Source    lexer.Token
	Dispatch  CallDispatchKind
	Execution CallExecutionRelation
}

type CallRoot struct {
	ID         CallRootID
	Kind       CallRootKind
	Node       CallableID
	Source     lexer.Token
	ParentSite CallSiteID
}

// CallGraph is the compiler-owned semantic call graph for one Analyzer run.
// This initial graph records closed, validated direct and static-method calls.
type CallGraph struct {
	nodes        map[CallableID]CallableNode
	nodeOrder    []CallableID
	sites        []CallSite
	siteIDs      map[CallSiteID]bool
	roots        map[CallRootID]CallRoot
	rootOrder    []CallRootID
	arenaEffects map[CallableID][]ArenaEffectSite
}

func newCallGraph() *CallGraph {
	return &CallGraph{
		nodes:        map[CallableID]CallableNode{},
		siteIDs:      map[CallSiteID]bool{},
		roots:        map[CallRootID]CallRoot{},
		arenaEffects: map[CallableID][]ArenaEffectSite{},
	}
}

func (g *CallGraph) addArenaEffect(caller CallableID, effect ArenaEffectSite) {
	if g == nil || caller == "" || effect.Source.Line <= 0 || effect.Source.Column <= 0 {
		return
	}
	g.arenaEffects[caller] = append(g.arenaEffects[caller], effect)
}

func (g *CallGraph) addRoot(kind CallRootKind, node CallableID, source lexer.Token) CallRootID {
	if g == nil || node == "" {
		return ""
	}
	id := CallRootID(fmt.Sprintf("%s|%s", kind, node))
	if _, exists := g.roots[id]; !exists {
		g.roots[id] = CallRoot{ID: id, Kind: kind, Node: node, Source: source}
		g.rootOrder = append(g.rootOrder, id)
	}
	return id
}

func callableID(function Function) CallableID {
	token := function.Token
	return CallableID(fmt.Sprintf("%s|%s|%s:%d:%d", function.Module, function.Name, token.File, token.Line, token.Column))
}

func (g *CallGraph) addCallable(function Function) CallableID {
	if g == nil {
		return ""
	}
	id := callableID(function)
	if _, exists := g.nodes[id]; !exists {
		g.nodes[id] = CallableNode{
			ID:          id,
			Name:        function.Name,
			Module:      function.Module,
			ImplTarget:  function.ImplTarget,
			Declaration: function.Token,
			Extern:      function.Extern,
		}
		g.nodeOrder = append(g.nodeOrder, id)
	}
	return id
}

func (g *CallGraph) addCall(caller CallableID, target Function, source lexer.Token, dispatch CallDispatchKind, execution CallExecutionRelation) {
	if g == nil || caller == "" || source.Line <= 0 || source.Column <= 0 {
		return
	}
	targetID := g.addCallable(target)
	id := CallSiteID(fmt.Sprintf("%s|%s:%d:%d|%s|%s", caller, source.File, source.Line, source.Column, dispatch, execution))
	if g.siteIDs[id] {
		return
	}
	g.siteIDs[id] = true
	g.sites = append(g.sites, CallSite{
		ID:        id,
		Caller:    caller,
		Targets:   []CallableID{targetID},
		Source:    source,
		Dispatch:  dispatch,
		Execution: execution,
	})
}

func (g *CallGraph) clone() *CallGraph {
	copyGraph := newCallGraph()
	if g == nil {
		return copyGraph
	}
	for _, id := range g.nodeOrder {
		copyGraph.nodes[id] = g.nodes[id]
		copyGraph.nodeOrder = append(copyGraph.nodeOrder, id)
	}
	for _, site := range g.sites {
		site.Targets = append([]CallableID(nil), site.Targets...)
		copyGraph.sites = append(copyGraph.sites, site)
		copyGraph.siteIDs[site.ID] = true
	}
	for _, id := range g.rootOrder {
		copyGraph.roots[id] = g.roots[id]
		copyGraph.rootOrder = append(copyGraph.rootOrder, id)
	}
	for id, effects := range g.arenaEffects {
		copyGraph.arenaEffects[id] = append([]ArenaEffectSite(nil), effects...)
	}
	return copyGraph
}

func (g *CallGraph) Nodes() []CallableNode {
	if g == nil {
		return nil
	}
	nodes := make([]CallableNode, 0, len(g.nodeOrder))
	for _, id := range g.nodeOrder {
		nodes = append(nodes, g.nodes[id])
	}
	return nodes
}

func (g *CallGraph) Node(id CallableID) (CallableNode, bool) {
	if g == nil {
		return CallableNode{}, false
	}
	node, ok := g.nodes[id]
	return node, ok
}

func (g *CallGraph) NodesForDeclaration(token lexer.Token) []CallableNode {
	if g == nil {
		return nil
	}
	var nodes []CallableNode
	for _, id := range g.nodeOrder {
		node := g.nodes[id]
		if sourceTokenLocation(node.Declaration) == sourceTokenLocation(token) {
			nodes = append(nodes, node)
		}
	}
	return nodes
}

func (g *CallGraph) Incoming(id CallableID) []CallSite {
	if g == nil {
		return nil
	}
	var sites []CallSite
	for _, site := range g.sites {
		for _, target := range site.Targets {
			if target == id {
				sites = append(sites, cloneCallSite(site))
				break
			}
		}
	}
	sortCallSites(sites)
	return sites
}

func (g *CallGraph) Outgoing(id CallableID) []CallSite {
	if g == nil {
		return nil
	}
	var sites []CallSite
	for _, site := range g.sites {
		if site.Caller == id {
			sites = append(sites, cloneCallSite(site))
		}
	}
	sortCallSites(sites)
	return sites
}

func (g *CallGraph) Roots() []CallRoot {
	if g == nil {
		return nil
	}
	roots := make([]CallRoot, 0, len(g.rootOrder))
	for _, id := range g.rootOrder {
		roots = append(roots, g.roots[id])
	}
	reachable := map[CallableID]bool{}
	for _, root := range roots {
		for id := range g.reachableIDsFrom(root.Node) {
			reachable[id] = true
		}
	}
	for _, site := range g.sites {
		if !reachable[site.Caller] {
			continue
		}
		kind, ok := derivedRootKind(site.Execution)
		if !ok {
			continue
		}
		for _, target := range site.Targets {
			roots = append(roots, CallRoot{
				ID:         CallRootID(fmt.Sprintf("%s|%s|%s", kind, site.ID, target)),
				Kind:       kind,
				Node:       target,
				Source:     site.Source,
				ParentSite: site.ID,
			})
		}
	}
	return roots
}

func (g *CallGraph) ReachableFrom(rootID CallRootID) []CallableNode {
	if g == nil {
		return nil
	}
	var root CallRoot
	found := false
	for _, candidate := range g.Roots() {
		if candidate.ID == rootID {
			root = candidate
			found = true
			break
		}
	}
	if !found {
		return nil
	}
	return g.nodesInOrder(g.reachableIDsFrom(root.Node))
}

func (g *CallGraph) RootsReaching(id CallableID) []CallRoot {
	if g == nil {
		return nil
	}
	var roots []CallRoot
	for _, root := range g.Roots() {
		for _, node := range g.ReachableFrom(root.ID) {
			if node.ID == id {
				roots = append(roots, root)
				break
			}
		}
	}
	return roots
}

func derivedRootKind(execution CallExecutionRelation) (CallRootKind, bool) {
	switch execution {
	case CallExecutionSpawnTask:
		return CallRootTaskEntry, true
	case CallExecutionSpawnThread:
		return CallRootThreadEntry, true
	default:
		return "", false
	}
}

func (g *CallGraph) reachableIDsFrom(start CallableID) map[CallableID]bool {
	reachable := map[CallableID]bool{}
	queue := []CallableID{start}
	for len(queue) > 0 {
		id := queue[0]
		queue = queue[1:]
		if reachable[id] {
			continue
		}
		reachable[id] = true
		for _, target := range g.reachabilityTargets(id) {
			if !reachable[target] {
				queue = append(queue, target)
			}
		}
	}
	return reachable
}

func (g *CallGraph) reachabilityTargets(id CallableID) []CallableID {
	seen := map[CallableID]bool{}
	var targets []CallableID
	for _, site := range g.sites {
		if site.Caller != id || !executionContributesReachability(site.Execution) {
			continue
		}
		for _, target := range site.Targets {
			if target == "" || seen[target] {
				continue
			}
			seen[target] = true
			targets = append(targets, target)
		}
	}
	return targets
}

func executionContributesReachability(execution CallExecutionRelation) bool {
	switch execution {
	case CallExecutionSynchronous, CallExecutionSpawnTask, CallExecutionSpawnThread:
		return true
	default:
		return false
	}
}

func (g *CallGraph) SameStackSCC(id CallableID) []CallableNode {
	if g == nil {
		return nil
	}
	for _, component := range g.sameStackComponents() {
		if component[id] {
			return g.nodesInOrder(component)
		}
	}
	return nil
}

func (g *CallGraph) IsSameStackRecursive(id CallableID) bool {
	component := g.SameStackSCC(id)
	if len(component) > 1 {
		return true
	}
	if len(component) == 0 {
		return false
	}
	for _, target := range g.sameStackTargets(id) {
		if target == id {
			return true
		}
	}
	return false
}

func (g *CallGraph) ArenaSummary(id CallableID) ArenaCallableSummary {
	if g == nil {
		return ArenaCallableSummary{}
	}
	summary := ArenaCallableSummary{
		DirectEffects: append([]ArenaEffectSite(nil), g.arenaEffects[id]...),
	}
	summary.AllocationPath = g.synchronousPathTo(id, func(candidate CallableID) bool {
		for _, effect := range g.arenaEffects[candidate] {
			if effect.MayAllocate {
				return true
			}
		}
		return false
	})
	summary.MayAllocate = len(summary.AllocationPath) > 0
	return summary
}

func (g *CallGraph) synchronousPathTo(start CallableID, predicate func(CallableID) bool) []CallableID {
	if start == "" || predicate == nil {
		return nil
	}
	type pathEntry struct {
		id   CallableID
		path []CallableID
	}
	queue := []pathEntry{{id: start, path: []CallableID{start}}}
	visited := map[CallableID]bool{}
	for len(queue) > 0 {
		entry := queue[0]
		queue = queue[1:]
		if visited[entry.id] {
			continue
		}
		visited[entry.id] = true
		if predicate(entry.id) {
			return entry.path
		}
		for _, target := range g.sameStackTargets(entry.id) {
			if visited[target] {
				continue
			}
			path := append([]CallableID(nil), entry.path...)
			path = append(path, target)
			queue = append(queue, pathEntry{id: target, path: path})
		}
	}
	return nil
}

func (g *CallGraph) sameStackComponents() []map[CallableID]bool {
	index := 0
	indices := map[CallableID]int{}
	lowlinks := map[CallableID]int{}
	onStack := map[CallableID]bool{}
	stack := make([]CallableID, 0, len(g.nodeOrder))
	components := make([]map[CallableID]bool, 0)

	var visit func(CallableID)
	visit = func(id CallableID) {
		indices[id] = index
		lowlinks[id] = index
		index++
		stack = append(stack, id)
		onStack[id] = true

		for _, target := range g.sameStackTargets(id) {
			if _, visited := indices[target]; !visited {
				visit(target)
				if lowlinks[target] < lowlinks[id] {
					lowlinks[id] = lowlinks[target]
				}
			} else if onStack[target] && indices[target] < lowlinks[id] {
				lowlinks[id] = indices[target]
			}
		}

		if lowlinks[id] != indices[id] {
			return
		}
		component := map[CallableID]bool{}
		for len(stack) > 0 {
			last := len(stack) - 1
			member := stack[last]
			stack = stack[:last]
			onStack[member] = false
			component[member] = true
			if member == id {
				break
			}
		}
		components = append(components, component)
	}

	for _, id := range g.nodeOrder {
		if _, visited := indices[id]; !visited {
			visit(id)
		}
	}
	return components
}

func (g *CallGraph) sameStackTargets(id CallableID) []CallableID {
	seen := map[CallableID]bool{}
	var targets []CallableID
	for _, site := range g.sites {
		if site.Caller != id || site.Execution != CallExecutionSynchronous {
			continue
		}
		for _, target := range site.Targets {
			if target == "" || seen[target] {
				continue
			}
			seen[target] = true
			targets = append(targets, target)
		}
	}
	return targets
}

func (g *CallGraph) nodesInOrder(included map[CallableID]bool) []CallableNode {
	nodes := make([]CallableNode, 0, len(included))
	for _, id := range g.nodeOrder {
		if included[id] {
			nodes = append(nodes, g.nodes[id])
		}
	}
	return nodes
}

func cloneCallSite(site CallSite) CallSite {
	site.Targets = append([]CallableID(nil), site.Targets...)
	return site
}

func sortCallSites(sites []CallSite) {
	sort.SliceStable(sites, func(i int, j int) bool {
		left := sites[i].Source
		right := sites[j].Source
		if left.File != right.File {
			return left.File < right.File
		}
		if left.Line != right.Line {
			return left.Line < right.Line
		}
		return left.Column < right.Column
	})
}
