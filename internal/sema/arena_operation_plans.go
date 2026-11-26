package sema

import (
	"fmt"
	"math/big"

	"sec/internal/ast"
	"sec/internal/diagnostics"
	"sec/internal/layout"
	"sec/internal/lexer"
)

// ArenaOperationKind separates owner creation, epoch-preserving allocation,
// epoch advancement and terminal release. It is not a new storage taxonomy.
// Rules: rules/mlir/packages/sec-mlir-dialect_package19.md — §§7–10,70–76.
type ArenaOperationKind string

const (
	ArenaOperationCreate  ArenaOperationKind = "create"
	ArenaOperationNew     ArenaOperationKind = "new"
	ArenaOperationAlloc   ArenaOperationKind = "alloc"
	ArenaOperationReset   ArenaOperationKind = "reset"
	ArenaOperationRelease ArenaOperationKind = "release"
)

// ResolvedArenaOperationPlan freezes the represented Arena decisions at the
// successful source operation. Unknown backing/provider/layout facts remain
// explicit; this first P19 plan surface is not an IR readiness certificate.
// State mutation is deliberately separate from validity epoch: allocations
// change state without advancing the epoch; Reset advances it; Release ends it.
// Domain is the existing frontend identity; InvalidationDomain/EpochDependency
// reuse P15's allocator. No new identity family or physical provider is invented.
// Rules: rules/mlir/packages/sec-mlir-dialect_package19.md — §§7–10,70–76;
// rules/memory/arena.md — §§4.2–4.5,10–13,19–25,35–46.
type ResolvedArenaOperationPlan struct {
	// Failure retains the canonical mapping, stable Result and atomic outcome.
	Failure *ArenaFailurePlan

	// Parents retain the borrowed child backing dependencies from creation.
	// Release ends these loans; it never reclaims their allocations.
	Parents []ArenaParentDependency

	// ElementDefault is the canonical semantic value, not a zero-fill recipe.
	// Physical initialization still requires the unresolved layout plan.
	ElementDefault *DefaultResolution

	// Effects reuse the call graph event and retain ordering/resource barriers.
	Effects *ArenaOperationEffectPlan

	// Capacity describes the initial byte source, never remaining capacity.
	Capacity ArenaCapacityFacts

	Operation                                        ArenaOperationKind
	Constructor                                      string
	Source                                           lexer.Token
	Domain                                           string
	InvalidationDomain                               InvalidationDomainID
	EpochBefore, EpochAfter                          EpochDependencyID
	EpochBeforeGeneration, EpochAfterGeneration      int
	ChangesState, EndsDomain, ConsumesOwner          bool
	Receiver, Backing                                *Place
	BackingRelation                                  BackingRelation
	ReclamationAuthority                             ReclamationAuthority
	BackingKind, GrowthKind                          string
	CapacityUnit                                     string
	CountUnit                                        string
	Count                                            *big.Int
	ElementType, ResultType, SuccessType, ErrorType  Type
	ElementDefaultable, ElementTriviallyDestructible bool
	Context                                          AllocationContext
	ContextIdentity                                  AllocationContextID
	Allocation                                       AllocationIdentityID
	Fallible                                         bool
	// FailureAtomic means no changed cursor/state/epoch or partial result is
	// observable on failure. ErrorCauses are canonical AllocationError variants.
	FailureAtomic bool
	ErrorCauses   []string
	// Unresolved names the additional facts needed before preserving lowering.
	Unresolved []string
	// LayoutFailure is the canonical layout query's diagnostic, projected to
	// CLI/LSP without reconstructing its type, target or representation facts.
	LayoutFailure string
	// StateBefore/StateAfter are the owner state versions of creation and
	// Reset: version = 1 + the epoch generation, which is exact while only
	// creation, Reset and Release change the owner (New/Alloc keep the
	// "owner state-version transition" unresolved until they lower).
	StateBefore, StateAfter int
	// ElementLayout is the resolved layout of New/Alloc's T under the plan;
	// CapacityPlan and DefaultPlan name the resolved capacity source and
	// canonical initialization of an allocation (P19 §§ 20–25, 113–114).
	ElementLayout             *ArenaElementLayout
	CapacityPlan, DefaultPlan string
	// ProviderPlan is "absent" for the MD-136 default Arena, which has no
	// verified provider (arena.md § 131(11)).
	ProviderPlan string
	// CountUpperBound is the proven interval maximum of a non-constant Alloc
	// count, used as a range-upper-bound demand (P19 § 79).
	CountUpperBound *big.Int
	// Reference is the dynamic-epoch reference/slice New/Alloc publish: a
	// fresh mutable borrow of the current ArenaDomain epoch (arena.md § 18(5)).
	Reference *ReferenceFacts
	// EpochAdvance is a Reset's epoch exhaustion fact (MD-140); zero for
	// every other operation and when the epoch policy is unresolved.
	EpochAdvance ArenaEpochAdvancePlan
	// PhysicalUnresolved names facts only physical lowering needs: the
	// Semantic IR epoch is a logical identity without a counter, so the
	// physical counter and its exhaustion endpoint belong to the physical
	// epoch representation, not to Semantic IR.
	PhysicalUnresolved []string
}

// ArenaEpochAdvanceForm is how a Reset advances its retained validity epoch
// (MD-140). Checked is the default for every width, including 64 bits; the
// two elimination forms need proof provenance from an owning analysis and
// are never inferred from the default width or an unknown advance count.
// Rules: rules/memory/arena.md — §§ 39(5)-(6), 39(9), 105(4).
type ArenaEpochAdvanceForm string

const (
	ArenaEpochAdvanceChecked            ArenaEpochAdvanceForm = "checked"
	ArenaEpochAdvanceProvenBounded      ArenaEpochAdvanceForm = "proven-bounded"
	ArenaEpochAdvanceMetadataEliminated ArenaEpochAdvanceForm = "metadata-eliminated"
)

// ArenaEpochAdvancePlan is the Reset exhaustion fact of the selected plan:
// the advance form and the proof provenance of an elimination, the logical
// epoch width from the one reference policy, the single panic outcome and
// its registered reason. Sema publishes only the checked form until a
// domain- and plan-qualified bound or metadata-elimination proof exists.
// Rules: rules/memory/arena.md — §§ 38(8), 39(1), 39(4)-(9), 42(5);
// rules/errors/panic.md — § 13(8).
type ArenaEpochAdvancePlan struct {
	Form             ArenaEpochAdvanceForm
	Proof            string
	LogicalEpochBits uint16
	Exhaustion       layout.ArenaEpochExhaustion
	PanicReason      diagnostics.PanicReasonID
}

// recordArenaOperationPlan records only successfully checked, reachable source
// operations during the final analysis. Failed calls and summary passes publish
// no plan. Existing dependency checking remains the authority for Reset/Release.
// Rules: rules/mlir/packages/sec-mlir-dialect_package19.md — §§70–76;
// rules/memory/arena.md — §§19–25,35–46; rules/memory/storage.md — §21.
func (a *Analyzer) recordArenaOperationPlan(call *ast.CallExpression, operation ArenaOperationKind, constructor, domain string, before, after int, result Type) {
	if a.summaryPass || !a.callGraphPathReachable || call == nil || domain == "" {
		return
	}
	plan := ResolvedArenaOperationPlan{
		Operation: operation, Constructor: constructor, Domain: domain, Source: callCalleeDefinitionToken(call),
		InvalidationDomain:    a.placeIdentities.invalidationDomain("arena:" + domain),
		EpochBeforeGeneration: before, EpochAfterGeneration: after,
		EpochBefore: a.placeIdentities.epochDependency(fmt.Sprintf("arena:%s@%d", domain, before)),
		EpochAfter:  a.placeIdentities.epochDependency(fmt.Sprintf("arena:%s@%d", domain, after)),
		ResultType:  semanticSnapshotType(result), CapacityUnit: "bytes", FailureAtomic: true,
		ChangesState: operation != ArenaOperationCreate,
	}
	plan.SuccessType = semanticSnapshotType(result)
	if result.Kind == ResultType && len(result.TypeArgs) == 2 {
		plan.Fallible = true
		plan.SuccessType = semanticSnapshotType(result.TypeArgs[0])
		plan.ErrorType = semanticSnapshotType(result.TypeArgs[1])
	}
	if operation == ArenaOperationCreate {
		plan.Context = a.activeAllocationContext()
		plan.BackingKind, plan.GrowthKind = "owned", "fixed"
		plan.BackingRelation = BackingRelationOwned
		plan.ReclamationAuthority = reclamationAuthorityOf(plan.Context.Origin)
		if constructor == "Growable" {
			plan.GrowthKind = "growable-nonrelocating"
		}
		if constructor == "FromBuffer" {
			plan.BackingKind, plan.GrowthKind = "borrowed", "fixed"
			plan.BackingRelation = BackingRelationBorrowed
			plan.ReclamationAuthority = ReclamationExternalOwner
			plan.Context = AllocationContext{}
			backing := call.Arguments[0]
			if reference, explicit := backing.(*ast.RefExpression); explicit {
				backing = reference.Value
			}
			if place, ok := a.resolvePlace(backing); ok {
				copy := snapshotArenaPlanPlace(place)
				plan.Backing = &copy
			}
		} else {
			plan.Unresolved = append(plan.Unresolved, "provider/reclamation contract")
		}
	} else {
		plan.Context = AllocationContext{Available: true, Origin: StorageOriginArena, Profile: a.targetProfile}
		plan.ReclamationAuthority = ReclamationArena
		if member, ok := call.Callee.(*ast.MemberExpression); ok {
			if place, ok := a.resolvePlace(member.Object); ok {
				copy := snapshotArenaPlanPlace(place)
				plan.Receiver = &copy
			}
		}
		if creation, known := a.arenaCreations[domain]; known {
			plan.Constructor = creation.Constructor
			switch creation.Constructor {
			case "FromBuffer":
				plan.BackingKind, plan.GrowthKind = "borrowed", "fixed"
				plan.BackingRelation = BackingRelationBorrowed
			case "WithCapacity":
				plan.BackingKind, plan.GrowthKind = "owned", "fixed"
				plan.BackingRelation = BackingRelationOwned
			case "Growable":
				plan.BackingKind, plan.GrowthKind = "owned", "growable-nonrelocating"
				plan.BackingRelation = BackingRelationOwned
			case ArenaDefaultConstructor:
				// MD-136: the empty default owns its domain, no backing;
				// its growth policy was frozen at creation.
				plan.BackingKind, plan.GrowthKind, plan.ProviderPlan = "none", "fixed", "absent"
				if a.defaultArenaGrows() {
					plan.GrowthKind, plan.ProviderPlan = "growable-nonrelocating", defaultArenaInterimProvider
				}
				plan.BackingRelation = BackingRelationOwned
			}
		} else {
			plan.Unresolved = append(plan.Unresolved, "Arena backing state (parameter or unspecified creation)")
		}
	}
	if operation == ArenaOperationNew || operation == ArenaOperationAlloc {
		// liveElement is the checked element type; its snapshot drops the
		// target-dependent integer domain the layout resolver needs.
		var liveElement *Type
		plan.CountUnit = "elements"
		plan.Count = big.NewInt(1)
		if operation == ArenaOperationAlloc {
			plan.Count, _ = a.integerConstantValue(call.Arguments[0])
			if plan.Count == nil {
				if countType, typed := a.ResolvedTypeOf(call.Arguments[0]); typed {
					if _, upper, bounded := a.scalarValueInterval(call.Arguments[0], countType); bounded {
						plan.CountUpperBound = new(big.Int).Set(upper)
					}
				}
			}
		}
		if len(call.GenericArguments) == 1 {
			// The element was resolved and checked immediately before this
			// publication; no consumer/query may resolve it again.
			if result.Kind == ResultType && len(result.TypeArgs) > 0 && result.TypeArgs[0].Element != nil {
				element := *result.TypeArgs[0].Element
				if operation == ArenaOperationAlloc && element.Element != nil {
					element = *element.Element
				}
				liveElement = &element
				plan.ElementType = semanticSnapshotType(element)
				resolvedDefault := snapshotArenaElementDefault(DefaultValueOf(element))
				plan.ElementDefault = &resolvedDefault
				plan.ElementDefaultable = resolvedDefault.Kind != NoDefault
				plan.ElementTriviallyDestructible = TriviallyDestructible(element)
			}
		}
		key := fmt.Sprintf("arena-allocation:%s:%d:%d", plan.Source.File, plan.Source.Line, plan.Source.Column)
		// MD-141/MD-142: the element layout comes from the canonical
		// resolver; the checked extent (count * size, alignment padding,
		// capacity) is evaluated by the allocation itself (arena.md § 16).
		if liveElement != nil {
			operationName := "Arena.New"
			if operation == ArenaOperationAlloc {
				operationName = "Arena.Alloc"
			}
			if resolved, missing := a.arenaElementLayout(*liveElement, operationName); resolved != nil {
				plan.ElementLayout = resolved
			} else {
				plan.LayoutFailure = missing
				plan.Unresolved = append(plan.Unresolved, missing)
			}
		} else {
			plan.Unresolved = append(plan.Unresolved, "CompilationPlan layout of T (unresolved element type)")
		}
		// arena.md §§ 24–26: one canonical infallible default, trivially
		// destructible T; the default value itself is ElementDefault.
		if plan.ElementDefaultable && plan.ElementTriviallyDestructible {
			plan.DefaultPlan = "canonical-default"
		} else {
			plan.Unresolved = append(plan.Unresolved, "canonical initialization plan")
		}
		// A borrowed FromBuffer Arena allocates from its fixed byte backing
		// (arena.md § 10); owned backing needs a verified provider (MD-151).
		if plan.BackingKind == "borrowed" && plan.GrowthKind == "fixed" {
			plan.CapacityPlan = "borrowed-fixed-bytes"
		} else if plan.BackingKind == "none" && plan.Count != nil && plan.Count.Sign() == 0 {
			// MD-136 § 131(10): a zero-element allocation needs no backing.
			plan.CapacityPlan = "zero-storage"
		} else if plan.BackingKind == "none" {
			plan.Unresolved = append(plan.Unresolved, "capacity/growth plan (default Arena growth: provider catalogs, MD-151)")
		} else {
			plan.Unresolved = append(plan.Unresolved, "capacity/growth plan (owned backing: provider catalogs, MD-151)")
		}
		// The owner state version is numbered by the Semantic IR builder in
		// mutation order (sec_mlir_dialect_v15.md § 9), like Reset's.
		reference := ReferenceFacts{
			Kind: ReferenceMutableDirect, Referent: semanticSnapshotType(plan.ElementType), MutableAuthority: true,
			AddressSpace: AddressSpaceOrdinary, Validity: ReferenceValidityDynamicEpoch,
			InvalidationDomain: plan.InvalidationDomain, EpochDependency: plan.EpochAfter,
			AddressStability: AddressStabilityStable, Provenance: ReferenceProvenanceArena,
			Borrow:   a.placeIdentities.borrow("borrow:" + key),
			Lifetime: a.placeIdentities.lifetime("lifetime:" + key),
		}
		plan.Reference = &reference
		if plan.Count == nil || plan.Count.Sign() != 0 {
			plan.Allocation = a.placeIdentities.allocation(key)
		}
	}
	if operation == ArenaOperationCreate && constructor != "FromBuffer" {
		plan.CountUnit = "bytes"
		plan.Count, _ = a.integerConstantValue(call.Arguments[0])
		plan.Allocation = a.placeIdentities.allocation(fmt.Sprintf("arena-backing:%s:%d:%d", plan.Source.File, plan.Source.Line, plan.Source.Column))
	}
	if plan.Context.Available {
		key := "allocation-context:" + plan.Context.Profile + ":" + string(plan.Context.Origin)
		if operation != ArenaOperationCreate {
			key = "arena-context:" + domain
		}
		plan.ContextIdentity = a.placeIdentities.allocationContext(key)
	}
	switch operation {
	case ArenaOperationCreate:
		plan.StateBefore, plan.StateAfter = 0, 1
	case ArenaOperationReset:
		plan.StateBefore, plan.StateAfter = before+1, after+1
		a.recordArenaEpochAdvance(call, &plan)
	}
	if operation == ArenaOperationRelease {
		plan.EndsDomain, plan.ConsumesOwner = true, true
		// Ending a domain has no successor live epoch, even though the legacy
		// flow checker increments its stale-use generation on Release.
		plan.EpochAfter = 0
		// arena.md §§ 45, 131(7): Release of borrowed backing ends the loan
		// and of an empty default reclaims nothing; owned backing needs its
		// provider's reclamation contract (MD-151).
		if plan.BackingKind != "borrowed" && plan.BackingKind != "none" {
			plan.Unresolved = append(plan.Unresolved, "backing release/implicit destruction plan (owned backing: provider catalogs, MD-151)")
		}
	}
	plan.Failure = arenaFailurePlan(operation, plan.Fallible, result)
	if plan.Failure != nil {
		for _, mapping := range plan.Failure.Mappings {
			plan.ErrorCauses = append(plan.ErrorCauses, mapping.Variant)
		}
	}
	plan.Parents = a.arenaParentDependencies(call, operation, constructor, domain)
	plan.Capacity = a.arenaCapacityFacts(call, plan)
	plan.Effects = a.arenaOperationEffectPlan(plan)
	if plan.Effects == nil {
		plan.Unresolved = append(plan.Unresolved, "ordered Arena effect event")
	}
	plan.ElementType = semanticSnapshotType(plan.ElementType)
	plan.ErrorType = semanticSnapshotType(plan.ErrorType)
	if a.arenaOperationPlans == nil {
		a.arenaOperationPlans = map[*ast.CallExpression]ResolvedArenaOperationPlan{}
	}
	a.arenaOperationPlans[call] = plan
}

// ResolvedArenaOperationPlanOf returns detached recorded facts, never inference,
// identity allocation, liveness analysis or implicit readiness certification.
// Rules: rules/mlir/packages/sec-mlir-dialect_package19.md — §§70–76.
func (a *Analyzer) ResolvedArenaOperationPlanOf(expr ast.Expression) (ResolvedArenaOperationPlan, bool) {
	if literal, ok := expr.(*ast.StructLiteral); ok && a != nil {
		plan, found := a.arenaDefaultPlans[literal]
		if found {
			plan.ResultType = semanticSnapshotType(plan.ResultType)
			plan.SuccessType = semanticSnapshotType(plan.SuccessType)
			plan.Capacity = cloneArenaCapacityFacts(plan.Capacity)
		}
		return plan, found
	}
	call, ok := expr.(*ast.CallExpression)
	if a == nil || !ok || call == nil {
		return ResolvedArenaOperationPlan{}, false
	}
	plan, ok := a.arenaOperationPlans[call]
	if !ok {
		return ResolvedArenaOperationPlan{}, false
	}
	if plan.Count != nil {
		plan.Count = new(big.Int).Set(plan.Count)
	}
	if plan.Receiver != nil {
		copy := snapshotArenaPlanPlace(*plan.Receiver)
		plan.Receiver = &copy
	}
	if plan.Backing != nil {
		copy := snapshotArenaPlanPlace(*plan.Backing)
		plan.Backing = &copy
	}
	plan.ElementType = semanticSnapshotType(plan.ElementType)
	plan.ResultType = semanticSnapshotType(plan.ResultType)
	plan.SuccessType = semanticSnapshotType(plan.SuccessType)
	plan.ErrorType = semanticSnapshotType(plan.ErrorType)
	plan.Failure = cloneArenaFailurePlan(plan.Failure)
	plan.Capacity = cloneArenaCapacityFacts(plan.Capacity)
	if plan.Effects != nil {
		effects := *plan.Effects
		plan.Effects = &effects
	}
	if plan.ElementDefault != nil {
		value := snapshotArenaElementDefault(*plan.ElementDefault)
		plan.ElementDefault = &value
	}
	if plan.CountUpperBound != nil {
		plan.CountUpperBound = new(big.Int).Set(plan.CountUpperBound)
	}
	if plan.ElementLayout != nil {
		layout := *plan.ElementLayout
		layout.SizeBytes, layout.AlignBytes = new(big.Int).Set(layout.SizeBytes), new(big.Int).Set(layout.AlignBytes)
		plan.ElementLayout = &layout
	}
	if plan.Reference != nil {
		reference := *plan.Reference
		reference.Referent = semanticSnapshotType(reference.Referent)
		reference.OriginPlaces = append([]Place(nil), reference.OriginPlaces...)
		reference.StorageIdentities = append([]StorageIdentityID(nil), reference.StorageIdentities...)
		plan.Reference = &reference
	}
	plan.Parents = append([]ArenaParentDependency(nil), plan.Parents...)
	plan.ErrorCauses = append([]string(nil), plan.ErrorCauses...)
	plan.Unresolved = append([]string(nil), plan.Unresolved...)
	plan.PhysicalUnresolved = append([]string(nil), plan.PhysicalUnresolved...)
	return plan, true
}

// ResolvedArenaCreatePlanOf reads a constructor plan only.
// Rules: rules/mlir/packages/sec-mlir-dialect_package19.md — §§72,76.
func (a *Analyzer) ResolvedArenaCreatePlanOf(expr ast.Expression) (ResolvedArenaOperationPlan, bool) {
	p, ok := a.ResolvedArenaOperationPlanOf(expr)
	return p, ok && p.Operation == ArenaOperationCreate
}

// ResolvedArenaAllocationPlanOf reads New/Alloc plans only.
// Rules: rules/mlir/packages/sec-mlir-dialect_package19.md — §§73,76.
func (a *Analyzer) ResolvedArenaAllocationPlanOf(expr ast.Expression) (ResolvedArenaOperationPlan, bool) {
	p, ok := a.ResolvedArenaOperationPlanOf(expr)
	return p, ok && (p.Operation == ArenaOperationNew || p.Operation == ArenaOperationAlloc)
}

// ResolvedArenaResetPlanOf reads a dependency-checked Reset plan only.
// Rules: rules/mlir/packages/sec-mlir-dialect_package19.md — §§74,76.
func (a *Analyzer) ResolvedArenaResetPlanOf(expr ast.Expression) (ResolvedArenaOperationPlan, bool) {
	p, ok := a.ResolvedArenaOperationPlanOf(expr)
	return p, ok && p.Operation == ArenaOperationReset
}

// ResolvedArenaReleasePlanOf reads a dependency-checked Release plan only.
// Rules: rules/mlir/packages/sec-mlir-dialect_package19.md — §§75–76.
func (a *Analyzer) ResolvedArenaReleasePlanOf(expr ast.Expression) (ResolvedArenaOperationPlan, bool) {
	p, ok := a.ResolvedArenaOperationPlanOf(expr)
	return p, ok && p.Operation == ArenaOperationRelease
}

// snapshotArenaPlanPlace detaches the retained backing/receiver path and its
// structural types, including alternative provenance and index type facts.
// Rules: rules/mlir/packages/sec-mlir-dialect_package19.md — §§72–76;
// rules/compiler/compiler_analysis.md — immutable analysis results.
func snapshotArenaPlanPlace(place Place) Place {
	place = clonePlace(place)
	place.Type = semanticSnapshotType(place.Type)
	for i := range place.Projections {
		place.Projections[i].IndexType = semanticSnapshotType(place.Projections[i].IndexType)
	}
	for i := range place.AlternativeOrigins {
		place.AlternativeOrigins[i] = snapshotArenaPlanPlace(place.AlternativeOrigins[i])
	}
	return place
}

// recordArenaEpochAdvance publishes the checked epoch advance of a reachable
// Reset under the plan's reference policy and its potential exhaustion panic.
// An unresolvable policy leaves the plan unresolved instead of guessing a
// width, and no bound or elimination proof is produced here, so every Reset
// keeps MayPanic with panic.arena-epoch-exhaustion (arena.md § 39(8)).
// Rules: rules/memory/arena.md — §§ 38(8), 39(5)-(9);
// rules/analysis/effect_analysis.md — §§ 23(6), 26(4).
func (a *Analyzer) recordArenaEpochAdvance(call *ast.CallExpression, plan *ResolvedArenaOperationPlan) {
	plan.PhysicalUnresolved = append(plan.PhysicalUnresolved, "physical epoch counter and exhaustion panic endpoint (MD-140 stage 4)")
	source := plan.Source
	a.callGraph.removeEffect(a.currentCallable, EffectMayPanicArenaEpochExhaustion, source)
	a.callGraph.addEffect(a.currentCallable, EffectSite{Kind: EffectMayPanicArenaEpochExhaustion, Source: source, PanicReasonIDs: []diagnostics.PanicReasonID{diagnostics.PanicReasonArenaEpochExhaustion}})
	policy, err := a.referencePolicy.ResolveArenaEpochPolicy()
	if err != nil {
		plan.Unresolved = append(plan.Unresolved, "Arena epoch policy: "+err.Error())
		return
	}
	plan.EpochAdvance = ArenaEpochAdvancePlan{
		Form: ArenaEpochAdvanceChecked, LogicalEpochBits: policy.LogicalEpochBits,
		Exhaustion: policy.Exhaustion, PanicReason: diagnostics.PanicReasonArenaEpochExhaustion,
	}
}
