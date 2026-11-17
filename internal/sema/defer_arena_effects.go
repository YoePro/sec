package sema

import (
	"sort"

	"sec/internal/lexer"
)

// DeferredArenaInvalidation describes a future direct Reset/Release site in a
// ResolvedDeferPlan's Body. The enclosing plan supplies registration/action and
// invocation identities. Receiver is a canonical Place, not a snapshot of the
// Arena owner, domain or epoch at registration: cleanup observes its then-current
// value. DirectEffectIndex refers to the body's existing effect sequence; it
// neither makes conditional sites unconditional nor flattens loops or callees.
// Rules: rules/control-flow/defer.md — §§3,6–10;
// rules/memory/arena.md — §§35–36,43–44,52;
// rules/corrections/applied/MD-125-replacement-cleanup-registration-point-applied-20261105.md — §1(5).
type DeferredArenaInvalidation struct {
	Operation         ArenaOperationKind
	Receiver          Place
	Source            lexer.Token
	DirectEffectIndex int
}

// deferredArenaInvalidations retains only checked direct invalidations owned by
// this deferred callable. It reuses resolved operation/effect facts, excluding
// the surrounding invocation and nested lambda bodies without traversing syntax
// or mutating continuation generations. Source visitation order is preserved;
// cleanup-edge execution and transitive callee effects require separate plans.
// Rules: rules/control-flow/defer.md — §§3,9,29;
// rules/memory/arena.md — §§35–36,43–44,52,76–77;
// rules/mlir/packages/sec-mlir-dialect_package19.md — §§70–76.
func (a *Analyzer) deferredArenaInvalidations(body CallableID) []DeferredArenaInvalidation {
	var effects []DeferredArenaInvalidation
	for _, operation := range a.arenaOperationPlans {
		if operation.Operation != ArenaOperationReset && operation.Operation != ArenaOperationRelease {
			continue
		}
		if operation.Effects == nil || operation.Effects.Callable != body || operation.Receiver == nil {
			continue
		}
		effects = append(effects, DeferredArenaInvalidation{
			Operation: operation.Operation, Receiver: cloneSlicePlanPlace(*operation.Receiver),
			Source: operation.Source, DirectEffectIndex: operation.Effects.DirectEffectIndex,
		})
	}
	sort.Slice(effects, func(i, j int) bool {
		return effects[i].DirectEffectIndex < effects[j].DirectEffectIndex
	})
	return effects
}
