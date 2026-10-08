// Package stackbound owns the immutable stack-resource proof values: stack
// bounds, recursion-depth bounds and finite stack budgets with their explicit
// comparison. It depends only on arbitrary-precision arithmetic and never
// imports its parent `sema` package; the parent composes bounds over the call
// graph, owns summaries and evidence, and reports diagnostics.
//
// Rules: rules/analysis/stack_analysis.md — "Stack-bound classification",
// "Recursion-depth classification", "Stack budgets", "Budget validation".
package stackbound

import (
	"errors"
	"math/big"
)

// Kind records the proof quality of a stack requirement. A bound is
// meaningful only in the scope, analysis level and CompilationPlan supplied by
// its producer; this representation does not establish those proofs itself.
// Rules: rules/analysis/stack_analysis.md — "Stack-bound classification",
// "Stack analysis levels", and "CompilationPlan dependence".
type Kind string

const (
	Exact      Kind = "Exact"
	UpperBound Kind = "UpperBound"
	Unknown    Kind = "Unknown"
	Unbounded  Kind = "Unbounded"
)

// Bound is an immutable stack requirement. Only finite bounds carry a
// byte count. Decimal storage preserves arbitrarily large counts without host
// integer overflow or mutable big.Int aliasing. The zero value is Unknown.
// Rules: rules/analysis/stack_analysis.md — "Stack-bound classification",
// "Unknown", "Unbounded", and "Partial information".
type Bound struct {
	kind  Kind
	bytes string
}

// NewExact records a producer's proven exact nonnegative byte count.
// Nil and negative counts are rejected rather than becoming zero or Unbounded.
// Rules: rules/analysis/stack_analysis.md — "Exact" and "Unbounded".
func NewExact(bytes *big.Int) (Bound, error) {
	return newFinite(Exact, bytes)
}

// NewUpper records a producer's proven finite upper byte bound,
// retaining UpperBound proof quality even when the byte count is zero.
// Rules: rules/analysis/stack_analysis.md — "UpperBound".
func NewUpper(bytes *big.Int) (Bound, error) {
	return newFinite(UpperBound, bytes)
}

// newFinite validates and detaches a finite byte count. Arbitrary
// precision prevents representational overflow from fabricating unboundedness.
// Rules: rules/analysis/stack_analysis.md — "Stack-bound classification",
// "Exact", "UpperBound", and "Unbounded".
func newFinite(kind Kind, bytes *big.Int) (Bound, error) {
	if bytes == nil || bytes.Sign() < 0 {
		return Bound{}, errors.New("finite stack bound requires a nonnegative byte count")
	}
	return Bound{kind: kind, bytes: bytes.String()}, nil
}

// UnknownBound records that no finite upper bound has been proved.
// Rules: rules/analysis/stack_analysis.md — "Unknown".
func UnknownBound() Bound { return Bound{} }

// UnboundedBound records a producer's proof of arbitrarily large reachable
// stack demand. It must not be used merely because recursion or a call is unknown.
// Rules: rules/analysis/stack_analysis.md — "Unbounded", "Unknown recursion depth",
// and "Proven unbounded recursion".
func UnboundedBound() Bound { return Bound{kind: Unbounded} }

// Kind returns the canonical classification, including Unknown for zero values.
// Rules: rules/analysis/stack_analysis.md — "Stack-bound classification" and "Unknown".
func (bound Bound) Kind() Kind {
	if bound.kind == "" {
		return Unknown
	}
	return bound.kind
}

// Bytes returns a fresh nonnegative byte count only for Exact and UpperBound.
// Unknown and Unbounded return nil, false, never a numeric zero or infinity.
// Rules: rules/analysis/stack_analysis.md — "Exact", "UpperBound", "Unknown",
// "Unbounded", and "Partial information".
func (bound Bound) Bytes() (*big.Int, bool) {
	if bound.Kind() != Exact && bound.Kind() != UpperBound {
		return nil, false
	}
	bytes, ok := new(big.Int).SetString(bound.bytes, 10)
	return bytes, ok
}

// String preserves proof quality when displaying a bound; uncertainty is never
// displayed as zero bytes. Messages and recommendations belong to consumers.
// Rules: rules/analysis/stack_analysis.md — "Stack-bound classification".
func (bound Bound) String() string {
	if bound.Kind() == Exact || bound.Kind() == UpperBound {
		return string(bound.Kind()) + "(" + bound.bytes + ")"
	}
	return string(bound.Kind())
}
