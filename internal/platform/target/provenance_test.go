package target

import (
	"testing"
)

// Plan resolution is deterministic and explains itself: the same request
// yields the same fingerprints, the provenance names where each fact came
// from without changing any fingerprint, and each domain fingerprint
// changes exactly with the facts of its domain.
// Rules: rules/platform/platform_model.md — §§ 20, 23, 27.
func TestPlanResolutionFingerprintsAndProvenance(t *testing.T) {
	linux := Target{OS: "linux", Arch: "amd64"}
	explicit, err := Resolve(linux, "command-line --target")
	if err != nil {
		t.Fatal(err)
	}
	for attempt := 0; attempt < 5; attempt++ {
		again, err := Resolve(linux, "host default")
		if err != nil {
			t.Fatal(err)
		}
		if again.Fingerprints != explicit.Fingerprints {
			t.Fatalf("fingerprints depend on provenance or attempt: %+v vs %+v", again.Fingerprints, explicit.Fingerprints)
		}
		if again.Provenance[0].Source != "host default" {
			t.Errorf("provenance = %+v", again.Provenance)
		}
	}
	if len(explicit.Provenance) != 7 || explicit.Provenance[0] != (FactProvenance{Fact: "os", Value: "linux", Source: "command-line --target"}) ||
		explicit.Provenance[2].Source != "target registry linux-amd64" {
		t.Errorf("provenance = %+v", explicit.Provenance)
	}
	arm, err := Resolve(Target{OS: "linux", Arch: "arm64"}, "command-line --target")
	if err != nil {
		t.Fatal(err)
	}
	if arm.Fingerprints.Architecture == explicit.Fingerprints.Architecture || arm.Fingerprints.Link == explicit.Fingerprints.Link ||
		arm.Fingerprints.Plan == explicit.Fingerprints.Plan || arm.Fingerprints.Runtime != explicit.Fingerprints.Runtime {
		t.Errorf("arm64 vs amd64 fingerprints: %+v vs %+v", arm.Fingerprints, explicit.Fingerprints)
	}
	if _, err := Resolve(Target{OS: "plan9", Arch: "mips"}, "manifest target shop"); err == nil {
		t.Error("an unknown target resolved")
	}
}
