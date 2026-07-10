package llvm

import "sec/internal/ast"

func llvmReturnType(ref *ast.TypeReference) string {
	if ref == nil {
		return "void"
	}

	switch ref.Name {
	case "bool":
		return "i1"
	case "void":
		return "void"
	case "int":
		return "i32"
	default:
		return "void"
	}
}
