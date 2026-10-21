package secmlir

import (
	"fmt"
	"sec/internal/ir/semantic"
	"strings"
)

// operationAttributes retains common proof, match and try provenance as entries
// so operation-specific metadata can join the same MLIR attribute dictionary.
// Rules: rules/compiler/semantic_ir.md — §124(2–3);
// rules/mlir/dialect-versions/sec_mlir_dialect_v6.md — §12;
// rules/mlir/dialect-versions/sec_mlir_dialect_v8.md — match provenance.
func operationAttributes(operation semantic.Operation, synthesized bool) []string {
	var attributes []string
	attribute := func(name, value string) { attributes = append(attributes, name+" = "+value) }
	// Preserve Sema's exactness contract through the dialect boundary; lower
	// passes consume it rather than rediscovering enum or integer domains.
	// Rules: rules/compiler/semantic_ir.md — § 124(2–3).
	if proof := operation.ConversionProof; proof != nil {
		attribute("sec.conversion_proof", fmt.Sprintf("{basis = %s, plan_pointer_width = %d : i32}", mlirString(proof.Basis), proof.PlanPointerWidthBits))
	}
	if synthesized {
		attribute("sec.synthesized", "true")
	}
	if operation.TryHandlerKind != "" {
		attribute("sec.try_handler_kind", mlirString(string(operation.TryHandlerKind)))
		attribute("sec.try_handler_index", fmt.Sprintf("%d : i32", operation.TryHandlerIndex))
		if operation.TryHandlerExhaustive {
			attribute("sec.try_handler_exhaustive", "true")
		}
		if operation.Variant != "" {
			attribute("sec.try_handler_variant", mlirString(operation.Variant))
		}
	}
	if operation.MatchID != 0 {
		attribute("sec.match_id", fmt.Sprintf("%d : i32", operation.MatchID))
		attribute("sec.match_arm_index", fmt.Sprintf("%d : i32", operation.MatchArmIndex))
		attribute("sec.match_stage", mlirString(operation.MatchStage))
		attribute("sec.match_pattern_kind", mlirString(operation.MatchPatternKind))
	}
	return attributes
}

// emitOperationAttributes writes the shared dictionary in its canonical order.
// Rules: rules/compiler/semantic_ir.md — §§107,124;
// rules/mlir/dialect-versions/sec_mlir_dialect_v6.md — §12.
func (e *emitter) emitOperationAttributes(operation semantic.Operation, synthesized bool) {
	if attributes := operationAttributes(operation, synthesized); len(attributes) != 0 {
		fmt.Fprintf(&e.out, "{%s} ", strings.Join(attributes, ", "))
	}
}
