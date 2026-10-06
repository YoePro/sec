package sema

import (
	"fmt"
	"sort"

	"sec/internal/diagnostics"
	"sec/internal/lexer"
)

// CallableID identifies a semantic declaration or lexical callable body,
// independently of current source coordinates. Concrete graph views qualify
// these identities by their explicit compilation scope.
type CallableID string

// CallableBodyID identifies one executable callable body in an analysis
// snapshot. It is distinct from function-type identity and runtime value
// identity.
//
// Rules:
//   - rules/analysis/closure_analysis.md — "Callable body"
type CallableBodyID string

// CallableContractID identifies a compiler-visible open callable contract.
// Exact facts leave it empty; later callable-flow widening may attach one
// without discarding still-known concrete targets.
//
// Rules:
//   - rules/analysis/closure_analysis.md — "Function type versus callable contract"
type CallableContractID string

// CallableTargetSet is the canonical call-graph may-target representation. A
// closed set contains every possible concrete target. An open set must carry a
// covering callable contract.
//
// Rules:
//   - rules/analysis/closure_analysis.md — "Callable target sets"
//   - rules/analysis/closure_analysis.md — "Soundness of target sets"
type CallableTargetSet struct {
	KnownTargets    []CallableBodyID
	IsClosed        bool
	OpenContract    CallableContractID
	HasOpenContract bool
}

// exactCallableTargetSet constructs the one-target closed form produced by a
// statically selected callable body.
//
// Rules:
//   - rules/analysis/closure_analysis.md — "Exact"
func exactCallableTargetSet(body CallableBodyID) CallableTargetSet {
	return CallableTargetSet{KnownTargets: []CallableBodyID{body}, IsClosed: true}
}

// CallSiteID identifies one semantic invocation within its containing callable.
type CallSiteID string

type CallRootID string

type CallRootKind string

const (
	CallRootTestEntry    CallRootKind = "test-entry"
	CallRootProgramEntry CallRootKind = "program-entry"
	CallRootTaskEntry    CallRootKind = "task-entry"
	CallRootThreadEntry  CallRootKind = "thread-entry"
)

type CallDispatchKind string

const (
	CallDispatchGenerated     CallDispatchKind = "compiler-generated"
	CallDispatchDirect        CallDispatchKind = "direct"
	CallDispatchStaticMethod  CallDispatchKind = "static-method"
	CallDispatchClosure       CallDispatchKind = "closure"
	CallDispatchFunctionValue CallDispatchKind = "function-value"
	CallDispatchForeign       CallDispatchKind = "foreign-direct"
)

type CallExecutionRelation string

const (
	CallExecutionDeferred     CallExecutionRelation = "deferred"
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
	// ArenaEffectUnknownCallee is a call whose target is not known here (a
	// function value, an interface method, or a constrained generic method),
	// so its allocation behavior is unknown.
	ArenaEffectUnknownCallee ArenaEffectKind = "may-allocate-unknown-callee"
	// ArenaEffectForeign is a call to an extern function without a trusted
	// @noAlloc foreign contract.
	ArenaEffectForeign ArenaEffectKind = "may-allocate-foreign"
)

type ArenaEffectSite struct {
	Kind        ArenaEffectKind
	Arena       string
	Source      lexer.Token
	MayAllocate bool
	// UnknownAllocation marks a site whose allocation behavior is unknown;
	// it is not proof of allocation, but never proof of allocation freedom
	// (rules/memory/allocation.md § 24(6)).
	UnknownAllocation bool
}

type ArenaCallableSummary struct {
	DirectEffects  []ArenaEffectSite
	MayAllocate    bool
	AllocationPath []CallableID
	// AllocationUnknown reports a synchronous path to a site whose allocation
	// behavior is unknown, and UnknownAllocationPath the shortest such path.
	AllocationUnknown     bool
	UnknownAllocationPath []CallableID
}

// EffectKind identifies a language-semantic callable effect. These facts are
// recorded before optimization and are therefore suitable for verified
// guarantees such as @noPanic.
type EffectKind string

const (
	EffectMayPanicArithmetic  EffectKind = "may-panic-arithmetic"
	EffectMayPanicBounds      EffectKind = "may-panic-bounds"
	EffectMayPanicExplicit    EffectKind = "may-panic-explicit"
	EffectMayPanicAssertion   EffectKind = "may-panic-assertion"
	EffectMayPanicUnreachable EffectKind = "may-panic-unreachable"
	// EffectMayPanicContract is a run-time conversion into a constrained
	// named type outside try (rules/errors/panic.md ContractFailure).
	EffectMayPanicContract EffectKind = "may-panic-contract"
	// EffectMayPanicForeign is a call to an extern function without a trusted
	// @noPanic foreign contract (rules/errors/panic.md § 19(3), ForeignAbort).
	EffectMayPanicForeign EffectKind = "may-panic-foreign"
	// EffectMayPanicUnknownCallee is a call through a function value whose
	// target set is not known, so its panic behavior cannot be proven.
	EffectMayPanicUnknownCallee       EffectKind = "may-panic-unknown-callee"
	EffectMayUseNondeterministicInput EffectKind = "may-use-nondeterministic-input"
	EffectVolatileRead                EffectKind = "volatile-read"
	EffectVolatileWrite               EffectKind = "volatile-write"
)

type EffectSite struct {
	Kind           EffectKind
	Source         lexer.Token
	PanicReasonIDs []diagnostics.PanicReasonID
}

type CallableEffectSummary struct {
	DirectEffects []EffectSite
	MayPanic      bool
	PanicPath     []CallableID
}

type CallableNode struct {
	Kind        CallableBodyKind
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
	TargetSet CallableTargetSet
	Source    lexer.Token
	Dispatch  CallDispatchKind
	Execution CallExecutionRelation
}

type CallRoot struct {
	// Scope qualifies concrete plan roots; unbound Sema roots have zero scope.
	Scope      CallGraphScope
	ID         CallRootID
	Kind       CallRootKind
	Node       CallableID
	Source     lexer.Token
	ParentSite CallSiteID
}

// CallGraph is the compiler-owned semantic call graph for one Analyzer run.
// It retains canonical targets and execution relationships; a concrete view is
// bound to exactly one explicit CompilationPlan.
type CallGraph struct {
	scope         CallGraphScope
	syntaxOrigins map[sourceTokenKey]string
	nodes         map[CallableID]CallableNode
	nodeOrder     []CallableID
	bodyNodes     map[CallableBodyID]CallableID
	sites         []CallSite
	siteIDs       map[CallSiteID]bool
	roots         map[CallRootID]CallRoot
	rootOrder     []CallRootID
	arenaEffects  map[CallableID][]ArenaEffectSite
	effects       map[CallableID][]EffectSite
	blockEffects  map[CallableID][]BlockEffectSite
}

func newCallGraph() *CallGraph {
	return &CallGraph{
		nodes:        map[CallableID]CallableNode{},
		bodyNodes:    map[CallableBodyID]CallableID{},
		siteIDs:      map[CallSiteID]bool{},
		roots:        map[CallRootID]CallRoot{},
		arenaEffects: map[CallableID][]ArenaEffectSite{},
		effects:      map[CallableID][]EffectSite{},
		blockEffects: map[CallableID][]BlockEffectSite{},
	}
}

func (g *CallGraph) addEffect(caller CallableID, effect EffectSite) {
	if g == nil || caller == "" || effect.Kind == "" || effect.Source.Line <= 0 || effect.Source.Column <= 0 {
		return
	}
	g.effects[caller] = append(g.effects[caller], effect)
}

func (g *CallGraph) removeEffect(caller CallableID, kind EffectKind, source lexer.Token) {
	if g == nil || caller == "" {
		return
	}
	effects := g.effects[caller]
	for index := len(effects) - 1; index >= 0; index-- {
		candidate := effects[index]
		if candidate.Kind == kind && sourceTokenLocation(candidate.Source) == sourceTokenLocation(source) {
			g.effects[caller] = append(effects[:index], effects[index+1:]...)
			return
		}
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

// callableBodyID maps a resolved named declaration to the body identity shared
// by callable creation facts and direct call-graph target sets.
//
// Rules:
//   - rules/analysis/closure_analysis.md — "Named functions", "Callable body"
func callableBodyID(function Function) CallableBodyID {
	return CallableBodyID("callable-body|" + string(callableID(function)))
}

// addCallable registers a resolved named declaration and its concrete body.
// Rules: rules/analysis/call_graph.md — "Callable node", "Callable node identity".
func (g *CallGraph) addCallable(function Function) CallableID {
	if g == nil {
		return ""
	}
	id := callableID(function)
	if _, exists := g.nodes[id]; !exists {
		g.nodes[id] = CallableNode{
			ID:          id,
			Kind:        CallableBodyNamedFunction,
			Name:        function.Name,
			Module:      function.Module,
			ImplTarget:  function.ImplTarget,
			Declaration: function.Token,
			Extern:      function.Extern,
		}
		g.nodeOrder = append(g.nodeOrder, id)
	}
	g.bodyNodes[callableBodyID(function)] = id
	return id
}

// addClosureCallable registers a lambda body as a concrete callable node while
// retaining its distinct callable-body identity for target-set resolution.
//
// Rules:
//   - rules/analysis/call_graph.md — "Callable node" and "Callable node identity"
//   - rules/analysis/closure_analysis.md — "Callable body"
func (g *CallGraph) addClosureCallable(identity ResolvedCallableIdentity, module string) CallableID {
	if g == nil || identity.Body == "" || identity.Source.Line <= 0 || identity.Source.Column <= 0 {
		return ""
	}
	if existing, ok := g.bodyNodes[identity.Body]; ok {
		return existing
	}
	id := CallableID(identity.Body)
	g.nodes[id] = CallableNode{
		ID: id, Kind: identity.Kind, Name: "lambda", Module: module, Declaration: identity.Source,
	}
	g.nodeOrder = append(g.nodeOrder, id)
	g.bodyNodes[identity.Body] = id
	return id
}

// addTargetSetCall records one indirect invocation without expanding it into
// unrelated anonymous call sites. Known bodies are linked to their canonical
// graph nodes and the complete target-set fact remains attached to the site.
//
// Rules:
//   - rules/analysis/call_graph.md — "Call-site record"
//   - rules/analysis/call_graph.md — "Dispatch kinds"
//   - rules/analysis/closure_analysis.md — "Soundness of target sets"
func (g *CallGraph) addTargetSetCall(caller CallableID, targets CallableTargetSet, source lexer.Token, dispatch CallDispatchKind, execution CallExecutionRelation) {
	if g == nil || caller == "" || source.Line <= 0 || source.Column <= 0 {
		return
	}
	resolved := make([]CallableID, 0, len(targets.KnownTargets))
	for _, body := range targets.KnownTargets {
		if target, ok := g.bodyNodes[body]; ok {
			resolved = append(resolved, target)
		}
	}
	// A closed target set must never lose a concrete body at the graph boundary.
	if targets.IsClosed && len(resolved) != len(targets.KnownTargets) {
		return
	}
	id := g.semanticCallSiteID(caller, source, dispatch, execution)
	if g.siteIDs[id] {
		return
	}
	g.siteIDs[id] = true
	g.sites = append(g.sites, CallSite{
		ID: id, Caller: caller, Targets: resolved, TargetSet: cloneCallableTargetSet(targets),
		Source: source, Dispatch: dispatch, Execution: execution,
	})
}

func (g *CallGraph) addCall(caller CallableID, target Function, source lexer.Token, dispatch CallDispatchKind, execution CallExecutionRelation) {
	if g == nil || caller == "" || source.Line <= 0 || source.Column <= 0 {
		return
	}
	targetID := g.addCallable(target)
	id := g.semanticCallSiteID(caller, source, dispatch, execution)
	if g.siteIDs[id] {
		return
	}
	g.siteIDs[id] = true
	g.sites = append(g.sites, CallSite{
		ID:        id,
		Caller:    caller,
		Targets:   []CallableID{targetID},
		TargetSet: exactCallableTargetSet(callableBodyID(target)),
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
	copyGraph.scope = g.scope
	if g.syntaxOrigins != nil {
		copyGraph.syntaxOrigins = map[sourceTokenKey]string{}
		for key, origin := range g.syntaxOrigins {
			copyGraph.syntaxOrigins[key] = origin
		}
	}
	for _, id := range g.nodeOrder {
		copyGraph.nodes[id] = g.nodes[id]
		copyGraph.nodeOrder = append(copyGraph.nodeOrder, id)
	}
	for body, id := range g.bodyNodes {
		copyGraph.bodyNodes[body] = id
	}
	for _, site := range g.sites {
		site = cloneCallSite(site)
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
	for id, effects := range g.effects {
		copyGraph.effects[id] = cloneEffectSites(effects)
	}
	for id, effects := range g.blockEffects {
		copyGraph.blockEffects[id] = append([]BlockEffectSite(nil), effects...)
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
				Scope:      g.scope,
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

// executionContributesReachability includes same-stack cleanup and represented
// worker entry relationships in the root reachability view.
// Rules: rules/analysis/call_graph.md — "Same-stack execution", "Reachability".
func executionContributesReachability(execution CallExecutionRelation) bool {
	switch execution {
	case CallExecutionSynchronous, CallExecutionDeferred, CallExecutionSpawnTask, CallExecutionSpawnThread:
		return true
	default:
		return false
	}
}

func (g *CallGraph) SameStackSCC(id CallableID) []CallableNode {
	if g == nil {
		return nil
	}
	for _, component := range g.componentsForTargets(g.sameStackTargets) {
		if component[id] {
			return g.nodesInOrder(component)
		}
	}
	return nil
}

// TaskSpawnSCC returns the strongly connected component containing id when
// only task-spawn execution relationships are followed.
//
// Rules:
//   - rules/analysis/call_graph.md — "Spawn cycles"
//   - rules/analysis/call_graph.md — Appendix A.20 "SCC views"
func (g *CallGraph) TaskSpawnSCC(id CallableID) []CallableNode {
	return g.executionSCC(id, CallExecutionSpawnTask)
}

// IsInTaskSpawnCycle reports whether id participates in mutual or direct
// self-spawning through task-spawn relationships.
//
// Rules:
//   - rules/analysis/call_graph.md — "Spawn cycles"
func (g *CallGraph) IsInTaskSpawnCycle(id CallableID) bool {
	return g.isInExecutionCycle(id, CallExecutionSpawnTask)
}

// ThreadStartSCC returns the strongly connected component containing id when
// only thread-start execution relationships are followed.
//
// Rules:
//   - rules/analysis/call_graph.md — "Thread-start cycles"
//   - rules/analysis/call_graph.md — Appendix A.20 "SCC views"
func (g *CallGraph) ThreadStartSCC(id CallableID) []CallableNode {
	return g.executionSCC(id, CallExecutionSpawnThread)
}

// IsInThreadStartCycle reports whether id participates in mutual or direct
// self-starting through thread-start relationships.
//
// Rules:
//   - rules/analysis/call_graph.md — "Thread-start cycles"
func (g *CallGraph) IsInThreadStartCycle(id CallableID) bool {
	return g.isInExecutionCycle(id, CallExecutionSpawnThread)
}

// ProcessLaunchSCC returns the strongly connected component containing id when
// only process-launch execution relationships are followed.
//
// Rules:
//   - rules/analysis/call_graph.md — "Process-launch cycles"
//   - rules/analysis/call_graph.md — Appendix A.20 "SCC views"
func (g *CallGraph) ProcessLaunchSCC(id CallableID) []CallableNode {
	return g.executionSCC(id, CallExecutionSpawnProcess)
}

// IsInProcessLaunchCycle reports whether id participates in mutual or direct
// self-launching through process-launch relationships.
//
// Rules:
//   - rules/analysis/call_graph.md — "Process-launch cycles"
func (g *CallGraph) IsInProcessLaunchCycle(id CallableID) bool {
	return g.isInExecutionCycle(id, CallExecutionSpawnProcess)
}

// CompleteExecutionSCC returns the strongly connected component containing id
// across every represented execution relationship. It is an analysis and
// visualization view and must not be used as proof of same-stack recursion.
//
// Rules:
//   - rules/analysis/call_graph.md — "Complete execution SCC"
//   - rules/analysis/call_graph.md — Appendix A.20 "SCC views"
func (g *CallGraph) CompleteExecutionSCC(id CallableID) []CallableNode {
	if g == nil {
		return nil
	}
	for _, component := range g.componentsForTargets(g.completeExecutionTargets) {
		if component[id] {
			return g.nodesInOrder(component)
		}
	}
	return nil
}

// executionSCC selects one execution-boundary relation from the canonical
// graph and returns the component containing id.
//
// Rules:
//   - rules/analysis/call_graph.md — "One canonical graph, multiple analysis views"
//   - rules/analysis/call_graph.md — Appendix A.20 "SCC views"
func (g *CallGraph) executionSCC(id CallableID, execution CallExecutionRelation) []CallableNode {
	if g == nil {
		return nil
	}
	targets := func(caller CallableID) []CallableID {
		return g.targetsForExecution(caller, execution)
	}
	for _, component := range g.componentsForTargets(targets) {
		if component[id] {
			return g.nodesInOrder(component)
		}
	}
	return nil
}

// isInExecutionCycle distinguishes a true execution-boundary cycle from the
// singleton SCC that every acyclic node occupies.
//
// Rules:
//   - rules/analysis/call_graph.md — "Spawn cycles"
//   - rules/analysis/call_graph.md — "Thread-start cycles"
//   - rules/analysis/call_graph.md — "Process-launch cycles"
func (g *CallGraph) isInExecutionCycle(id CallableID, execution CallExecutionRelation) bool {
	component := g.executionSCC(id, execution)
	if len(component) > 1 {
		return true
	}
	if len(component) == 0 {
		return false
	}
	for _, target := range g.targetsForExecution(id, execution) {
		if target == id {
			return true
		}
	}
	return false
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
	summary.UnknownAllocationPath = g.synchronousPathTo(id, func(candidate CallableID) bool {
		for _, effect := range g.arenaEffects[candidate] {
			if effect.UnknownAllocation {
				return true
			}
		}
		return false
	})
	summary.AllocationUnknown = len(summary.UnknownAllocationPath) > 0
	return summary
}

// EffectSummary returns direct semantic effects and a deterministic shortest
// synchronous cause path for transitive panic behavior, including reachable
// checked unreachable statements.
//
// Rules:
//   - rules/errors/panic.md — § 16(5)–(6) "Checked unreachable"
//   - rules/errors/panic.md — § 21 "@noPanic"
func (g *CallGraph) EffectSummary(id CallableID) CallableEffectSummary {
	if g == nil {
		return CallableEffectSummary{}
	}
	summary := CallableEffectSummary{DirectEffects: cloneEffectSites(g.effects[id])}
	summary.PanicPath = g.synchronousPathTo(id, func(candidate CallableID) bool {
		for _, effect := range g.effects[candidate] {
			if isPanicEffectKind(effect.Kind) {
				return true
			}
		}
		return false
	})
	summary.MayPanic = len(summary.PanicPath) > 0
	return summary
}

func cloneEffectSites(sites []EffectSite) []EffectSite {
	cloned := append([]EffectSite(nil), sites...)
	for index := range cloned {
		cloned[index].PanicReasonIDs = append([]diagnostics.PanicReasonID(nil), cloned[index].PanicReasonIDs...)
	}
	return cloned
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

// componentsForTargets computes deterministic Tarjan components over one
// execution-specific target view of the canonical call graph.
//
// Rules:
//   - rules/analysis/call_graph.md — "One canonical graph, multiple analysis views"
//   - rules/analysis/call_graph.md — Appendix A.20 "SCC views"
func (g *CallGraph) componentsForTargets(targets func(CallableID) []CallableID) []map[CallableID]bool {
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

		for _, target := range targets(id) {
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

// sameStackComponents supplies the canonical same-stack SCC ordering to
// interprocedural fixed-point consumers.
//
// Rules:
//   - rules/analysis/call_graph.md — "Recursion"
//   - rules/analysis/call_graph.md — Appendix A.20 "SCC views"
func (g *CallGraph) sameStackComponents() []map[CallableID]bool {
	if g == nil {
		return nil
	}
	return g.componentsForTargets(g.sameStackTargets)
}

// targetsForExecution returns targets connected by exactly one execution
// relation for execution-specific analyses.
//
// Rules:
//   - rules/analysis/call_graph.md — "Execution relations"
func (g *CallGraph) targetsForExecution(id CallableID, execution CallExecutionRelation) []CallableID {
	return g.filteredTargets(id, func(site CallSite) bool {
		return site.Execution == execution
	})
}

// completeExecutionTargets returns targets from every currently represented
// execution relationship without collapsing their metadata in the graph.
//
// Rules:
//   - rules/analysis/call_graph.md — "Complete execution SCC"
func (g *CallGraph) completeExecutionTargets(id CallableID) []CallableID {
	return g.filteredTargets(id, func(site CallSite) bool {
		switch site.Execution {
		case CallExecutionSynchronous, CallExecutionDeferred, CallExecutionSpawnTask, CallExecutionSpawnThread, CallExecutionSpawnProcess:
			return true
		default:
			return false
		}
	})
}

// sameStackTargets retains ordinary and deferred edges for effect, recursion
// and stack consumers without flattening their execution metadata.
// Rules: rules/analysis/call_graph.md — "Same-stack execution", "Recursive `defer`".
func (g *CallGraph) sameStackTargets(id CallableID) []CallableID {
	return g.filteredTargets(id, func(site CallSite) bool {
		return sameStackExecution(site.Execution)
	})
}

// filteredTargets derives one deterministic edge view while preserving the
// canonical call sites as the sole relationship source.
//
// Rules:
//   - rules/analysis/call_graph.md — "One canonical graph, multiple analysis views"
func (g *CallGraph) filteredTargets(id CallableID, include func(CallSite) bool) []CallableID {
	seen := map[CallableID]bool{}
	var targets []CallableID
	for _, site := range g.sites {
		if site.Caller != id || !include(site) {
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
	site.TargetSet = cloneCallableTargetSet(site.TargetSet)
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

// isPanicEffectKind reports the direct effect kinds that make a callable
// panic-capable for @noPanic and transitive panic paths.
//
// Rules:
//   - rules/errors/panic.md — § 21 "@noPanic", registered panic reasons
//
// IsPanicEffectKind exposes the panic-effect classification to tooling.
func IsPanicEffectKind(kind EffectKind) bool { return isPanicEffectKind(kind) }

func isPanicEffectKind(kind EffectKind) bool {
	switch kind {
	case EffectMayPanicArithmetic, EffectMayPanicBounds, EffectMayPanicExplicit, EffectMayPanicAssertion, EffectMayPanicUnreachable, EffectMayPanicContract, EffectMayPanicForeign, EffectMayPanicUnknownCallee:
		return true
	}
	return false
}
