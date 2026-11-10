package sema

import (
	"os"
	"testing"
)

// TestPropertyEffects checks inferred accessor effects and the exact operations
// selected by reads, writes, compound updates, static and nested access.
// Rules: rules/declarations/properties.md — §§4–7,14;
// rules/analysis/effect_analysis.md — §§10,15–17.
func TestPropertyEffects(t *testing.T) {
	data, err := os.ReadFile("../../testdata/sema/property_effects/valid.sec")
	if err != nil {
		t.Fatal(err)
	}
	a, errors := analyzeSourceWithAnalyzerRaw(t, string(data))
	assertSemaErrors(t, errors, nil)
	graph := a.CallGraph()
	for name, want := range map[string]bool{
		"Read": true, "Store": false, "ReadSafe": false, "StoreRisk": true,
		"Update": true, "StaticRead": true, "DeferredRead": true, "NestedRead": true,
		"CheckedStore": false, "ForeignRead": true,
	} {
		id := callGraphNodeIDByName(t, graph, name)
		if summary := graph.EffectSummary(id); summary.MayPanic != want {
			t.Fatalf("%s summary = %+v, want panic %v", name, summary, want)
		}
	}
	for name, targets := range map[string][]string{
		"Read": {"ReadRisk.get"}, "Store": {"ReadRisk.set"},
		"ReadSafe": {"WriteRisk.get"}, "StoreRisk": {"WriteRisk.set"},
		"Update":     {"WriteRisk.get", "WriteRisk.set"},
		"StaticRead": {"StaticRisk.get"}, "CheckedStore": {"Checked.try-set"},
	} {
		sites := graph.Outgoing(callGraphNodeIDByName(t, graph, name))
		if len(sites) != len(targets) {
			t.Fatalf("%s: %+v", name, sites)
		}
		seen := map[string]bool{}
		ids := map[CallSiteID]bool{}
		for _, site := range sites {
			if site.Dispatch != CallDispatchGenerated || site.Execution != CallExecutionSynchronous || len(site.Targets) != 1 || ids[site.ID] {
				t.Fatalf("invalid accessor site: %+v", site)
			}
			ids[site.ID] = true
			node, _ := graph.Node(site.Targets[0])
			seen[node.Name] = true
		}
		for _, target := range targets {
			if !seen[target] {
				t.Fatalf("%s lacks %s: %v", name, target, seen)
			}
		}
	}
	// Even an unused accessor gets its own inferred summary.
	if summary := graph.EffectSummary(callGraphNodeIDByName(t, graph, "Checked.get")); summary.MayPanic {
		t.Fatal(summary)
	}
	// A foreign effect reached through source property syntax must preserve
	// conservative allocation/blocking facts as well as its panic cause.
	foreign := callGraphNodeIDByName(t, graph, "ForeignRead")
	if !graph.ArenaSummary(foreign).AllocationUnknown || !graph.BlockSummary(foreign).MayBlock {
		t.Fatal("property lost foreign effect uncertainty", graph.DomainSummary(foreign))
	}

}

// TestPropertyEffectsNoPanic checks that implicit accessor invocation is visible
// to the same guarantee verifier as an ordinary explicit call.
// Rules: rules/declarations/properties.md — §14; rules/errors/panic.md — §21.
func TestPropertyEffectsNoPanic(t *testing.T) {
	data, err := os.ReadFile("../../testdata/sema/property_effects/callers_invalid.sec")
	if err != nil {
		t.Fatal(err)
	}
	errors := analyzeSourceRaw(t, string(data))
	if len(errors) != 3 {
		t.Fatalf("want three caller guarantee failures: %v", errors)
	}
	for _, diagnostic := range errors {
		if diagnostic.ID != "S1092" || diagnostic.PreviousLine == 0 {
			t.Fatalf("missing panic provenance: %+v", diagnostic)
		}
	}
}

// TestPropertyEffectsUnreachable keeps impossible property calls out of effect
// propagation while retaining the owning reachability diagnostic.
// Rules: rules/analysis/effect_analysis.md — §19; rules/control-flow/flowcontrol_if.md — reachability.
func TestPropertyEffectsUnreachable(t *testing.T) {
	data, err := os.ReadFile("../../testdata/sema/property_effects/unreachable_invalid.sec")
	if err != nil {
		t.Fatal(err)
	}
	a, errors := analyzeSourceWithAnalyzerRaw(t, string(data))
	if len(errors) != 1 || errors[0].ID != "S3001" {
		t.Fatal(errors)
	}
	g := a.CallGraph()
	if summary := g.EffectSummary(callGraphNodeIDByName(t, g, "Unreachable")); summary.MayPanic {
		t.Fatal(summary)
	}
}
