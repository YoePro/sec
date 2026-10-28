package sema

import "sec/internal/ast"

// arithmeticErrorFromDeclaration accepts only the exact canonical core
// declaration of ArithmeticError, keeping the one compiler-known identity
// (all five variants, MD-067) shared by Sema, Semantic IR and the LSP.
// Rules: rules/library/core-library.md — § 20.1;
// rules/corrections/md066-md067-md068-decimal-correction-20261028.md — §§ 7(1), 7(9), 12(7).
func (a *Analyzer) arithmeticErrorFromDeclaration(enum *ast.EnumDeclaration) (Type, bool) {
	if enum.Name.Value != "ArithmeticError" || !a.isTrustedCoreBuiltinDeclaration("ArithmeticError", enum.Name.Token) {
		return Type{}, false
	}
	typ := builtinType("ArithmeticError")
	consts := make(map[string]EnumValue, len(typ.EnumConsts))
	for name, value := range typ.EnumConsts {
		consts[name] = value
	}
	typ.EnumConsts = consts
	return a.exactCoreEnumDeclaration(enum, typ, "rules/library/core-library.md § 20.1"), true
}
