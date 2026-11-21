package layout

import (
	"encoding/json"
	"fmt"
	"reflect"
	"sort"
	"strings"
)

// MutexAcquisition selects a complete source acquisition contract, not an OS API.
// Rules: rules/concurrency/mutex.md §§20–24,27,65.
type MutexAcquisition string

const (
	MutexBlocking MutexAcquisition = "blocking"
	MutexTry      MutexAcquisition = "try"
	MutexTimeout  MutexAcquisition = "timeout"
	MutexContext  MutexAcquisition = "context"
)

// MutexAvailability separates absent evidence from a proven platform denial.
// Rules: rules/concurrency/mutex.md §§27(6),63; rules/platform/platform_model.md §9.
type MutexAvailability string

const (
	MutexUnknown     MutexAvailability = "unknown"
	MutexSupported   MutexAvailability = "supported"
	MutexUnsupported MutexAvailability = "unsupported"
)

// MutexCapability is trusted evidence for a conforming acquisition contract,
// including its guard release semantics; it does not implement that contract
// or prove additional interrupt/nonblocking context permission (§51).
// Rules: rules/concurrency/mutex.md §§27,51,60,63,65.
type MutexCapability struct {
	Acquisition  MutexAcquisition
	Availability MutexAvailability
	Provenance   string
	Reason       string
}

// MutexCapabilities privately freezes facts against one selected target/profile.
// Ordinary atomics and IPCMutex support do not prove an ordinary Mutex contract.
// Rules: rules/concurrency/mutex.md §§27,60; rules/platform/target_profiles.md §§29–31.
type MutexCapabilities struct{ snapshot *mutexCapabilitySnapshot }
type mutexCapabilitySnapshot struct {
	plan  ResolvedScalarPlan
	facts map[MutexAcquisition]MutexCapability
}

// validateMutexAcquisition rejects unknown acquisition contracts.
// Rules: rules/concurrency/mutex.md §§20–24.
func validateMutexAcquisition(mode MutexAcquisition) error {
	switch mode {
	case MutexBlocking, MutexTry, MutexTimeout, MutexContext:
		return nil
	}
	return fmt.Errorf("unknown mutex acquisition %q", mode)
}

// WithMutexCapabilities validates and freezes producer evidence without deriving
// support from host, architecture, profile family, atomics or IPC facilities.
// Rules: rules/concurrency/mutex.md §§27(6),60,63; rules/platform/target_profiles.md §31.
func (p ResolvedScalarPlan) WithMutexCapabilities(facts []MutexCapability) (ResolvedScalarPlan, error) {
	if err := p.Validate(); err != nil {
		return ResolvedScalarPlan{}, err
	}
	if p.Profile == "" {
		return ResolvedScalarPlan{}, fmt.Errorf("mutex capabilities require a resolved target profile")
	}
	snapshot := &mutexCapabilitySnapshot{plan: capabilityPlanBase(p), facts: map[MutexAcquisition]MutexCapability{}}
	for _, fact := range facts {
		if err := validateMutexAcquisition(fact.Acquisition); err != nil {
			return ResolvedScalarPlan{}, err
		}
		switch fact.Availability {
		case MutexUnknown:
		case MutexSupported, MutexUnsupported:
			if strings.TrimSpace(fact.Provenance) == "" {
				return ResolvedScalarPlan{}, fmt.Errorf("known mutex capability requires contract provenance")
			}
			if fact.Availability == MutexUnsupported && strings.TrimSpace(fact.Reason) == "" {
				return ResolvedScalarPlan{}, fmt.Errorf("unsupported mutex capability requires a reason")
			}
		default:
			return ResolvedScalarPlan{}, fmt.Errorf("invalid mutex availability %q", fact.Availability)
		}
		if _, exists := snapshot.facts[fact.Acquisition]; exists {
			return ResolvedScalarPlan{}, fmt.Errorf("duplicate mutex capability for %s", fact.Acquisition)
		}
		snapshot.facts[fact.Acquisition] = fact
	}
	p.Mutex = MutexCapabilities{snapshot: snapshot}
	return p, nil
}

// validateFor rejects evidence reused after a target/profile/representation change.
// Rules: rules/platform/target_profiles.md §31; rules/concurrency/mutex.md §60.
func (c MutexCapabilities) validateFor(p ResolvedScalarPlan) error {
	if c.snapshot != nil && !reflect.DeepEqual(capabilityPlanBase(p), c.snapshot.plan) {
		return fmt.Errorf("mutex capability snapshot belongs to a different resolved target/profile plan")
	}
	return nil
}

// MutexCapability returns exact selected-plan evidence; missing modes stay Unknown.
// Rules: rules/concurrency/mutex.md §§20–24,27,60,63.
func (p ResolvedScalarPlan) MutexCapability(mode MutexAcquisition) (MutexCapability, error) {
	if err := p.Validate(); err != nil {
		return MutexCapability{}, err
	}
	if err := validateMutexAcquisition(mode); err != nil {
		return MutexCapability{}, err
	}
	if p.Mutex.snapshot != nil {
		if fact, ok := p.Mutex.snapshot.facts[mode]; ok {
			return fact, nil
		}
	}
	return MutexCapability{Acquisition: mode, Availability: MutexUnknown}, nil
}

// Facts returns detached deterministic inspection evidence.
// Rules: rules/platform/platform_model.md §23; rules/concurrency/mutex.md §63.
func (c MutexCapabilities) Facts() []MutexCapability {
	var out []MutexCapability
	if c.snapshot != nil {
		for _, f := range c.snapshot.facts {
			out = append(out, f)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Acquisition < out[j].Acquisition })
	return out
}

// MarshalJSON exposes evidence without permitting unchecked restoration of trust.
// Rules: rules/platform/platform_model.md §23.
func (c MutexCapabilities) MarshalJSON() ([]byte, error) { return json.Marshal(c.Facts()) }

// semanticFacts excludes descriptive evidence and explicit Unknown from reuse keys.
// Rules: rules/platform/platform_model.md §23; rules/concurrency/mutex.md §60.
func (c MutexCapabilities) semanticFacts() []struct {
	Mode         MutexAcquisition
	Availability MutexAvailability
} {
	var out []struct {
		Mode         MutexAcquisition
		Availability MutexAvailability
	}
	for _, f := range c.Facts() {
		if f.Availability != MutexUnknown {
			out = append(out, struct {
				Mode         MutexAcquisition
				Availability MutexAvailability
			}{f.Acquisition, f.Availability})
		}
	}
	return out
}
