package layout

import (
	"fmt"
	"reflect"
)

// AtomicRequirement is a concrete, already semantically checked atomic request.
// TypeID is the compiler's canonical nominal type identity, including generic
// arguments; it is not a display name, size class or host Go type. ContextID
// identifies the resolved execution contract. NoBlocking and InterruptSafe
// retain its additional requirements, not claims about the implementation.
// A Fence has no contained type or storage layout and has one explicit Order.
// Rules: rules/concurrency/atomics.md — §§24,26,37(1–5),39;
// rules/platform/target_profiles.md — §10 and atomic capability separation.
type AtomicRequirement struct {
	TypeID         string
	SizeBytes      uint64
	AlignmentBytes uint64
	Operation      string
	Order          string
	FailureOrder   string
	ContextID      string
	NoBlocking     bool
	InterruptSafe  bool
}

// AtomicAvailability describes a concrete target implementation fact, never
// language eligibility. An absent fact is Unknown, not Unsupported or Supported.
// Rules: rules/concurrency/atomics.md — §§8(7),37–39;
// rules/platform/platform_model.md — §9 "Architecture capability".
type AtomicAvailability string

const (
	AtomicUnknown     AtomicAvailability = "unknown"
	AtomicSupported   AtomicAvailability = "supported"
	AtomicUnsupported AtomicAvailability = "unsupported"
)

// AtomicCapability retains a target producer's fact and its proof/registry
// provenance. A supported fact names its implementation separately from its
// progress/context guarantees. None of these facts changes language eligibility
// or substitutes for execution-context validation before lowering.
// Rules: rules/concurrency/atomics.md — §§37–39,47–48.
type AtomicCapability struct {
	Requirement    AtomicRequirement
	Availability   AtomicAvailability
	Provenance     string
	Reason         string
	Implementation AtomicImplementation
}

// AtomicCapabilities is an immutable, plan-bound lookup. Its zero value has
// no positive or negative evidence. Input slices and returned facts never
// expose the private table; copying a plan safely shares its frozen snapshot.
// Rules: rules/concurrency/atomics.md — §37(1);
// rules/platform/target_profiles.md — §§23,29–31.
type AtomicCapabilities struct {
	snapshot *atomicCapabilitySnapshot
}

type atomicCapabilitySnapshot struct {
	plan  ResolvedScalarPlan
	facts map[AtomicRequirement]AtomicCapability
}

// validate checks completeness and canonical operation/order vocabulary only.
// The source-language operation/order matrix remains Sema's responsibility:
// a target fact can never legalize Load(Release), a relaxed Fence, or an
// ineligible T. This model does not reclassify language semantics.
// Rules: rules/concurrency/atomics.md — §§24(1–4),37(2–3),45.
func (r AtomicRequirement) validate() error {
	if r.ContextID == "" {
		return fmt.Errorf("atomic requirement needs a resolved execution context identity")
	}
	switch r.Operation {
	case "Fence":
		if r.TypeID != "" || r.SizeBytes != 0 || r.AlignmentBytes != 0 {
			return fmt.Errorf("atomic Fence has no contained type or storage layout")
		}
	case "Load", "Store", "Swap", "CompareExchange", "FetchAdd", "FetchSub", "FetchAnd", "FetchOr", "FetchXor":
		if r.TypeID == "" || r.SizeBytes == 0 || r.AlignmentBytes == 0 {
			return fmt.Errorf("atomic %s requires a concrete type identity, size and alignment", r.Operation)
		}
	default:
		return fmt.Errorf("unknown canonical atomic operation %q", r.Operation)
	}
	if !atomicOrderSpelling(r.Order) {
		return fmt.Errorf("atomic requirement needs a resolved MemoryOrder, got %q", r.Order)
	}
	if r.Operation == "CompareExchange" {
		if !atomicOrderSpelling(r.FailureOrder) {
			return fmt.Errorf("CompareExchange requirement needs a separate resolved failure order")
		}
	} else if r.FailureOrder != "" {
		return fmt.Errorf("only CompareExchange has a failure order")
	}
	return nil
}

// atomicOrderSpelling recognizes the canonical enum vocabulary without imposing
// an extra target-side language validity matrix or relative-strength rule.
// Rules: rules/concurrency/atomics.md — §§5,24.
func atomicOrderSpelling(order string) bool {
	switch order {
	case "Relaxed", "Acquire", "Release", "AcqRel", "SeqCst":
		return true
	}
	return false
}

// WithAtomicCapabilities freezes producer facts for this exact resolved plan.
// A later change to any represented plan fact invalidates the binding instead
// of carrying support across targets, profiles, ABIs or reference policies.
// It declares no registry capabilities and never consults the compiler host.
// Rules: rules/concurrency/atomics.md — §§37(1–5),48(2);
// rules/platform/target_profiles.md — §§23,29–31.
func (p ResolvedScalarPlan) WithAtomicCapabilities(facts []AtomicCapability) (ResolvedScalarPlan, error) {
	if err := p.Validate(); err != nil {
		return ResolvedScalarPlan{}, err
	}
	if p.Profile == "" {
		return ResolvedScalarPlan{}, fmt.Errorf("atomic capabilities require a resolved target profile")
	}
	base := p
	base.Atomics = AtomicCapabilities{}
	snapshot := &atomicCapabilitySnapshot{plan: base, facts: make(map[AtomicRequirement]AtomicCapability, len(facts))}
	for _, fact := range facts {
		if err := fact.validate(); err != nil {
			return ResolvedScalarPlan{}, fmt.Errorf("target %s/%s profile %s: %w", p.TargetOS, p.TargetArch, p.Profile, err)
		}
		if _, duplicate := snapshot.facts[fact.Requirement]; duplicate {
			return ResolvedScalarPlan{}, fmt.Errorf("duplicate atomic capability for %+v", fact.Requirement)
		}
		snapshot.facts[fact.Requirement] = fact
	}
	p.Atomics = AtomicCapabilities{snapshot: snapshot}
	return p, nil
}

// validateFor refuses a snapshot copied into a different target/profile plan.
// All scalar facts participate so future represented scalar fields cannot be
// silently omitted from this binding.
// Rules: rules/concurrency/atomics.md — §37(1–5);
// rules/platform/target_profiles.md — §31 "Canonical identity".
func (c AtomicCapabilities) validateFor(p ResolvedScalarPlan) error {
	if c.snapshot == nil {
		return nil
	}
	p.Atomics = AtomicCapabilities{}
	if !reflect.DeepEqual(p, c.snapshot.plan) {
		return fmt.Errorf("atomic capability snapshot belongs to a different resolved target/profile plan")
	}
	return nil
}

// AtomicCapability queries only an exact request in the selected frozen plan.
// Unknown is conservative absence of evidence, including on registered hosted
// targets. Neither width, a similar type/order/context nor a stronger-order
// implementation is substituted; lowering owns any proven strengthening.
// Rules: rules/concurrency/atomics.md — §§24(3),37,47(1–4);
// rules/platform/platform_model.md — §9 "Architecture capability".
func (p ResolvedScalarPlan) AtomicCapability(request AtomicRequirement) (AtomicCapability, error) {
	if err := p.Validate(); err != nil {
		return AtomicCapability{}, err
	}
	if err := request.validate(); err != nil {
		return AtomicCapability{}, err
	}
	if p.Atomics.snapshot != nil {
		if fact, found := p.Atomics.snapshot.facts[request]; found {
			return fact, nil
		}
	}
	return AtomicCapability{Requirement: request, Availability: AtomicUnknown}, nil
}
