package parser

import (
	"fmt"
	"sec/internal/ast"
)

// unimplementedFunctionHelp offers a valid placeholder for void and built-in
// return types without inventing a value for a named or aggregate type.
// Rules: rules/declarations/functions.md — "4. No ordinary bodyless prototypes".
func unimplementedFunctionHelp(fn *ast.FunctionDeclaration) string {
	prefix := fmt.Sprintf("The function `%s` has been declared with a signature, but its body is missing. ", fn.Name.Value)
	if parserTypeReferenceName(fn.ReturnType) == "void" {
		return prefix + "If you intended to implement this function later, add a placeholder body like `{}`."
	}
	value := ""
	simple := !fn.ReturnType.Ref && fn.ReturnType.ElementType == nil && fn.ReturnType.Unit == "" && len(fn.ReturnType.TypeArgs) == 0 && len(fn.ReturnType.ConstArgs) == 0
	switch parserTypeReferenceName(fn.ReturnType) {
	case "int", "uint", "int8", "int16", "int32", "int64", "int128", "int256", "uint8", "uint16", "uint32", "uint64", "uint128", "uint256":
		value = "0"
	case "string":
		value = `""`
	case "bool":
		value = "false"
	}
	if value != "" && simple {
		return prefix + fmt.Sprintf("If you intended to implement this function later, add a placeholder body like `{ return %s }` so the rest of the code can compile while you test.", value)
	}
	return prefix + "If you intended to implement this function later, add a placeholder body that returns a valid value of the declared return type so the rest of the code can compile while you test."
}
