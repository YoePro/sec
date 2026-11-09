package sema

import (
	"os"
	"reflect"
	"testing"
)

// Ordinary memory events and canonical volatile events keep distinct operation
// identities. Raw address possession is not access or proof of a device domain.
// Rules: rules/memory/raw_pointers.md §31(1–6);
// rules/analysis/effect_analysis.md §§35–37.
func TestRawMemoryEffects(t *testing.T) {
	data, err := os.ReadFile("../../testdata/sema/raw_memory_effects.sec")
	if err != nil {
		t.Fatal(err)
	}
	a, errors := analyzeSourceWithAnalyzerRaw(t, string(data))
	assertSemaErrors(t, errors, nil)
	graph := a.CallGraph()
	for _, tc := range []struct {
		name    string
		events  []EffectKind
		domains []SummaryEffect
	}{
		{"Ordinary", []EffectKind{EffectMemoryWrite, EffectMemoryRead}, []SummaryEffect{SummaryMayMutateExternalState}},
		{"Volatile", []EffectKind{EffectVolatileWrite, EffectVolatileRead}, []SummaryEffect{SummaryMayAccessVolatile}},
		{"ReadOnly", []EffectKind{EffectMemoryRead}, nil},
		{"Possess", nil, nil},
		{"Indirect", nil, []SummaryEffect{SummaryMayMutateExternalState}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			id := callGraphNodeIDByName(t, graph, tc.name)
			var events []EffectKind
			for _, event := range graph.EffectSummary(id).DirectEffects {
				events = append(events, event.Kind)
				if event.Source.Line == 0 {
					t.Fatal("effect lost source")
				}
			}
			summary := graph.DomainSummary(id)
			if !reflect.DeepEqual(events, tc.events) || !reflect.DeepEqual(summary.Effects, tc.domains) || len(summary.UnknownEffects) != 0 {
				t.Fatalf("events=%v summary=%+v", events, summary)
			}
			if tc.name == "Indirect" {
				if len(summary.Causes) != 1 || len(summary.Causes[0].Path) != 1 || summary.Causes[0].Effect == nil || summary.Causes[0].Effect.Kind != EffectMemoryWrite {
					t.Fatalf("transitive write lost provenance: %+v", summary)
				}
			}
		})
	}
}

// Rejected accesses must not publish successful memory-access facts.
// Rules: rules/memory/raw_pointers.md §§3(4),11–12,31;
// rules/memory/unsafe.md — explicit unsafe authorization.
func TestRejectedRawMemoryEffects(t *testing.T) {
	data, err := os.ReadFile("../../testdata/sema/raw_memory_effects_invalid.sec")
	if err != nil {
		t.Fatal(err)
	}
	a, errors := analyzeSourceWithAnalyzerRaw(t, string(data))
	if len(errors) == 0 {
		t.Fatal("invalid fixture accepted")
	}
	for _, name := range []string{"MissingUnsafe", "WrongValue", "Untyped"} {
		id := callGraphNodeIDByName(t, a.CallGraph(), name)
		if events := a.CallGraph().EffectSummary(id).DirectEffects; len(events) != 0 {
			t.Fatalf("%s published rejected access: %+v", name, events)
		}
	}
}
