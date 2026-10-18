package sema

import (
	"math/big"
	"sec/internal/ast"
)

// SliceRangeFailure is the canonical RangeError classification of one slice
// range (package16.md § 18). The empty value means the range is valid for the
// known facts.
type SliceRangeFailure string

const (
	SliceRangeValid         SliceRangeFailure = ""
	SliceRangeInvalidRange  SliceRangeFailure = "InvalidRange"
	SliceRangeStartAfterEnd SliceRangeFailure = "StartAfterEnd"
	SliceRangeOutOfBounds   SliceRangeFailure = "OutOfBounds"
)

// SliceRangeCause names which endpoint condition produced a failure, so a
// diagnostic can point at the responsible bound.
type SliceRangeCause string

const (
	SliceRangeCauseNone              SliceRangeCause = ""
	SliceRangeCauseNegativeStart     SliceRangeCause = "negative-start"
	SliceRangeCauseNegativeEnd       SliceRangeCause = "negative-end"
	SliceRangeCauseStartAfterEnd     SliceRangeCause = "start-after-end"
	SliceRangeCauseStartPastLength   SliceRangeCause = "start-past-length"
	SliceRangeCauseExclusiveEndPast  SliceRangeCause = "exclusive-end-past-length"
	SliceRangeCauseInclusiveEndAtLen SliceRangeCause = "inclusive-end-at-or-past-length"
)

// SliceRangeEndpoints are the exact known facts of one slice range. A nil
// Start or End means the bound is omitted (Omitted* set) or not a
// compile-time constant; a nil Length means the source length is not known.
type SliceRangeEndpoints struct {
	Start, End               *big.Int
	StartOmitted, EndOmitted bool
	Inclusive                bool
	Length                   *big.Int
}

// SliceRangeClassification is the first failing condition under the
// canonical precedence, with its cause.
type SliceRangeClassification struct {
	Failure SliceRangeFailure
	Cause   SliceRangeCause
}

// ClassifySliceRange applies the canonical slice-range failure precedence to
// the known endpoint facts with exact arithmetic:
//
//  1. a negative endpoint (start before end) -> InvalidRange;
//  2. an explicit start greater than the explicit end -> StartAfterEnd;
//  3. a source-bound violation — start beyond the length, an exclusive end
//     beyond the length, or an inclusive end at or beyond the length ->
//     OutOfBounds.
//
// The same function classifies a constant range at compile time and defines
// the RangeError a runtime check must produce for the same values, so the
// observable error is independent of optimization. Unknown facts never
// produce a failure.
//
// Rules:
//   - rules/mlir/packages/sec-mlir-dialect_package16.md — §§ 14–19
//   - rules/mlir/normative-sync/sec_slice_sync_v1.md
func ClassifySliceRange(r SliceRangeEndpoints) SliceRangeClassification {
	if r.Start != nil && !r.StartOmitted && r.Start.Sign() < 0 {
		return SliceRangeClassification{SliceRangeInvalidRange, SliceRangeCauseNegativeStart}
	}
	if r.End != nil && !r.EndOmitted && r.End.Sign() < 0 {
		return SliceRangeClassification{SliceRangeInvalidRange, SliceRangeCauseNegativeEnd}
	}
	if r.Start != nil && !r.StartOmitted && r.End != nil && !r.EndOmitted && r.Start.Cmp(r.End) > 0 {
		return SliceRangeClassification{SliceRangeStartAfterEnd, SliceRangeCauseStartAfterEnd}
	}
	if r.Length == nil {
		return SliceRangeClassification{}
	}
	if r.Start != nil && !r.StartOmitted && r.Start.Cmp(r.Length) > 0 {
		return SliceRangeClassification{SliceRangeOutOfBounds, SliceRangeCauseStartPastLength}
	}
	if r.End != nil && !r.EndOmitted {
		if r.Inclusive && r.End.Cmp(r.Length) >= 0 {
			return SliceRangeClassification{SliceRangeOutOfBounds, SliceRangeCauseInclusiveEndAtLen}
		}
		if !r.Inclusive && r.End.Cmp(r.Length) > 0 {
			return SliceRangeClassification{SliceRangeOutOfBounds, SliceRangeCauseExclusiveEndPast}
		}
	}
	return SliceRangeClassification{}
}

// checkSliceBounds reports a compile-time-provable invalid slice range as
// exactly one error: the first failing condition under the canonical
// precedence (negative endpoint, then start after end, then source bound),
// which is also the RangeError the runtime check would produce. The
// start-after-end check applies to every source, not only fixed arrays.
//
// Rules:
//   - rules/mlir/packages/sec-mlir-dialect_package16.md — §§ 18–19
func (a *Analyzer) checkSliceBounds(expr *ast.SliceExpression, typ Type) {
	start, startOK := a.integerConstantValue(expr.Start)
	end, endOK := a.integerConstantValue(expr.End)
	endpoints := SliceRangeEndpoints{StartOmitted: expr.Start == nil, EndOmitted: expr.End == nil, Inclusive: !expr.Exclusive}
	if startOK && expr.Start != nil {
		endpoints.Start = start
	}
	if endOK && expr.End != nil {
		endpoints.End = end
	}
	if typ.Kind == ArrayType {
		if length, fixed := exactFixedArrayLength(typ); fixed {
			endpoints.Length = length
		}
	}
	if typ.Kind == SliceType {
		if source, ok := a.sliceSourcePlace(expr.Left, a.expressionTypes[expr.Left]); ok {
			endpoints.Length = staticSliceExtent(source)
		}
	}
	classification := ClassifySliceRange(endpoints)
	switch classification.Cause {
	case SliceRangeCauseNegativeStart:
		a.addErrorAtToken(expressionToken(expr.Start), "slice lower bound must be non-negative")
	case SliceRangeCauseNegativeEnd:
		a.addErrorAtToken(expressionToken(expr.End), "slice upper bound must be non-negative")
	case SliceRangeCauseStartAfterEnd:
		op := ".."
		if expr.Exclusive {
			op = "..<"
		}
		a.addErrorAtToken(expr.Token, "descending slice range %s%s%s is invalid", start.String(), op, end.String())
	case SliceRangeCauseStartPastLength:
		a.addErrorAtToken(expr.Token, "slice lower bound %s exceeds length %s", start.String(), endpoints.Length.String())
	case SliceRangeCauseExclusiveEndPast:
		a.addErrorAtToken(expr.Token, "exclusive slice upper bound %s exceeds length %s", end.String(), endpoints.Length.String())
	case SliceRangeCauseInclusiveEndAtLen:
		a.addErrorAtToken(expr.Token, "inclusive slice upper bound %s exceeds final index %s", end.String(), new(big.Int).Sub(endpoints.Length, big.NewInt(1)).String())
	}
}
