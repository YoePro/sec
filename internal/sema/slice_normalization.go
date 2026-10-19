package sema

import "math/big"

// SliceOperand identifies one original source operand, not a synthesized bound
// expression or an additional evaluation. The creation plan retains these in
// source order for later operation producers.
// Rules: rules/mlir/packages/sec-mlir-dialect_package16.md — §§15–17,35–37.
type SliceOperand string

const (
	SliceOperandSource SliceOperand = "source"
	SliceOperandStart  SliceOperand = "start"
	SliceOperandEnd    SliceOperand = "end"
)

// sliceEvaluationOrder schedules the source once and only explicitly present
// endpoints, each once. Omitted bounds are supplied by normalization, never by
// a fabricated expression. Validity and range checks follow operand evaluation.
// Rules: rules/mlir/packages/sec-mlir-dialect_package16.md — §§15–17,35–37.
func sliceEvaluationOrder(startPresent, endPresent bool) []SliceOperand {
	order := []SliceOperand{SliceOperandSource}
	if startPresent {
		order = append(order, SliceOperandStart)
	}
	if endPresent {
		order = append(order, SliceOperandEnd)
	}
	return order
}

// HasValidEvaluationOrder checks that a recorded plan evaluates each original
// operand exactly once in source/start/end order without inventing omitted
// operands. It checks existing facts and performs no inference or evaluation.
// Rules: rules/mlir/packages/sec-mlir-dialect_package16.md — §§15–17,35–37.
func (p ResolvedSliceCreationPlan) HasValidEvaluationOrder() bool {
	want := sliceEvaluationOrder(p.Start.Present, p.End.Present)
	if len(p.EvaluationOrder) != len(want) {
		return false
	}
	for index, operand := range want {
		if p.EvaluationOrder[index] != operand {
			return false
		}
	}
	return true
}

// normalizeSliceRange first rejects known invalid endpoints using canonical
// RangeError precedence, then produces detached, exact half-open bounds. Nil
// remains unknown; missing start is zero and missing end uses the source extent.
// Inclusive constants are incremented with arbitrary precision, never a target
// integer operation. Partial facts are not a proof of runtime bounds safety.
// Rules: rules/mlir/packages/sec-mlir-dialect_package16.md — §§13–19,25;
// rules/collections/collections.md — §§7.4–7.5.
func normalizeSliceRange(r SliceRangeEndpoints) (start, end *big.Int, failure SliceRangeClassification) {
	failure = ClassifySliceRange(r)
	if failure.Failure != SliceRangeValid {
		return nil, nil, failure
	}
	if r.StartOmitted {
		start = new(big.Int)
	} else {
		start = clonePlaceConstantIndex(r.Start)
	}
	if r.EndOmitted {
		end = clonePlaceConstantIndex(r.Length)
	} else {
		end = clonePlaceConstantIndex(r.End)
		if end != nil && r.Inclusive {
			end.Add(end, big.NewInt(1))
		}
	}
	return start, end, failure
}
