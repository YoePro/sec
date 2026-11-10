package sema

import (
	"sec/internal/ast"
	"sec/internal/diagnostics"
	"sec/internal/lexer"
)

// builtinCpuSetType preserves an opaque immutable CPU-index set as its own
// nominal identity, never as an ordinary set[uint] or a struct with public fields.
// Rules: rules/concurrency/threads.md — §7(1–6).
func builtinCpuSetType() Type {
	return Type{Name: "CpuSet", Module: "core", Kind: StructType,
		Named: true, Declared: true, Intrinsic: true}
}

// registerCpuSetDeclaration refines only the exact loader-proven core opaque
// declaration, retaining its source identity without exposing implementation data.
// Rules: rules/concurrency/threads.md — §7(1–3);
// rules/library/core-library.md — compiler-known core declarations.
func (a *Analyzer) registerCpuSetDeclaration(stmt *ast.TypeDeclStatement) bool {
	if stmt.Name.Value != "CpuSet" || !a.isTrustedCoreBuiltinDeclaration("CpuSet", stmt.Name.Token) {
		return false
	}
	valid := stmt.BaseType == nil && stmt.AssignedType == nil && stmt.StructType == nil && stmt.RegisterType == nil && !stmt.Union && len(stmt.Variants) == 0 && !stmt.ErrorType && stmt.Contract == nil && stmt.Default == nil && len(stmt.Implements) == 0 && len(stmt.GenericParameters) == 0 && len(stmt.Attributes) == 0
	if !valid {
		a.addErrorAtToken(stmt.Name.Token, "CpuSet requires its canonical opaque core declaration: type CpuSet")
	}
	typ := builtinCpuSetType()
	typ.DeclarationToken = stmt.Name.Token
	a.types["CpuSet"] = typ
	return true
}

// rejectCpuSetNonOpaque prevents a trusted enum/interface declaration from
// replacing the canonical CPU-index set identity.
// Rules: rules/concurrency/threads.md — §7(1–3).
func (a *Analyzer) rejectCpuSetNonOpaque(name string, token lexer.Token) bool {
	if name != "CpuSet" || !a.isTrustedCoreBuiltinDeclaration(name, token) {
		return false
	}
	a.addErrorAtToken(token, "CpuSet requires its canonical opaque core declaration: type CpuSet")
	a.invalidTypeDeclarations[sourceTokenLocation(token)] = true
	return true
}

// inferCpuSetLiteral checks positional uint indices and known duplicate members.
// Unknown target CPU topology supplies no invented host-derived upper bound;
// affinity application is validated separately against the CompilationPlan.
// Rules: rules/concurrency/threads.md — §7(2–6).
func (a *Analyzer) inferCpuSetLiteral(expr *ast.StructLiteral) (Type, bool) {
	if expr == nil || expr.Type == nil || expr.Type.Name != "CpuSet" {
		return Type{}, false
	}
	invalid := Type{Kind: InvalidType}
	typ, ok := a.resolveType(expr.Type)
	if !ok {
		return invalid, true
	}
	valid := true
	indexTypeExpected := a.types["uint"]
	seen := map[string]lexer.Token{}
	for _, field := range expr.Fields {
		_, spread := field.Value.(*ast.SpreadExpression)
		if !field.Positional || field.Spread || spread {
			a.addErrorAtTokenWithMetadata(field.Token, diagnostics.CpuSetLiteral,
				"Write CpuSet { index, ... }; CpuSet has no public named fields or spread literal form.",
				"CpuSet literal requires positional CPU indices")
			valid = false
			continue
		}
		indexType, _ := a.inferExpressionWithExpected(field.Value, indexTypeExpected)
		if indexType.Kind == InvalidType {
			valid = false
			continue
		}
		if !a.canInitialize(indexTypeExpected, indexType, field.Value) {
			a.addErrorAtTokenWithMetadata(field.Token, diagnostics.CpuSetLiteral,
				"Use a non-negative CPU index representable as uint for the selected target.",
				"CpuSet index must be uint, got %s", typeDisplayName(indexType))
			valid = false
			continue
		}
		if value, constant := a.integerConstantValue(field.Value); constant {
			if value.Sign() < 0 || indexTypeExpected.MaxInteger != nil && value.Cmp(indexTypeExpected.MaxInteger) > 0 {
				a.addErrorAtTokenWithMetadata(field.Token, diagnostics.CpuSetLiteral,
					"Use a non-negative CPU index representable as uint for the selected target.",
					"CpuSet index %s is outside the target uint range", value.String())
				valid = false
				continue
			}
			if previous, duplicate := seen[value.String()]; duplicate {
				a.addErrorAtTokenWithMetadataAndPrevious(field.Token, previous, diagnostics.CpuSetLiteral,
					"List each CPU index once; repeated indices do not name additional CPUs.",
					"duplicate CpuSet index %s", value.String())
				valid = false
			}
			seen[value.String()] = field.Token
		}
	}
	if !valid {
		return invalid, true
	}
	a.expressionTypes[expr] = typ
	return typ, true
}
