package sema

import (
	"sort"

	"sec/internal/lexer"
)

// ResolvedCleanupPlan (P17-24; package17 §§ 34–47, 79–80) is the detached,
// read-only cleanup plan of one invocation: its lexical scope tree, every
// registration site in source order (owned initialization, reinitialization
// and replacement, executed defer) and every exit edge with the scopes it
// leaves. Semantic IR builders consume it instead of reconstructing cleanup
// from variable names or SSA use counts. Registration multiplicity and
// conditionality are explicit (P17-21): a site inside a loop registers once
// per execution, and a site below the root scope registers only on the
// paths that execute it.

// CleanupRegistrationKind classifies a registration site.
type CleanupRegistrationKind string

const (
	CleanupRegisterOwned            CleanupRegistrationKind = "owned-initialization"
	CleanupRegisterReinitialization CleanupRegistrationKind = "reinitialization"
	CleanupRegisterReplacement      CleanupRegistrationKind = "replacement"
	CleanupRegisterDefer            CleanupRegistrationKind = "defer"
)

// ResolvedCleanupScope is one lexical scope of the plan.
type ResolvedCleanupScope struct {
	ID     CleanupScopeID
	Parent CleanupScopeID
	Source lexer.Token
	InLoop bool
}

// ResolvedCleanupRegistrationSite is one registration site. Owned sites carry
// the destination binding/Place and destruction plan; a defer site its
// capture plan. PerExecution marks a site inside a loop (one registration per
// execution, never collapsed); Conditional a site that not every path from
// the function entry executes. Replacement terminates the previous
// incarnation's action (Previous) before this one registers.
type ResolvedCleanupRegistrationSite struct {
	Kind         CleanupRegistrationKind
	Action       CleanupActionID
	Scope        CleanupScopeID
	Source       lexer.Token
	Binding      BindingID
	Place        Place
	Destruction  DestructionPlanSet
	Defer        *ResolvedDeferPlan
	Previous     CleanupPreviousValue
	PerExecution bool
	Conditional  bool
}

// ResolvedCleanupPlan is one invocation's cleanup plan.
type ResolvedCleanupPlan struct {
	Invocation    CallableID
	Root          CleanupScopeID
	Scopes        []ResolvedCleanupScope
	Registrations []ResolvedCleanupRegistrationSite
	Exits         []ResolvedCleanupExit
}

// ResolvedCleanupPlanOf returns the detached cleanup plan of a function. It
// neither infers nor changes analysis state; repeated queries are equal.
func (a *Analyzer) ResolvedCleanupPlanOf(function Function) (ResolvedCleanupPlan, bool) {
	if a == nil {
		return ResolvedCleanupPlan{}, false
	}
	return a.resolvedCleanupPlanOfInvocation(callableID(function))
}

func (a *Analyzer) resolvedCleanupPlanOfInvocation(invocation CallableID) (ResolvedCleanupPlan, bool) {
	flow := a.cleanupFlow
	root, ok := flow.roots[invocation]
	if !ok {
		return ResolvedCleanupPlan{}, false
	}
	plan := ResolvedCleanupPlan{Invocation: invocation, Root: root}
	for id, facts := range flow.scopes {
		if facts.Invocation == invocation && !facts.Defer {
			plan.Scopes = append(plan.Scopes, ResolvedCleanupScope{ID: id, Parent: facts.Parent, Source: facts.Source, InLoop: facts.InLoop})
		}
	}
	sort.Slice(plan.Scopes, func(i, j int) bool { return plan.Scopes[i].ID < plan.Scopes[j].ID })
	site := func(kind CleanupRegistrationKind, registration ResolvedOwnedCleanupRegistration, source lexer.Token) ResolvedCleanupRegistrationSite {
		detached := cloneOwnedCleanupRegistration(registration)
		return ResolvedCleanupRegistrationSite{Kind: kind, Action: detached.Action, Scope: detached.Scope, Source: source,
			Binding: detached.Binding, Place: detached.Place, Destruction: detached.Destruction}
	}
	for stmt, registration := range a.ownedCleanupInitializations {
		if registration.Invocation == invocation {
			plan.Registrations = append(plan.Registrations, site(CleanupRegisterOwned, registration, stmt.Name.Token))
		}
	}
	for stmt, assignment := range a.ownedCleanupAssignments {
		if assignment.Executor != invocation {
			continue
		}
		kind := CleanupRegisterReplacement
		if assignment.Previous != CleanupPreviousAvailable {
			kind = CleanupRegisterReinitialization
		}
		registration := site(kind, assignment.Registration, stmt.Token)
		registration.Previous = assignment.Previous
		plan.Registrations = append(plan.Registrations, registration)
	}
	for stmt, deferPlan := range a.resolvedDeferPlans {
		if deferPlan.Invocation != invocation {
			continue
		}
		detached := cloneResolvedDeferPlan(deferPlan)
		plan.Registrations = append(plan.Registrations, ResolvedCleanupRegistrationSite{Kind: CleanupRegisterDefer,
			Action: detached.Identity.Action, Scope: detached.Identity.Scope, Source: stmt.Token, Defer: &detached})
	}
	for index := range plan.Registrations {
		scope := plan.Registrations[index].Scope
		plan.Registrations[index].PerExecution = flow.scopes[scope].InLoop
		plan.Registrations[index].Conditional = scope != root
	}
	sort.Slice(plan.Registrations, func(i, j int) bool {
		return tokenBefore(plan.Registrations[i].Source, plan.Registrations[j].Source)
	})
	for _, exit := range flow.exits {
		if exit.Invocation == invocation {
			exit.Exited = append([]CleanupScopeID(nil), exit.Exited...)
			plan.Exits = append(plan.Exits, exit)
		}
	}
	for expr, try := range flow.trySites {
		if try.Invocation != invocation {
			continue
		}
		resolved, ok := a.resolvedTries[expr]
		if !ok || !tryPropagates(resolved.Kind) || resolved.TestBoundary {
			continue
		}
		plan.Exits = append(plan.Exits, ResolvedCleanupExit{Kind: CleanupExitErrorPropagation, Source: expr.Token, Invocation: invocation,
			Scope: try.Scope, Exited: a.cleanupScopeChain(try.Scope, 0), FunctionExit: true, RunsCleanup: true,
			Protected: expr.Token, ProtectedKind: "error"})
	}
	sort.SliceStable(plan.Exits, func(i, j int) bool {
		if plan.Exits[i].Source == plan.Exits[j].Source {
			return plan.Exits[i].Kind < plan.Exits[j].Kind
		}
		return tokenBefore(plan.Exits[i].Source, plan.Exits[j].Source)
	})
	return plan, true
}

// tryPropagates reports a try kind whose failure leaves the function.
func tryPropagates(kind ResolvedTryKind) bool {
	switch kind {
	case ResolvedTryResultPropagation, ResolvedTryResultReturnForwarding, ResolvedTryArithmeticPropagation,
		ResolvedTryBoundsPropagation, ResolvedTryOptionPropagation, ResolvedTryFailureSetPropagation:
		return true
	}
	return false
}
