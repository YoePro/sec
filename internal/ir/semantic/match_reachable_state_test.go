package semantic

import (
	"strings"
	"testing"
)

// MD-042: a match over an immutable union binding with a proven active
// variant carries the proven reachable subset and its provenance into the
// match record, separate from the declared domain and arm coverage; the
// proven-inactive arm keeps its lowered body and the residual stays the
// verbatim exhaustive-match-fallthrough.
// Rules: rules/compiler/semantic_ir.md — § 71(6);
// rules/control-flow/flowcontrol_match.md — §§ 5, 25A.
func TestMatchRecordsCarryReachableStateProof(t *testing.T) {
	module, err := analyzedModule(t, `module main
type Figure union { Dot Line(int) }
fn Constructed() int {
  let figure: Figure := Figure.Dot
  return match figure {
    Dot => 1
    Line(length) => length
  }
}
fn Unproven(figure: Figure) int {
  return match figure {
    Dot => 1
    Line(length) => length
  }
}`, 12)
	if err != nil {
		t.Fatal(err)
	}
	if err := Verify(module); err != nil {
		t.Fatal(err)
	}
	records := map[string]MatchRecord{}
	for _, function := range module.Functions {
		if len(function.Matches) != 1 {
			t.Fatalf("%s matches = %#v", function.Name, function.Matches)
		}
		records[function.Name] = function.Matches[0]
	}
	constructed := records["Constructed"]
	if proof := constructed.ProvenActive; proof == nil || proof.Variant != 0 || proof.Binding != "figure" || proof.Basis != "construction" || proof.Location.Line != 4 {
		t.Fatalf("constructed proof = %#v", constructed.ProvenActive)
	}
	if len(constructed.Arms) != 2 || constructed.Arms[0].ProvenInactive || !constructed.Arms[1].ProvenInactive {
		t.Fatalf("constructed arms = %#v", constructed.Arms)
	}
	if unproven := records["Unproven"]; unproven.ProvenActive != nil || unproven.Arms[0].ProvenInactive || unproven.Arms[1].ProvenInactive {
		t.Fatalf("unproven match = %#v", unproven)
	}
	if text := Format(module); !strings.Contains(text, `reason="exhaustive-match-fallthrough"`) {
		t.Fatal(text)
	}

	for _, mutate := range []func(*MatchRecord){
		func(record *MatchRecord) { record.Arms[0].ProvenInactive = true },
		func(record *MatchRecord) { record.ProvenActive.Basis = "guess" },
		func(record *MatchRecord) { record.ProvenActive = nil },
	} {
		function := functionNamed(t, module, "Constructed")
		original := function.Matches[0]
		changed := original
		changed.Arms = append([]MatchArmRecord(nil), original.Arms...)
		proof := *original.ProvenActive
		changed.ProvenActive = &proof
		mutate(&changed)
		function.Matches[0] = changed
		if err := Verify(module); err == nil || !strings.Contains(err.Error(), "invalid match") {
			t.Fatalf("mutated record verified: %v", err)
		}
		function.Matches[0] = original
	}
}
