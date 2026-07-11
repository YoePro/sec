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
	case "uint":
		return "i32"
	case "int64":
		return "i64"
	case "uint64":
		return "i64"
	case "byte":
		return "i8"
	case "string":
		return "ptr"
	default:
		return "void"
	}
}

func llvmParameterType(param *ast.Parameter) string {
	if param.Ref {
		return "ptr"
	}
	return llvmReturnType(param.Type)
}
