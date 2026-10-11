package sema

import (
	"fmt"
	"sort"

	"sec/internal/ast"
	"sec/internal/layout"
	"sec/internal/lexer"
)

// ReferenceKind is the P15 Semantic IR direct-reference category. Stable and
// weak handles are separate future categories and RawPtr is neither kind.
type ReferenceKind string

const (
	ReferenceSharedDirect  ReferenceKind = "shared-direct"
	ReferenceMutableDirect ReferenceKind = "mutable-direct"
)

// ReferenceValidityPolicy classifies how a direct reference's validity is
// established. Empty means neither proven nor dynamically checkable; such a
// reference must not reach Semantic IR as a safe reference.
type ReferenceValidityPolicy string

const (
	ReferenceValidityProven       ReferenceValidityPolicy = "proven"
	ReferenceValidityDynamicEpoch ReferenceValidityPolicy = "dynamic-epoch"
)

// ReferenceProvenanceKind is the semantic provenance class of a reference's
// origin storage.
type ReferenceProvenanceKind string

const (
	ReferenceProvenanceLocal     ReferenceProvenanceKind = "local"
	ReferenceProvenanceParameter ReferenceProvenanceKind = "parameter"
	ReferenceProvenanceReceiver  ReferenceProvenanceKind = "receiver"
	ReferenceProvenanceStatic    ReferenceProvenanceKind = "static"
	ReferenceProvenanceArena     ReferenceProvenanceKind = "arena"
	ReferenceProvenanceDerived   ReferenceProvenanceKind = "derived"
	ReferenceProvenanceUnknown   ReferenceProvenanceKind = "unknown"
)

// ReferenceFacts is the complete immutable fact set of one direct reference
// (package15.md § 31). There is no single "safe pointer" boolean: authority,
// validity, epoch dependency, address stability and provenance are separate.
type ReferenceFacts struct {
	Kind              ReferenceKind
	Referent          Type
	OriginPlaces      []Place
	StorageIdentities []StorageIdentityID
	AddressSpace      AddressSpaceID
	Borrow            BorrowID
	Lifetime          LifetimeID
	MutableAuthority  bool
	Validity          ReferenceValidityPolicy
	// ProofProvenance names the closed proof of a proven reference.
	ProofProvenance string
	// InvalidationDomain and EpochDependency identify the live incarnation a
	// dynamic-epoch reference depends on.
	InvalidationDomain InvalidationDomainID
	EpochDependency    EpochDependencyID
	// AddressStability replaces the withdrawn RelocationClass
	// (package15.md § 28 revised 2026-10-03; storage.md § 14); Pinned is the
	// separate pin dependency.
	AddressStability AddressStability
	Pinned           bool
	Provenance       ReferenceProvenanceKind
}

// Checkable reports whether the reference is proven or dynamically
// checkable; anything else must not be lowered as a safe reference.
func (facts ReferenceFacts) Checkable() bool {
	return facts.Validity == ReferenceValidityProven || facts.Validity == ReferenceValidityDynamicEpoch
}

// ResolvedReferencePlan is the read-only plan of one source reference
// creation or reborrow (package15.md § 33). `ref T` is CopyTrivial and
// `ref mut T` MoveOnly: copying a shared reference duplicates only the
// non-owning holder, and moving a mutable reference transfers the holder and
// its borrow obligation, never the referent (P15-21).
type ResolvedReferencePlan struct {
	Facts     ReferenceFacts
	Place     Place
	CopyClass CopyClassification
	// Reborrow is set when the borrowed Place is reached through an existing
	// reference; Implicit when the borrow is a marker-free call-site borrow.
	Reborrow bool
	Implicit bool
	Source   lexer.Token
}

// ResolvedReferenceUseKind is one reference-mediated access (package15.md § 36).
type ResolvedReferenceUseKind string

const (
	ReferenceUseRead     ResolvedReferenceUseKind = "read"
	ReferenceUseWrite    ResolvedReferenceUseKind = "write"
	ReferenceUseReborrow ResolvedReferenceUseKind = "reborrow"
	ReferenceUseEquality ResolvedReferenceUseKind = "equality"
	ReferenceUsePass     ResolvedReferenceUseKind = "pass"
	ReferenceUseReturn   ResolvedReferenceUseKind = "return"
	ReferenceUseEnd      ResolvedReferenceUseKind = "end"
)

// ResolvedReferenceUse is the read-only plan of one reference-mediated use:
// the reference facts, the validity policy to honour, and the ownership
// requirements of the access. The Semantic IR builder consumes it instead of
// re-running borrow checking or inferring behavior from syntax.
type ResolvedReferenceUse struct {
	Kind  ResolvedReferenceUseKind
	Place Place
	Facts ReferenceFacts
	// CopyTrivialRead reports a whole-value read of a copy-trivial referent;
	// TrivialReplacement a write whose referent is trivially replaceable.
	CopyTrivialRead    bool
	TrivialReplacement bool
	Source             lexer.Token
}

// ReferenceLifetimeEnd records a resolved end of a reference holder's borrow
// (package15.md § 49). It is compile-time structure, not runtime tracking.
type ReferenceLifetimeEnd struct {
	Holder string
	Place  Place
	Source lexer.Token
}

// referenceProvenance classifies the origin storage of a Place.
func referenceProvenance(place Place) ReferenceProvenanceKind {
	switch {
	case place.Root == "" || place.AmbiguousProvenance:
		return ReferenceProvenanceUnknown
	case place.StorageOrigin == StorageOriginArena:
		return ReferenceProvenanceArena
	case place.RootKind == PlaceRootParameter:
		return ReferenceProvenanceParameter
	case place.RootKind == PlaceRootReceiver:
		return ReferenceProvenanceReceiver
	case place.RootKind == PlaceRootStatic || place.StorageOrigin == StorageOriginStatic:
		return ReferenceProvenanceStatic
	case place.RootKind == PlaceRootDeref:
		return ReferenceProvenanceDerived
	default:
		return ReferenceProvenanceLocal
	}
}

// referenceFactsForPlace derives the reference facts of a borrow of place
// created at site. Validity is classified independently of representation:
// a known non-Arena origin is proven (the Sema borrow/lifetime proof closes
// it), an Arena-backed origin is a dynamic-epoch reference on its Arena
// domain's live epoch, and unknown or ambiguous provenance is neither, so
// the reference is not lowerable as a safe reference (P15-25).
//
// Rules:
//   - rules/mlir/packages/sec-mlir-dialect_package15.md — §§ 25-31
//   - rules/compiler/semantic_ir.md — §§ 45(5)-(13), 46(6)
//   - rules/memory/reference_model.md — §§ 33-34, 45
func (a *Analyzer) referenceFactsForPlace(place Place, mutable bool, site lexer.Token) ReferenceFacts {
	location := sourceTokenLocation(site)
	key := fmt.Sprintf("%s:%d:%d", location.File, location.Line, location.Column)
	facts := ReferenceFacts{
		Kind:             ReferenceSharedDirect,
		Referent:         semanticSnapshotType(place.Type),
		MutableAuthority: mutable,
		Borrow:           a.placeIdentities.borrow("borrow:" + key),
		Lifetime:         a.placeIdentities.lifetime("lifetime:" + key),
		AddressSpace:     place.AddressSpace,
		AddressStability: AddressStabilityStable,
		Provenance:       referenceProvenance(place),
	}
	if mutable {
		facts.Kind = ReferenceMutableDirect
	}
	if facts.AddressSpace == AddressSpaceNone {
		facts.AddressSpace = AddressSpaceOrdinary
	}
	origins := append([]Place{clonePlace(place)}, clonePlaces(place.AlternativeOrigins)...)
	for index := range origins {
		origins[index].AlternativeOrigins = nil
	}
	facts.OriginPlaces = origins
	seen := map[StorageIdentityID]bool{}
	for _, origin := range origins {
		if origin.StorageIdentity != 0 && !seen[origin.StorageIdentity] {
			seen[origin.StorageIdentity] = true
			facts.StorageIdentities = append(facts.StorageIdentities, origin.StorageIdentity)
		}
	}
	sort.Slice(facts.StorageIdentities, func(i, j int) bool { return facts.StorageIdentities[i] < facts.StorageIdentities[j] })
	if symbol, ok := a.symbols[place.Root]; ok && symbol.AddressStability != "" {
		facts.AddressStability = symbol.AddressStability
	}
	if facts.Provenance == ReferenceProvenanceUnknown {
		facts.AddressStability = AddressStabilityUnknown
		return facts
	}
	if domain, generation, arena := a.arenaOriginOfPlace(place); arena {
		facts.Provenance = ReferenceProvenanceArena
		facts.Validity = ReferenceValidityDynamicEpoch
		facts.InvalidationDomain = a.placeIdentities.invalidationDomain("arena:" + domain)
		facts.EpochDependency = a.placeIdentities.epochDependency(fmt.Sprintf("arena:%s@%d", domain, generation))
		return facts
	}
	facts.Validity = ReferenceValidityProven
	switch facts.Provenance {
	case ReferenceProvenanceStatic:
		facts.ProofProvenance = "static-storage"
	case ReferenceProvenanceParameter, ReferenceProvenanceReceiver:
		facts.ProofProvenance = "caller-owned-borrow"
	case ReferenceProvenanceDerived:
		facts.ProofProvenance = "reborrow-of-valid-reference"
	default:
		facts.ProofProvenance = "lexical-borrow"
	}
	if facts.AddressStability == AddressStabilityFixed {
		facts.ProofProvenance = "fixed-address-storage"
	}
	return facts
}

// arenaOriginOfPlace reports the Arena domain and expected generation of a
// Place whose root is Arena-backed storage reached through a reference holder.
func (a *Analyzer) arenaOriginOfPlace(place Place) (string, int, bool) {
	symbol, ok := a.symbols[place.Root]
	if !ok || !typeCarriesReferenceOrigin(symbol.Type) || symbol.Type.ReferenceOriginStorage != StorageOriginArena || symbol.Type.ReferenceOriginName == "" {
		return "", 0, false
	}
	return symbol.Type.ReferenceOriginName, symbol.Type.ReferenceOriginGeneration, true
}

// placeReachedThroughReference reports a Place whose storage is reached
// through an existing reference holder rather than owned by its root.
func (a *Analyzer) placeReachedThroughReference(place Place) bool {
	if place.ReferenceHolder != "" {
		return true
	}
	for _, projection := range place.Projections {
		if projection.Kind == PlaceDereference {
			return true
		}
	}
	symbol, ok := a.symbols[place.Root]
	return ok && typeCarriesReferenceOrigin(symbol.Type)
}

// recordResolvedPlace publishes the analyzed Place of one expression; later
// resolutions of the same expression replace it with the newest flow facts.
func (a *Analyzer) recordResolvedPlace(expr ast.Expression, place Place) {
	if a.summaryPass || expr == nil {
		return
	}
	if a.resolvedPlaces == nil {
		a.resolvedPlaces = map[ast.Expression]Place{}
	}
	a.resolvedPlaces[expr] = clonePlace(place)
}

// recordReferencePlan publishes the plan of one reference creation or
// reborrow keyed by its exact AST node.
func (a *Analyzer) recordReferencePlan(node ast.Expression, place Place, mutable, implicit bool, site lexer.Token) {
	if a.summaryPass || node == nil {
		return
	}
	if a.resolvedReferencePlans == nil {
		a.resolvedReferencePlans = map[ast.Expression]ResolvedReferencePlan{}
	}
	plan := ResolvedReferencePlan{
		Facts:     a.referenceFactsForPlace(place, mutable, site),
		Place:     clonePlace(place),
		CopyClass: CopyTrivial,
		Reborrow:  a.placeReachedThroughReference(place),
		Implicit:  implicit,
		Source:    site,
	}
	if mutable {
		plan.CopyClass = CopyMoveOnly
	}
	a.resolvedReferencePlans[node] = plan
	if plan.Reborrow {
		a.recordReferenceUse(node, ReferenceUseReborrow, place, mutable, site)
	}
}

// recordReferenceUse publishes one reference-mediated use keyed by its exact
// AST node; a later, more specific use of the same node (a write after the
// read of its target) replaces the earlier one.
func (a *Analyzer) recordReferenceUse(node ast.Expression, kind ResolvedReferenceUseKind, place Place, mutable bool, site lexer.Token) {
	if a.summaryPass || node == nil {
		return
	}
	if a.resolvedReferenceUses == nil {
		a.resolvedReferenceUses = map[ast.Expression]ResolvedReferenceUse{}
	}
	use := ResolvedReferenceUse{
		Kind:   kind,
		Place:  clonePlace(place),
		Facts:  a.referenceFactsForPlace(place, mutable, site),
		Source: site,
	}
	use.CopyTrivialRead = kind == ReferenceUseRead && CopyClassificationOf(place.Type) == CopyTrivial
	use.TrivialReplacement = kind == ReferenceUseWrite && TriviallyDestructible(place.Type)
	a.resolvedReferenceUses[node] = use
}

// recordReferenceReadUse marks an expression evaluated as a value through an
// existing reference as a read use.
func (a *Analyzer) recordReferenceReadUse(expr ast.Expression, typ Type) {
	if a.summaryPass || typ.Kind == InvalidType {
		return
	}
	switch expr.(type) {
	case *ast.Identifier, *ast.MemberExpression, *ast.IndexExpression:
	default:
		return
	}
	place, ok := a.resolvedPlaces[expr]
	if !ok {
		place, ok = a.resolvePlace(expr)
	}
	if !ok || !a.placeReachedThroughReference(place) {
		return
	}
	if existing, recorded := a.resolvedReferenceUses[expr]; recorded && existing.Kind != ReferenceUseRead {
		return
	}
	if identifier, ok := expr.(*ast.Identifier); ok {
		// Naming a reference holder reads the holder, not the referent.
		if symbol, found := a.symbols[identifier.Value]; found && typeCarriesReferenceOrigin(symbol.Type) && typeCarriesReferenceOrigin(typ) {
			return
		}
	}
	a.recordReferenceUse(expr, ReferenceUseRead, place, false, expressionToken(expr))
}

// recordReferenceWriteUse marks an assignment target reached through a
// mutable reference as a write use.
func (a *Analyzer) recordReferenceWriteUse(target ast.Expression) {
	place, ok := a.resolvePlace(target)
	if !ok || !a.placeReachedThroughReference(place) {
		return
	}
	a.recordReferenceUse(target, ReferenceUseWrite, place, true, expressionToken(target))
}

// recordReferenceLifetimeEnd records the resolved end of a holder's borrow.
func (a *Analyzer) recordReferenceLifetimeEnd(holder string, source lexer.Token) {
	if a.summaryPass || holder == "" {
		return
	}
	symbol, ok := a.symbols[holder]
	if !ok || !typeCarriesReferenceOrigin(symbol.Type) {
		return
	}
	place, ok := a.rootPlace(holder)
	if !ok {
		return
	}
	a.referenceLifetimeEnds = append(a.referenceLifetimeEnds, ReferenceLifetimeEnd{Holder: holder, Place: clonePlace(place), Source: source})
}

// recordCallReferencePlans publishes the borrows of one validated call: an
// owned argument bound to a borrowed parameter is an implicit call-site
// borrow plan (borrowing.md § 15.2), and an existing reference passed on is a
// pass use.
func (a *Analyzer) recordCallReferencePlans(callee Function, args []ast.Expression, preparedSpreadValues []bool) {
	if a.summaryPass {
		return
	}
	for index, arg := range args {
		if index < len(preparedSpreadValues) && preparedSpreadValues[index] {
			continue
		}
		parameter, ok := functionParameterForArgument(callee, index)
		if !ok {
			return
		}
		mutable, borrowed := callParameterBorrowMode(parameter)
		if !borrowed {
			continue
		}
		place, ok := a.resolvePlace(arg)
		if !ok {
			continue
		}
		if argType, typed := a.expressionTypes[arg]; typed && typeCarriesReferenceOrigin(argType) {
			a.recordReferenceUse(arg, ReferenceUsePass, place, mutable, expressionToken(arg))
			continue
		}
		a.recordReferencePlan(arg, place, mutable, true, expressionToken(arg))
	}
}

// ResolvedPlaceOf returns the analyzed Place of an expression. It performs no
// Place resolution, borrow creation, origin inference or diagnostics.
//
// Rules:
//   - rules/mlir/packages/sec-mlir-dialect_package15.md — § 35 "Place-plan query"
func (a *Analyzer) ResolvedPlaceOf(expr ast.Expression) (Place, bool) {
	if a == nil || expr == nil {
		return Place{}, false
	}
	place, ok := a.resolvedPlaces[expr]
	if !ok {
		return Place{}, false
	}
	return clonePlace(place), true
}

// ResolvedReferencePlanOf returns the plan of one reference creation or
// reborrow keyed by its exact AST node (a ref expression or an implicitly
// borrowed call argument). It never creates borrows or re-resolves anything.
//
// Rules:
//   - rules/mlir/packages/sec-mlir-dialect_package15.md — §§ 33-34
func (a *Analyzer) ResolvedReferencePlanOf(expr ast.Expression) (ResolvedReferencePlan, bool) {
	if a == nil || expr == nil {
		return ResolvedReferencePlan{}, false
	}
	plan, ok := a.resolvedReferencePlans[expr]
	if !ok {
		return ResolvedReferencePlan{}, false
	}
	return cloneResolvedReferencePlan(plan), true
}

// ResolvedReferenceUseOf returns the reference-mediated use of one expression.
//
// Rules:
//   - rules/mlir/packages/sec-mlir-dialect_package15.md — § 36 "Reference use plan"
func (a *Analyzer) ResolvedReferenceUseOf(expr ast.Expression) (ResolvedReferenceUse, bool) {
	if a == nil || expr == nil {
		return ResolvedReferenceUse{}, false
	}
	use, ok := a.resolvedReferenceUses[expr]
	if !ok {
		return ResolvedReferenceUse{}, false
	}
	use.Place = clonePlace(use.Place)
	use.Facts = cloneReferenceFacts(use.Facts)
	return use, true
}

// ReferenceLifetimeEnds returns the resolved borrow ends in source order.
func (a *Analyzer) ReferenceLifetimeEnds() []ReferenceLifetimeEnd {
	if a == nil {
		return nil
	}
	out := make([]ReferenceLifetimeEnd, len(a.referenceLifetimeEnds))
	for index, end := range a.referenceLifetimeEnds {
		end.Place = clonePlace(end.Place)
		out[index] = end
	}
	return out
}

// ReferencePolicy returns the CompilationPlan reference policy of this
// analysis (default 64-bit logical epoch on every target).
func (a *Analyzer) ReferencePolicy() layout.ResolvedReferencePolicy {
	if a == nil {
		return layout.ResolvedReferencePolicy{}
	}
	return a.referencePolicy
}

func cloneReferenceFacts(facts ReferenceFacts) ReferenceFacts {
	facts.Referent = semanticSnapshotType(facts.Referent)
	facts.OriginPlaces = clonePlaces(facts.OriginPlaces)
	facts.StorageIdentities = append([]StorageIdentityID(nil), facts.StorageIdentities...)
	return facts
}

func cloneResolvedReferencePlan(plan ResolvedReferencePlan) ResolvedReferencePlan {
	plan.Facts = cloneReferenceFacts(plan.Facts)
	plan.Place = clonePlace(plan.Place)
	return plan
}

// recordReferenceExpressionUse classifies a value expression after inference:
// equality of two references is an equality use, and a value read through an
// existing reference a read use.
func (a *Analyzer) recordReferenceExpressionUse(expr ast.Expression, typ Type) {
	if a.summaryPass || typ.Kind == InvalidType {
		return
	}
	if infix, ok := expr.(*ast.InfixExpression); ok && (infix.Operator == "==" || infix.Operator == "!=") {
		left, leftTyped := a.expressionTypes[infix.Left]
		right, rightTyped := a.expressionTypes[infix.Right]
		if leftTyped && rightTyped && left.Kind == ReferenceType && right.Kind == ReferenceType {
			if place, placed := a.resolvePlace(infix.Left); placed {
				a.recordReferenceUse(expr, ReferenceUseEquality, place, false, infix.Token)
			}
		}
		return
	}
	a.recordReferenceReadUse(expr, typ)
}

// recordReferenceReturnUse marks a returned reference value as a return use.
func (a *Analyzer) recordReferenceReturnUse(value ast.Expression, returnType Type) {
	if a.summaryPass || value == nil || !typeCarriesReferenceOrigin(returnType) {
		return
	}
	if place, placed := a.resolvePlace(value); placed {
		a.recordReferenceUse(value, ReferenceUseReturn, place, returnType.ReferenceMutable, expressionToken(value))
	}
}
