package sema

import (
	"errors"
	"sort"

	"sec/internal/lexer"
)

// StackCauseStep retains producer-supplied evidence for a frame, call, SCC or
// unknown boundary. This model does not derive a maximum-cause path.
// Rules: rules/analysis/stack_analysis.md — "Maximum cause" and "Stack cause paths".
type StackCauseStep struct {
	Callable CallableID
	Source   lexer.Token
	Detail   string
}

// SemanticStackSummary records canonical semantic frame requirements, without
// backend spills or ABI frame effects. Empty CompilationPlanID denotes a
// producer-established target-independent result, not an unresolved target.
// Rules: rules/analysis/stack_analysis.md — "Semantic stack requirement",
// "CompilationPlan dependence", and "Per-function stack summaries".
type SemanticStackSummary struct {
	Callable          CallableID
	CompilationPlanID string
	OwnFrame          StackBound
	TransitiveMaximum StackBound
	MaximumCause      []StackCauseStep
	Evidence          StackEvidence
}

// MachineStackSummary records final target/ABI/backend stack requirements in a
// separate type. CompilationPlanID is mandatory for every machine result.
// Rules: rules/analysis/stack_analysis.md — "Machine stack requirement",
// "CompilationPlan dependence", and "Semantic and machine frame authority".
type MachineStackSummary struct {
	Callable          CallableID
	CompilationPlanID string
	OwnFrame          StackBound
	TransitiveMaximum StackBound
	MaximumCause      []StackCauseStep
	Evidence          StackEvidence
}

type stackSummaryKey struct {
	callable CallableID
	plan     string
}

// StackSummaryStore keeps producer-supplied semantic and machine facts in
// independent namespaces, preserving plan variants and defensive snapshots.
// It does not infer frames, validate budgets or persist separate-compilation
// summaries. The zero value is an empty usable store.
// Rules: rules/analysis/stack_analysis.md — "Stack analysis levels",
// "CompilationPlan dependence", and "Per-function stack summaries";
// rules/compiler/compiler_analysis.md — immutable analysis results.
type StackSummaryStore struct {
	semantic map[stackSummaryKey]SemanticStackSummary
	machine  map[stackSummaryKey]MachineStackSummary
}

// RecordSemantic publishes a detached semantic summary for its callable and
// optional target-dependent plan, replacing only that semantic entry.
// Rules: rules/analysis/stack_analysis.md — "Semantic stack requirement",
// "CompilationPlan dependence", and "Semantic and machine frame authority".
func (store *StackSummaryStore) RecordSemantic(summary SemanticStackSummary) error {
	if store == nil || summary.Callable == "" {
		return errors.New("semantic stack summary requires a store and callable identity")
	}
	if store.semantic == nil {
		store.semantic = make(map[stackSummaryKey]SemanticStackSummary)
	}
	summary.MaximumCause = append([]StackCauseStep(nil), summary.MaximumCause...)
	summary.Evidence = cloneStackEvidence(summary.Evidence)
	store.semantic[stackSummaryKey{summary.Callable, summary.CompilationPlanID}] = summary
	return nil
}

// RecordMachine publishes a detached machine summary under a mandatory plan
// identity, replacing only that machine entry and retaining semantic facts.
// Rules: rules/analysis/stack_analysis.md — "Machine stack requirement",
// "CompilationPlan dependence", and "Semantic and machine frame authority".
func (store *StackSummaryStore) RecordMachine(summary MachineStackSummary) error {
	if store == nil || summary.Callable == "" || summary.CompilationPlanID == "" {
		return errors.New("machine stack summary requires a store, callable and CompilationPlan identity")
	}
	if store.machine == nil {
		store.machine = make(map[stackSummaryKey]MachineStackSummary)
	}
	summary.MaximumCause = append([]StackCauseStep(nil), summary.MaximumCause...)
	summary.Evidence = cloneStackEvidence(summary.Evidence)
	store.machine[stackSummaryKey{summary.Callable, summary.CompilationPlanID}] = summary
	return nil
}

// Semantic returns only the semantic result for the exact requested identity.
// A missing entry never silently substitutes a result from another plan.
// Rules: rules/analysis/stack_analysis.md — "CompilationPlan dependence" and "Stack analysis levels".
func (store *StackSummaryStore) Semantic(callable CallableID, plan string) (SemanticStackSummary, bool) {
	if store == nil {
		return SemanticStackSummary{}, false
	}
	summary, ok := store.semantic[stackSummaryKey{callable, plan}]
	summary.MaximumCause = append([]StackCauseStep(nil), summary.MaximumCause...)
	summary.Evidence = cloneStackEvidence(summary.Evidence)
	return summary, ok
}

// Machine returns only the machine result for the exact requested plan.
// Semantic facts cannot stand in for missing machine evidence.
// Rules: rules/analysis/stack_analysis.md — "CompilationPlan dependence",
// "Semantic and machine frame authority", and "Machine-level revalidation".
func (store *StackSummaryStore) Machine(callable CallableID, plan string) (MachineStackSummary, bool) {
	if store == nil {
		return MachineStackSummary{}, false
	}
	summary, ok := store.machine[stackSummaryKey{callable, plan}]
	summary.MaximumCause = append([]StackCauseStep(nil), summary.MaximumCause...)
	summary.Evidence = cloneStackEvidence(summary.Evidence)
	return summary, ok
}

// SemanticSummaries returns detached semantic facts in stable callable/plan order.
// Rules: rules/analysis/stack_analysis.md — "Per-function stack summaries" and "Determinism".
func (store *StackSummaryStore) SemanticSummaries() []SemanticStackSummary {
	if store == nil {
		return nil
	}
	var summaries []SemanticStackSummary
	for key := range store.semantic {
		summary, _ := store.Semantic(key.callable, key.plan)
		summaries = append(summaries, summary)
	}
	sort.Slice(summaries, func(i, j int) bool {
		if summaries[i].Callable != summaries[j].Callable {
			return summaries[i].Callable < summaries[j].Callable
		}
		return summaries[i].CompilationPlanID < summaries[j].CompilationPlanID
	})
	return summaries
}

// MachineSummaries returns detached machine facts in stable callable/plan order.
// Rules: rules/analysis/stack_analysis.md — "Per-function stack summaries" and "Determinism".
func (store *StackSummaryStore) MachineSummaries() []MachineStackSummary {
	if store == nil {
		return nil
	}
	var summaries []MachineStackSummary
	for key := range store.machine {
		summary, _ := store.Machine(key.callable, key.plan)
		summaries = append(summaries, summary)
	}
	sort.Slice(summaries, func(i, j int) bool {
		if summaries[i].Callable != summaries[j].Callable {
			return summaries[i].Callable < summaries[j].Callable
		}
		return summaries[i].CompilationPlanID < summaries[j].CompilationPlanID
	})
	return summaries
}
