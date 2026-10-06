package sema

import (
	"errors"

	"sec/internal/lexer"
)

// StackExternalKind identifies the verified boundary producer, not source syntax.
// Rules: rules/analysis/stack_analysis.md — "Foreign, runtime, and platform calls".
type StackExternalKind string

const (
	StackExternalForeign  StackExternalKind = "Foreign"
	StackExternalRuntime  StackExternalKind = "Runtime"
	StackExternalPlatform StackExternalKind = "Platform"
)

// StackExternalGuarantee bounds the entire boundary contribution on the caller's
// stack, including ABI/call areas, cleanup and panic paths at the stated level.
// A verified NoReentry excludes unsupported callbacks into active frames.
// Producers own verification and trust; these facts do not define FFI imports.
// Rules: rules/analysis/stack_analysis.md — "Foreign, runtime, and platform calls",
// "CompilationPlan dependence", and "Reentry through callable and foreign boundaries".
type StackExternalGuarantee struct {
	MeasurementLevel  StackMeasurementLevel
	CompilationPlanID string
	Maximum           StackBound
	NoReentry         bool
}

// StackExternalCallContract binds a whole-call guarantee to one canonical
// synchronous invocation and resolved target, preventing accidental retargeting.
// Rules: rules/analysis/call_graph.md — "Call-site record";
// rules/analysis/stack_analysis.md — "Foreign, runtime, and platform calls".
type StackExternalCallContract struct {
	Site      CallSiteID
	Caller    CallableID
	Target    CallableID
	Source    lexer.Token
	Kind      StackExternalKind
	Guarantee StackExternalGuarantee
}

// StackRuntimeEffectContract bounds an explicit runtime path at one semantic
// operation. It never supplies the body bound of an opaque callback.
// Rules: rules/analysis/stack_analysis.md — "Error and panic paths",
// "Foreign, runtime, and platform calls", and "Open calls without stack contracts".
type StackRuntimeEffectContract struct {
	Caller    CallableID
	Kind      EffectKind
	Source    lexer.Token
	Guarantee StackExternalGuarantee
}

type stackExternalCallKey struct {
	site  CallSiteID
	level StackMeasurementLevel
	plan  string
}
type stackRuntimeEffectKey struct {
	caller CallableID
	kind   EffectKind
	source lexer.Token
	level  StackMeasurementLevel
	plan   string
}

// StackExternalContractStore keeps immutable target/level/plan-scoped boundary
// facts independently of source frame and open-callable contract summaries.
// Rules: rules/analysis/stack_analysis.md — "CompilationPlan dependence" and "Stack analysis levels".
type StackExternalContractStore struct {
	calls   map[stackExternalCallKey]StackExternalCallContract
	effects map[stackRuntimeEffectKey]StackRuntimeEffectContract
}

// RecordCall validates binding identity and plan scope before publication;
// it does not certify an external producer's stack or trust proof itself.
// Rules: rules/analysis/stack_analysis.md — "Foreign, runtime, and platform calls" and "CompilationPlan dependence".
func (store *StackExternalContractStore) RecordCall(contract StackExternalCallContract) error {
	if store == nil || contract.Site == "" || contract.Caller == "" || contract.Target == "" || contract.Source.Line <= 0 || contract.Source.Column <= 0 || !validExternalGuarantee(contract.Guarantee) || (contract.Kind != StackExternalForeign && contract.Kind != StackExternalRuntime && contract.Kind != StackExternalPlatform) {
		return errors.New("external stack call contract requires canonical site/caller/target/source, supported kind/level and explicit plan")
	}
	if store.calls == nil {
		store.calls = map[stackExternalCallKey]StackExternalCallContract{}
	}
	store.calls[stackExternalCallKey{contract.Site, contract.Guarantee.MeasurementLevel, contract.Guarantee.CompilationPlanID}] = contract
	return nil
}

// RecordEffect publishes only concrete runtime panic-path guarantees. An opaque
// callee effect requires a callable contract, not a fabricated runtime bound.
// Rules: rules/analysis/stack_analysis.md — "Error and panic paths", "Open calls without stack contracts",
// and "Foreign, runtime, and platform calls".
func (store *StackExternalContractStore) RecordEffect(contract StackRuntimeEffectContract) error {
	if store == nil || contract.Caller == "" || contract.Source.Line <= 0 || contract.Source.Column <= 0 || !validExternalGuarantee(contract.Guarantee) || !isPanicEffectKind(contract.Kind) || contract.Kind == EffectMayPanicUnknownCallee {
		return errors.New("runtime stack effect contract requires a canonical caller/source, concrete panic effect and explicit level/plan")
	}
	if store.effects == nil {
		store.effects = map[stackRuntimeEffectKey]StackRuntimeEffectContract{}
	}
	store.effects[stackRuntimeEffectKey{contract.Caller, contract.Kind, contract.Source, contract.Guarantee.MeasurementLevel, contract.Guarantee.CompilationPlanID}] = contract
	return nil
}

// validExternalGuarantee requires a supported measurement and explicit target
// plan even for semantic external boundaries whose runtime/ABI may vary.
// Rules: rules/analysis/stack_analysis.md — "Foreign, runtime, and platform calls" and "CompilationPlan dependence".
func validExternalGuarantee(guarantee StackExternalGuarantee) bool {
	return guarantee.CompilationPlanID != "" && (guarantee.MeasurementLevel == StackMeasurementSemantic || guarantee.MeasurementLevel == StackMeasurementMachine)
}

// finiteExternalGuarantee consumes only verified finite, non-reentrant boundary
// facts. Unknown, Unbounded and omitted reentry guarantees cannot prove a cap.
// Rules: rules/analysis/stack_analysis.md — "Reentry through callable and foreign boundaries" and "Unknown".
func finiteExternalGuarantee(guarantee StackExternalGuarantee) (StackBound, bool) {
	bytes, finite := guarantee.Maximum.Bytes()
	if !finite || !guarantee.NoReentry {
		return UnknownStackBound(), false
	}
	bound, _ := NewUpperStackBound(bytes)
	return bound, true
}

// externalStackCallContribution consumes an exact closed synchronous target
// contract, including normal/cleanup/panic paths. Foreign declarations must be
// foreign-dispatched externs; trusted runtime/platform producers may identify
// ordinary direct or static-method canonical invocation sites explicitly.
// Rules: rules/analysis/stack_analysis.md — "Foreign, runtime, and platform calls",
// "Call graph ownership", and "Physical stack domains".
func externalStackCallContribution(graph *CallGraph, site CallSite, store *StackExternalContractStore, level StackMeasurementLevel, plan string) (stackCompositionValue, bool) {
	if store == nil || site.Execution != CallExecutionSynchronous || !site.TargetSet.IsClosed || site.TargetSet.HasOpenContract || site.TargetSet.OpenContract != "" || len(site.Targets) != 1 || len(site.TargetSet.KnownTargets) != 1 {
		return stackCompositionValue{}, false
	}
	contract, exists := store.calls[stackExternalCallKey{site.ID, level, plan}]
	if !exists || contract.Caller != site.Caller || contract.Target != site.Targets[0] || contract.Source != site.Source || graph.bodyNodes[site.TargetSet.KnownTargets[0]] != contract.Target {
		return stackCompositionValue{}, false
	}
	node, exists := graph.Node(contract.Target)
	if !exists {
		return stackCompositionValue{}, false
	}
	if contract.Kind == StackExternalForeign {
		if site.Dispatch != CallDispatchForeign || !node.Extern {
			return stackCompositionValue{}, false
		}
	} else if site.Dispatch != CallDispatchForeign && site.Dispatch != CallDispatchDirect && site.Dispatch != CallDispatchStaticMethod {
		return stackCompositionValue{}, false
	}
	bound, finite := finiteExternalGuarantee(contract.Guarantee)
	if !finite {
		return stackCompositionValue{}, false
	}
	return stackCompositionValue{bound: bound, cause: []StackCauseStep{{Callable: contract.Target, Source: site.Source, Detail: "verified " + string(contract.Kind) + " whole-call stack contract: " + bound.String()}}}, true
}

// runtimeStackEffectContribution consumes only an exact semantic operation,
// level and plan; one handler guarantee never covers another operation.
// Rules: rules/analysis/stack_analysis.md — "Error and panic paths", "CompilationPlan dependence", and "Stack cause paths".
func runtimeStackEffectContribution(caller CallableID, effect EffectSite, store *StackExternalContractStore, level StackMeasurementLevel, plan string) (stackCompositionValue, bool) {
	if store == nil {
		return stackCompositionValue{}, false
	}
	contract, exists := store.effects[stackRuntimeEffectKey{caller, effect.Kind, effect.Source, level, plan}]
	if !exists {
		return stackCompositionValue{}, false
	}
	bound, finite := finiteExternalGuarantee(contract.Guarantee)
	if !finite {
		return stackCompositionValue{}, false
	}
	return stackCompositionValue{bound: bound, cause: []StackCauseStep{{Source: effect.Source, Detail: "verified runtime stack contract for " + string(effect.Kind) + ": " + bound.String()}}}, true
}

// stackCallEffectCovered recognizes only whole-call contracts for the very
// invocation behind a legacy foreign/opaque effect marker. Other runtime effects
// remain independent paths; guarantees never silently imply @noPanic.
// Rules: rules/analysis/stack_analysis.md — "Error and panic paths", "Open callable contracts", and "Foreign, runtime, and platform calls".
func stackCallEffectCovered(graph *CallGraph, sites []CallSite, effect EffectSite, callables *StackCallableContractStore, external *StackExternalContractStore, level StackMeasurementLevel, plan string) bool {
	if effect.Kind != EffectMayPanicUnknownCallee && effect.Kind != EffectMayPanicForeign {
		return false
	}
	for _, site := range sites {
		if site.Source != effect.Source {
			continue
		}
		if effect.Kind == EffectMayPanicUnknownCallee {
			if _, usable := usableOpenStackContract(site, callables, level, plan); usable {
				return true
			}
		}
		if _, usable := externalStackCallContribution(graph, site, external, level, plan); usable {
			return true
		}
	}
	return false
}
