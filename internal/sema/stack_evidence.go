package sema

import (
	"sec/internal/lexer"
	"sort"
)

// StackFrameContribution retains an independently known frame on a permitted
// call path. A collection of such contributions is not a simultaneous total.
// Rules: rules/analysis/stack_analysis.md — "Partial information" and "Stack cause paths".
type StackFrameContribution struct {
	Callable CallableID
	Source   lexer.Token
	Frame    StackBound
}

// StackUnknownPath preserves a representative frame chain to one uncertainty
// boundary. Unknown frame entries mark gaps instead of erasing later known
// frames. The path is explanatory evidence, never an overall resource proof.
// Rules: rules/analysis/stack_analysis.md — "Partial information" and "Stack cause paths".
type StackUnknownPath struct {
	Frames   []StackFrameContribution
	Boundary StackCauseStep
}

// StackEvidence keeps a representative known prefix, independent contributors
// from all covered targets and every distinct uncertainty boundary. Prefix
// frames retain their own proof quality: their maxima need not coincide. Neither
// these frames nor their sum prove an upper bound on the overall stack demand.
// Rules: rules/analysis/stack_analysis.md — "Partial information", "Control-flow composition",
// "Stack cause paths", and "Closed indirect-call target sets".
type StackEvidence struct {
	KnownPrefix   []StackFrameContribution
	Contributors  []StackFrameContribution
	UnknownCauses []StackCauseStep
	UnknownPaths  []StackUnknownPath
}

// cloneStackEvidence detaches all evidence slices for producer/consumer ownership.
// Rules: rules/compiler/compiler_analysis.md — immutable analysis results;
// rules/analysis/stack_analysis.md — "Partial information".
func cloneStackEvidence(evidence StackEvidence) StackEvidence {
	evidence.KnownPrefix = append([]StackFrameContribution(nil), evidence.KnownPrefix...)
	evidence.Contributors = append([]StackFrameContribution(nil), evidence.Contributors...)
	evidence.UnknownCauses = append([]StackCauseStep(nil), evidence.UnknownCauses...)
	evidence.UnknownPaths = append([]StackUnknownPath(nil), evidence.UnknownPaths...)
	for i := range evidence.UnknownPaths {
		evidence.UnknownPaths[i].Frames = append([]StackFrameContribution(nil), evidence.UnknownPaths[i].Frames...)
	}
	return evidence
}

// mergeStackEvidence retains independent contributors and unknown boundaries
// without changing the chosen prefix or promoting partial facts to a proof.
// Sorting and deduplication make shared DAG paths and target order immaterial.
// Rules: rules/analysis/stack_analysis.md — "Partial information", "Determinism",
// "Closed indirect-call target sets", and "Stack cause paths".
func mergeStackEvidence(left, right StackEvidence) StackEvidence {
	left = cloneStackEvidence(left)
	frames := map[CallableID]StackFrameContribution{}
	for _, contribution := range append(left.Contributors, right.Contributors...) {
		frames[contribution.Callable] = contribution
	}
	left.Contributors = nil
	for _, contribution := range frames {
		left.Contributors = append(left.Contributors, contribution)
	}
	sort.Slice(left.Contributors, func(i, j int) bool { return left.Contributors[i].Callable < left.Contributors[j].Callable })
	causes := append(left.UnknownCauses, right.UnknownCauses...)
	sort.Slice(causes, func(i, j int) bool {
		a, b := causes[i], causes[j]
		if a.Source.File != b.Source.File {
			return a.Source.File < b.Source.File
		}
		if a.Source.Line != b.Source.Line {
			return a.Source.Line < b.Source.Line
		}
		if a.Source.Column != b.Source.Column {
			return a.Source.Column < b.Source.Column
		}
		if a.Callable != b.Callable {
			return a.Callable < b.Callable
		}
		return a.Detail < b.Detail
	})
	left.UnknownCauses = nil
	for _, cause := range causes {
		if len(left.UnknownCauses) == 0 || left.UnknownCauses[len(left.UnknownCauses)-1] != cause {
			left.UnknownCauses = append(left.UnknownCauses, cause)
		}
	}
	paths := map[StackCauseStep]StackUnknownPath{}
	for _, path := range append(left.UnknownPaths, right.UnknownPaths...) {
		previous, exists := paths[path.Boundary]
		if !exists || stackFramePathLess(path.Frames, previous.Frames) {
			paths[path.Boundary] = path
		}
	}
	left.UnknownPaths = nil
	for _, cause := range left.UnknownCauses {
		if path, exists := paths[cause]; exists {
			path.Frames = append([]StackFrameContribution(nil), path.Frames...)
			left.UnknownPaths = append(left.UnknownPaths, path)
		}
	}
	return left
}

// stackFramePathLess selects a stable representative when shared DAG paths
// reach the same boundary, without enumerating exponentially many paths.
// Rules: rules/analysis/stack_analysis.md — "Determinism" and "Stack cause paths".
func stackFramePathLess(left, right []StackFrameContribution) bool {
	for i := 0; i < len(left) && i < len(right); i++ {
		if left[i].Callable != right[i].Callable {
			return left[i].Callable < right[i].Callable
		}
	}
	return len(left) < len(right)
}

// prependStackEvidence preserves the caller frame on every uncertain callee
// path, including unknown-frame gaps; independent contributors remain separate.
// Rules: rules/analysis/stack_analysis.md — "Partial information" and "Call-path composition".
func prependStackEvidence(frame StackFrameContribution, evidence StackEvidence) StackEvidence {
	evidence = cloneStackEvidence(evidence)
	for i := range evidence.UnknownPaths {
		evidence.UnknownPaths[i].Frames = append([]StackFrameContribution{frame}, evidence.UnknownPaths[i].Frames...)
	}
	return evidence
}

// stackBoundaryEvidence records a sourced uncertainty and its representative
// frame chain without changing the independent total bound.
// Rules: rules/analysis/stack_analysis.md — "Partial information" and "Unknown".
func stackBoundaryEvidence(frames []StackFrameContribution, boundary StackCauseStep) StackEvidence {
	return StackEvidence{UnknownCauses: []StackCauseStep{boundary}, UnknownPaths: []StackUnknownPath{{Frames: append([]StackFrameContribution(nil), frames...), Boundary: boundary}}}
}
