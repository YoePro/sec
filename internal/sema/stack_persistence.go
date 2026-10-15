package sema

import (
	"encoding/json"
	"errors"
	"math/big"
	"path/filepath"
	"strings"

	"sec/internal/lexer"
)

const stackSummarySchema = 1
const maxStackArchiveBytes = 4 << 20
const maxStackArchiveEntries = 4096

// StackSummaryScope keeps measurement levels and exact plans independent.
// An empty plan is allowed only for target-independent semantic facts.
// Rules: rules/analysis/stack_analysis.md — "Summary versioning", "Stack analysis levels".
type StackSummaryScope struct {
	Callable          CallableID
	Level             StackMeasurementLevel
	CompilationPlanID string
}

// StackSummaryDependencies are independently observed by the importing driver,
// never learned from the archive itself. CompilerModel includes the semantic
// version; Fingerprint covers body, callees, targets/SCCs, layout, storage,
// cleanup and contracts. Specialization explicitly names a template or instance.
// Machine facts additionally require the ABI/backend model. A digest detects
// damage; it is not authentication or proof of a foreign producer's guarantees.
// Rules: rules/analysis/stack_analysis.md — "Summary versioning", "Summary invalidation".
type StackSummaryDependencies struct {
	Signature      string
	Specialization string
	CompilerModel  string
	Fingerprint    string
	ABI            string
}

type persistedStackBound struct {
	Kind  StackBoundKind
	Bytes string `json:",omitempty"`
}

type persistedStackFrame struct {
	Callable CallableID
	Source   lexer.Token
	Frame    persistedStackBound
}

type persistedStackPath struct {
	Frames   []persistedStackFrame
	Boundary StackCauseStep
}

type persistedStackSummary struct {
	Scope         StackSummaryScope
	Dependencies  StackSummaryDependencies
	OwnFrame      persistedStackBound
	Maximum       persistedStackBound
	MaximumCause  []StackCauseStep
	KnownPrefix   []persistedStackFrame
	Contributors  []persistedStackFrame
	UnknownCauses []StackCauseStep
	UnknownPaths  []persistedStackPath
}

type stackSummaryArchive struct {
	Format  string
	Version int
	Payload json.RawMessage
	Digest  string
}

// MarshalStackSummaries exports selected facts in stable level/callable/plan
// order. No bound's private representation is serialized: exact zero, upper
// bounds, Unknown and Unbounded have explicit, independently validated encodings.
// The driver owns artifact storage and supplies module-relative source paths.
// Rules: rules/analysis/stack_analysis.md — "Summary model", "Summary versioning",
// "CompilationPlan dependence", "Determinism".
func (store *StackSummaryStore) MarshalStackSummaries(dependencies map[StackSummaryScope]StackSummaryDependencies) ([]byte, error) {
	entries := []persistedStackSummary{}
	appendEntry := func(scope StackSummaryScope, own, maximum StackBound, cause []StackCauseStep, evidence StackEvidence) error {
		deps, selected := dependencies[scope]
		if !selected {
			return nil
		}
		entry := persistedStackSummary{Scope: scope, Dependencies: deps, OwnFrame: persistStackBound(own), Maximum: persistStackBound(maximum), MaximumCause: cause,
			KnownPrefix: persistStackFrames(evidence.KnownPrefix), Contributors: persistStackFrames(evidence.Contributors), UnknownCauses: evidence.UnknownCauses}
		for _, path := range evidence.UnknownPaths {
			entry.UnknownPaths = append(entry.UnknownPaths, persistedStackPath{persistStackFrames(path.Frames), path.Boundary})
		}
		if _, _, _, err := restoreStackEntry(entry); err != nil {
			return err
		}
		entries = append(entries, entry)
		if len(entries) > maxStackArchiveEntries {
			return errors.New("too many stack summary records")
		}
		return nil
	}
	for _, summary := range store.SemanticSummaries() {
		if err := appendEntry(StackSummaryScope{summary.Callable, StackMeasurementSemantic, summary.CompilationPlanID}, summary.OwnFrame, summary.TransitiveMaximum, summary.MaximumCause, summary.Evidence); err != nil {
			return nil, err
		}
	}
	for _, summary := range store.MachineSummaries() {
		if err := appendEntry(StackSummaryScope{summary.Callable, StackMeasurementMachine, summary.CompilationPlanID}, summary.OwnFrame, summary.TransitiveMaximum, summary.MaximumCause, summary.Evidence); err != nil {
			return nil, err
		}
	}
	payload, err := json.Marshal(entries)
	if err != nil {
		return nil, err
	}
	data, err := json.Marshal(stackSummaryArchive{"sec.stack-summaries", stackSummarySchema, payload, graphIdentityDigest(payload)})
	if len(data) > maxStackArchiveBytes {
		return nil, errors.New("stack summary archive exceeds byte limit")
	}
	return data, err
}

// LoadStackSummaries replaces a dedicated imported-summary store atomically.
// It first clears previous imports, including on failure; stale or missing
// independently observed dependencies remain cache misses, never zero bounds.
// Every record is validated even if stale. A corrupt suffix cannot publish a
// valid prefix. Drivers must supply fresh dependencies for each compilation.
// Rules: rules/analysis/stack_analysis.md — "Summary versioning", "Summary invalidation",
// "Required separate-compilation tests", "Partial information".
func (store *StackSummaryStore) LoadStackSummaries(data []byte, current map[StackSummaryScope]StackSummaryDependencies) error {
	if store == nil {
		return errors.New("stack import requires a store")
	}
	*store = StackSummaryStore{}
	if len(data) == 0 {
		return nil
	}
	if len(data) > maxStackArchiveBytes {
		return errors.New("stack summary archive exceeds byte limit")
	}
	var archive stackSummaryArchive
	if err := decodeGraphCheckpoint(data, &archive); err != nil {
		return err
	}
	if archive.Format != "sec.stack-summaries" || archive.Version != stackSummarySchema {
		return errors.New("incompatible stack summary schema")
	}
	if archive.Digest != graphIdentityDigest(archive.Payload) {
		return errors.New("stack summary integrity mismatch")
	}
	var entries []persistedStackSummary
	if err := decodeGraphCheckpoint(archive.Payload, &entries); err != nil {
		return err
	}
	if len(entries) > maxStackArchiveEntries {
		return errors.New("too many stack summary records")
	}
	seen := map[StackSummaryScope]bool{}
	var restored StackSummaryStore
	for _, entry := range entries {
		if seen[entry.Scope] {
			return errors.New("duplicate stack summary scope")
		}
		seen[entry.Scope] = true
		own, maximum, evidence, err := restoreStackEntry(entry)
		if err != nil {
			return err
		}
		if expected, ok := current[entry.Scope]; !ok || expected != entry.Dependencies {
			continue
		}
		if entry.Scope.Level == StackMeasurementSemantic {
			err = restored.RecordSemantic(SemanticStackSummary{entry.Scope.Callable, entry.Scope.CompilationPlanID, own, maximum, entry.MaximumCause, evidence})
		} else {
			err = restored.RecordMachine(MachineStackSummary{entry.Scope.Callable, entry.Scope.CompilationPlanID, own, maximum, entry.MaximumCause, evidence})
		}
		if err != nil {
			return err
		}
	}
	*store = restored
	return nil
}

// persistStackBound retains proof quality and canonical arbitrary-precision bytes.
// Rules: rules/analysis/stack_analysis.md — "Stack-bound classification", "Summary versioning".
func persistStackBound(bound StackBound) persistedStackBound {
	result := persistedStackBound{Kind: bound.Kind()}
	if count, finite := bound.Bytes(); finite {
		result.Bytes = count.String()
	}
	return result
}

// restoreStackBound rejects missing classifications and malformed finite counts.
// Unknown/Unbounded carry no bytes; neither can silently become exact zero.
// Rules: rules/analysis/stack_analysis.md — "Stack-bound classification", "Summary versioning".
func restoreStackBound(bound persistedStackBound) (StackBound, error) {
	switch bound.Kind {
	case StackBoundUnknown, StackBoundUnbounded:
		if bound.Bytes != "" {
			break
		}
		if bound.Kind == StackBoundUnbounded {
			return UnboundedStackBound(), nil
		}
		return UnknownStackBound(), nil
	case StackBoundExact, StackBoundUpperBound:
		count, ok := new(big.Int).SetString(bound.Bytes, 10)
		if !ok || count.Sign() < 0 || count.String() != bound.Bytes {
			break
		}
		if bound.Kind == StackBoundExact {
			return NewExactStackBound(count)
		}
		return NewUpperStackBound(count)
	}
	return StackBound{}, errors.New("invalid persisted stack bound")
}

// persistStackFrames retains independently known contributions, including gaps.
// Rules: rules/analysis/stack_analysis.md — "Partial information", "Summary model".
func persistStackFrames(frames []StackFrameContribution) []persistedStackFrame {
	if frames == nil {
		return nil
	}
	result := make([]persistedStackFrame, len(frames))
	for i, frame := range frames {
		result[i] = persistedStackFrame{frame.Callable, frame.Source, persistStackBound(frame.Frame)}
	}
	return result
}

// restoreStackEntry validates compatibility identity, every bound and all nested
// evidence before reuse. Unknown dimensions remain explicit and source tokens
// must use portable module-relative paths rather than producer machine paths.
// Rules: rules/analysis/stack_analysis.md — "Summary versioning", "Partial information",
// "Stack cause paths", "Semantic and machine frame authority".
func restoreStackEntry(entry persistedStackSummary) (StackBound, StackBound, StackEvidence, error) {
	fail := func() (StackBound, StackBound, StackEvidence, error) {
		return StackBound{}, StackBound{}, StackEvidence{}, errors.New("invalid stack summary identity or evidence")
	}
	scope, deps := entry.Scope, entry.Dependencies
	if scope.Callable == "" || deps.Signature == "" || deps.Specialization == "" || deps.CompilerModel == "" || deps.Fingerprint == "" {
		return fail()
	}
	if scope.Level != StackMeasurementSemantic && scope.Level != StackMeasurementMachine {
		return fail()
	}
	if scope.Level == StackMeasurementMachine && (scope.CompilationPlanID == "" || deps.ABI == "") {
		return fail()
	}
	valid := true
	checkSource := func(source lexer.Token) {
		if filepath.IsAbs(source.File) || strings.HasPrefix(source.File, "\\") || len(source.File) > 1 && source.File[1] == ':' || source.Line < 0 || source.Column < 0 {
			valid = false
		}
	}
	frames := func(wire []persistedStackFrame) []StackFrameContribution {
		if wire == nil {
			return nil
		}
		result := make([]StackFrameContribution, len(wire))
		for i, frame := range wire {
			bound, err := restoreStackBound(frame.Frame)
			if err != nil || frame.Callable == "" {
				valid = false
			}
			checkSource(frame.Source)
			result[i] = StackFrameContribution{frame.Callable, frame.Source, bound}
		}
		return result
	}
	checkCauses := func(causes []StackCauseStep) {
		for _, cause := range causes {
			checkSource(cause.Source)
		}
	}
	own, err := restoreStackBound(entry.OwnFrame)
	if err != nil {
		return StackBound{}, StackBound{}, StackEvidence{}, err
	}
	maximum, err := restoreStackBound(entry.Maximum)
	if err != nil {
		return StackBound{}, StackBound{}, StackEvidence{}, err
	}
	evidence := StackEvidence{KnownPrefix: frames(entry.KnownPrefix), Contributors: frames(entry.Contributors), UnknownCauses: entry.UnknownCauses}
	checkCauses(entry.MaximumCause)
	checkCauses(entry.UnknownCauses)
	for _, path := range entry.UnknownPaths {
		checkCauses([]StackCauseStep{path.Boundary})
		evidence.UnknownPaths = append(evidence.UnknownPaths, StackUnknownPath{frames(path.Frames), path.Boundary})
	}
	if !valid {
		return fail()
	}
	return own, maximum, evidence, nil
}
