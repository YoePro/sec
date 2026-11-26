package deadlock

import (
	"fmt"
	"sort"
)

// CauseID identifies a shared call-path prefix in the same producer snapshot
// as the dependency graph; it is not a source spelling or global callable ID.
type CauseID string

// CauseFrame adds one canonical caller -> callee invocation to Parent's prefix.
// Recursive callables and repeated call sites are valid at distinct prefix IDs;
// cyclic prefix references are not finite cause paths. Source may be absent,
// independently of canonical identity and dependency proof strength.
// Rules: rules/analysis/deadlock_analysis.md §§49,61,82;
// rules/analysis/call_graph.md — "Call-site record".
type CauseFrame struct {
	ID, Parent               CauseID
	CallSite, Caller, Callee string
	File                     string
	Line, Column             int
	// Execution retains the producer's call execution relationship verbatim.
	Execution string
}

// CycleEdgeDetails exposes only one representative edge and its recorded path,
// ordered outermost call first. Unavailable paths stay explicit; a source label
// or path alone never establishes reachability, simultaneity or deadlock.
// Rules: rules/analysis/deadlock_analysis.md §§8,49-51,61.
type CycleEdgeDetails struct {
	Edge           EdgeID
	Evidence       EdgeEvidence
	CauseAvailable bool
	Calls          []CauseFrame
}

// validateCauses validates shared prefixes before any result is published.
// Caller/callee continuity and all evidence references are checked without
// inferring missing calls; iterative state tracking handles long/reentrant
// paths in linear work after canonical ordering.
// Rules: rules/analysis/deadlock_analysis.md §§49,59,61,82.
func validateCauses(frames []CauseFrame, evidence map[EdgeID]EdgeEvidence) (map[CauseID]CauseFrame, error) {
	ordered := append([]CauseFrame(nil), frames...)
	sort.Slice(ordered, func(i, j int) bool { return ordered[i].ID < ordered[j].ID })
	byID := make(map[CauseID]CauseFrame, len(ordered))
	for _, frame := range ordered {
		if _, duplicate := byID[frame.ID]; duplicate || frame.ID == "" || frame.CallSite == "" || frame.Caller == "" || frame.Callee == "" || frame.Line < 0 || frame.Column < 0 {
			return nil, fmt.Errorf("invalid or duplicate dependency cause frame %q", frame.ID)
		}
		byID[frame.ID] = frame
	}
	states := make(map[CauseID]uint8, len(frames))
	for _, frame := range ordered {
		var path []CauseID
		for id := frame.ID; id != "" && states[id] != 2; {
			if states[id] == 1 {
				return nil, fmt.Errorf("cyclic dependency cause prefix %q", id)
			}
			current, ok := byID[id]
			if !ok {
				return nil, fmt.Errorf("missing dependency cause prefix %q", id)
			}
			if current.Parent != "" {
				parent, ok := byID[current.Parent]
				if !ok {
					return nil, fmt.Errorf("missing dependency cause prefix %q", current.Parent)
				}
				if parent.Callee != current.Caller {
					return nil, fmt.Errorf("disconnected dependency cause frame %q", id)
				}
			}
			states[id] = 1
			path = append(path, id)
			id = current.Parent
		}
		for _, id := range path {
			states[id] = 2
		}
	}
	ids := make([]EdgeID, 0, len(evidence))
	for id := range evidence {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	for _, id := range ids {
		if cause := evidence[id].Cause; cause != "" {
			if _, ok := byID[cause]; !ok {
				return nil, fmt.Errorf("edge %q references missing dependency cause %q", id, cause)
			}
		}
	}
	return byID, nil
}

// retainCycleCauses retains each shared prefix once for all selected edges,
// omitting unrelated/nonrepresentative paths from the compact result. Original
// edge facts remain intact; no proof or call-graph coverage is manufactured.
// Rules: rules/analysis/deadlock_analysis.md §§49,59,61,82.
func retainCycleCauses(components []Component, evidence map[EdgeID]EdgeEvidence, frames map[CauseID]CauseFrame) []CauseFrame {
	retained := map[CauseID]bool{}
	for _, component := range components {
		for _, edge := range component.Cycle {
			for id := evidence[edge].Cause; id != "" && !retained[id]; id = frames[id].Parent {
				retained[id] = true
			}
		}
	}
	var result []CauseFrame
	for id := range retained {
		result = append(result, frames[id])
	}
	sort.Slice(result, func(i, j int) bool { return result[i].ID < result[j].ID })
	return result
}

// CycleDetails expands only the requested witness for detailed navigation.
// Normal output can keep edge IDs and shared prefixes compact. Every edge is
// represented, including ones without a path; returned slices are detached.
// Mutated/malformed result tables fail rather than inventing missing frames.
// Rules: rules/analysis/deadlock_analysis.md §§49,61,82.
func (r Result) CycleDetails(component int) ([]CycleEdgeDetails, error) {
	if component < 0 || component >= len(r.Components) {
		return nil, fmt.Errorf("dependency component %d is unavailable", component)
	}
	selected := map[EdgeID]EdgeEvidence{}
	for _, edge := range r.Components[component].Cycle {
		selected[edge] = r.Evidence[edge]
	}
	frames, err := validateCauses(r.Causes, selected)
	if err != nil {
		return nil, err
	}
	var details []CycleEdgeDetails
	for _, edge := range r.Components[component].Cycle {
		fact := selected[edge]
		fact.Uncertainty = append([]string(nil), fact.Uncertainty...)
		detail := CycleEdgeDetails{Edge: edge, Evidence: fact, CauseAvailable: fact.Cause != ""}
		for id := fact.Cause; id != ""; id = frames[id].Parent {
			detail.Calls = append(detail.Calls, frames[id])
		}
		for i, j := 0, len(detail.Calls)-1; i < j; i, j = i+1, j-1 {
			detail.Calls[i], detail.Calls[j] = detail.Calls[j], detail.Calls[i]
		}
		details = append(details, detail)
	}
	return details, nil
}
