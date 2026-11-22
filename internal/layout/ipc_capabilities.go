package layout

import (
	"encoding/json"
	"fmt"
	"reflect"
	"sort"
	"strings"
)

// IPCFamily identifies one complete semantic capability, never a host API or
// a universal IPC flag. Composite abstractions derive their own requirements.
// Rules: rules/concurrency/ipc.md §§118–121,124.
type IPCFamily string

const (
	IPCPipes                IPCFamily = "IPCPipes"
	IPCTypedMessages        IPCFamily = "IPCTypedMessages"
	IPCSharedMemory         IPCFamily = "IPCSharedMemory"
	IPCMutex                IPCFamily = "IPCMutex"
	IPCSemaphore            IPCFamily = "IPCSemaphore"
	IPCAtomicRepresentation IPCFamily = "IPCAtomicRepresentation"
)

// IPCRequirement selects one family, or one exact atomic representation width.
// Width zero is invalid for IPCAtomicRepresentation; other families have no
// width qualifier. No neighbouring width or pointer-size fallback is implied.
// Rules: rules/concurrency/ipc.md §§7.3,118,121–123.
type IPCRequirement struct {
	Family    IPCFamily
	WidthBits uint16
}

// IPCAvailability is independent of ordinary atomics and of runtime failure.
// Unknown is missing evidence, not static Unsupported and never Supported.
// Rules: rules/concurrency/ipc.md §§118,122–124.
type IPCAvailability string

const (
	IPCUnknown     IPCAvailability = "unknown"
	IPCSupported   IPCAvailability = "supported"
	IPCUnsupported IPCAvailability = "unsupported"
)

// IPCCapability is a trusted target producer's complete-family assertion.
// Supported covers the entire canonical family, including lock-free process-
// shared operations for IPCAtomicRepresentation, not merely a native function.
// Provenance identifies that contract's evidence; this model does not establish
// conformance itself or advertise any registered target as supporting IPC.
// Rules: rules/concurrency/ipc.md §§118,122–124; rules/platform/target_profiles.md §§29–31.
type IPCCapability struct {
	Requirement  IPCRequirement
	Availability IPCAvailability
	Provenance   string
	Reason       string
}

// IPCCapabilities privately freezes facts against one target/profile scalar
// contract. Its zero value has no capability evidence. Copies safely share the
// frozen table; mutable producer slices and query results never expose it.
// Rules: rules/concurrency/ipc.md §118; rules/platform/target_profiles.md §§29–31.
type IPCCapabilities struct{ snapshot *ipcCapabilitySnapshot }

type ipcCapabilitySnapshot struct {
	plan  ResolvedScalarPlan
	facts map[IPCRequirement]IPCCapability
}

// validate enforces the canonical vocabulary without deriving type eligibility
// or accepting a redundant SharedValue/IPC-wide support flag.
// Rules: rules/concurrency/ipc.md §§118,121.
func (r IPCRequirement) validate() error {
	switch r.Family {
	case IPCPipes, IPCTypedMessages, IPCSharedMemory, IPCMutex, IPCSemaphore:
		if r.WidthBits != 0 {
			return fmt.Errorf("%s has no width qualifier", r.Family)
		}
	case IPCAtomicRepresentation:
		if r.WidthBits == 0 {
			return fmt.Errorf("IPCAtomicRepresentation requires an exact nonzero width")
		}
	default:
		return fmt.Errorf("unknown IPC capability family %q", r.Family)
	}
	return nil
}

// validate rejects malformed/unsupported evidence states at the freeze boundary.
// A known lack of support must retain an explanation for static diagnostics.
// Rules: rules/concurrency/ipc.md §§122–124,139.
func (f IPCCapability) validate() error {
	if err := f.Requirement.validate(); err != nil {
		return err
	}
	switch f.Availability {
	case IPCUnknown:
		return nil
	case IPCSupported, IPCUnsupported:
		if strings.TrimSpace(f.Provenance) == "" {
			return fmt.Errorf("known IPC capability requires contract provenance")
		}
		if f.Availability == IPCUnsupported && strings.TrimSpace(f.Reason) == "" {
			return fmt.Errorf("unsupported IPC capability requires a reason")
		}
	default:
		return fmt.Errorf("invalid IPC availability %q", f.Availability)
	}
	return nil
}

// WithIPCCapabilities freezes explicit producer facts, refusing duplicates and
// foreign plans before publishing a table. No target/provider is inferred from
// OS, architecture, host, allocation profile or ordinary atomic availability.
// Rules: rules/concurrency/ipc.md §§118,122–124.
func (p ResolvedScalarPlan) WithIPCCapabilities(facts []IPCCapability) (ResolvedScalarPlan, error) {
	if err := p.Validate(); err != nil {
		return ResolvedScalarPlan{}, err
	}
	if p.Profile == "" {
		return ResolvedScalarPlan{}, fmt.Errorf("IPC capabilities require a resolved target profile")
	}
	snapshot := &ipcCapabilitySnapshot{plan: capabilityPlanBase(p), facts: make(map[IPCRequirement]IPCCapability, len(facts))}
	for _, fact := range facts {
		if err := fact.validate(); err != nil {
			return ResolvedScalarPlan{}, err
		}
		if _, duplicate := snapshot.facts[fact.Requirement]; duplicate {
			return ResolvedScalarPlan{}, fmt.Errorf("duplicate IPC capability for %+v", fact.Requirement)
		}
		snapshot.facts[fact.Requirement] = fact
	}
	p.IPC = IPCCapabilities{snapshot: snapshot}
	return p, nil
}

// validateFor rejects reuse after any scalar/profile/address-domain change.
// Other capability tables remain independently bound, not sources of IPC proof.
// Rules: rules/concurrency/ipc.md §118; rules/platform/target_profiles.md §31.
func (c IPCCapabilities) validateFor(p ResolvedScalarPlan) error {
	if c.snapshot != nil && !reflect.DeepEqual(capabilityPlanBase(p), c.snapshot.plan) {
		return fmt.Errorf("IPC capability snapshot belongs to a different resolved target/profile plan")
	}
	return nil
}

// IPCCapability looks up the exact semantic requirement in the selected plan.
// A missing family or width is Unknown even on a registered hosted target.
// Rules: rules/concurrency/ipc.md §§118,121–123.
func (p ResolvedScalarPlan) IPCCapability(request IPCRequirement) (IPCCapability, error) {
	if err := p.Validate(); err != nil {
		return IPCCapability{}, err
	}
	if err := request.validate(); err != nil {
		return IPCCapability{}, err
	}
	if p.IPC.snapshot != nil {
		if fact, ok := p.IPC.snapshot.facts[request]; ok {
			return fact, nil
		}
	}
	return IPCCapability{Requirement: request, Availability: IPCUnknown}, nil
}

// Facts returns detached evidence in canonical family/width order for target
// inspection and diagnostics, independent of map or producer registration order.
// Rules: rules/concurrency/ipc.md §§118,139; rules/platform/platform_model.md §23.
func (c IPCCapabilities) Facts() []IPCCapability {
	var facts []IPCCapability
	if c.snapshot != nil {
		for _, fact := range c.snapshot.facts {
			facts = append(facts, fact)
		}
	}
	sort.Slice(facts, func(i, j int) bool {
		l, r := facts[i].Requirement, facts[j].Requirement
		if l.Family != r.Family {
			return l.Family < r.Family
		}
		return l.WidthBits < r.WidthBits
	})
	return facts
}

// MarshalJSON preserves explicit capability evidence for plan inspection,
// rather than serializing an unexported snapshot as an empty object. Restoring
// trusted facts still requires the validating, target-bound freeze API.
// Rules: rules/concurrency/ipc.md §139; rules/platform/platform_model.md §23.
func (c IPCCapabilities) MarshalJSON() ([]byte, error) { return json.Marshal(c.Facts()) }

type ipcCapabilityMeaning struct {
	Requirement  IPCRequirement
	Availability IPCAvailability
}

// semanticFacts excludes descriptive evidence/provenance and explicit Unknown
// entries from dependency fingerprints; only resolved meaning affects reuse.
// Rules: rules/platform/platform_model.md §23; rules/concurrency/ipc.md §118.
func (c IPCCapabilities) semanticFacts() []ipcCapabilityMeaning {
	var facts []ipcCapabilityMeaning
	for _, f := range c.Facts() {
		if f.Availability != IPCUnknown {
			facts = append(facts, ipcCapabilityMeaning{f.Requirement, f.Availability})
		}
	}
	return facts
}
