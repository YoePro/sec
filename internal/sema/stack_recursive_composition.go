package sema

import (
	"fmt"
	"math/big"
)

const maxRecursiveStackWork = 1 << 20
const maxRecursiveStackDepthBits = 4096

// composeRecursiveStackComponent computes an upper bound for every entry to a
// canonical SCC. For at most N live SCC frames, R_i(N) is the maximum of its
// nonrecursive base and Own_i + R_j(N-1) over actual internal edges. Distinct
// cycles retain their different costs; outgoing calls/runtime paths are base
// contributions and are not multiplied once per simultaneously active frame.
// Path correlation may overestimate demand, so even ExactDepth yields UpperBound.
// Rules: rules/analysis/stack_analysis.md — "Recursive SCC stack contribution",
// "Control-flow composition", "UpperBound", "Partial information", "Determinism".
func composeRecursiveStackComponent(graph *CallGraph, proof *StackRecursionDepthProof, own map[CallableID]StackBound, bases map[CallableID]stackCompositionValue) map[CallableID]stackCompositionValue {
	members := stackRecursionMembers(proof)
	count, _ := proof.depth.Count()
	n := len(members)
	withinBudget := n <= 64 && count.BitLen() <= maxRecursiveStackDepthBits && count.BitLen() <= maxRecursiveStackWork/n/n/n
	index := map[CallableID]int{}
	vector := make([]*big.Int, n)
	matrix := make(stackMaxPlusMatrix, n)
	var evidence StackEvidence
	usable := true
	for i, member := range members {
		index[member] = i
		if withinBudget {
			matrix[i] = make([]*big.Int, n)
			matrix[i][i] = new(big.Int) // May stop before the depth limit.
		}
		var finite bool
		vector[i], finite = bases[member].bound.Bytes()
		_, frameFinite := own[member].Bytes()
		usable = usable && finite && frameFinite
		evidence = mergeStackEvidence(evidence, bases[member].evidence)
	}
	if usable && withinBudget {
		for i, member := range members {
			weight, _ := own[member].Bytes()
			for _, target := range graph.sameStackTargets(member) {
				if j, internal := index[target]; internal {
					matrix[i][j] = new(big.Int).Set(weight)
				}
			}
		}
		// Binary exponentiation avoids unrolling source recursion or allocating
		// one frame record for each invocation, including counts beyond uint64.
		exponent := new(big.Int).Sub(count, big.NewInt(1))
		for exponent.Sign() != 0 {
			if exponent.Bit(0) != 0 {
				vector = matrix.apply(vector)
			}
			exponent.Rsh(exponent, 1)
			if exponent.Sign() != 0 {
				matrix = matrix.square()
			}
		}
	}
	results := map[CallableID]stackCompositionValue{}
	for i, member := range members {
		node, _ := graph.Node(member)
		result := stackCompositionValue{bound: UnknownStackBound(), evidence: cloneStackEvidence(evidence)}
		detail := "recursive SCC has an unknown frame or external contribution"
		if !withinBudget {
			detail = "recursive SCC stack-composition resource limit exceeded"
		} else if usable {
			detail = fmt.Sprintf("recursive SCC upper bound with at most %s simultaneously live component frames", count.String())
		}
		boundary := StackCauseStep{Callable: member, Source: proof.source, Detail: detail}
		result.cause = []StackCauseStep{boundary}
		for _, componentMember := range members {
			result.cause = append(result.cause, StackCauseStep{Callable: componentMember, Detail: "recursive component own frame: " + own[componentMember].String()})
		}
		frame := StackFrameContribution{Callable: member, Source: node.Declaration, Frame: own[member]}
		result.evidence.KnownPrefix = nil
		if _, finite := own[member].Bytes(); finite {
			result.evidence.KnownPrefix = []StackFrameContribution{frame}
		}
		if usable && withinBudget {
			result.bound, _ = NewUpperStackBound(vector[i])
		} else {
			result.evidence = mergeStackEvidence(result.evidence, stackBoundaryEvidence([]StackFrameContribution{frame}, boundary))
		}
		results[member] = result
	}
	return results
}

// stackMaxPlusMatrix is a weighted projection of the canonical SCC, never a
// separately discovered call graph. Nil is an absent edge; zero is a valid cost.
// Rules: rules/analysis/stack_analysis.md — "Recursive SCC stack contribution", "Call graph ownership".
type stackMaxPlusMatrix [][]*big.Int

// apply composes one bounded segment with a vector of terminating contributions.
// Rules: rules/analysis/stack_analysis.md — "Recursive SCC stack contribution".
func (matrix stackMaxPlusMatrix) apply(vector []*big.Int) []*big.Int {
	result := make([]*big.Int, len(matrix))
	for i, row := range matrix {
		for j, edge := range row {
			if edge == nil || vector[j] == nil {
				continue
			}
			candidate := new(big.Int).Add(edge, vector[j])
			if result[i] == nil || candidate.Cmp(result[i]) > 0 {
				result[i] = candidate
			}
		}
	}
	return result
}

// square doubles the maximum admitted segment length using arbitrary precision.
// Rules: rules/analysis/stack_analysis.md — "Recursive SCC stack contribution",
// "Required performance and termination tests".
func (matrix stackMaxPlusMatrix) square() stackMaxPlusMatrix {
	result := make(stackMaxPlusMatrix, len(matrix))
	for i := range result {
		result[i] = make([]*big.Int, len(matrix))
		for j := range result[i] {
			for k := range matrix {
				if matrix[i][k] == nil || matrix[k][j] == nil {
					continue
				}
				candidate := new(big.Int).Add(matrix[i][k], matrix[k][j])
				if result[i][j] == nil || candidate.Cmp(result[i][j]) > 0 {
					result[i][j] = candidate
				}
			}
		}
	}
	return result
}
