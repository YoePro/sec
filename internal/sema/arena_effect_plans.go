package sema

// ArenaProviderEffect identifies the provider work associated with a source
// Arena operation. It does not pretend that a provider contract is resolved.
// Rules: rules/mlir/packages/sec-mlir-dialect_package19.md — §§65–66,73–75.
type ArenaProviderEffect string

const (
	ArenaProviderNone        ArenaProviderEffect = "none"
	ArenaProviderAcquire     ArenaProviderEffect = "acquire-backing"
	ArenaProviderGrow        ArenaProviderEffect = "grow-backing-if-needed"
	ArenaProviderReclaim     ArenaProviderEffect = "reclaim-backing"
	ArenaProviderFromBacking ArenaProviderEffect = "according-to-backing"
)

// ArenaOperationEffectPlan links an operation to the existing call graph event,
// its position in the callable's direct-effect sequence, and canonical storage
// resource/transition. The sequence is source visitation order, not a claim that
// mutually exclusive branches or repeated loop bodies execute exactly once.
// Ordered effects cannot be CSE'd or moved across Reset/Release. Unknown provider
// effects remain explicit and never prove allocation/effect freedom.
// Rules: rules/mlir/packages/sec-mlir-dialect_package19.md — §§65–69,73–76;
// rules/memory/arena.md — §§76–77; rules/memory/storage.md — §21.
type ArenaOperationEffectPlan struct {
	Callable               CallableID
	DirectEffectIndex      int
	Event                  ArenaEffectSite
	Resource               InvalidationDomainID
	Transition             BackingTransitionKind
	Provider               ArenaProviderEffect
	ProviderEffectsUnknown bool
	EndsBackingBorrow      bool
	MustPreserveOrder      bool
	CSESafe                bool
}

// arenaOperationEffectPlan attaches the already-recorded direct event rather
// than generating a second event stream. Missing events produce no effect plan,
// preventing a later consumer from treating incomplete facts as preservation.
// Rules: rules/mlir/packages/sec-mlir-dialect_package19.md — §§65–69,73–76;
// rules/memory/arena.md — §76(1–4), §77(1–6).
func (a *Analyzer) arenaOperationEffectPlan(plan ResolvedArenaOperationPlan) *ArenaOperationEffectPlan {
	if a.callGraph == nil {
		return nil
	}
	events := a.callGraph.arenaEffects[a.currentCallable]
	for i := len(events) - 1; i >= 0; i-- {
		event := events[i]
		if event.Domain != plan.Domain || sourceTokenLocation(event.Source) != sourceTokenLocation(plan.Source) {
			continue
		}
		effect := &ArenaOperationEffectPlan{
			Callable: a.currentCallable, DirectEffectIndex: i,
			Event: event, Resource: plan.InvalidationDomain,
			Transition: BackingTransitionNone, Provider: ArenaProviderNone,
			MustPreserveOrder: true,
		}
		switch plan.Operation {
		case ArenaOperationCreate:
			effect.Transition = BackingTransitionEstablishDomain
			if plan.BackingRelation == BackingRelationOwned {
				effect.Provider, effect.ProviderEffectsUnknown = ArenaProviderAcquire, true
			}
		case ArenaOperationNew, ArenaOperationAlloc:
			if plan.Count != nil && plan.Count.Sign() == 0 {
				// §29 proves no capacity use or provider growth, while the
				// existing source MayAllocate/Result contracts stay stable.
			} else if plan.GrowthKind == "growable-nonrelocating" {
				effect.Provider, effect.ProviderEffectsUnknown = ArenaProviderGrow, true
			} else if plan.GrowthKind == "" {
				effect.Provider, effect.ProviderEffectsUnknown = ArenaProviderFromBacking, true
			}
		case ArenaOperationReset:
			effect.Transition = BackingTransitionAdvanceEpoch
		case ArenaOperationRelease:
			effect.Transition = BackingTransitionEndDomain
			switch plan.BackingRelation {
			case BackingRelationBorrowed:
				effect.EndsBackingBorrow = true
			case BackingRelationOwned:
				effect.Provider, effect.ProviderEffectsUnknown = ArenaProviderReclaim, true
			default:
				effect.Provider, effect.ProviderEffectsUnknown = ArenaProviderFromBacking, true
			}
		}
		return effect
	}
	return nil
}
