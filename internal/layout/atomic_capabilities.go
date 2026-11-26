package layout

import (
	"encoding/json"
	"fmt"
	"reflect"
	"sort"
)

// atomicKeySchema names the capability key schema. MD-095 replaced the
// semantic-type-only exact-match key and free-string contexts; a snapshot
// built under another schema is never reused (correction § 8(5), T61).
// Rules: rules/concurrency/atomics.md — § 37A(14).
const atomicKeySchema = "sec.atomic-capability-key/3"

// AtomicContext is a compiler-known execution category. A task, thread,
// function name or interrupt vector is never a category; the producer
// derives it from execution-context analysis and the invocation contract.
// Rules: rules/concurrency/atomics.md — § 37A(8);
// rules/platform/target_profiles.md — § 19.
type AtomicContext string

const (
	AtomicContextOrdinary  AtomicContext = "Ordinary"
	AtomicContextTask      AtomicContext = "Task"
	AtomicContextThread    AtomicContext = "Thread"
	AtomicContextInterrupt AtomicContext = "Interrupt"
	AtomicContextStartup   AtomicContext = "Startup"
	AtomicContextShutdown  AtomicContext = "Shutdown"
)

// canonical reports whether c is one of the six categories of
// target_profiles.md § 19; any other spelling is noncanonical input (T23).
// Rules: rules/concurrency/atomics.md — § 37A(8).
func (c AtomicContext) canonical() bool {
	switch c {
	case AtomicContextOrdinary, AtomicContextTask, AtomicContextThread, AtomicContextInterrupt, AtomicContextStartup, AtomicContextShutdown:
		return true
	}
	return false
}

// AtomicContextRefinement holds canonical target facts that restrict a
// category's proof when the selected target distinguishes them. A
// refinement creates no context and enables no forbidden capability; empty
// fields mean the category-wide case.
// Rules: rules/concurrency/atomics.md — § 37A(8);
// rules/platform/target_profiles.md — § 19 (MD-095).
type AtomicContextRefinement struct {
	InterruptVector string
	Priority        string
	MaskingDomain   string
	Core            string
	ExecutionDomain string
}

// AtomicCapabilityKey is the physical capability key: carrier,
// representation, access extent, guaranteed alignment, operation, requested
// orders, canonical context and additional requirements within the selected
// plan. It never contains the semantic source type, so equivalent nominal
// carriers share one target entry. A Fence has no carrier, representation,
// size or alignment.
// Rules: rules/concurrency/atomics.md — §§ 37A(4)-(9);
// rules/memory/layout.md — § 47.6(4).
type AtomicCapabilityKey struct {
	CarrierTypeID  string
	Representation StorageRepresentation
	SizeBytes      uint64
	AlignmentBytes uint64
	Operation      string
	Order          string
	FailureOrder   string
	Context        AtomicContext
	NoBlocking     bool
	InterruptSafe  bool
	Refinement     AtomicContextRefinement
}

// AtomicRequirement is a concrete, already semantically checked atomic
// request. SemanticTypeID is the complete canonical identity of T from the
// compiler type-identity service and is retained for validation and
// diagnostics only; Key is what the plan is queried by. Provenance names
// the source of the request.
// Rules: rules/concurrency/atomics.md — §§ 37A(4), 37A(9).
type AtomicRequirement struct {
	SemanticTypeID string
	Key            AtomicCapabilityKey
	Provenance     string
}

// AtomicAvailability describes a concrete target implementation fact, never
// language eligibility. An absent fact is Unknown, not Unsupported or Supported.
// Rules: rules/concurrency/atomics.md — §§8(7),37–39, 37A(11);
// rules/platform/platform_model.md — §§ 9, 14.
type AtomicAvailability string

const (
	AtomicUnknown     AtomicAvailability = "unknown"
	AtomicSupported   AtomicAvailability = "supported"
	AtomicUnsupported AtomicAvailability = "unsupported"
)

// AtomicCapability retains a target producer's fact for one key and its
// proof/registry provenance. A supported fact names its implementation
// separately from its progress/context guarantees. None of these facts
// changes language eligibility or substitutes for execution-context
// validation before lowering. Activation fields are query output from the
// plan-wide profile policy, never an operation producer's permission override;
// Supported availability alone does not establish Enabled activation.
// Rules: rules/concurrency/atomics.md — §§37–39, 37A(10), 47–48.
type AtomicCapability struct {
	Key                  AtomicCapabilityKey
	Availability         AtomicAvailability
	Provenance           string
	Reason               string
	Implementation       AtomicImplementation
	Activation           AtomicActivation
	ActivationProvenance string
	ActivationReason     string
}

// AtomicCapabilities is an immutable, plan-bound lookup. Its zero value has
// no positive or negative evidence. Input slices and returned facts never
// expose the private table; copying a plan safely shares its frozen snapshot.
// Rules: rules/concurrency/atomics.md — §§ 37(1), 37A(14);
// rules/platform/target_profiles.md — §§23,29–31.
type AtomicCapabilities struct {
	snapshot *atomicCapabilitySnapshot
}

type atomicCapabilitySnapshot struct {
	schema string
	plan   ResolvedScalarPlan
	facts  map[AtomicCapabilityKey]AtomicCapability
	policy AtomicProfilePolicy
}

// validate checks completeness and canonical vocabulary of a key. The
// source-language operation/order matrix remains Sema's responsibility: a
// target fact can never legalize Load(Release), a relaxed Fence or an
// ineligible T. A storage operation names a carrier and a scalar storage
// representation whose width is the access extent; nothing unknown is a
// usable zero (T45).
// Rules: rules/concurrency/atomics.md — §§24(1–4), 37A(4)-(9), 45;
// rules/memory/layout.md — § 47.6(4), (6).
func (k AtomicCapabilityKey) validate() error {
	if !k.Context.canonical() {
		return fmt.Errorf("atomic requirement needs a canonical execution category (Ordinary, Task, Thread, Interrupt, Startup or Shutdown), got %q", k.Context)
	}
	if k.Context == AtomicContextInterrupt && !k.InterruptSafe {
		return fmt.Errorf("an Interrupt atomic requirement must require interrupt safety")
	}
	if k.Refinement.InterruptVector != "" && k.Context != AtomicContextInterrupt {
		return fmt.Errorf("an interrupt-vector refinement applies only to the Interrupt category")
	}
	switch k.Operation {
	case "Fence":
		if k.CarrierTypeID != "" || k.Representation != (StorageRepresentation{}) || k.SizeBytes != 0 || k.AlignmentBytes != 0 {
			return fmt.Errorf("atomic Fence has no carrier, representation or storage layout")
		}
	case "Load", "Store", "Swap", "CompareExchange", "FetchAdd", "FetchSub", "FetchAnd", "FetchOr", "FetchXor":
		if k.CarrierTypeID == "" {
			return fmt.Errorf("atomic %s requires a resolved carrier identity", k.Operation)
		}
		if k.Representation.Class != IntegerStorage || !validStorageWidth(k.Representation.WidthBits) {
			return fmt.Errorf("atomic %s requires an integer storage representation, got %v", k.Operation, k.Representation)
		}
		if k.SizeBytes != uint64(k.Representation.WidthBits)/8 {
			return fmt.Errorf("atomic %s access extent %d does not match representation %v", k.Operation, k.SizeBytes, k.Representation)
		}
		if k.AlignmentBytes == 0 || k.AlignmentBytes&(k.AlignmentBytes-1) != 0 {
			return fmt.Errorf("atomic %s requires a resolved power-of-two storage alignment, got %d", k.Operation, k.AlignmentBytes)
		}
	default:
		return fmt.Errorf("unknown canonical atomic operation %q", k.Operation)
	}
	if !atomicOrderSpelling(k.Order) {
		return fmt.Errorf("atomic requirement needs a resolved MemoryOrder, got %q", k.Order)
	}
	if k.Operation == "CompareExchange" {
		if !atomicOrderSpelling(k.FailureOrder) {
			return fmt.Errorf("CompareExchange requirement needs a separate resolved failure order")
		}
	} else if k.FailureOrder != "" {
		return fmt.Errorf("only CompareExchange has a failure order")
	}
	return nil
}

// validStorageWidth admits the IntegerStorage widths of layout.md § 47.1(2).
// Rules: rules/memory/layout.md — § 47.1(2).
func validStorageWidth(bits uint16) bool {
	switch bits {
	case 8, 16, 32, 64, 128, 256:
		return true
	}
	return false
}

// validate requires the semantic identity of a storage operation (none for a
// Fence) and source provenance alongside a valid key.
// Rules: rules/concurrency/atomics.md — §§ 37A(4), 37A(9).
func (r AtomicRequirement) validate() error {
	if err := r.Key.validate(); err != nil {
		return err
	}
	if (r.Key.Operation == "Fence") != (r.SemanticTypeID == "") {
		return fmt.Errorf("atomic %s semantic type identity must be present exactly for storage operations", r.Key.Operation)
	}
	if r.Provenance == "" {
		return fmt.Errorf("atomic requirement needs source provenance")
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

// WithAtomicCapabilities freezes producer facts for this exact resolved plan
// under the current key schema. A later change to any represented plan fact
// invalidates the binding instead of carrying support across targets,
// profiles, ABIs or reference policies. It declares no registry capabilities
// and never consults the compiler host.
// Without explicit profile policy, activation remains Unknown. Producers with
// activation evidence use WithAtomicCapabilityRules, including exact-only rules.
// Rules: rules/concurrency/atomics.md — §§37(1–5), 37A(10), 37A(14), 48(2);
// rules/platform/target_profiles.md — §§23,29–31.
func (p ResolvedScalarPlan) WithAtomicCapabilities(facts []AtomicCapability) (ResolvedScalarPlan, error) {
	return p.withAtomicCapabilities(AtomicProfilePolicy{}, facts)
}

// withAtomicCapabilities validates and detaches producer data before publishing
// the one authoritative snapshot. Activation belongs to the plan, not a key.
// Rules: rules/concurrency/atomics.md — § 37A(10)-(11), (14).
func (p ResolvedScalarPlan) withAtomicCapabilities(policy AtomicProfilePolicy, facts []AtomicCapability) (ResolvedScalarPlan, error) {
	if err := p.Validate(); err != nil {
		return ResolvedScalarPlan{}, err
	}
	if p.Profile == "" {
		return ResolvedScalarPlan{}, fmt.Errorf("atomic capabilities require a resolved target profile")
	}
	if err := policy.validate(); err != nil {
		return ResolvedScalarPlan{}, err
	}
	base := capabilityPlanBase(p)
	snapshot := &atomicCapabilitySnapshot{schema: atomicKeySchema, plan: base, policy: policy, facts: make(map[AtomicCapabilityKey]AtomicCapability, len(facts))}
	for _, fact := range facts {
		if fact.Activation != AtomicActivationUnknown || fact.ActivationProvenance != "" || fact.ActivationReason != "" {
			return ResolvedScalarPlan{}, fmt.Errorf("atomic activation must come from the plan policy, not an operation fact")
		}
		if err := fact.validate(); err != nil {
			return ResolvedScalarPlan{}, fmt.Errorf("target %s/%s profile %s: %w", p.TargetOS, p.TargetArch, p.Profile, err)
		}
		if _, duplicate := snapshot.facts[fact.Key]; duplicate {
			return ResolvedScalarPlan{}, fmt.Errorf("duplicate atomic capability for %+v", fact.Key)
		}
		snapshot.facts[fact.Key] = fact
	}
	p.Atomics = AtomicCapabilities{snapshot: snapshot}
	return p, nil
}

// validateFor refuses a snapshot copied into a different target/profile plan
// or built under another key schema. All scalar facts participate so future
// represented scalar fields cannot be silently omitted from this binding.
// Rules: rules/concurrency/atomics.md — §§ 37(1–5), 37A(14);
// rules/platform/target_profiles.md — §31 "Canonical identity".
func (c AtomicCapabilities) validateFor(p ResolvedScalarPlan) error {
	if c.snapshot == nil {
		return nil
	}
	if c.snapshot.schema != atomicKeySchema {
		return fmt.Errorf("atomic capability snapshot uses key schema %q, not %q; it must be migrated and revalidated", c.snapshot.schema, atomicKeySchema)
	}
	if !reflect.DeepEqual(capabilityPlanBase(p), c.snapshot.plan) {
		return fmt.Errorf("atomic capability snapshot belongs to a different resolved target/profile plan")
	}
	return nil
}

// atomicCapabilityMeaning is the fingerprinted meaning of one fact: its key,
// availability and implementation guarantees, without descriptive
// provenance or reason.
type atomicCapabilityMeaning struct {
	Key            AtomicCapabilityKey
	Availability   AtomicAvailability
	Implementation AtomicImplementation
}

// semanticFacts is the deterministic dependency meaning of the snapshot:
// the key schema and every non-Unknown fact in canonical order. A changed
// fact, implementation guarantee or key schema changes the plan
// fingerprints, so dependent proofs are not reused (T60, T61).
// Rules: rules/concurrency/atomics.md — § 37A(14);
// rules/platform/platform_model.md — §§ 21, 23.
func (c AtomicCapabilities) semanticFacts() []any {
	if c.snapshot == nil {
		return nil
	}
	type keyed struct {
		order   string
		meaning atomicCapabilityMeaning
	}
	var facts []keyed
	for key, fact := range c.snapshot.facts {
		if fact.Availability == AtomicUnknown {
			continue
		}
		encoded, _ := json.Marshal(key)
		facts = append(facts, keyed{string(encoded), atomicCapabilityMeaning{key, fact.Availability, fact.Implementation}})
	}
	sort.Slice(facts, func(i, j int) bool { return facts[i].order < facts[j].order })
	out := []any{c.snapshot.schema, c.snapshot.policy.Activation}
	for _, fact := range facts {
		out = append(out, fact.meaning)
	}
	return out
}

// AtomicCapability queries the selected frozen plan by the request's key.
// Unknown is conservative absence of evidence, including on registered
// hosted targets. The semantic type never selects an entry, and neither
// width, a similar carrier/order/context nor a stronger-order implementation
// is substituted. Producers expand their explicitly covered refinements before
// freezing; lookup never treats an empty refinement as a wildcard. Profile
// activation is returned independently, including when operation evidence is
// missing. Lowering owns any proven order strengthening.
// Rules: rules/concurrency/atomics.md — §§24(3), 37, 37A(6), 37A(9)-(11), 47(1–4);
// rules/platform/platform_model.md — §§ 9, 14.
func (p ResolvedScalarPlan) AtomicCapability(request AtomicRequirement) (AtomicCapability, error) {
	if err := p.Validate(); err != nil {
		return AtomicCapability{}, err
	}
	if err := request.validate(); err != nil {
		return AtomicCapability{}, err
	}
	result := AtomicCapability{Key: request.Key, Availability: AtomicUnknown}
	if p.Atomics.snapshot != nil {
		if fact, found := p.Atomics.snapshot.facts[request.Key]; found {
			result = fact
		}
		result.Activation = p.Atomics.snapshot.policy.Activation
		result.ActivationProvenance = p.Atomics.snapshot.policy.Provenance
		result.ActivationReason = p.Atomics.snapshot.policy.Reason
	}
	return result, nil
}

// Facts lists the frozen capability facts in canonical key order for
// inspection; the result is detached from the snapshot.
// Rules: rules/concurrency/atomics.md — § 37A(14); rules/compiler/compiler.md — § 64(3).
func (c AtomicCapabilities) Facts() []AtomicCapability {
	if c.snapshot == nil {
		return nil
	}
	type keyed struct {
		order string
		fact  AtomicCapability
	}
	var facts []keyed
	for key, fact := range c.snapshot.facts {
		encoded, _ := json.Marshal(key)
		facts = append(facts, keyed{string(encoded), fact})
	}
	sort.Slice(facts, func(i, j int) bool { return facts[i].order < facts[j].order })
	out := make([]AtomicCapability, len(facts))
	for i, f := range facts {
		out[i] = f.fact
	}
	return out
}

