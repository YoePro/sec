// Package scalar maps builtin numeric carriers from canonical target facts.
package scalar

import (
	"fmt"
	"sec/internal/layout"
)

// NumericType keeps native integer widths target-selected and explicit widths fixed.
// Rules: rules/types/types.md — "int and uint"; correction5.md — backend scalar facts.
func NumericType(name string, plan layout.ResolvedScalarPlan) string {
	switch name {
	case "int", "uint":
		return fmt.Sprintf("i%d", plan.PointerWidthBits)
	case "int8", "uint8", "byte":
		return "i8"
	case "int16", "uint16":
		return "i16"
	case "int32", "uint32":
		return "i32"
	case "int64", "uint64":
		return "i64"
	case "int128", "uint128":
		return "i128"
	case "int256", "uint256":
		return "i256"
	case "float", "float64":
		return "f64"
	case "float32":
		return "f32"
	case "decimal":
		return "!llvm.struct<(i64, i32)>"
	case "decimal128":
		return "!llvm.struct<(i128, i32)>"
	default:
		return ""
	}
}
