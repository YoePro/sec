package sema

import (
	"bytes"
	"encoding/json"
	"reflect"
	"testing"

	"sec/internal/ast"
)

// Rules: rules/analysis/parameter_usage_analysis.md — "Separate compilation",
// "Persisted summary versioning", "Receiver demand", "Dimension-specific unknown".
func TestParameterDemandPersistenceSeparateCompilation(t *testing.T) {
	producer := NewAnalyzer()
	if errors := producer.Analyze(parameterRegressionProgram(t, "parameter_demand_persistence_valid")); len(errors) != 0 {
		t.Fatal(errors)
	}
	analysis := producer.ParameterUsageAnalysis()
	identities := map[CallableID]ParameterDemandSummaryIdentity{}
	for _, summary := range analysis.Summaries() {
		identities[summary.Callable] = ParameterDemandSummaryIdentity{Specialization: "template", CompilerModel: "test-model", DependencyFingerprint: "current-semantic-inputs"}
	}
	data, err := analysis.MarshalDemandSummaries(identities)
	if err != nil {
		t.Fatal(err)
	}
	repeated, err := analysis.MarshalDemandSummaries(identities)
	if err != nil || !bytes.Equal(data, repeated) {
		t.Fatal("nondeterministic export", err)
	}
	if bytes.Contains(data, []byte("EstimatedSize")) || bytes.Contains(data, []byte("Binding")) || bytes.Contains(data, []byte("Candidate")) {
		t.Fatal("persisted derived advice or binding identity")
	}

	for _, depth := range []AnalysisDepth{AnalysisInteractive, AnalysisStandard, AnalysisDeep} {
		consumer := NewAnalyzerWithDepth(depth)
		if err := consumer.SetImportedParameterDemands(data, identities); err != nil {
			t.Fatal(err)
		}
		program := parameterPersistenceDeclarations(t)
		if errors := consumer.Analyze(program); len(errors) != 0 {
			t.Fatal(errors)
		}
		imported := consumer.ParameterUsageAnalysis()
		for _, name := range []string{"Forward", "ForwardWrite", "ForwardReceiver", "ForwardStatic", "ForwardIndexed"} {
			original := parameterUsageSummaryNamed(t, analysis, name)
			restored := parameterUsageSummaryNamed(t, imported, name)
			for index, parameter := range original.Parameters {
				got := restored.Parameters[index]
				if !reflect.DeepEqual(parameter.Demand, got.Demand) {
					t.Fatalf("%s depth %s: got %+v, want %+v", name, depth, got.Demand, parameter.Demand)
				}
				for _, use := range parameter.Uses {
					if use.Kind == ParameterUseCall {
						found := false
						for _, candidate := range got.Uses {
							if candidate.Kind == ParameterUseCall && candidate.Place.String() == use.Place.String() {
								found = true
							}
						}
						if !found {
							t.Fatalf("lost symbolic path %s: %+v", use.Place.String(), got.Uses)
						}
					}
				}
			}
		}
		// Imports belong to one analysis request; analyzer reuse cannot silently
		// keep previous request freshness evidence.
		if errors := consumer.Analyze(parameterPersistenceDeclarations(t)); len(errors) != 0 {
			t.Fatal(errors)
		}
		demand := parameterUsageSummaryNamed(t, consumer.ParameterUsageAnalysis(), "Forward").Parameters[0].Demand
		if demand.Precision != ParameterDemandUnknown {
			t.Fatal("stale imports survived analyzer reuse", demand)
		}
	}
}

// parameterPersistenceDeclarations simulates a separately loaded native unit
// whose signatures are available but whose bodies are absent.
// Rules: rules/analysis/parameter_usage_analysis.md — "Separate compilation".
func parameterPersistenceDeclarations(t *testing.T) *ast.Program {
	program := parameterRegressionProgram(t, "parameter_demand_persistence_valid")
	for _, statement := range program.Statements {
		switch declaration := statement.(type) {
		case *ast.FunctionDeclaration:
			if declaration.Name.Value == "First" || declaration.Name.Value == "Write" || declaration.Name.Value == "Indexed" {
				declaration.Body = nil
			}
		case *ast.ImplStatement:
			for _, member := range declaration.Members {
				if function, ok := member.(*ast.FunctionDeclaration); ok {
					function.Body = nil
				}
			}
		}
	}
	return program
}

// Rules: rules/analysis/parameter_usage_analysis.md — "Persisted summary versioning",
// "Summary invalidation", "Dimension-specific unknown".
func TestParameterDemandPersistenceValidationAndFallback(t *testing.T) {
	producer := NewAnalyzer()
	if errors := producer.Analyze(parameterRegressionProgram(t, "parameter_demand_persistence_valid")); len(errors) != 0 {
		t.Fatal(errors)
	}
	analysis := producer.ParameterUsageAnalysis()
	first := parameterUsageSummaryNamed(t, analysis, "First")
	identity := ParameterDemandSummaryIdentity{Specialization: "Pair", CompilationPlan: "plan-a", CompilerModel: "model-a", DependencyFingerprint: "body-and-dependencies-a"}
	identities := map[CallableID]ParameterDemandSummaryIdentity{first.Callable: identity}
	data, err := analysis.MarshalDemandSummaries(identities)
	if err != nil {
		t.Fatal(err)
	}
	for _, field := range []string{"missing", "specialization", "plan", "model", "dependencies"} {
		t.Run(field, func(t *testing.T) {
			expected := identity
			switch field {
			case "specialization":
				expected.Specialization = "Other"
			case "plan":
				expected.CompilationPlan = "plan-b"
			case "model":
				expected.CompilerModel = "model-b"
			case "dependencies":
				expected.DependencyFingerprint = "new-body"
			}
			current := map[CallableID]ParameterDemandSummaryIdentity{first.Callable: expected}
			if field == "missing" {
				current = nil
			}
			consumer := NewAnalyzer()
			if err := consumer.SetImportedParameterDemands(data, current); err != nil {
				t.Fatal(err)
			}
			if errors := consumer.Analyze(parameterPersistenceDeclarations(t)); len(errors) != 0 {
				t.Fatal(errors)
			}
			if parameterUsageSummaryNamed(t, consumer.ParameterUsageAnalysis(), "Forward").Parameters[0].Demand.Precision != ParameterDemandUnknown {
				t.Fatal("stale summary used")
			}
		})
	}
	// Known reads and shape survive an explicitly unknown lifetime dimension.
	summary := analysis.summaries[first.Callable]
	summary.Parameters[0].Demand.Lifetime = ParameterLifetimeUnknown
	summary.Parameters[0].Demand.Precision = ParameterDemandPartial
	summary.Parameters[0].Demand.Shapes = []ParameterShapeDemand{ParameterShapeContiguousSequence}
	data, err = analysis.MarshalDemandSummaries(identities)
	if err != nil {
		t.Fatal(err)
	}
	consumer := NewAnalyzer()
	if err := consumer.SetImportedParameterDemands(data, identities); err != nil {
		t.Fatal(err)
	}
	if errors := consumer.Analyze(parameterPersistenceDeclarations(t)); len(errors) != 0 {
		t.Fatal(errors)
	}
	demand := parameterUsageSummaryNamed(t, consumer.ParameterUsageAnalysis(), "Forward").Parameters[0].Demand
	if demand.Access != ParameterAccessRead || demand.Lifetime != ParameterLifetimeUnknown || !parameterDemandHasShape(demand, ParameterShapeContiguousSequence) || demand.Precision != ParameterDemandPartial {
		t.Fatal("dimension-specific unknown collapsed", demand)
	}
	// An available local body wins over a weaker imported fact.
	if err := consumer.SetImportedParameterDemands(data, identities); err != nil {
		t.Fatal(err)
	}
	if errors := consumer.Analyze(parameterRegressionProgram(t, "parameter_demand_persistence_valid")); len(errors) != 0 {
		t.Fatal(errors)
	}
	if parameterUsageSummaryNamed(t, consumer.ParameterUsageAnalysis(), "Forward").Parameters[0].Demand.Lifetime != ParameterLifetimeCallOnly {
		t.Fatal("persisted demand replaced local analysis")
	}
}

// Rules: rules/analysis/parameter_usage_analysis.md — "Persisted summary versioning";
// rules/compiler/incremental_compilation.md — §§31–34.
func TestParameterDemandPersistenceRejectsMalformedMetadata(t *testing.T) {
	producer := NewAnalyzer()
	if errors := producer.Analyze(parameterRegressionProgram(t, "parameter_demand_persistence_valid")); len(errors) != 0 {
		t.Fatal(errors)
	}
	summary := parameterUsageSummaryNamed(t, producer.ParameterUsageAnalysis(), "First")
	identities := map[CallableID]ParameterDemandSummaryIdentity{summary.Callable: {Specialization: "template", CompilerModel: "model", DependencyFingerprint: "fresh"}}
	original, err := producer.ParameterUsageAnalysis().MarshalDemandSummaries(identities)
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"version", "digest", "dimension", "negative-extent", "duplicate", "capability", "projection", "trailing", "partial", "unknown-field"} {
		t.Run(name, func(t *testing.T) {
			var envelope parameterDemandEnvelope
			if err := json.Unmarshal(original, &envelope); err != nil {
				t.Fatal(err)
			}
			var entries []persistedParameterDemand
			if err := json.Unmarshal(envelope.Payload, &entries); err != nil {
				t.Fatal(err)
			}
			switch name {
			case "version":
				envelope.Version++
			case "digest":
				envelope.Digest = "bad"
			case "dimension":
				entries[0].Parameters[0].Demand.Access = "optimistically-safe"
			case "negative-extent":
				entries[0].Parameters[0].Demand.MinimumExtent = -1
			case "duplicate":
				entries = append(entries, entries[0])
			case "capability":
				entries[0].Parameters[0].Capability.Receiver = true
			case "projection":
				entries[0].Parameters[0].Projections = [][]PlaceProjection{{{Kind: "invented"}}}
			}
			if name != "digest" {
				envelope.Payload, _ = json.Marshal(entries)
				envelope.Digest = graphIdentityDigest(envelope.Payload)
			}
			data, _ := json.Marshal(envelope)
			switch name {
			case "trailing":
				data = append(data, []byte("{}")...)
			case "partial":
				data = data[:len(data)/2]
			case "unknown-field":
				data = append([]byte(`{"Extra":true,`), data[1:]...)
			}
			consumer := NewAnalyzer()
			if err := consumer.SetImportedParameterDemands(original, identities); err != nil {
				t.Fatal(err)
			}
			if err := consumer.SetImportedParameterDemands(data, identities); err == nil {
				t.Fatal("malformed metadata accepted")
			}
			if len(consumer.importedParameterDemands) != 0 {
				t.Fatal("failed import left positive facts")
			}
		})
	}
	// A well-formed but declaration-incompatible capability is a miss.
	var envelope parameterDemandEnvelope
	_ = json.Unmarshal(original, &envelope)
	var entries []persistedParameterDemand
	_ = json.Unmarshal(envelope.Payload, &entries)
	entries[0].Parameters[0].Capability.Type = "different-signature"
	envelope.Payload, _ = json.Marshal(entries)
	envelope.Digest = graphIdentityDigest(envelope.Payload)
	data, _ := json.Marshal(envelope)
	consumer := NewAnalyzer()
	if err := consumer.SetImportedParameterDemands(data, identities); err != nil {
		t.Fatal(err)
	}
	if errors := consumer.Analyze(parameterPersistenceDeclarations(t)); len(errors) != 0 {
		t.Fatal(errors)
	}
	if parameterUsageSummaryNamed(t, consumer.ParameterUsageAnalysis(), "Forward").Parameters[0].Demand.Precision != ParameterDemandUnknown {
		t.Fatal("mismatched signature used")
	}
}
