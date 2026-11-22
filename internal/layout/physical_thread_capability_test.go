package layout

import (
	"encoding/json"
	"strings"
	"testing"
)

// TestPhysicalThreadCapability freezes exact selected-target identity evidence;
// provenance changes do not alter semantic reuse, while capability changes do.
// Rules: rules/concurrency/cancellation.md §§11,24; rules/platform/platform_model.md §23.
func TestPhysicalThreadCapability(t *testing.T) {
	base := atomicTestPlan()
	before := base.Fingerprints()
	for _, state := range []PhysicalThreadAvailability{PhysicalThreadUnknown, PhysicalThreadSupported, PhysicalThreadUnsupported} {
		input := PhysicalThreadCapability{Availability: state, Provenance: "test contract", Reason: "no physical execution identity"}
		plan, err := base.WithPhysicalThreadCapability(input)
		if err != nil {
			t.Fatal(err)
		}
		input.Reason = "mutated"
		got, err := plan.PhysicalThreadCapability()
		if err != nil || got.Availability != state || got.Reason == "mutated" {
			t.Fatal(got, err)
		}
		data, err := json.Marshal(plan.PhysicalThreads)
		if err != nil || !strings.Contains(string(data), "test contract") {
			t.Fatal(string(data), err)
		}
		changed := plan
		changed.Profile = "other"
		if _, err := changed.PhysicalThreadCapability(); err == nil {
			t.Fatal("evidence crossed target/profile boundary")
		}
		fp := plan.Fingerprints()
		if state == PhysicalThreadUnknown {
			if fp != before {
				t.Fatal("explicit unknown changed reuse identity")
			}
		} else if fp.Memory == before.Memory || fp.Runtime == before.Runtime || fp.Execution == before.Execution || fp.Layout != before.Layout || fp.ABI != before.ABI {
			t.Fatal("wrong dependency domains")
		}
		fact := plan.PhysicalThreads.Fact()
		fact.Provenance = "different evidence"
		other, err := base.WithPhysicalThreadCapability(fact)
		if err != nil || other.Fingerprints() != fp {
			t.Fatal("display evidence changed semantic reuse", err)
		}
	}
	for _, bad := range []PhysicalThreadCapability{{Availability: "probably"}, {Availability: PhysicalThreadSupported}, {Availability: PhysicalThreadUnsupported, Provenance: "p"}} {
		if _, err := base.WithPhysicalThreadCapability(bad); err == nil {
			t.Fatal("accepted invalid capability", bad)
		}
	}
	unknown, err := base.PhysicalThreadCapability()
	if err != nil || unknown.Availability != PhysicalThreadUnknown {
		t.Fatal("unproven host support", unknown, err)
	}
	first, err := base.WithMutexCapabilities([]MutexCapability{{Acquisition: MutexTry, Availability: MutexSupported, Provenance: "mutex contract"}})
	if err != nil {
		t.Fatal(err)
	}
	combined, err := first.WithPhysicalThreadCapability(PhysicalThreadCapability{Availability: PhysicalThreadUnsupported, Provenance: "thread contract", Reason: "no physical identity"})
	if err != nil || combined.Validate() != nil {
		t.Fatal("independent capability tables nested evidence", err)
	}
	mutex, err := combined.MutexCapability(MutexTry)
	if err != nil || mutex.Availability != MutexSupported {
		t.Fatal(mutex, err)
	}
}
