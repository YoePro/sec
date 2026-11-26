package layout

// layoutStability derives evidence at the canonical publication boundary, also
// on cache restoration. TargetStable means the owning representation rules plus
// this exact plan determine the layout; it never promises another target, ABI,
// profile or compiler rule revision. Ordinary native aggregates retain their
// weaker native guarantee even when all fields happen to have stable carriers.
// An unsupported explicit contract cannot become ContractStable merely by name;
// the explicit-contract resolver must first implement and prove that contract.
// Rules: rules/memory/layout.md §§18-19,24,33-35,39,47.1,47.6.
func (r *Resolver) layoutStability(value ResolvedLayout) (LayoutStability, string) {
	if !value.Complete() {
		return LayoutStabilityUnknown, "layout is not complete; no representation stability is proven"
	}
	if value.Contract != NativeLayoutContract {
		return LayoutStabilityUnknown, "explicit contract has no verified stability producer: " + string(value.Contract)
	}
	dependency := value.Carrier
	switch value.Kind {
	case LayoutScalar:
		// Primitive storage and decimal coefficient/scale representation are
		// canonical; alignment/decimal placement are resolved under PlanKey.
		return LayoutTargetStable, "canonical scalar representation and resolved target storage facts"
	case LayoutArray:
		dependency = value.Element
	case LayoutNamed, LayoutEnum, LayoutAtomic:
	default:
		return LayoutNativeUnstable, "native aggregate or descriptor; no independent stable representation contract"
	}
	child, ok := r.Layout(dependency)
	if !ok || !child.Complete() || child.Stability == LayoutStabilityUnknown || child.Stability == "" {
		return LayoutStabilityUnknown, "carrier or element stability is not proven"
	}
	switch child.Stability {
	case LayoutNativeUnstable:
		return LayoutNativeUnstable, "native representation dependency: " + child.TypeIdentity
	case LayoutTargetStable, LayoutContractStable:
		return LayoutTargetStable, "canonical representation over " + child.TypeIdentity + " under the same resolved plan"
	default:
		return LayoutStabilityUnknown, "unrecognized carrier or element stability evidence"
	}
}
