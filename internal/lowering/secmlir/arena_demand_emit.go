package secmlir

import (
	"fmt"
	"strings"

	"sec/internal/ir/semantic"
)

// arenaDemandAttribute writes the P19 § 130 function metadata
// sec.arena_demand_summary: one dictionary per created Arena domain with its
// kind, upper-bound bytes (constant demand only), symbolic expression and
// fixed backing capacity when known.
// Rules: rules/mlir/packages/sec-mlir-dialect_package19.md — §§ 79–81, 130.
func arenaDemandAttribute(facts []semantic.ArenaDemandFacts) string {
	entries := make([]string, 0, len(facts))
	for _, fact := range facts {
		fields := []string{}
		if fact.Bytes != nil {
			fields = append(fields, "bytes = "+mlirString(fact.Bytes.String()))
		}
		if fact.CapacityBytes != nil {
			fields = append(fields, "capacity_bytes = "+mlirString(fact.CapacityBytes.String()))
		}
		fields = append(fields, fmt.Sprintf("domain = %d : i32", fact.Domain), "expression = "+mlirString(fact.Expression), "kind = "+mlirString(fact.Kind))
		entries = append(entries, "{"+strings.Join(fields, ", ")+"}")
	}
	return "[" + strings.Join(entries, ", ") + "]"
}
