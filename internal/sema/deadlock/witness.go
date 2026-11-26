package deadlock

import (
	"fmt"
	"sort"
	"strings"
)

// representativeCycle first finds the strongest minimum edge proof admitting
// any cycle, rather than upgrading a weak cycle from one strong seed edge.
// Within that scope it favors short, precise, source-backed, low-uncertainty
// explanations. At most eight deterministic seeds per cyclic subcomponent
// bound the search; §60(4) does not require a globally shortest cycle. Every
// seed gets a shortest return path; no source or runtime proof is invented.
// Rules: rules/analysis/deadlock_analysis.md §§8,49-51,59-60,82.
func representativeCycle(nodes []NodeID, internal []int, all []Edge, evidence map[EdgeID]EdgeEvidence) []EdgeID {
	index := make(map[NodeID]int, len(nodes))
	for i, node := range nodes {
		index[node] = i
	}
	for threshold := 2; threshold >= 0; threshold-- {
		var edges []Edge
		out, reverse := make([][]int, len(nodes)), make([][]int, len(nodes))
		for _, id := range internal {
			edge := all[id]
			if proofRank(evidence[edge.ID].Strength) < threshold {
				continue
			}
			n := len(edges)
			edges = append(edges, edge)
			from, to := index[edge.From], index[edge.To]
			out[from], reverse[to] = append(out[from], n), append(reverse[to], from)
		}
		groups := components(nodes, edges, out, reverse, index)
		membership := make([]int, len(nodes))
		for group, members := range groups {
			for _, node := range members {
				membership[node] = group
			}
		}
		buckets := make([][]int, len(groups))
		for id, edge := range edges {
			from, to := membership[index[edge.From]], membership[index[edge.To]]
			if from == to {
				buckets[from] = append(buckets[from], id)
			}
		}
		visited, previous := make([]bool, len(nodes)), make([]int, len(nodes))
		seedQuality := make([]witnessQuality, len(edges))
		for i, edge := range edges {
			seedQuality[i] = quality([]EdgeID{edge.ID}, evidence)
		}
		var best []EdgeID
		var bestQuality witnessQuality
		for group, seeds := range buckets {
			if len(seeds) == 0 {
				continue
			}
			sort.Slice(seeds, func(i, j int) bool {
				a, b := edges[seeds[i]], edges[seeds[j]]
				// A self-edge already is a one-edge closed candidate.
				if (a.From == a.To) != (b.From == b.To) {
					return a.From == a.To
				}
				return seedQuality[seeds[i]].less(seedQuality[seeds[j]])
			})
			if len(seeds) > 8 {
				seeds = seeds[:8]
			}
			for _, seed := range seeds {
				cycle := cycleWitness(group, seed, edges, out, index, membership, visited, previous)
				cycle = canonicalCycle(cycle)
				q := quality(cycle, evidence)
				if best == nil || q.less(bestQuality) {
					best, bestQuality = cycle, q
				}
			}
		}
		if best != nil {
			return best
		}
	}
	return nil
}

// proofRank orders producer dependency evidence; it never classifies a
// candidate as ProvenDeadlock, which requires simultaneous realizability.
// Rules: rules/analysis/deadlock_analysis.md §§8,50-51.
func proofRank(strength ProofStrength) int {
	switch strength {
	case ProvenPossible:
		return 2
	case ConservativePossible:
		return 1
	default:
		return 0
	}
}

type witnessQuality struct {
	proof, length, imprecise, missingSource, uncertainty int
	sources, identity                                    string
}

// quality compares complete cycles using their weakest edge proof. Omitted
// metadata is unknown; source positions and identity precision never upgrade
// proof strength. Duplicate/reordered uncertainty labels cannot affect choice.
// Rules: rules/analysis/deadlock_analysis.md §§49-51,60,82.
func quality(cycle []EdgeID, evidence map[EdgeID]EdgeEvidence) witnessQuality {
	q := witnessQuality{proof: 2, length: len(cycle)}
	var sources []string
	for _, id := range cycle {
		fact := evidence[id]
		if rank := proofRank(fact.Strength); rank < q.proof {
			q.proof = rank
		}
		if !fact.PreciseIdentity {
			q.imprecise++
		}
		if fact.File == "" || fact.Line <= 0 || fact.Column <= 0 {
			q.missingSource++
		} else {
			sources = append(sources, fmt.Sprintf("%s:%020d:%020d", fact.File, fact.Line, fact.Column))
		}
		uncertain := map[string]bool{}
		for _, reason := range fact.Uncertainty {
			uncertain[reason] = true
		}
		q.uncertainty += len(uncertain)
	}
	sort.Strings(sources)
	q.sources, q.identity = strings.Join(sources, "\x00"), strings.Join(edgeStrings(cycle), "\x00")
	return q
}

// less implements deterministic explanation preferences after proof strength;
// no preference creates a semantic proof or removes another dependency.
// Rules: rules/analysis/deadlock_analysis.md §§60,82.
func (a witnessQuality) less(b witnessQuality) bool {
	if a.proof != b.proof {
		return a.proof > b.proof
	}
	left, right := []int{a.length, a.imprecise, a.missingSource, a.uncertainty}, []int{b.length, b.imprecise, b.missingSource, b.uncertainty}
	for i := range left {
		if left[i] != right[i] {
			return left[i] < right[i]
		}
	}
	if a.sources != b.sources {
		return a.sources < b.sources
	}
	return a.identity < b.identity
}

// canonicalCycle rotates a witness to its smallest producer edge ID, retaining
// edge direction and closure while removing the selected seed from identity.
// Rules: rules/analysis/deadlock_analysis.md §§60,82.
func canonicalCycle(cycle []EdgeID) []EdgeID {
	first := 0
	for i := range cycle {
		if cycle[i] < cycle[first] {
			first = i
		}
	}
	return append(append([]EdgeID(nil), cycle[first:]...), cycle[:first]...)
}

// edgeStrings retains opaque edge IDs as deterministic comparison keys without
// interpreting source names or inferring a stronger dependency relationship.
// Rules: rules/analysis/deadlock_analysis.md §§49-50,60,82.
func edgeStrings(ids []EdgeID) []string {
	result := make([]string, len(ids))
	for i, id := range ids {
		result[i] = string(id)
	}
	return result
}
