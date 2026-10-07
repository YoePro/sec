package sema

import (
	"encoding/json"
	"errors"
	"fmt"
	"sort"

	"sec/internal/lexer"
)

const parameterDemandSchemaVersion = 1

var zeroParameterSource lexer.Token

// ParameterDemandSummaryIdentity binds demand to producer-observed semantic
// dependencies. Drivers supply current fingerprints, including body, callee,
// callable/FFI, ownership, escape, storage and type inputs; persisted bytes are
// never their own freshness evidence. Specialization must explicitly name the
// template or concrete instantiation. CompilationPlan may be empty only for
// demand whose semantics are plan independent. CompilerModel is always required.
// Rules: rules/analysis/parameter_usage_analysis.md — "Persisted summary versioning",
// "Summary invalidation", "CompilationPlan", "Generic functions".
type ParameterDemandSummaryIdentity struct {
	Specialization        string
	CompilationPlan       string
	CompilerModel         string
	DependencyFingerprint string
}

type persistedParameterCapability struct {
	Type                          string
	Ref, Mut, Consuming, Receiver bool
}

type persistedParameter struct {
	Capability  persistedParameterCapability
	Demand      ParameterDemand
	Projections [][]PlaceProjection
}

type persistedParameterDemand struct {
	Callable   CallableID
	Identity   ParameterDemandSummaryIdentity
	Parameters []persistedParameter
	Receiver   *persistedParameter
}

type parameterDemandEnvelope struct {
	Format  string
	Version int
	Payload json.RawMessage
	Digest  string
}

// MarshalDemandSummaries exports the semantic demand core and symbolic access
// paths, excluding local binding IDs, source locations, recommendations and
// layout/copy costs. The driver owns storage and supplies dependency identity.
// Rules: rules/analysis/parameter_usage_analysis.md — "Function summaries",
// "Separate compilation", "Persisted summary versioning", "Summary invalidation".
func (p *ParameterUsageAnalysis) MarshalDemandSummaries(identities map[CallableID]ParameterDemandSummaryIdentity) ([]byte, error) {
	if p == nil {
		p = newParameterUsageAnalysis()
	}
	budget := p.budget
	if budget.MaxPersistenceBytes == 0 || budget.MaxPersistenceRecords == 0 || budget.MaxTypeDepth == 0 {
		return nil, &ParameterUsageBudgetError{"export", "transport disabled"}
	}
	work := parameterWorkBudget{remaining: budget.MaxPersistenceRecords, maxDepth: budget.MaxTypeDepth}
	entries := []persistedParameterDemand{}
	for _, id := range p.summaryOrder {
		summary := p.summaries[id]
		if summary == nil {
			continue
		}
		identity, selected := identities[summary.Callable]
		if !selected {
			continue
		}
		if !validParameterDemandIdentity(identity) {
			return nil, errors.New("incomplete parameter demand identity")
		}
		if !work.summaryFits(summary) {
			return nil, &ParameterUsageBudgetError{"export", "semantic records or type depth"}
		}
		entry := persistedParameterDemand{Callable: summary.Callable, Identity: identity}
		for _, parameter := range summary.Parameters {
			entry.Parameters = append(entry.Parameters, persistParameter(parameter))
		}
		if summary.Receiver != nil {
			receiver := persistParameter(*summary.Receiver)
			entry.Receiver = &receiver
		}
		if err := validatePersistedParameterDemand(entry); err != nil {
			return nil, err
		}
		entries = append(entries, entry)
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Callable < entries[j].Callable })
	payload, err := json.Marshal(entries)
	if err != nil {
		return nil, err
	}
	data, err := json.Marshal(parameterDemandEnvelope{"sec.parameter-demand", parameterDemandSchemaVersion, payload, graphIdentityDigest(payload)})
	if err == nil && len(data) > budget.MaxPersistenceBytes {
		return nil, &ParameterUsageBudgetError{"export", "encoded bytes"}
	}
	return data, err
}

// SetImportedParameterDemands validates the complete artifact atomically, then
// installs only entries matching independently observed current dependencies.
// Missing/stale entries stay misses and normal call propagation widens them.
// Every invocation clears earlier imports, including on malformed input; local
// bodies always take precedence. Drivers must call this for each request with
// freshly observed identity, or pass nil to clear imports. No trust is inferred
// from names or from the artifact's integrity digest.
// Rules: rules/analysis/parameter_usage_analysis.md — "Separate compilation",
// "Persisted summary versioning", "Summary invalidation", "Dimension-specific unknown".
func (a *Analyzer) SetImportedParameterDemands(data []byte, current map[CallableID]ParameterDemandSummaryIdentity) error {
	a.importedParameterDemands = nil
	if len(data) == 0 {
		return nil
	}
	budget := a.parameterBudget
	if budget.MaxPersistenceRecords == 0 || budget.MaxTypeDepth == 0 || len(data) > budget.MaxPersistenceBytes {
		return &ParameterUsageBudgetError{"import", "encoded bytes or transport disabled"}
	}
	work := parameterWorkBudget{remaining: budget.MaxPersistenceRecords, maxDepth: budget.MaxTypeDepth}
	var envelope parameterDemandEnvelope
	if err := decodeGraphCheckpoint(data, &envelope); err != nil {
		return err
	}
	if envelope.Format != "sec.parameter-demand" || envelope.Version != parameterDemandSchemaVersion {
		return errors.New("incompatible parameter demand schema")
	}
	if envelope.Digest != graphIdentityDigest(envelope.Payload) {
		return errors.New("parameter demand integrity mismatch")
	}
	var entries []persistedParameterDemand
	if err := decodeGraphCheckpoint(envelope.Payload, &entries); err != nil {
		return err
	}
	seen := map[CallableID]bool{}
	imported := map[CallableID]persistedParameterDemand{}
	for _, entry := range entries {
		if !work.persistedEntryFits(entry) {
			return &ParameterUsageBudgetError{"import", "semantic records"}
		}
		if seen[entry.Callable] {
			return errors.New("duplicate parameter demand callable")
		}
		seen[entry.Callable] = true
		if err := validatePersistedParameterDemand(entry); err != nil {
			return err
		}
		if expected, ok := current[entry.Callable]; ok && validParameterDemandIdentity(expected) && expected == entry.Identity {
			imported[entry.Callable] = entry
		}
	}
	a.importedParameterDemands = imported
	return nil
}

// validParameterDemandIdentity requires explicit specialization/model and a
// current semantic dependency fingerprint, without forcing target dependence.
// Rules: rules/analysis/parameter_usage_analysis.md — "Persisted summary versioning", "CompilationPlan".
func validParameterDemandIdentity(identity ParameterDemandSummaryIdentity) bool {
	return identity.Specialization != "" && identity.CompilerModel != "" && identity.DependencyFingerprint != ""
}

// persistParameter detaches semantic dimensions and root-relative projections;
// local tokens and binding identities cannot survive separate compilation.
// Rules: rules/analysis/parameter_usage_analysis.md — "Separate compilation", "Receiver demand".
func persistParameter(parameter ParameterUsageParameterSummary) persistedParameter {
	result := persistedParameter{Capability: persistedParameterCapability{graphTypeIdentity(parameter.DeclaredType), parameter.DeclaredRef, parameter.DeclaredMut, parameter.Consuming, parameter.Receiver}, Demand: cloneParameterDemand(parameter.Demand)}
	sortParameterDemand(&result.Demand)
	seen := map[string]bool{}
	for _, use := range parameter.Uses {
		projections := clonePlaceProjections(use.Place.Projections)
		for i := range projections {
			projections[i].Token = zeroParameterSource
		}
		encoded, _ := json.Marshal(projections)
		if !seen[string(encoded)] {
			result.Projections = append(result.Projections, projections)
			seen[string(encoded)] = true
		}
	}
	sort.Slice(result.Projections, func(i, j int) bool {
		left, _ := json.Marshal(result.Projections[i])
		right, _ := json.Marshal(result.Projections[j])
		return string(left) < string(right)
	})
	return result
}

// installImportedDemands binds validated metadata to current resolved native
// declarations. It neither imports missing declarations nor reuses producer
// binding/type objects. Signature mismatch remains conservative Unknown; local
// body summaries win even if their precision is weaker than persisted demand.
// Rules: rules/analysis/parameter_usage_analysis.md — "Persisted summary versioning",
// "Calls propagate demand", "Separate compilation", "Receiver demand".
func (b *parameterUsageBuilder) installImportedDemands() {
	defer func() { b.analyzer.importedParameterDemands = nil }()
	work := parameterWorkBudget{remaining: b.result.budget.MaxPersistenceRecords, maxDepth: b.result.budget.MaxTypeDepth}
	defer func() { b.result.importCoverage.VisitedWork = work.visited }()
	b.result.importCoverage.MaxWork = work.remaining
	b.result.importCoverage.MaxTypeDepth = work.maxDepth
	if len(b.analyzer.importedParameterDemands) == 0 {
		return
	}
	var declarations []Function
	for _, functions := range b.analyzer.functions {
		for _, function := range functions {
			id := callableID(function)
			if _, imported := b.analyzer.importedParameterDemands[id]; imported && b.result.summaries[id] == nil {
				declarations = append(declarations, function)
			}
		}
	}
	sort.Slice(declarations, func(i, j int) bool { return callableID(declarations[i]) < callableID(declarations[j]) })
	for _, function := range declarations {
		id := callableID(function)
		entry, ok := b.analyzer.importedParameterDemands[id]
		if !ok || b.result.summaries[id] != nil || len(entry.Parameters) != len(function.Parameters) {
			continue
		}
		summary := &ParameterUsageCallableSummary{Callable: id, Name: function.Name, Declaration: function.Token}
		compatible := true
		for index, parameter := range function.Parameters {
			if !work.typeFits(parameter.Type, 1, true, map[*Type]bool{}) {
				b.result.importCoverage.SkippedCallables++
				compatible = false
				break
			}
			item := ParameterUsageParameterSummary{Index: index, Name: parameter.Name, Declaration: parameter.Token, DeclaredType: semanticSnapshotType(parameter.Type), DeclaredRef: parameter.Ref, DeclaredMut: parameter.MutableRef, Consuming: parameter.Consuming}
			if persistParameter(item).Capability != entry.Parameters[index].Capability {
				compatible = false
				break
			}
			restoreParameterDemand(&item, entry.Parameters[index])
			summary.Parameters = append(summary.Parameters, item)
		}
		hasReceiver := function.ImplTarget != "" && !function.Static
		if hasReceiver != (entry.Receiver != nil) {
			compatible = false
		}
		if compatible && hasReceiver {
			receiverType, known := b.analyzer.types[function.ImplTarget]
			if !work.typeFits(receiverType, 1, true, map[*Type]bool{}) {
				b.result.importCoverage.SkippedCallables++
				continue
			}
			item := ParameterUsageParameterSummary{Index: -1, Name: "self", DeclaredType: semanticSnapshotType(receiverType), Receiver: true}
			if !known || persistParameter(item).Capability != entry.Receiver.Capability {
				compatible = false
			} else {
				restoreParameterDemand(&item, *entry.Receiver)
				summary.Receiver = &item
			}
		}
		if compatible {
			b.finishSummary(summary)
			b.result.summaries[id] = summary
			b.result.importCoverage.InstalledCallables++
		}
	}
}

// restoreParameterDemand keeps dimensions independent and rebinds symbolic
// accesses to the current parameter. Call propagation supplies caller locations.
// Rules: rules/analysis/parameter_usage_analysis.md — "Separate compilation", "Calls propagate demand".
func restoreParameterDemand(parameter *ParameterUsageParameterSummary, wire persistedParameter) {
	parameter.Demand = cloneParameterDemand(wire.Demand)
	for _, path := range wire.Projections {
		parameter.Uses = append(parameter.Uses, ParameterUse{Kind: ParameterUseCall, Place: Place{Root: parameter.Name, Projections: clonePlaceProjections(path)}})
	}
}

// validatePersistedParameterDemand rejects malformed dimensions and access
// metadata even when an artifact's checksum is internally consistent.
// Rules: rules/analysis/parameter_usage_analysis.md — "Persisted summary versioning",
// "Dimension-specific unknown", "Recursive functions".
func validatePersistedParameterDemand(entry persistedParameterDemand) error {
	if entry.Callable == "" || !validParameterDemandIdentity(entry.Identity) {
		return errors.New("invalid parameter demand identity")
	}
	parameters := append([]persistedParameter(nil), entry.Parameters...)
	if entry.Receiver != nil {
		parameters = append(parameters, *entry.Receiver)
	}
	for index, parameter := range parameters {
		if parameter.Capability.Type == "" || parameter.Capability.Receiver != (index == len(entry.Parameters)) || parameter.Capability.Mut && !parameter.Capability.Ref {
			return errors.New("invalid parameter demand capability")
		}
		if err := validatePersistedDemand(parameter.Demand); err != nil {
			return err
		}
		for _, path := range parameter.Projections {
			if len(path) > parameterUsageProjectionLimit {
				return errors.New("parameter demand projection budget exceeded")
			}
			for _, projection := range path {
				switch projection.Kind {
				case PlaceField, PlaceProperty, PlaceIndex, PlaceSlice, PlaceDereference, PlaceUnionPayload:
				default:
					return errors.New("invalid parameter demand projection")
				}
				if projection.Token != zeroParameterSource {
					return errors.New("persisted parameter projection contains local source identity")
				}
			}
		}
	}
	return nil
}

// validatePersistedDemand accepts only versioned vocabulary, preserving known
// dimensions beside explicitly unknown ones and rejecting impossible extents.
// Rules: rules/analysis/parameter_usage_analysis.md — "ParameterDemand", "Dimension-specific unknown", "Persisted summary versioning".
func validatePersistedDemand(d ParameterDemand) error {
	valid := func(value string, choices ...string) bool {
		for _, choice := range choices {
			if value == choice {
				return true
			}
		}
		return false
	}
	if !valid(string(d.Access), "unused", "read", "write", "unknown") || !valid(string(d.Mutation), "no-mutation", "element-or-field-mutation", "structural-mutation", "unknown") || !valid(string(d.Ownership), "borrow-sufficient", "ownership-required", "consumption-required", "unknown") || !valid(string(d.Lifetime), "call-only", "returned", "retained", "cross-task", "cross-thread", "foreign-retention", "unknown") || !valid(string(d.Identity), "value-only", "address-required", "stable-identity-required", "unknown") || !valid(string(d.Representation), "none", "exact", "unknown") || !valid(string(d.Precision), "exact", "partial", "unknown") || d.MinimumExtent < 0 {
		return fmt.Errorf("invalid persisted parameter demand dimensions")
	}
	seen := map[ParameterShapeDemand]bool{}
	for _, shape := range d.Shapes {
		if seen[shape] || !valid(string(shape), "whole-value", "sequence", "contiguous-sequence", "random-access-sequence", "exact-extent", "minimum-extent", "known-range", "unknown") {
			return errors.New("invalid persisted parameter shape")
		}
		seen[shape] = true
	}
	storageSeen := map[ParameterStorageDemand]bool{}
	for _, storage := range d.Storage {
		if storageSeen[storage] || !valid(string(storage), "no-special-storage", "contiguous", "stable-address", "aligned", "pinned", "specific-memory-space", "unknown") {
			return errors.New("invalid persisted parameter storage")
		}
		storageSeen[storage] = true
	}
	if len(d.Storage) == 0 || len(d.Storage) > 1 && storageSeen[ParameterStorageNone] {
		return errors.New("inconsistent persisted parameter storage")
	}
	return nil
}
