package scalar

import (
	"fmt"
	"sec/internal/ast"
	"sec/internal/layout"
)

// Type maps built-in Sec scalar representations for the legacy LLVM
// backend, including distinct char and rune widths.
//
// Rules:
//   - rules/types/types.md — "char", "rune", "int and uint"
//   - rules/compiler/semantic_ir.md — §11 "Constants"
func Type(ref *ast.TypeReference, plan layout.ResolvedScalarPlan) string {
	if ref == nil {
		return "void"
	}

	if ref.Name == "fn" || ref.FunctionReturnType != nil {
		return "ptr"
	}
	if ref.Name == "RawPtr" {
		return "ptr"
	}

	switch ref.Name {
	case "bool":
		return "i1"
	case "void":
		return "void"
	case "int":
		return fmt.Sprintf("i%d", plan.PointerWidthBits)
	case "uint":
		return fmt.Sprintf("i%d", plan.PointerWidthBits)
	case "int8", "uint8", "byte", "char":
		return "i8"
	case "int16", "uint16":
		return "i16"
	case "int32", "uint32":
		return "i32"
	case "rune":
		return "i32"
	case "int64", "uint64":
		return "i64"
	case "int128", "uint128":
		return "i128"
	case "int256", "uint256":
		return "i256"
	case "float64":
		return "double"
	case "float32":
		return "float"
	case "string":
		return "ptr"
	case "decimal":
		return "%sec.decimal"
	default:
		return "void"
	}
}
