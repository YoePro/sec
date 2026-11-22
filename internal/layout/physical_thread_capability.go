package layout

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
)

// PhysicalThreadAvailability is target evidence, independent of logical tasks,
// ordinary scalar layout, runtime creation exhaustion and compiler-host support.
// Rules: rules/concurrency/cancellation.md §§11,24; rules/platform/platform_model.md §9.
type PhysicalThreadAvailability string

const (
	PhysicalThreadUnknown     PhysicalThreadAvailability = "unknown"
	PhysicalThreadSupported   PhysicalThreadAvailability = "supported"
	PhysicalThreadUnsupported PhysicalThreadAvailability = "unsupported"
)

// PhysicalThreadCapability is a trusted producer's physical execution identity
// contract. It does not establish thread creation, scheduling or lowering support.
// Rules: rules/concurrency/cancellation.md §§11,24; rules/concurrency/threads.md §§47–49.
type PhysicalThreadCapability struct {
	Availability PhysicalThreadAvailability
	Provenance   string
	Reason       string
}

// PhysicalThreadCapabilities freezes identity evidence against one selected
// target/profile. Its zero value is Unknown, never an assumed host capability.
// Rules: rules/platform/target_profiles.md §31; rules/concurrency/cancellation.md §24.
type PhysicalThreadCapabilities struct {
	snapshot *physicalThreadCapabilitySnapshot
}
type physicalThreadCapabilitySnapshot struct {
	plan ResolvedScalarPlan
	fact PhysicalThreadCapability
}

// Defined reports whether a producer supplied a bound snapshot, including an
// explicit Unknown fact that must still pass selected-plan validation.
// Rules: rules/platform/target_profiles.md §31; rules/concurrency/cancellation.md §24.
func (c PhysicalThreadCapabilities) Defined() bool { return c.snapshot != nil }

// WithPhysicalThreadCapability validates/fixes producer evidence to a complete
// selected plan; untrusted serialized data cannot populate the private snapshot.
// Rules: rules/concurrency/cancellation.md §§11,24; rules/platform/target_profiles.md §31.
func (p ResolvedScalarPlan) WithPhysicalThreadCapability(fact PhysicalThreadCapability) (ResolvedScalarPlan, error) {
	if err := p.Validate(); err != nil {
		return ResolvedScalarPlan{}, err
	}
	if p.Profile == "" {
		return ResolvedScalarPlan{}, fmt.Errorf("physical-thread capability requires a resolved target profile")
	}
	switch fact.Availability {
	case PhysicalThreadUnknown:
	case PhysicalThreadSupported, PhysicalThreadUnsupported:
		if strings.TrimSpace(fact.Provenance) == "" {
			return ResolvedScalarPlan{}, fmt.Errorf("known physical-thread capability requires contract provenance")
		}
		if fact.Availability == PhysicalThreadUnsupported && strings.TrimSpace(fact.Reason) == "" {
			return ResolvedScalarPlan{}, fmt.Errorf("unsupported physical-thread capability requires a reason")
		}
	default:
		return ResolvedScalarPlan{}, fmt.Errorf("invalid physical-thread availability %q", fact.Availability)
	}
	p.PhysicalThreads = PhysicalThreadCapabilities{snapshot: &physicalThreadCapabilitySnapshot{plan: capabilityPlanBase(p), fact: fact}}
	return p, nil
}

// validateFor rejects reuse after any selected target/profile/layout change.
// Rules: rules/platform/target_profiles.md §31; rules/concurrency/cancellation.md §24.
func (c PhysicalThreadCapabilities) validateFor(p ResolvedScalarPlan) error {
	if c.snapshot != nil && !reflect.DeepEqual(capabilityPlanBase(p), c.snapshot.plan) {
		return fmt.Errorf("physical-thread capability snapshot belongs to a different resolved target/profile plan")
	}
	return nil
}

// PhysicalThreadCapability returns exact selected-plan evidence without host inference.
// Rules: rules/concurrency/cancellation.md §§11,24; rules/platform/platform_model.md §9.
func (p ResolvedScalarPlan) PhysicalThreadCapability() (PhysicalThreadCapability, error) {
	if err := p.Validate(); err != nil {
		return PhysicalThreadCapability{}, err
	}
	return p.PhysicalThreads.Fact(), nil
}

// Fact exposes an immutable value copy. Missing facts stay Unknown and never
// become proof that a current physical-thread implementation exists.
// Rules: rules/concurrency/cancellation.md §24; rules/platform/platform_model.md §23.
func (c PhysicalThreadCapabilities) Fact() PhysicalThreadCapability {
	if c.snapshot != nil {
		return c.snapshot.fact
	}
	return PhysicalThreadCapability{Availability: PhysicalThreadUnknown}
}

// MarshalJSON projects inspectable evidence, without a trusted unmarshal path.
// Rules: rules/platform/platform_model.md §23.
func (c PhysicalThreadCapabilities) MarshalJSON() ([]byte, error) { return json.Marshal(c.Fact()) }

// semanticFact excludes descriptive provenance from semantic reuse identities.
// Rules: rules/platform/platform_model.md §23; rules/concurrency/cancellation.md §24.
func (c PhysicalThreadCapabilities) semanticFact() PhysicalThreadAvailability {
	return c.Fact().Availability
}
