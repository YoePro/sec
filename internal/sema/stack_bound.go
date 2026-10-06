package sema

import (
	"errors"
	"math/big"
)

// StackBoundKind records the proof quality of a stack requirement. A bound is
// meaningful only in the scope, analysis level and CompilationPlan supplied by
// its producer; this representation does not establish those proofs itself.
// Rules: rules/analysis/stack_analysis.md — "Stack-bound classification",
// "Stack analysis levels", and "CompilationPlan dependence".
type StackBoundKind string

const (
	StackBoundExact      StackBoundKind = "Exact"
	StackBoundUpperBound StackBoundKind = "UpperBound"
	StackBoundUnknown    StackBoundKind = "Unknown"
	StackBoundUnbounded  StackBoundKind = "Unbounded"
)

// StackBound is an immutable stack requirement. Only finite bounds carry a
// byte count. Decimal storage preserves arbitrarily large counts without host
// integer overflow or mutable big.Int aliasing. The zero value is Unknown.
// Rules: rules/analysis/stack_analysis.md — "Stack-bound classification",
// "Unknown", "Unbounded", and "Partial information".
type StackBound struct {
	kind  StackBoundKind
	bytes string
}

// NewExactStackBound records a producer's proven exact nonnegative byte count.
// Nil and negative counts are rejected rather than becoming zero or Unbounded.
// Rules: rules/analysis/stack_analysis.md — "Exact" and "Unbounded".
func NewExactStackBound(bytes *big.Int) (StackBound, error) {
	return newFiniteStackBound(StackBoundExact, bytes)
}

// NewUpperStackBound records a producer's proven finite upper byte bound,
// retaining UpperBound proof quality even when the byte count is zero.
// Rules: rules/analysis/stack_analysis.md — "UpperBound".
func NewUpperStackBound(bytes *big.Int) (StackBound, error) {
	return newFiniteStackBound(StackBoundUpperBound, bytes)
}

// newFiniteStackBound validates and detaches a finite byte count. Arbitrary
// precision prevents representational overflow from fabricating unboundedness.
// Rules: rules/analysis/stack_analysis.md — "Stack-bound classification",
// "Exact", "UpperBound", and "Unbounded".
func newFiniteStackBound(kind StackBoundKind, bytes *big.Int) (StackBound, error) {
	if bytes == nil || bytes.Sign() < 0 {
		return StackBound{}, errors.New("finite stack bound requires a nonnegative byte count")
	}
	return StackBound{kind: kind, bytes: bytes.String()}, nil
}

// UnknownStackBound records that no finite upper bound has been proved.
// Rules: rules/analysis/stack_analysis.md — "Unknown".
func UnknownStackBound() StackBound { return StackBound{} }

// UnboundedStackBound records a producer's proof of arbitrarily large reachable
// stack demand. It must not be used merely because recursion or a call is unknown.
// Rules: rules/analysis/stack_analysis.md — "Unbounded", "Unknown recursion depth",
// and "Proven unbounded recursion".
func UnboundedStackBound() StackBound { return StackBound{kind: StackBoundUnbounded} }

// Kind returns the canonical classification, including Unknown for zero values.
// Rules: rules/analysis/stack_analysis.md — "Stack-bound classification" and "Unknown".
func (bound StackBound) Kind() StackBoundKind {
	if bound.kind == "" {
		return StackBoundUnknown
	}
	return bound.kind
}

// Bytes returns a fresh nonnegative byte count only for Exact and UpperBound.
// Unknown and Unbounded return nil, false, never a numeric zero or infinity.
// Rules: rules/analysis/stack_analysis.md — "Exact", "UpperBound", "Unknown",
// "Unbounded", and "Partial information".
func (bound StackBound) Bytes() (*big.Int, bool) {
	if bound.Kind() != StackBoundExact && bound.Kind() != StackBoundUpperBound {
		return nil, false
	}
	bytes, ok := new(big.Int).SetString(bound.bytes, 10)
	return bytes, ok
}

// String preserves proof quality when displaying a bound; uncertainty is never
// displayed as zero bytes. Messages and recommendations belong to consumers.
// Rules: rules/analysis/stack_analysis.md — "Stack-bound classification".
func (bound StackBound) String() string {
	if bound.Kind() == StackBoundExact || bound.Kind() == StackBoundUpperBound {
		return string(bound.Kind()) + "(" + bound.bytes + ")"
	}
	return string(bound.Kind())
}
