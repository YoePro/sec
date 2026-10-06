package sema

import (
	"errors"

	"sec/internal/lexer"
)

// StackCallableContract is a producer-verified stack guarantee for all
// targets permitted by one canonical open contract, including their cleanup and
// panic paths. NoReentry is an independently verified exclusion of callbacks
// into the active frame chain; an absent guarantee remains conservative.
// Source is the contract's evidence location, not new declaration syntax.
// Rules: rules/analysis/stack_analysis.md — "Open callable contracts" and
// "Reentry through callable and foreign boundaries";
// rules/analysis/call_graph.md — "Callable contract" and "Reentry contracts".
type StackCallableContract struct {
	ID                CallableContractID
	MeasurementLevel  StackMeasurementLevel
	CompilationPlanID string
	Maximum           StackBound
	NoReentry         bool
	Source            lexer.Token
}

type stackCallableContractKey struct {
	id    CallableContractID
	level StackMeasurementLevel
	plan  string
}

// StackCallableContractStore retains separate level/plan variants under the
// canonical contract identity. It neither verifies imported trust provenance
// nor invents contract metadata syntax. The zero value is usable.
// Rules: rules/analysis/stack_analysis.md — "Open callable contracts",
// "CompilationPlan dependence", and "Stack analysis levels".
type StackCallableContractStore struct {
	contracts map[stackCallableContractKey]StackCallableContract
}

// Record publishes a producer-established guarantee, preserving unrelated
// levels and plans. Machine guarantees require explicit plan identity.
// Rules: rules/analysis/stack_analysis.md — "CompilationPlan dependence",
// "Open callable contracts", and "Machine stack requirement".
func (store *StackCallableContractStore) Record(contract StackCallableContract) error {
	if store == nil || contract.ID == "" || (contract.MeasurementLevel != StackMeasurementSemantic && contract.MeasurementLevel != StackMeasurementMachine) || (contract.MeasurementLevel == StackMeasurementMachine && contract.CompilationPlanID == "") {
		return errors.New("stack callable contract requires a store, canonical identity, supported measurement level and machine plan")
	}
	if store.contracts == nil {
		store.contracts = map[stackCallableContractKey]StackCallableContract{}
	}
	store.contracts[stackCallableContractKey{contract.ID, contract.MeasurementLevel, contract.CompilationPlanID}] = contract
	return nil
}

// Lookup consumes only an exact contract/level/plan identity, with no fallback
// to semantic estimates, other plans or similarly named callable declarations.
// Rules: rules/analysis/stack_analysis.md — "CompilationPlan dependence" and "Stack analysis levels".
func (store *StackCallableContractStore) Lookup(id CallableContractID, level StackMeasurementLevel, plan string) (StackCallableContract, bool) {
	if store == nil {
		return StackCallableContract{}, false
	}
	contract, exists := store.contracts[stackCallableContractKey{id, level, plan}]
	return contract, exists
}

// usableOpenStackContract requires an explicitly open synchronous function-value
// or closure call and a verified finite bound excluding unsupported reentry.
// A contract must cover runtime/cleanup/panic paths as well as ordinary return.
// Rules: rules/analysis/stack_analysis.md — "Open callable contracts",
// "Open calls without stack contracts", and "Reentry through callable and foreign boundaries".
func usableOpenStackContract(site CallSite, store *StackCallableContractStore, level StackMeasurementLevel, plan string) (StackCallableContract, bool) {
	if site.Execution != CallExecutionSynchronous || site.TargetSet.IsClosed || !site.TargetSet.HasOpenContract || site.TargetSet.OpenContract == "" || (site.Dispatch != CallDispatchFunctionValue && site.Dispatch != CallDispatchClosure) {
		return StackCallableContract{}, false
	}
	contract, exists := store.Lookup(site.TargetSet.OpenContract, level, plan)
	_, finite := contract.Maximum.Bytes()
	return contract, exists && finite && contract.NoReentry
}

// openStackCallContribution combines every known concrete target with the open
// guarantee. Missing, nonfinite or wrong-plan contracts never close a
// target set. Known target evidence survives unusable open guarantees, and the
// closed consumer still validates body-to-node coverage for the known subset.
// Rules: rules/analysis/stack_analysis.md — "Open callable contracts",
// "Open calls without stack contracts", "Closed indirect-call target sets", and "Partial information".
func openStackCallContribution(graph *CallGraph, site CallSite, visit func(CallableID) stackCompositionValue, store *StackCallableContractStore, level StackMeasurementLevel, plan string) stackCompositionValue {
	var known stackCompositionValue
	var knownExact StackBound
	hasKnown := len(site.TargetSet.KnownTargets) > 0 || len(site.Targets) > 0
	if hasKnown {
		closed := site
		closed.TargetSet = cloneCallableTargetSet(site.TargetSet)
		closed.TargetSet.IsClosed, closed.TargetSet.HasOpenContract, closed.TargetSet.OpenContract = true, false, ""
		known = closedStackCallContribution(graph, closed, func(target CallableID) stackCompositionValue {
			candidate := visit(target)
			if candidate.bound.Kind() == StackBoundExact {
				bytes, _ := candidate.bound.Bytes()
				previous, finite := knownExact.Bytes()
				if !finite || bytes.Cmp(previous) > 0 {
					knownExact = candidate.bound
				}
			}
			return candidate
		})
	}
	contract, usable := usableOpenStackContract(site, store, level, plan)
	if !usable {
		result := unknownStackCall(site.Source, "verified finite open-callable stack bound and no-reentry guarantee unavailable")
		result.evidence = mergeStackEvidence(result.evidence, known.evidence)
		return result
	}
	bytes, _ := contract.Maximum.Bytes()
	if knownExact.Kind() == StackBoundExact {
		knownBytes, _ := knownExact.Bytes()
		if knownBytes.Cmp(bytes) > 0 {
			result := unknownStackCall(site.Source, "known exact target demand exceeds the open callable contract guarantee")
			result.evidence = mergeStackEvidence(result.evidence, known.evidence)
			return result
		}
	}
	bound, _ := NewUpperStackBound(bytes)
	result := stackCompositionValue{bound: bound, cause: []StackCauseStep{{Source: site.Source, Detail: "verified open callable contract " + string(contract.ID)}}}
	result.evidence = mergeStackEvidence(result.evidence, known.evidence)
	if hasKnown && stackCompositionStronger(known.bound, result.bound) {
		result.bound, result.cause = known.bound, append([]StackCauseStep(nil), known.cause...)
		result.evidence.KnownPrefix = append([]StackFrameContribution(nil), known.evidence.KnownPrefix...)
	}
	return result
}
