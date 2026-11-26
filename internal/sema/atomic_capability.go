package sema

import (
	"fmt"
	"sort"

	"sec/internal/ast"
	"sec/internal/diagnostics"
	"sec/internal/layout"
	"sec/internal/lexer"
)

// atomicContextRequirement is one resolved invocation contract of an atomic
// operation: a compiler-known category and the additional requirements the
// contract places on the implementation (not claims about it).
// Rules: rules/concurrency/atomics.md — § 37A(8); rules/platform/target_profiles.md — § 19.
type atomicContextRequirement struct {
	Context       layout.AtomicContext
	NoBlocking    bool
	InterruptSafe bool
}

// ResolvedAtomicCapability is the MD-095 capability result of one atomic
// operation under one applicable execution context: the requirement queried
// from the selected plan and its availability. Supported keeps the evidence;
// Unsupported has also been reported (S1261); Unknown is an unfulfilled proof
// obligation whose Missing names what kept it open (no plan, symbolic or
// unresolved carrier, incomplete atomic layout, unresolved reachability, or
// a plan without a fact). Analysis continues on Unknown; executable emission
// must discharge it or fail closed (lowering.atomics-v2).
// Rules: rules/concurrency/atomics.md — §§ 37A(2)-(3), 37A(9)-(13), 46(7).
type ResolvedAtomicCapability struct {
	Source       lexer.Token
	Callable     CallableID
	Requirement  layout.AtomicRequirement
	Availability layout.AtomicAvailability
	Capability   layout.AtomicCapability
	Missing      string
}

// AtomicCapabilities lists the capability results in source order, then by
// context, as detached values.
// Rules: rules/concurrency/atomics.md — § 37A(12).
func (a *Analyzer) AtomicCapabilities() []ResolvedAtomicCapability {
	if a == nil {
		return nil
	}
	return append([]ResolvedAtomicCapability(nil), a.atomicCapabilities...)
}

// resolveAtomicCapabilities runs after every callable body is analyzed, so
// the call graph and the validated atomic facts are complete. Source
// validity was already established: a target fact never legalizes or hides
// a source error (§ 37A(2)-(3)).
// Rules: rules/concurrency/atomics.md — §§ 37A(2), 37A(8)-(12), 48(4).
func (a *Analyzer) resolveAtomicCapabilities(program *ast.Program) {
	a.atomicCapabilities = nil
	contexts := a.atomicExecutionContexts(program)
	for _, operation := range a.AtomicMemoryOperations() {
		a.resolveAtomicOperationCapability(operation, contexts)
	}
	for _, node := range a.callGraph.Nodes() {
		for _, effect := range a.callGraph.EffectSummary(node.ID).DirectEffects {
			if effect.Kind != EffectAtomicFence || effect.AtomicOrder == "" {
				continue
			}
			key := layout.AtomicCapabilityKey{Operation: "Fence", Order: effect.AtomicOrder}
			a.queryAtomicCapability(effect.Source, node.ID, "", key, contexts, "")
		}
	}
	sort.SliceStable(a.atomicCapabilities, func(i, j int) bool {
		l, r := a.atomicCapabilities[i].Source, a.atomicCapabilities[j].Source
		if l.File != r.File {
			return l.File < r.File
		}
		return l.ByteStart < r.ByteStart
	})
}

// resolveAtomicOperationCapability builds the carrier key of one object
// operation: carrier from the type-identity service, representation, access
// extent and guaranteed alignment from the canonical Atomic[T] layout, and
// the requested orders (a CompareExchange keeps both).
// Rules: rules/concurrency/atomics.md — §§ 37A(4)-(7), 37A(9); rules/memory/layout.md — § 47.6.
func (a *Analyzer) resolveAtomicOperationCapability(operation ResolvedAtomicMemoryOperation, contexts map[CallableID][]atomicContextRequirement) {
	key := layout.AtomicCapabilityKey{Operation: operation.Operation}
	if operation.Operation == "CompareExchange" && len(operation.Paths) == 2 {
		key.Order, key.FailureOrder = operation.Paths[0].Order, operation.Paths[1].Order
	} else if len(operation.Paths) == 1 {
		key.Order = operation.Paths[0].Order
	}
	missing := ""
	identity := operation.TypeIdentity
	switch {
	case identity.Symbolic:
		missing = "symbolic carrier of " + identity.SemanticTypeID + " (needs specialization)"
	case identity.CarrierTypeID == "":
		missing = "unresolved atomic carrier of " + identity.SemanticTypeID
	default:
		key.CarrierTypeID = identity.CarrierTypeID
		access, why := a.atomicStorageAccess(operation.valueType)
		if why != "" {
			missing = why
		} else {
			key.Representation, key.SizeBytes, key.AlignmentBytes = access.Representation, access.SizeBytes, access.AlignBytes
		}
	}
	a.queryAtomicCapability(operation.Source, operation.Callable, identity.SemanticTypeID, key, contexts, missing)
}

// atomicStorageAccess resolves Atomic[T] through the plan's canonical layout
// resolver and returns its access facts, or the named missing fact.
// Rules: rules/memory/layout.md — §§ 47.5, 47.6; rules/concurrency/atomics.md — § 37A(7).
func (a *Analyzer) atomicStorageAccess(element Type) (layout.ResolvedAtomicAccess, string) {
	plan := a.targetCapabilityPlan
	if plan.PointerWidthBits == 0 || plan.Alignments == nil {
		return layout.ResolvedAtomicAccess{}, "the selected target has no verified data layout"
	}
	if a.atomicLayoutResolver == nil {
		resolver, err := layout.NewResolver(plan, plan.Alignments)
		if err != nil {
			return layout.ResolvedAtomicAccess{}, err.Error()
		}
		a.atomicLayoutResolver = resolver
	}
	contained := LayoutRequirementOf(element)
	resolved := a.atomicLayoutResolver.Resolve(layout.LayoutRequirement{TypeIdentity: "core:struct:Atomic[" + contained.TypeIdentity + "]", Kind: layout.LayoutAtomic, Carrier: &contained})
	if !resolved.Complete() {
		why := "incomplete atomic layout"
		if len(resolved.Missing) > 0 {
			why += ": " + resolved.Missing[0]
		}
		return layout.ResolvedAtomicAccess{}, why
	}
	return resolved.Atomic, ""
}

// queryAtomicCapability records one result per applicable context. A
// callable no root reaches keeps an unresolved-reachability obligation and is
// never assumed Ordinary; proven Unsupported is a focused target error.
// Rules: rules/concurrency/atomics.md — §§ 37A(8), 37A(11)-(12), 48(4).
func (a *Analyzer) queryAtomicCapability(source lexer.Token, callable CallableID, semantic string, key layout.AtomicCapabilityKey, contexts map[CallableID][]atomicContextRequirement, missing string) {
	required := contexts[callable]
	if len(required) == 0 {
		a.atomicCapabilities = append(a.atomicCapabilities, ResolvedAtomicCapability{Source: source, Callable: callable,
			Requirement:  layout.AtomicRequirement{SemanticTypeID: semantic, Key: key, Provenance: tokenProvenance(source)},
			Availability: layout.AtomicUnknown, Missing: "unresolved execution-context reachability"})
		return
	}
	for _, context := range required {
		keyed := key
		keyed.Context, keyed.NoBlocking, keyed.InterruptSafe = context.Context, context.NoBlocking, context.InterruptSafe
		result := ResolvedAtomicCapability{Source: source, Callable: callable,
			Requirement:  layout.AtomicRequirement{SemanticTypeID: semantic, Key: keyed, Provenance: tokenProvenance(source)},
			Availability: layout.AtomicUnknown, Missing: missing}
		if missing == "" {
			if a.targetCapabilityPlan.PointerWidthBits == 0 {
				result.Missing = "no selected CompilationPlan"
			} else if fact, err := a.targetCapabilityPlan.AtomicCapability(result.Requirement); err != nil {
				result.Missing = err.Error()
			} else {
				result.Availability, result.Capability = fact.Availability, fact
				if fact.Availability == layout.AtomicUnknown {
					result.Missing = "the selected plan has no capability fact for this request"
				}
			}
		}
		if result.Availability == layout.AtomicUnsupported {
			subject := "atomic.Fence"
			if semantic != "" {
				subject = "Atomic[" + semantic + "]." + key.Operation
			}
			orders := key.Order
			if key.FailureOrder != "" {
				orders += "/" + key.FailureOrder
			}
			a.addErrorAtTokenWithMetadata(source, diagnostics.AtomicTargetCapability,
				"Choose an operation or order the selected target supports, or select a target/profile that provides it. "+result.Capability.Reason,
				"selected target %s/%s profile %s does not support %s with order %s in %s context",
				a.targetCapabilityPlan.TargetOS, a.targetCapabilityPlan.TargetArch, a.targetCapabilityPlan.Profile, subject, orders, context.Context)
		}
		a.atomicCapabilities = append(a.atomicCapabilities, result)
	}
}

// tokenProvenance spells the source provenance of a capability requirement
// (rules/concurrency/atomics.md § 37A(9)).
func tokenProvenance(token lexer.Token) string {
	return fmt.Sprintf("%s:%d:%d", token.File, token.Line, token.Column)
}

// atomicExecutionContexts derives every callable's applicable invocation
// contracts from the existing call graph and attributes: program and test
// roots run Ordinary, spawned task and thread entries Task and Thread, and
// an @isr/@interrupt function is an Interrupt root. NoBlocking and
// InterruptSafe flow from @noBlock (including implied) and @interruptSafe or
// ISR callables to everything they reach; Interrupt always requires both.
// Contexts and contracts follow only same-execution edges (synchronous and
// deferred calls): a spawn starts its own Task or Thread root, so its body
// neither inherits the spawner's category nor its @noBlock contract. A
// logical task is never reclassified as Thread, and nothing is assumed
// Ordinary for a callable no root reaches.
// Rules: rules/concurrency/atomics.md — §§ 37A(8), 39(6); rules/platform/target_profiles.md — § 19.
func (a *Analyzer) atomicExecutionContexts(program *ast.Program) map[CallableID][]atomicContextRequirement {
	categories := map[CallableID]map[layout.AtomicContext]bool{}
	add := func(id CallableID, context layout.AtomicContext) {
		if categories[id] == nil {
			categories[id] = map[layout.AtomicContext]bool{}
		}
		categories[id][context] = true
	}
	for _, root := range a.callGraph.Roots() {
		context := layout.AtomicContextOrdinary
		switch root.Kind {
		case CallRootTaskEntry:
			context = layout.AtomicContextTask
		case CallRootThreadEntry:
			context = layout.AtomicContextThread
		}
		for id := range a.callGraph.sameExecutionReachable(root.Node) {
			add(id, context)
		}
	}
	noBlocking, interruptSafe := map[CallableID]bool{}, map[CallableID]bool{}
	visit := func(fn *ast.FunctionDeclaration, name string) {
		function, ok := a.lookupFunctionByToken(name, fn.Name.Token)
		if !ok {
			return
		}
		id := callableID(function)
		isr := hasAttribute(fn.Attributes, "isr") || hasAttribute(fn.Attributes, "interrupt")
		_, blocks := EffectiveGuaranteeOf(fn.Attributes, GuaranteeNoBlock)
		safe := isr || hasAttribute(fn.Attributes, "interruptSafe")
		for reached := range a.callGraph.sameExecutionReachable(id) {
			if isr {
				add(reached, layout.AtomicContextInterrupt)
			}
			noBlocking[reached] = noBlocking[reached] || blocks || safe
			interruptSafe[reached] = interruptSafe[reached] || safe
		}
	}
	a.withProgramModules(program, func(statement ast.Statement) {
		switch statement := statement.(type) {
		case *ast.FunctionDeclaration:
			visit(statement, statement.Name.Value)
		case *ast.ImplStatement:
			if statement.Target == nil || !a.validImplStatements[statement] {
				return
			}
			for _, member := range statement.Members {
				if fn, ok := member.(*ast.FunctionDeclaration); ok && fn != nil {
					visit(fn, statement.Target.Name+"."+fn.Name.Value)
				}
			}
		}
	})
	order := []layout.AtomicContext{layout.AtomicContextOrdinary, layout.AtomicContextTask, layout.AtomicContextThread, layout.AtomicContextInterrupt, layout.AtomicContextStartup, layout.AtomicContextShutdown}
	contexts := map[CallableID][]atomicContextRequirement{}
	for id, set := range categories {
		for _, context := range order {
			if !set[context] {
				continue
			}
			requirement := atomicContextRequirement{Context: context, NoBlocking: noBlocking[id], InterruptSafe: interruptSafe[id]}
			if context == layout.AtomicContextInterrupt {
				requirement.NoBlocking, requirement.InterruptSafe = true, true
			}
			contexts[id] = append(contexts[id], requirement)
		}
	}
	return contexts
}

// sameExecutionReachable lists the callables start reaches on its own
// execution: synchronous and deferred call edges, never spawn edges, which
// begin a separate Task, Thread or Process execution.
// Rules: rules/analysis/call_graph.md — "Same-stack execution";
// rules/concurrency/atomics.md — § 37A(8).
func (g *CallGraph) sameExecutionReachable(start CallableID) map[CallableID]bool {
	reachable := map[CallableID]bool{}
	queue := []CallableID{start}
	for len(queue) > 0 {
		id := queue[0]
		queue = queue[1:]
		if reachable[id] {
			continue
		}
		reachable[id] = true
		for _, site := range g.sites {
			if site.Caller != id || site.Execution != CallExecutionSynchronous && site.Execution != CallExecutionDeferred {
				continue
			}
			for _, target := range site.Targets {
				if target != "" && !reachable[target] {
					queue = append(queue, target)
				}
			}
		}
	}
	return reachable
}
