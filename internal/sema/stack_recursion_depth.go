package sema

import (
	"errors"
	"math/big"
)

// RecursionDepthKind classifies a depth proof independently of stack bytes.
// Rules: rules/analysis/stack_analysis.md — "Recursion-depth classification".
type RecursionDepthKind string

const (
	RecursionDepthExact      RecursionDepthKind = "ExactDepth"
	RecursionDepthUpperBound RecursionDepthKind = "UpperBoundDepth"
	RecursionDepthUnknown    RecursionDepthKind = "UnknownDepth"
	RecursionDepthUnbounded  RecursionDepthKind = "UnboundedDepth"
)

// RecursionDepthBound is an immutable producer-supplied depth fact. Its scope,
// initial inputs and covered recursive transitions must be established by the
// producer. This representation neither infers depth nor converts it to bytes.
// Only finite facts carry a count; the zero value is UnknownDepth.
// Rules: rules/analysis/stack_analysis.md — "Recursion-depth classification",
// "Finite recursion and finite global bounds", and "Recursive progress must cover relevant transitions".
type RecursionDepthBound struct {
	kind  RecursionDepthKind
	count string
}

// NewExactRecursionDepth records a producer's exact nonnegative depth count.
// Rules: rules/analysis/stack_analysis.md — "Recursion-depth classification".
func NewExactRecursionDepth(count *big.Int) (RecursionDepthBound, error) {
	return newFiniteRecursionDepth(RecursionDepthExact, count)
}

// NewUpperRecursionDepth records a proven finite upper bound without promoting
// it to an exact fact, including when its count is zero.
// Rules: rules/analysis/stack_analysis.md — "Recursion-depth classification" and "Sources of finite recursion bounds".
func NewUpperRecursionDepth(count *big.Int) (RecursionDepthBound, error) {
	return newFiniteRecursionDepth(RecursionDepthUpperBound, count)
}

// newFiniteRecursionDepth rejects absent/negative counts and preserves arbitrary
// precision without aliasing or turning representation overflow into a proof
// of unbounded recursion.
// Rules: rules/analysis/stack_analysis.md — "Recursion-depth classification" and "Proven unbounded recursion".
func newFiniteRecursionDepth(kind RecursionDepthKind, count *big.Int) (RecursionDepthBound, error) {
	if count == nil || count.Sign() < 0 {
		return RecursionDepthBound{}, errors.New("finite recursion depth requires a nonnegative count")
	}
	return RecursionDepthBound{kind: kind, count: count.String()}, nil
}

// UnknownRecursionDepth records absence of a proven finite maximum. Detecting
// recursion or a decreasing measure alone cannot establish unbounded depth.
// Rules: rules/analysis/stack_analysis.md — "Unknown recursion depth" and "Finite recursion and finite global bounds".
func UnknownRecursionDepth() RecursionDepthBound { return RecursionDepthBound{} }

// UnboundedRecursionDepth records a producer's proof of arbitrarily large
// reachable recursive depth under the relevant execution model.
// Rules: rules/analysis/stack_analysis.md — "Proven unbounded recursion".
func UnboundedRecursionDepth() RecursionDepthBound {
	return RecursionDepthBound{kind: RecursionDepthUnbounded}
}

// Kind returns the depth classification, normalizing zero values to UnknownDepth.
// Rules: rules/analysis/stack_analysis.md — "Recursion-depth classification" and "Unknown recursion depth".
func (depth RecursionDepthBound) Kind() RecursionDepthKind {
	if depth.kind == "" {
		return RecursionDepthUnknown
	}
	return depth.kind
}

// Count returns a detached finite depth count. Unknown and Unbounded have no
// numeric count and must not be consumed as zero-depth proofs.
// Rules: rules/analysis/stack_analysis.md — "Recursion-depth classification".
func (depth RecursionDepthBound) Count() (*big.Int, bool) {
	if depth.Kind() != RecursionDepthExact && depth.Kind() != RecursionDepthUpperBound {
		return nil, false
	}
	count, ok := new(big.Int).SetString(depth.count, 10)
	return count, ok
}

// String preserves proof quality and keeps absent counts distinct from zero.
// Rules: rules/analysis/stack_analysis.md — "Recursion-depth classification".
func (depth RecursionDepthBound) String() string {
	if depth.Kind() == RecursionDepthExact || depth.Kind() == RecursionDepthUpperBound {
		return string(depth.Kind()) + "(" + depth.count + ")"
	}
	return string(depth.Kind())
}
