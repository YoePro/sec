package sema

import (
	"fmt"
	"sec/internal/lexer"
	"sort"
	"strings"
)

// StackRecursionExplanation identifies one responsible canonical component,
// its representative entry contribution and optional verified depth source.
// Absence of an attached proof is not a proof of unbounded recursion.
// Rules: rules/analysis/stack_analysis.md — "Stack cause paths", "Unknown recursion depth".
type StackRecursionExplanation struct {
	Members          []CallableID
	Entry            CallableID
	Contribution     StackBound
	HasVerifiedDepth bool
	Depth            RecursionDepthBound
	DepthSource      lexer.Token
}

// StackExplanation is a detached per-callable report. MajorContributors are
// ordered independent frame facts, never a sum of simultaneously live frames.
// Measurement levels and CompilationPlans remain explicit; reports neither
// estimate missing layout nor upgrade Unknown evidence into a finite proof.
// Rules: rules/analysis/stack_analysis.md — "Stack cause paths", "Partial information", "OptionalInsight", "Stack analysis levels".
type StackExplanation struct {
	Callable          CallableID
	CompilationPlanID string
	Level             StackMeasurementLevel
	OwnFrame, Maximum StackBound
	MaximumCause      []StackCauseStep
	MajorContributors []StackFrameContribution
	Evidence          StackEvidence
	Recursion         []StackRecursionExplanation
}

// SemanticExplanations reports every stored semantic summary with its exact
// plan identity and existing canonical cause/evidence. It does not substitute
// machine layout or facts from another plan.
// Rules: rules/analysis/stack_analysis.md — "Semantic stack requirement", "Stack cause paths", "CompilationPlan dependence".
func (store *StackSummaryStore) SemanticExplanations(graph *CallGraph, proofs []*StackRecursionDepthProof) []StackExplanation {
	var reports []StackExplanation
	context := newStackExplanationContext(graph, proofs)
	for _, summary := range store.SemanticSummaries() {
		report := StackExplanation{Callable: summary.Callable, CompilationPlanID: summary.CompilationPlanID, Level: StackMeasurementSemantic, OwnFrame: summary.OwnFrame, Maximum: summary.TransitiveMaximum, MaximumCause: summary.MaximumCause, Evidence: summary.Evidence}
		reports = append(reports, explainStackSummary(context, report, proofs, func(id CallableID, plan string) StackBound {
			summary, _ := store.Semantic(id, plan)
			return summary.TransitiveMaximum
		}))
	}
	return reports
}

// MachineExplanations keeps ABI/backend frame authority independent of
// semantic estimates and consumes only exact machine-plan summary entries.
// Rules: rules/analysis/stack_analysis.md — "Machine stack requirement", "Stack cause paths", "Semantic and machine frame authority".
func (store *StackSummaryStore) MachineExplanations(graph *CallGraph, proofs []*StackRecursionDepthProof) []StackExplanation {
	var reports []StackExplanation
	context := newStackExplanationContext(graph, proofs)
	for _, summary := range store.MachineSummaries() {
		report := StackExplanation{Callable: summary.Callable, CompilationPlanID: summary.CompilationPlanID, Level: StackMeasurementMachine, OwnFrame: summary.OwnFrame, Maximum: summary.TransitiveMaximum, MaximumCause: summary.MaximumCause, Evidence: summary.Evidence}
		reports = append(reports, explainStackSummary(context, report, proofs, func(id CallableID, plan string) StackBound {
			summary, _ := store.Machine(id, plan)
			return summary.TransitiveMaximum
		}))
	}
	return reports
}

// explainStackSummary presents the existing deterministic cause path, ranks
// independent contributors and attaches only SCCs referenced by this summary's
// cause/evidence. It consumes canonical graph facts without walking AST or
// reconstructing reachability. Depth evidence must match graph, level and plan.
// Rules: rules/analysis/stack_analysis.md — "Stack cause paths", "Partial information", "Determinism", "Recursive SCC stack contribution".
func explainStackSummary(context stackExplanationContext, report StackExplanation, proofs []*StackRecursionDepthProof, maximum func(CallableID, string) StackBound) StackExplanation {
	report.MaximumCause = append([]StackCauseStep(nil), report.MaximumCause...)
	prefix := append([]StackFrameContribution(nil), report.Evidence.KnownPrefix...)
	report.Evidence = mergeStackEvidence(StackEvidence{}, report.Evidence)
	report.Evidence.KnownPrefix = prefix
	report.MajorContributors = append([]StackFrameContribution(nil), report.Evidence.Contributors...)
	sort.Slice(report.MajorContributors, func(i, j int) bool {
		a, b := report.MajorContributors[i], report.MajorContributors[j]
		left, leftKnown := a.Frame.Bytes()
		right, rightKnown := b.Frame.Bytes()
		if leftKnown != rightKnown {
			return leftKnown
		}
		if leftKnown && left.Cmp(right) != 0 {
			return left.Cmp(right) > 0
		}
		if a.Frame.Kind() != b.Frame.Kind() {
			return a.Frame.Kind() < b.Frame.Kind()
		}
		if a.Callable != b.Callable {
			return a.Callable < b.Callable
		}
		if a.Source.File != b.Source.File {
			return a.Source.File < b.Source.File
		}
		if a.Source.Line != b.Source.Line {
			return a.Source.Line < b.Source.Line
		}
		return a.Source.Column < b.Source.Column
	})
	referenced := map[CallableID]bool{report.Callable: true}
	for _, cause := range report.MaximumCause {
		referenced[cause.Callable] = true
	}
	for _, frame := range report.Evidence.Contributors {
		referenced[frame.Callable] = true
	}
	for _, cause := range report.Evidence.UnknownCauses {
		referenced[cause.Callable] = true
	}
	relevant := map[int]bool{}
	for id := range referenced {
		if index, ok := context.byMember[id]; ok {
			relevant[index] = true
		}
	}
	order := make([]int, 0, len(relevant))
	for index := range relevant {
		order = append(order, index)
	}
	sort.Ints(order)
	for _, index := range order {
		component := context.components[index]
		members := map[CallableID]bool{}
		entry := CallableID("")
		for _, member := range component.Members {
			members[member] = true
			if entry == "" && referenced[member] {
				entry = member
			}
		}
		if entry == "" {
			continue
		}
		for _, cause := range report.MaximumCause {
			if members[cause.Callable] {
				entry = cause.Callable
				break
			}
		}
		explanation := StackRecursionExplanation{Members: append([]CallableID(nil), component.Members...), Entry: entry, Contribution: maximum(entry, report.CompilationPlanID), Depth: UnknownRecursionDepth()}
		if proof := matchingStackRecursionProof(context.graph, context.stamp, component.Members, report.Level, report.CompilationPlanID, proofs); proof != nil {
			explanation.HasVerifiedDepth = true
			explanation.Depth = proof.depth
			explanation.DepthSource = proof.source
		}
		report.Recursion = append(report.Recursion, explanation)
	}
	return report
}

// stackExplanationContext indexes the canonical component view once per batch,
// avoiding repeated graph traversal or snapshot hashing for every report.
type stackExplanationContext struct {
	graph      *CallGraph
	stamp      string
	components []SameStackComponent
	byMember   map[CallableID]int
}

// newStackExplanationContext shares immutable graph facts across one reporting
// batch while preserving exact snapshot qualification of attached depth proofs.
// Rules: rules/analysis/stack_analysis.md — "Call graph ownership", "Stack cause paths", "CompilationPlan dependence".
func newStackExplanationContext(graph *CallGraph, proofs []*StackRecursionDepthProof) stackExplanationContext {
	context := stackExplanationContext{graph: graph, components: graph.SameStackComponents(), byMember: map[CallableID]int{}}
	if len(proofs) != 0 {
		context.stamp = stackRecursionGraphStamp(graph)
	}
	for index, component := range context.components {
		if !component.Recursive {
			continue
		}
		for _, member := range component.Members {
			context.byMember[member] = index
		}
	}
	return context
}

// FormatStackExplanation renders deterministic mentor/report text from the
// existing explanation, preserving proof classes and source navigation. It
// never treats independent contributors as a simultaneous total or missing
// depth evidence as proof of unboundedness. Graph labels are presentation only.
// Rules: rules/analysis/stack_analysis.md — "Stack cause paths", "Partial information", "OptionalInsight", "Determinism".
func FormatStackExplanation(graph *CallGraph, report StackExplanation) string {
	label := func(id CallableID) string {
		if graph != nil {
			if node, ok := graph.Node(id); ok && node.Name != "" {
				if node.Module != "" {
					return node.Module + "::" + node.Name
				}
				return node.Name
			}
		}
		return string(id)
	}
	source := func(token lexer.Token) string {
		if token.Line == 0 {
			return ""
		}
		if token.File == "" {
			return fmt.Sprintf(" @ line %d:%d", token.Line, token.Column)
		}
		return fmt.Sprintf(" @ %s:%d:%d", token.File, token.Line, token.Column)
	}
	var out strings.Builder
	fmt.Fprintf(&out, "Stack %s [%s; plan=%q]\nown: %s; maximum: %s\n", label(report.Callable), report.Level, report.CompilationPlanID, report.OwnFrame, report.Maximum)
	for _, cause := range report.MaximumCause {
		fmt.Fprintf(&out, "  -> %s: %s%s\n", label(cause.Callable), cause.Detail, source(cause.Source))
	}
	for _, recursive := range report.Recursion {
		names := make([]string, len(recursive.Members))
		for i, id := range recursive.Members {
			names[i] = label(id)
		}
		fmt.Fprintf(&out, "recursion SCC [%s]; entry=%s; contribution=%s\n", strings.Join(names, ", "), label(recursive.Entry), recursive.Contribution)
		if recursive.HasVerifiedDepth {
			fmt.Fprintf(&out, "  depth: %s%s\n", recursive.Depth, source(recursive.DepthSource))
		} else {
			out.WriteString("  depth: Unknown (no matching attached verified depth proof)\n")
		}
	}
	if len(report.MajorContributors) > 0 {
		out.WriteString("major independent frames (not a simultaneous total):\n")
	}
	for _, frame := range report.MajorContributors {
		fmt.Fprintf(&out, "  %s: %s%s\n", label(frame.Callable), frame.Frame, source(frame.Source))
	}
	for _, cause := range report.Evidence.UnknownCauses {
		fmt.Fprintf(&out, "unproven boundary: %s: %s%s\n", label(cause.Callable), cause.Detail, source(cause.Source))
	}
	for _, path := range report.Evidence.UnknownPaths {
		out.WriteString("  unknown path:")
		for _, frame := range path.Frames {
			fmt.Fprintf(&out, " %s(%s) ->", label(frame.Callable), frame.Frame)
		}
		fmt.Fprintf(&out, " %s: %s%s\n", label(path.Boundary.Callable), path.Boundary.Detail, source(path.Boundary.Source))
	}
	return out.String()
}
