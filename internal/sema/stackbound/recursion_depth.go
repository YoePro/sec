package stackbound

import (
	"errors"
	"math/big"
)

// DepthKind classifies a depth proof independently of stack bytes.
// Rules: rules/analysis/stack_analysis.md — "Recursion-depth classification".
type DepthKind string

const (
	DepthExact      DepthKind = "ExactDepth"
	DepthUpperBound DepthKind = "UpperBoundDepth"
	DepthUnknown    DepthKind = "UnknownDepth"
	DepthUnbounded  DepthKind = "UnboundedDepth"
)

// Depth is an immutable producer-supplied depth fact. Its scope,
// initial inputs and covered recursive transitions must be established by the
// producer. This representation neither infers depth nor converts it to bytes.
// Only finite facts carry a count; the zero value is UnknownDepth.
// Rules: rules/analysis/stack_analysis.md — "Recursion-depth classification",
// "Finite recursion and finite global bounds", and "Recursive progress must cover relevant transitions".
type Depth struct {
	kind  DepthKind
	count string
}

// NewExactDepth records a producer's exact nonnegative depth count.
// Rules: rules/analysis/stack_analysis.md — "Recursion-depth classification".
func NewExactDepth(count *big.Int) (Depth, error) {
	return newFiniteDepth(DepthExact, count)
}

// NewUpperDepth records a proven finite upper bound without promoting
// it to an exact fact, including when its count is zero.
// Rules: rules/analysis/stack_analysis.md — "Recursion-depth classification" and "Sources of finite recursion bounds".
func NewUpperDepth(count *big.Int) (Depth, error) {
	return newFiniteDepth(DepthUpperBound, count)
}

// newFiniteDepth rejects absent/negative counts and preserves arbitrary
// precision without aliasing or turning representation overflow into a proof
// of unbounded recursion.
// Rules: rules/analysis/stack_analysis.md — "Recursion-depth classification" and "Proven unbounded recursion".
func newFiniteDepth(kind DepthKind, count *big.Int) (Depth, error) {
	if count == nil || count.Sign() < 0 {
		return Depth{}, errors.New("finite recursion depth requires a nonnegative count")
	}
	return Depth{kind: kind, count: count.String()}, nil
}

// UnknownDepth records absence of a proven finite maximum. Detecting
// recursion or a decreasing measure alone cannot establish unbounded depth.
// Rules: rules/analysis/stack_analysis.md — "Unknown recursion depth" and "Finite recursion and finite global bounds".
func UnknownDepth() Depth { return Depth{} }

// UnboundedDepth records a producer's proof of arbitrarily large
// reachable recursive depth under the relevant execution model.
// Rules: rules/analysis/stack_analysis.md — "Proven unbounded recursion".
func UnboundedDepth() Depth {
	return Depth{kind: DepthUnbounded}
}

// Kind returns the depth classification, normalizing zero values to UnknownDepth.
// Rules: rules/analysis/stack_analysis.md — "Recursion-depth classification" and "Unknown recursion depth".
func (depth Depth) Kind() DepthKind {
	if depth.kind == "" {
		return DepthUnknown
	}
	return depth.kind
}

// Count returns a detached finite depth count. Unknown and Unbounded have no
// numeric count and must not be consumed as zero-depth proofs.
// Rules: rules/analysis/stack_analysis.md — "Recursion-depth classification".
func (depth Depth) Count() (*big.Int, bool) {
	if depth.Kind() != DepthExact && depth.Kind() != DepthUpperBound {
		return nil, false
	}
	count, ok := new(big.Int).SetString(depth.count, 10)
	return count, ok
}

// String preserves proof quality and keeps absent counts distinct from zero.
// Rules: rules/analysis/stack_analysis.md — "Recursion-depth classification".
func (depth Depth) String() string {
	if depth.Kind() == DepthExact || depth.Kind() == DepthUpperBound {
		return string(depth.Kind()) + "(" + depth.count + ")"
	}
	return string(depth.Kind())
}
