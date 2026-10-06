package sema

import (
	"math/big"
	"os"
	"reflect"
	"testing"
)

// TestStackExternalContracts binds verified producer facts to real source graph
// sites. It does not claim an FFI import syntax or automatic frame producer.
// Rules: rules/analysis/stack_analysis.md — "Foreign, runtime, and platform calls",
// "CompilationPlan dependence", "Error and panic paths", and "Call-path composition".
func TestStackExternalContracts(t *testing.T) {
	source, err := os.ReadFile("../../testdata/sema/stack_external_contracts_valid.sec")
	if err != nil {
		t.Fatal(err)
	}
	analyzer, errs := analyzeSourceWithAnalyzerRaw(t, string(source))
	assertSemaErrors(t, errs, nil)
	graph := analyzer.CallGraph()
	ids := map[string]CallableID{}
	var frames StackSummaryStore
	for _, node := range graph.Nodes() {
		ids[node.Name] = node.ID
		if node.Extern {
			continue
		}
		semantic, _ := NewExactStackBound(big.NewInt(100))
		machine, _ := NewExactStackBound(big.NewInt(200))
		if err := frames.RecordSemantic(SemanticStackSummary{Callable: node.ID, CompilationPlanID: "plan", OwnFrame: semantic}); err != nil {
			t.Fatal(err)
		}
		if err := frames.RecordMachine(MachineStackSummary{Callable: node.ID, CompilationPlanID: "plan", OwnFrame: machine}); err != nil {
			t.Fatal(err)
		}
	}
	var contracts StackExternalContractStore
	guarantee := func(level StackMeasurementLevel, bytes int64) StackExternalGuarantee {
		bound, _ := NewExactStackBound(big.NewInt(bytes))
		return StackExternalGuarantee{MeasurementLevel: level, CompilationPlanID: "plan", Maximum: bound, NoReentry: true}
	}
	call := func(name string, index int, kind StackExternalKind, level StackMeasurementLevel, bytes int64) StackExternalCallContract {
		t.Helper()
		site := graph.Outgoing(ids[name])[index]
		return StackExternalCallContract{Site: site.ID, Caller: site.Caller, Target: site.Targets[0], Source: site.Source, Kind: kind, Guarantee: guarantee(level, bytes)}
	}
	recordCall := func(contract StackExternalCallContract) {
		t.Helper()
		if err := contracts.RecordCall(contract); err != nil {
			t.Fatal(err)
		}
	}
	compose := func() []SemanticStackSummary {
		return ComposeSemanticStackSummariesWithBoundaryContracts(graph, &frames, "plan", StackBoundaryContracts{External: &contracts})
	}
	get := func(results []SemanticStackSummary, name string) SemanticStackSummary {
		t.Helper()
		for _, summary := range results {
			if summary.Callable == ids[name] {
				return summary
			}
		}
		t.Fatal("missing summary", name)
		return SemanticStackSummary{}
	}
	check := func(name string, bytes int64) {
		t.Helper()
		result := get(compose(), name)
		count, finite := result.TransitiveMaximum.Bytes()
		if !finite || count.Cmp(big.NewInt(bytes)) != 0 || result.TransitiveMaximum.Kind() != StackBoundUpperBound || len(result.Evidence.UnknownCauses) != 0 {
			t.Fatal("incorrect composed contract", name, result)
		}
	}
	unknown := func(name string) {
		t.Helper()
		result := get(compose(), name)
		if result.TransitiveMaximum.Kind() != StackBoundUnknown || len(result.Evidence.UnknownCauses) == 0 || len(result.Evidence.Contributors) == 0 {
			t.Fatal("lost unknown boundary or known frames", name, result)
		}
	}
	unknown("ForeignOnce")
	beforeGraph, beforeFrames := graph.clone(), frames.SemanticSummaries()
	base := call("ForeignOnce", 0, StackExternalForeign, StackMeasurementSemantic, 700)
	recordCall(base)
	check("ForeignOnce", 800)
	check("Nested", 900)
	unknown("ForeignTwice")
	recordCall(call("ForeignTwice", 0, StackExternalForeign, StackMeasurementSemantic, 700))
	unknown("ForeignTwice")
	recordCall(call("ForeignTwice", 1, StackExternalForeign, StackMeasurementSemantic, 900))
	check("ForeignTwice", 1000)
	recordCall(call("PlatformCaller", 0, StackExternalPlatform, StackMeasurementSemantic, 300))
	recordCall(call("RuntimeCaller", 0, StackExternalRuntime, StackMeasurementSemantic, 400))
	check("PlatformCaller", 400)
	check("RuntimeCaller", 500)
	recordCall(call("Mixed", 0, StackExternalForeign, StackMeasurementSemantic, 700))
	unknown("Mixed") // whole-call foreign proof cannot cover independent arithmetic
	for _, name := range []string{"Arithmetic", "TwoEffects", "Mixed"} {
		count := 0
		for _, effect := range graph.effects[ids[name]] {
			if effect.Kind != EffectMayPanicArithmetic {
				continue
			}
			count++
			if err := contracts.RecordEffect(StackRuntimeEffectContract{Caller: ids[name], Kind: effect.Kind, Source: effect.Source, Guarantee: guarantee(StackMeasurementSemantic, int64(200*count))}); err != nil {
				t.Fatal(err)
			}
			if name == "TwoEffects" && count == 1 {
				unknown(name)
			}
		}
		if count == 0 {
			t.Fatal("fixture lacks runtime effects", name)
		}
		expected := int64(100 + 200*count)
		if name == "Mixed" {
			expected = 800
		}
		check(name, expected)
	}
	unknown("Opaque")
	// Runtime handler facts are scoped independently and cannot supply a foreign
	// body's normal contribution, or prove another level's handler requirement.
	for _, effect := range graph.effects[ids["Arithmetic"]] {
		if effect.Kind != EffectMayPanicArithmetic {
			continue
		}
		for _, noReentry := range []bool{false, true} {
			var isolated StackExternalContractStore
			fact := StackRuntimeEffectContract{Caller: ids["Arithmetic"], Kind: effect.Kind, Source: effect.Source, Guarantee: guarantee(StackMeasurementSemantic, 200)}
			fact.Guarantee.NoReentry = noReentry
			if noReentry {
				fact.Source.Column++
			}
			if err := isolated.RecordEffect(fact); err != nil {
				t.Fatal(err)
			}
			result := get(ComposeSemanticStackSummariesWithBoundaryContracts(graph, &frames, "plan", StackBoundaryContracts{External: &isolated}), "Arithmetic")
			if result.TransitiveMaximum.Kind() != StackBoundUnknown {
				t.Fatal("unsupported reentry or source accepted for runtime path", result)
			}
		}
	}
	var handlerOnly StackExternalContractStore
	for _, effect := range graph.effects[ids["ForeignOnce"]] {
		if effect.Kind == EffectMayPanicForeign {
			if err := handlerOnly.RecordEffect(StackRuntimeEffectContract{Caller: ids["ForeignOnce"], Kind: effect.Kind, Source: effect.Source, Guarantee: guarantee(StackMeasurementSemantic, 200)}); err != nil {
				t.Fatal(err)
			}
		}
	}
	if get(ComposeSemanticStackSummariesWithBoundaryContracts(graph, &frames, "plan", StackBoundaryContracts{External: &handlerOnly}), "ForeignOnce").TransitiveMaximum.Kind() != StackBoundUnknown {
		t.Fatal("runtime handler guarantee substituted for foreign whole-call bound")
	}
	machine, err := ComposeMachineStackSummariesWithBoundaryContracts(graph, &frames, "plan", StackBoundaryContracts{External: &contracts})
	if err != nil {
		t.Fatal(err)
	}
	for _, summary := range machine {
		if summary.Callable == ids["ForeignOnce"] && summary.TransitiveMaximum.Kind() != StackBoundUnknown {
			t.Fatal("semantic contract reused as machine proof")
		}
	}
	recordCall(call("ForeignOnce", 0, StackExternalForeign, StackMeasurementMachine, 1400))
	machine, err = ComposeMachineStackSummariesWithBoundaryContracts(graph, &frames, "plan", StackBoundaryContracts{External: &contracts})
	if err != nil {
		t.Fatal(err)
	}
	for _, summary := range machine {
		if summary.Callable == ids["Nested"] {
			bytes, finite := summary.TransitiveMaximum.Bytes()
			if !finite || bytes.Cmp(big.NewInt(1800)) != 0 {
				t.Fatal("machine contract not composed", summary)
			}
		}
	}
	if !reflect.DeepEqual(beforeGraph, graph.clone()) || !reflect.DeepEqual(beforeFrames, frames.SemanticSummaries()) {
		t.Fatal("consumer mutated graph, panic facts or frames")
	}
	for _, name := range []string{"ForeignOnce", "Nested"} {
		if get(ComposeSemanticStackSummariesWithBoundaryContracts(graph, &frames, "other-plan", StackBoundaryContracts{External: &contracts}), name).TransitiveMaximum.Kind() != StackBoundUnknown {
			t.Fatal("contract crossed plans")
		}
	}
	for _, change := range []struct {
		name   string
		mutate func(*StackExternalCallContract)
	}{
		{"reentry", func(c *StackExternalCallContract) { c.Guarantee.NoReentry = false }},
		{"unknown", func(c *StackExternalCallContract) { c.Guarantee.Maximum = UnknownStackBound() }},
		{"unbounded", func(c *StackExternalCallContract) { c.Guarantee.Maximum = UnboundedStackBound() }},
		{"caller", func(c *StackExternalCallContract) { c.Caller = ids["Nested"] }},
		{"target", func(c *StackExternalCallContract) { c.Target = ids["Platform"] }},
		{"source", func(c *StackExternalCallContract) { c.Source.Column++ }},
	} {
		t.Run(change.name, func(t *testing.T) {
			changed := base
			change.mutate(&changed)
			recordCall(changed)
			unknown("ForeignOnce")
		})
	}
	recordCall(base)
	for _, change := range []struct {
		name   string
		mutate func(*CallSite)
	}{
		{"execution", func(s *CallSite) { s.Execution = CallExecutionRelation("new-stack") }},
		{"open", func(s *CallSite) { s.TargetSet.IsClosed = false }},
		{"coverage", func(s *CallSite) { s.TargetSet.KnownTargets = nil }},
		{"dispatch", func(s *CallSite) { s.Dispatch = CallDispatchDirect }},
	} {
		t.Run(change.name, func(t *testing.T) {
			changed := graph.clone()
			for i := range changed.sites {
				if changed.sites[i].ID == base.Site {
					change.mutate(&changed.sites[i])
				}
			}
			if get(ComposeSemanticStackSummariesWithBoundaryContracts(changed, &frames, "plan", StackBoundaryContracts{External: &contracts}), "ForeignOnce").TransitiveMaximum.Kind() != StackBoundUnknown {
				t.Fatal("malformed boundary accepted")
			}
		})
	}
	zero := base
	zero.Guarantee.Maximum, _ = NewExactStackBound(big.NewInt(0))
	recordCall(zero)
	check("ForeignOnce", 100)
	huge := new(big.Int).Lsh(big.NewInt(1), 128)
	large := base
	large.Guarantee.Maximum, _ = NewUpperStackBound(huge)
	recordCall(large)
	huge.Add(huge, big.NewInt(100))
	bytes, finite := get(compose(), "ForeignOnce").TransitiveMaximum.Bytes()
	if !finite || bytes.Cmp(huge) != 0 {
		t.Fatal("arbitrary precision or immutable publication lost")
	}
	recordCall(base)
	reversed := graph.clone()
	for i, j := 0, len(reversed.sites)-1; i < j; i, j = i+1, j-1 {
		reversed.sites[i], reversed.sites[j] = reversed.sites[j], reversed.sites[i]
	}
	for id, effects := range reversed.effects {
		for i, j := 0, len(effects)-1; i < j; i, j = i+1, j-1 {
			effects[i], effects[j] = effects[j], effects[i]
		}
		reversed.effects[id] = effects
	}
	if !reflect.DeepEqual(compose(), ComposeSemanticStackSummariesWithBoundaryContracts(reversed, &frames, "plan", StackBoundaryContracts{External: &contracts})) {
		t.Fatal("registration order changed results")
	}
	invalid := base
	invalid.Guarantee.CompilationPlanID = ""
	if contracts.RecordCall(invalid) == nil {
		t.Fatal("unscoped external contract accepted")
	}
	invalid = base
	invalid.Kind = "implicit-name"
	if contracts.RecordCall(invalid) == nil {
		t.Fatal("unsupported external kind accepted")
	}
	var absent *StackExternalContractStore
	if absent.RecordCall(base) == nil {
		t.Fatal("nil store accepted publication")
	}
	effect := graph.effects[ids["Opaque"]][0]
	if contracts.RecordEffect(StackRuntimeEffectContract{Caller: ids["Opaque"], Kind: effect.Kind, Source: effect.Source, Guarantee: guarantee(StackMeasurementSemantic, 200)}) == nil {
		t.Fatal("opaque body bounded by runtime handler")
	}
}
