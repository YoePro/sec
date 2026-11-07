package sema

import (
	"fmt"
	"strings"

	"sec/internal/ast"
)

// ResolvedDynamicArrayPlan is the read-only Sema fact of one owning dynamic
// array `T[]` (P18-05). Its type-level part holds for every value of the
// type: `T[]` is MoveOnly and non-trivially destructible for every T, and its
// canonical empty default is an initialized owner with length and capacity
// zero, no backing storage and no constructed element, which allocates
// nothing. Owner is set when the value is a Place: the storage of the owning
// descriptor and the separate invalidation domain of its backing store.
//
// Capacity, allocation context, backing relation and reclamation authority
// belong to the producing operation's allocation plan (P18-06/07), not to the
// type: the empty owner has no backing at all.
//
// Rules:
//   - rules/mlir/packages/sec-mlir-dialect_package18.md — §§ 5, 7–11, 24, 54–55
//   - rules/collections/collections.md — §§ 6.1–6.3, 12.2
//   - rules/types/default_values.md — "Owning dynamic arrays"
//   - rules/memory/storage.md — §§ 3.6, 3.9(2), 22
type ResolvedDynamicArrayPlan struct {
	ArrayType        Type
	ElementType      Type
	CopyClass        CopyClassification
	ElementCopyClass CopyClassification
	// EmptyDefault: the canonical default exists independently of T's
	// defaultability and constructs no element.
	EmptyDefault bool
	// DefaultAllocates is always false: implicit default initialization
	// never dynamically allocates, so a no-allocation profile still has the
	// empty owner (package18 §§ 6, 18).
	DefaultAllocates bool
	// MutableDefaultDeclaration: `let mut v: T[]` initializes the empty
	// owner; an immutable declaration still requires an initializer.
	MutableDefaultDeclaration    bool
	ImmutableRequiresInitializer bool
	// RequiresDestruction is always true: another value of the type may own
	// backing storage. Destroying the empty owner is a no-op specialization.
	RequiresDestruction        bool
	ElementRequiresDestruction bool
	DestructionPlan            DestructionPlanID
	// ElementDestructionPlan is empty when T is trivially destructible.
	ElementDestructionPlan DestructionPlanID
	// EmptyBackingRelation and EmptyReclamation describe the canonical empty
	// owner: no backing and no reclamation until backing is established
	// (package18 § 22).
	EmptyBackingRelation BackingRelation
	EmptyReclamation     ReclamationAuthority
	// BackingAddressStability is the normal class of established backing;
	// the descriptor itself may move independently (§ 70).
	BackingAddressStability AddressStability
	// DestructionTransitions is the ordered backing work of destroying an
	// owner with live backing: destroy elements in reverse index order when
	// T needs destruction, end the backing domain, then reclaim the bytes
	// only under the backing's OwningValue authority (§§ 13, 30). The empty
	// owner skips it (§ 9).
	DestructionTransitions []BackingTransitionKind
	Owner                  *DynamicArrayOwner
}

// DynamicArrayOwner identifies a dynamic-array value held in a Place. The
// collection object (the owning descriptor in OwnerStorage) is distinct from
// its backing domain: element references and slices depend on the backing
// incarnation, not on the descriptor's storage (package18 § 24). A zero
// BackingDomain is unknown, for an owner reached through a runtime index or
// storage Sema cannot identify.
type DynamicArrayOwner struct {
	Place         Place
	OwnerStorage  StorageIdentityID
	OwnerOrigin   StorageOrigin
	BackingDomain InvalidationDomainID
}

// ResolveDynamicArrayPlan derives the type-level plan of an owning dynamic
// array. It is pure and reports false for any other type.
func ResolveDynamicArrayPlan(typ Type) (ResolvedDynamicArrayPlan, bool) {
	if typ.Kind != ArrayType || arrayShapeOf(typ) != ArrayShapeDynamic || typ.Element == nil {
		return ResolvedDynamicArrayPlan{}, false
	}
	element := *typ.Element
	plan := ResolvedDynamicArrayPlan{
		ArrayType: typ, ElementType: element,
		CopyClass: CopyClassificationOf(typ), ElementCopyClass: CopyClassificationOf(element),
		EmptyDefault: true, MutableDefaultDeclaration: true, ImmutableRequiresInitializer: true,
		RequiresDestruction:        !TriviallyDestructible(typ),
		ElementRequiresDestruction: !TriviallyDestructible(element),
		DestructionPlan:            destructionPlanIDOf(typ),
	}
	if plan.ElementRequiresDestruction {
		plan.ElementDestructionPlan = destructionPlanIDOf(element)
		plan.DestructionTransitions = append(plan.DestructionTransitions, BackingTransitionDestroyElements)
	}
	plan.EmptyBackingRelation, plan.EmptyReclamation = BackingRelationNone, ReclamationNone
	plan.BackingAddressStability = AddressStabilityMovable
	plan.DestructionTransitions = append(plan.DestructionTransitions, BackingTransitionEndDomain, BackingTransitionReclaimPerPlan)
	return plan, true
}

// recordDynamicArrayOwner allocates the backing-domain identity of a
// dynamic-array owner Place when Sema publishes the Place, so the query
// never allocates (P17-14 discipline).
func (a *Analyzer) recordDynamicArrayOwner(expr ast.Expression, place Place) {
	if _, dynamic := ResolveDynamicArrayPlan(place.Type); !dynamic {
		return
	}
	owner := DynamicArrayOwner{Place: clonePlace(place), OwnerStorage: place.StorageIdentity, OwnerOrigin: place.StorageOrigin}
	if key, known := dynamicArrayBackingKey(place); known {
		owner.BackingDomain = a.placeIdentities.invalidationDomain(key)
	}
	if a.dynamicArrayOwners == nil {
		a.dynamicArrayOwners = map[ast.Expression]DynamicArrayOwner{}
	}
	a.dynamicArrayOwners[expr] = owner
}

// dynamicArrayBackingKey names the backing store of the owner at one exact
// statically identified Place: its root storage plus the field and constant
// index path. A runtime index or unknown root storage has no static backing
// identity.
func dynamicArrayBackingKey(place Place) (string, bool) {
	if place.StorageIdentity == 0 {
		return "", false
	}
	var path strings.Builder
	for _, projection := range place.Projections {
		switch {
		case projection.Kind == PlaceField || projection.Kind == PlaceUnionPayload:
			path.WriteString("." + projection.Name)
		case projection.Kind == PlaceIndex && projection.ConstantIndex != nil:
			path.WriteString("[" + projection.ConstantIndex.String() + "]")
		default:
			return "", false
		}
	}
	return fmt.Sprintf("dynamic-array-backing:%d%s", place.StorageIdentity, path.String()), true
}

// ResolvedDynamicArrayPlanOf reads the plan of an expression of type `T[]`,
// with its owner identities when the expression is a published Place. It
// allocates nothing and infers nothing; the result is detached.
func (a *Analyzer) ResolvedDynamicArrayPlanOf(expr ast.Expression) (ResolvedDynamicArrayPlan, bool) {
	if a == nil || expr == nil {
		return ResolvedDynamicArrayPlan{}, false
	}
	typ, ok := a.ResolvedTypeOf(expr)
	if !ok {
		return ResolvedDynamicArrayPlan{}, false
	}
	plan, dynamic := ResolveDynamicArrayPlan(typ)
	if !dynamic {
		return ResolvedDynamicArrayPlan{}, false
	}
	if owner, held := a.dynamicArrayOwners[expr]; held {
		owner.Place = clonePlace(owner.Place)
		plan.Owner = &owner
	}
	plan.DestructionTransitions = append([]BackingTransitionKind(nil), plan.DestructionTransitions...)
	return plan, true
}
