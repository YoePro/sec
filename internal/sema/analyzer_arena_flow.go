// Arena epoch state across continuing branches, loop backedges and exits.
// Rules: rules/control-flow/flowcontrol_while.md — §§28–29;
// rules/control-flow/flowcontrol_for.md — §§30–31, 42;
// rules/memory/arena.md — §§4.5, 38;
// rules/corrections/applied/correction25-20260823.md — Parts I–II.
package sema

// copyArenaGenerations detaches per-domain epoch flow state so speculative
// branches and loop replays cannot mutate their entry snapshots.
// Rules: rules/memory/arena.md — §4.5;
// rules/corrections/applied/correction25-20260823.md — Part I, loop-header state.
func copyArenaGenerations(in map[string]int) map[string]int {
	out := make(map[string]int, len(in))
	for name, generation := range in {
		out[name] = generation
	}
	return out
}

// mergeContinuingArenaGenerations joins represented epochs from branches
// that continue, excluding terminated paths from the continuation state.
// Rules: rules/control-flow/flowcontrol_if.md — §§19, 27; rules/memory/arena.md — §38.
func mergeContinuingArenaGenerations(before map[string]int, branches ...branchAnalysis) map[string]int {
	merged := copyArenaGenerations(before)
	for _, branch := range branches {
		if !branch.continues {
			continue
		}
		for name, generation := range branch.arenaGenerations {
			if generation > merged[name] {
				merged[name] = generation
			}
		}
	}
	return merged
}

// loopBackedgeArenaGenerationState joins entry, reachable fallthrough and
// continue epochs for the next loop iteration. Break-only paths are excluded.
// Rules: rules/control-flow/flowcontrol_while.md — §§28–29;
// rules/control-flow/flowcontrol_for.md — §§30–31, 42;
// rules/corrections/applied/correction25-20260823.md — Part I, required corrections 1–4.
func loopBackedgeArenaGenerationState(entry, loop map[string]int, frame loopBreakFrame, bodyFallsThrough bool) map[string]int {
	// rules/control-flow/flowcontrol_while.md and flowcontrol_for.md;
	// correction25.md: the next iteration sees the greatest epoch reachable
	// through entry, normal fallthrough, or any continue edge.
	header := copyArenaGenerations(entry)
	if bodyFallsThrough {
		mergeArenaGenerationMaxInto(header, loop)
	}
	for _, generations := range frame.continueArenaGenerations {
		mergeArenaGenerationMaxInto(header, generations)
	}
	return header
}

// arenaGenerationStatesEqual compares the complete represented epoch
// state so an Arena-only backedge change can trigger next-iteration checking.
// Rules: rules/corrections/applied/correction25-20260823.md — Part I, required correction 3;
// rules/control-flow/flowcontrol_while.md — §28; rules/control-flow/flowcontrol_for.md — §§30–31, 42.
func arenaGenerationStatesEqual(left, right map[string]int) bool {
	if len(left) != len(right) {
		return false
	}
	for name, generation := range left {
		if right[name] != generation {
			return false
		}
	}
	return true
}

// mergeLoopArenaGenerations joins the represented entry, loop and break
// epochs at a conservative loop exit. Reachability selection belongs to the
// caller; this helper does not create a next-iteration edge from a break.
// Rules: rules/control-flow/flowcontrol_while.md — §§28–29;
// rules/control-flow/flowcontrol_for.md — §§30–31, 42; rules/memory/arena.md — §38.
func mergeLoopArenaGenerations(before, loop map[string]int, breaks []map[string]int) map[string]int {
	merged := copyArenaGenerations(before)
	mergeArenaGenerationMaxInto(merged, loop)
	for _, breakGenerations := range breaks {
		mergeArenaGenerationMaxInto(merged, breakGenerations)
	}
	return merged
}

// mergeArenaGenerationMaxInto preserves the greatest represented epoch
// on a flow join so earlier references cannot become current through merging.
// The integer approximation does not implement runtime epoch rekey/retirement.
// Rules: rules/memory/arena.md — §38(1–6);
// rules/corrections/applied/correction25-20260823.md — Part I, loop-header state.
func mergeArenaGenerationMaxInto(merged, next map[string]int) {
	for name, generation := range next {
		if current, ok := merged[name]; !ok || generation > current {
			merged[name] = generation
		}
	}
}
