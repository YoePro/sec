package sema

import (
	"bytes"
	"encoding/json"
	"math/big"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"sec/internal/lexer"
)

// stackArchiveInputs supplies independent proof identities at both levels.
// Rules: rules/analysis/stack_analysis.md — "Summary versioning".
func stackArchiveInputs(t *testing.T) (*StackSummaryStore, map[StackSummaryScope]StackSummaryDependencies) {
	t.Helper()
	store := &StackSummaryStore{}
	exact, _ := NewExactStackBound(big.NewInt(1536))
	upper, _ := NewUpperStackBound(big.NewInt(8192))
	zero, _ := NewExactStackBound(new(big.Int))
	wide, _ := new(big.Int).SetString("184467440737095516170000000000", 10)
	large, _ := NewExactStackBound(wide)
	source := lexer.Token{File: "lib/decode.sec", Line: 4, Column: 2}
	boundary := StackCauseStep{Callable: "opaque", Source: source, Detail: "unknown callee"}
	frame := StackFrameContribution{Callable: "decode", Source: source, Frame: exact}
	evidence := StackEvidence{KnownPrefix: []StackFrameContribution{frame}, Contributors: []StackFrameContribution{frame}, UnknownCauses: []StackCauseStep{boundary},
		UnknownPaths: []StackUnknownPath{{Frames: []StackFrameContribution{frame, {Callable: "gap", Frame: UnknownStackBound()}}, Boundary: boundary}}}
	for _, summary := range []SemanticStackSummary{
		{Callable: "decode", OwnFrame: exact, TransitiveMaximum: upper, MaximumCause: []StackCauseStep{{Callable: "decode", Source: source, Detail: "semantic frame"}}},
		{Callable: "unknown", OwnFrame: exact, TransitiveMaximum: UnknownStackBound(), Evidence: evidence},
		{Callable: "unbounded", OwnFrame: zero, TransitiveMaximum: UnboundedStackBound(), MaximumCause: []StackCauseStep{{Detail: "proven unbounded demand"}}},
		{Callable: "zero", CompilationPlanID: "other-plan", OwnFrame: zero, TransitiveMaximum: zero},
	} {
		if err := store.RecordSemantic(summary); err != nil {
			t.Fatal(err)
		}
	}
	if err := store.RecordMachine(MachineStackSummary{Callable: "decode", CompilationPlanID: "plan", OwnFrame: exact, TransitiveMaximum: large, Evidence: evidence}); err != nil {
		t.Fatal(err)
	}
	return store, stackArchiveDependencies(store)
}

// stackArchiveDependencies models driver-observed dependencies, not metadata
// read from the imported artifact.
// Rules: rules/analysis/stack_analysis.md — "Summary invalidation".
func stackArchiveDependencies(store *StackSummaryStore) map[StackSummaryScope]StackSummaryDependencies {
	deps := map[StackSummaryScope]StackSummaryDependencies{}
	for _, summary := range store.SemanticSummaries() {
		deps[StackSummaryScope{summary.Callable, StackMeasurementSemantic, summary.CompilationPlanID}] = StackSummaryDependencies{Signature: "signature-v1", Specialization: "nongeneric", CompilerModel: "compiler-model-v1", Fingerprint: "all-semantic-dependencies-v1"}
	}
	for _, summary := range store.MachineSummaries() {
		deps[StackSummaryScope{summary.Callable, StackMeasurementMachine, summary.CompilationPlanID}] = StackSummaryDependencies{Signature: "signature-v1", Specialization: "nongeneric", CompilerModel: "compiler-model-v1", Fingerprint: "all-machine-dependencies-v1", ABI: "backend-abi-v1"}
	}
	return deps
}

// Persistence retains exact proof quality, arbitrary precision and detached
// cause/evidence paths without mixing semantic, machine or plan namespaces.
// Rules: rules/analysis/stack_analysis.md — "Required separate-compilation tests".
func TestStackPersistenceRoundTrip(t *testing.T) {
	store, deps := stackArchiveInputs(t)
	data, err := store.MarshalStackSummaries(deps)
	if err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(t.TempDir(), "stack-summary.json")
	if err := os.WriteFile(file, data, 0600); err != nil {
		t.Fatal(err)
	}
	data, err = os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	var imported StackSummaryStore
	if err := imported.LoadStackSummaries(data, deps); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(store.SemanticSummaries(), imported.SemanticSummaries()) || !reflect.DeepEqual(store.MachineSummaries(), imported.MachineSummaries()) {
		t.Fatal("summary meaning changed in transport")
	}
	roundTrip, err := imported.MarshalStackSummaries(deps)
	if err != nil || !bytes.Equal(data, roundTrip) {
		t.Fatal("archive is not stable", err)
	}
	var reversed StackSummaryStore
	semantic := store.SemanticSummaries()
	for i := len(semantic) - 1; i >= 0; i-- {
		_ = reversed.RecordSemantic(semantic[i])
	}
	for _, summary := range store.MachineSummaries() {
		_ = reversed.RecordMachine(summary)
	}
	reordered, err := reversed.MarshalStackSummaries(deps)
	if err != nil || !bytes.Equal(data, reordered) {
		t.Fatal("archive depends on registration order", err)
	}
	summary, _ := imported.Semantic("unknown", "")
	summary.Evidence.UnknownPaths[0].Frames[0].Callable = "modified"
	again, _ := imported.Semantic("unknown", "")
	if again.Evidence.UnknownPaths[0].Frames[0].Callable == "modified" {
		t.Fatal("imported evidence is mutable")
	}
}

// Each incompatible dimension causes a localized cache miss. Compatible facts
// at another measurement level remain usable; archive metadata cannot certify
// its own freshness or substitute a different CompilationPlan.
// Rules: rules/analysis/stack_analysis.md — "Summary versioning", "Summary invalidation".
func TestStackPersistenceCompatibility(t *testing.T) {
	store, original := stackArchiveInputs(t)
	data, err := store.MarshalStackSummaries(original)
	if err != nil {
		t.Fatal(err)
	}
	key := StackSummaryScope{"decode", StackMeasurementMachine, "plan"}
	for _, field := range []string{"Signature", "Specialization", "CompilerModel", "Fingerprint", "ABI", "Plan", "Missing"} {
		t.Run(field, func(t *testing.T) {
			current := stackArchiveDependencies(store)
			deps := current[key]
			if field == "Plan" || field == "Missing" {
				delete(current, key)
				if field == "Plan" {
					current[StackSummaryScope{key.Callable, key.Level, "wrong-plan"}] = deps
				}
			} else {
				reflect.ValueOf(&deps).Elem().FieldByName(field).SetString("changed")
				current[key] = deps
			}
			var imported StackSummaryStore
			if err := imported.LoadStackSummaries(data, current); err != nil {
				t.Fatal(err)
			}
			if _, ok := imported.Machine("decode", "plan"); ok {
				t.Fatal("stale machine proof reused")
			}
			if _, ok := imported.Machine("decode", "wrong-plan"); ok {
				t.Fatal("plan substituted")
			}
			if _, ok := imported.Semantic("decode", ""); !ok {
				t.Fatal("independent semantic proof lost")
			}
		})
	}
}

// Malformed artifacts never install even their valid prefix and clear previous
// imported proofs. Integrity is checked separately from structural validity.
// Rules: rules/analysis/stack_analysis.md — "Summary versioning".
func TestStackPersistenceRejectsInvalid(t *testing.T) {
	store, deps := stackArchiveInputs(t)
	data, err := store.MarshalStackSummaries(deps)
	if err != nil {
		t.Fatal(err)
	}
	var envelope stackSummaryArchive
	if err := json.Unmarshal(data, &envelope); err != nil {
		t.Fatal(err)
	}
	cases := map[string][]byte{"truncated": data[:len(data)-1], "trailing": append(append([]byte(nil), data...), []byte(" {}")...), "too large": []byte(strings.Repeat(" ", maxStackArchiveBytes+1))}
	mutateArchive := func(name string, change func(*stackSummaryArchive)) {
		copy := envelope
		change(&copy)
		cases[name], _ = json.Marshal(copy)
	}
	mutateArchive("schema", func(a *stackSummaryArchive) { a.Version++ })
	mutateArchive("format", func(a *stackSummaryArchive) { a.Format = "other" })
	mutateArchive("digest", func(a *stackSummaryArchive) { a.Digest = "invalid" })
	mutations := map[string]func(*persistedStackSummary){
		"missing kind":           func(e *persistedStackSummary) { e.OwnFrame.Kind = "" },
		"unknown kind":           func(e *persistedStackSummary) { e.Maximum.Kind = "Approximate" },
		"negative bytes":         func(e *persistedStackSummary) { e.OwnFrame.Bytes = "-1" },
		"missing bytes":          func(e *persistedStackSummary) { e.OwnFrame.Bytes = "" },
		"noncanonical bytes":     func(e *persistedStackSummary) { e.OwnFrame.Bytes = "01" },
		"unknown with bytes":     func(e *persistedStackSummary) { e.OwnFrame.Kind = StackBoundUnknown },
		"missing callable":       func(e *persistedStackSummary) { e.Scope.Callable = "" },
		"missing signature":      func(e *persistedStackSummary) { e.Dependencies.Signature = "" },
		"missing dependencies":   func(e *persistedStackSummary) { e.Dependencies.Fingerprint = "" },
		"missing model":          func(e *persistedStackSummary) { e.Dependencies.CompilerModel = "" },
		"missing specialization": func(e *persistedStackSummary) { e.Dependencies.Specialization = "" },
		"wrong level":            func(e *persistedStackSummary) { e.Scope.Level = "Other" },
		"missing machine plan": func(e *persistedStackSummary) {
			e.Scope.Level = StackMeasurementMachine
			e.Scope.CompilationPlanID = ""
		},
		"missing machine ABI": func(e *persistedStackSummary) {
			e.Scope.Level = StackMeasurementMachine
			e.Scope.CompilationPlanID = "plan"
			e.Dependencies.ABI = ""
		},
		"nested bad bound": func(e *persistedStackSummary) {
			e.Contributors = []persistedStackFrame{{Callable: "a", Frame: persistedStackBound{Kind: StackBoundExact, Bytes: "bad"}}}
		},
		"private source": func(e *persistedStackSummary) {
			e.MaximumCause = []StackCauseStep{{Source: lexer.Token{File: filepath.Join(string(filepath.Separator), "private", "file.sec")}}}
		},
	}
	for name, change := range mutations {
		mutateArchive(name, func(a *stackSummaryArchive) {
			var entries []persistedStackSummary
			_ = json.Unmarshal(a.Payload, &entries)
			change(&entries[len(entries)-1]) // A valid prefix must not escape.
			a.Payload, _ = json.Marshal(entries)
			a.Digest = graphIdentityDigest(a.Payload)
		})
	}
	mutateArchive("duplicate scope", func(a *stackSummaryArchive) {
		var entries []persistedStackSummary
		_ = json.Unmarshal(a.Payload, &entries)
		entries = append(entries, entries[0])
		a.Payload, _ = json.Marshal(entries)
		a.Digest = graphIdentityDigest(a.Payload)
	})
	mutateArchive("too many records", func(a *stackSummaryArchive) {
		var entries []persistedStackSummary
		_ = json.Unmarshal(a.Payload, &entries)
		many := make([]persistedStackSummary, maxStackArchiveEntries+1)
		for i := range many {
			many[i] = entries[0]
		}
		a.Payload, _ = json.Marshal(many)
		a.Digest = graphIdentityDigest(a.Payload)
	})
	mutateArchive("unsupported payload field", func(a *stackSummaryArchive) {
		var entries []map[string]any
		_ = json.Unmarshal(a.Payload, &entries)
		entries[0]["Unrecognized"] = true
		a.Payload, _ = json.Marshal(entries)
		a.Digest = graphIdentityDigest(a.Payload)
	})
	for name, invalid := range cases {
		t.Run(name, func(t *testing.T) {
			var imported StackSummaryStore
			if err := imported.LoadStackSummaries(data, deps); err != nil {
				t.Fatal(err)
			}
			if err := imported.LoadStackSummaries(invalid, deps); err == nil {
				t.Fatal("invalid artifact accepted")
			}
			if len(imported.SemanticSummaries()) != 0 || len(imported.MachineSummaries()) != 0 {
				t.Fatal("stale or partial proof survived")
			}
		})
	}
	key := StackSummaryScope{"decode", StackMeasurementMachine, "plan"}
	invalidDeps := stackArchiveDependencies(store)
	invalidDeps[key] = StackSummaryDependencies{}
	if _, err := store.MarshalStackSummaries(invalidDeps); err == nil {
		t.Fatal("invalid producer identity exported")
	}
	var absent *StackSummaryStore
	if err := absent.LoadStackSummaries(data, deps); err == nil {
		t.Fatal("nil import store accepted")
	}
	if _, err := absent.MarshalStackSummaries(nil); err != nil {
		t.Fatal("empty export failed", err)
	}
}

// Imported compatible own frames compose exactly like body-derived producer
// facts on a real canonical source call graph. A missing leaf affects only its
// dependent paths, preserving other independently known leaf frames.
// Rules: rules/analysis/stack_analysis.md — "Required separate-compilation tests".
func TestStackPersistenceComposition(t *testing.T) {
	graph, frames, ids := stackCallableInputs(t)
	deps := stackArchiveDependencies(frames)
	data, err := frames.MarshalStackSummaries(deps)
	if err != nil {
		t.Fatal(err)
	}
	var imported StackSummaryStore
	if err := imported.LoadStackSummaries(data, deps); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(ComposeSemanticStackSummaries(graph, frames, ""), ComposeSemanticStackSummaries(graph, &imported, "")) {
		t.Fatal("import changed semantic composition")
	}
	want, err := ComposeMachineStackSummaries(graph, frames, "plan")
	if err != nil {
		t.Fatal(err)
	}
	got, err := ComposeMachineStackSummaries(graph, &imported, "plan")
	if err != nil || !reflect.DeepEqual(want, got) {
		t.Fatal("import changed machine composition", err)
	}
	delete(deps, StackSummaryScope{ids["Small"], StackMeasurementSemantic, ""})
	if err := imported.LoadStackSummaries(data, deps); err != nil {
		t.Fatal(err)
	}
	composed := ComposeSemanticStackSummaries(graph, &imported, "")
	for _, summary := range composed {
		if summary.Callable == ids["Small"] && summary.TransitiveMaximum.Kind() != StackBoundUnknown {
			t.Fatal("missing frame treated as zero")
		}
		if summary.Callable == ids["Large"] && summary.TransitiveMaximum.Kind() != StackBoundExact {
			t.Fatal("unrelated leaf lost proof")
		}
	}
	if err := imported.LoadStackSummaries(nil, deps); err != nil || len(imported.SemanticSummaries()) != 0 || len(imported.MachineSummaries()) != 0 {
		t.Fatal("empty import did not clear old proofs", err)
	}
}
