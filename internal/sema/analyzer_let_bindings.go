package sema

import (
	"sec/internal/ast"
	"sec/internal/diagnostics"
	"sec/internal/lexer"
)

// analyzeLetStatement resolves binding types, initializer compatibility,
// default synthesis, ownership and immutable scalar evidence.
// Rules: rules/types/types.md — Variable declarations; rules/types/default_values.md — Mutable declarations;
// rules/types/units.md — Exact fixed conversions; rules/memory/ownership.md — Ownership transfer.
func (a *Analyzer) analyzeLetStatement(stmt *ast.LetStatement) {
	if stmt != nil && stmt.Static && a.inFunctionBody {
		a.validateStaticInitializerExpression(stmt.Name.Value, stmt.Value, stmt.Name.Token)
	}
	var declaredType Type
	var ok bool
	inlineContract := a.rejectStorageSiteContract(stmt.Contract, "variable", stmt.Name.Value)

	if stmt.Type != nil && stmt.Type.UnitOnly && stmt.Value != nil {
		declaredType, ok = a.resolveUnitOnlyType(stmt.Type)
	} else if stmt.Type != nil {
		declaredType, ok = a.resolveType(stmt.Type)
	} else if stmt.Value != nil {
		declaredType, _ = a.inferExpression(stmt.Value)
		ok = declaredType.Kind != InvalidType
		// rules/types/types.md "Unsuffixed literal inference": an untyped
		// integer literal without context becomes int (or its suffix family),
		// whose target-selected range it must fit.
		if ok && (declaredType.Kind == IntType || declaredType.Kind == UintType) && isUntypedNumericExpression(stmt.Value) {
			canonical := declaredType
			if known, exists := a.types[declaredType.Name]; exists && !declaredType.Named {
				canonical = known
			}
			a.checkIntegerExpressionRange(canonical, stmt.Value)
		}
	}
	if ok && stmt.Contract != nil && !inlineContract {
		a.checkContractLiteralBounds(declaredType, stmt.Contract)
		declaredType = a.applyContracts(declaredType, stmt.Contract)
	}
	if ok && declaredType.Kind == VoidType {
		token := stmt.Name.Token
		if stmt.Type != nil {
			token = stmt.Type.Token
		}
		a.addErrorAtToken(token, "variable %s cannot have type void; void is only valid as a function result or in an explicitly permitted type argument", stmt.Name.Value)
		ok = false
	}
	if ok && stmt.Value == nil && stmt.Mutable && stmt.Address == nil {
		resolution := DefaultValueOf(declaredType)
		stmt.Value = defaultExpression(resolution, declaredType, stmt.Name.Token)
		stmt.SynthesizedDefault = stmt.Value != nil
		a.recordSynthesizedDefaultTypes(stmt.Value, declaredType)
		if stmt.Value == nil {
			if declaredType.Kind != UnionType {
				start := len(a.errors)
				// rules/types/default_values.md, "Diagnostics": an ambiguous
				// nearest-to-zero tie keeps types.ambiguous-implicit-default.
				if id, help, reason, ambiguous := noDefaultDiagnostic(declaredType); ambiguous {
					a.addErrorAtTokenWithMetadata(stmt.Name.Token, id, "provide an explicit initializer or "+help, "mutable variable %s requires an initializer because %s", stmt.Name.Value, reason)
				} else {
					a.addErrorAtTokenWithMetadata(stmt.Name.Token, diagnostics.NoDefaultValue, "provide an explicit initializer", "mutable variable %s of type %s requires an initializer because the type has no default value", stmt.Name.Value, typeDisplayName(declaredType))
				}
				a.relateMissingDefault(start, declaredType, lexer.Token{})
				ok = false
			}
		}
	}

	defined := false
	if ok {
		if isBareSliceType(declaredType) {
			token := stmt.Name.Token
			if stmt.Type != nil {
				token = stmt.Type.Token
			} else if stmt.Value != nil {
				token = expressionToken(stmt.Value)
			}
			a.addErrorAtToken(token, "bare slice type %s must be used behind ref", typeDisplayName(declaredType))
			ok = false
		}
	}
	if !ok && stmt.Name != nil {
		// Keep a poisoned binding after an invalid initializer. Later references
		// then retain the root diagnostic instead of becoming unrelated
		// "undefined variable" errors.
		defined = a.defineOrReuseStaticSymbol(stmt.Name.Value, Type{Kind: InvalidType}, stmt.Mutable, stmt.Name.Token)
		if defined {
			a.assigned[stmt.Name.Value] = true
		}
	}
	if ok {
		defined = a.defineOrReuseStaticSymbol(stmt.Name.Value, declaredType, stmt.Mutable, stmt.Name.Token)
		if defined {
			if stmt.Static {
				symbol := a.symbols[stmt.Name.Value]
				symbol.Storage = StorageOriginStatic
				symbol.Local = false
				a.symbols[stmt.Name.Value] = symbol
			}
			a.assigned[stmt.Name.Value] = stmt.Value != nil
			if stmt.Address != nil {
				a.analyzeAddressedLetStatement(stmt, declaredType)
			}
		}
	}

	if ok && stmt.Value == nil && !stmt.Mutable && stmt.Address == nil {
		a.reportImmutableRequiresInitializer(stmt.Name, stmt.Type)
		return
	}

	if !ok || stmt.Value == nil || stmt.Type == nil {
		if defined && stmt.Value != nil {
			referenceOrigin, hasReferenceOrigin := a.localReferenceOriginForTransfer(stmt.Value)
			if !a.validateLetOwnership(stmt) {
				return
			}
			if typeCarriesReferenceOrigin(declaredType) {
				a.updateReferenceSymbolOrigin(stmt.Name.Value, declaredType)
			}
			a.applyLetOwnership(stmt)
			a.bindBorrowHoldersFromExpression(stmt.Value, stmt.Name.Value)
			a.markLocalRefContainerFromValue(stmt.Name.Value, stmt.Value)
			if hasReferenceOrigin {
				a.localRefContainers[stmt.Name.Value] = referenceOrigin
			}
			a.recordBoundCallableIdentity(stmt.Name.Value, stmt.Value)
			a.recordResultConstruction(stmt)
			a.setConstInt(stmt.Name.Value, stmt.Value)
			a.recordFloatingBindingConstant(stmt.Name.Value, stmt.Value)
		}
		return
	}

	var exprType Type
	if stmt.SynthesizedDefault && declaredType.Kind == ArrayType && arrayShapeOf(declaredType) == ArrayShapeFixed {
		// SEC-MLIR Package 14 sections 24-26: the compact DefaultResolution is
		// authoritative. The bounded legacy ArrayLiteral may be empty for large N
		// and must never be reinterpreted as a user-written zero-length literal.
		exprType = declaredType
		a.expressionTypes[stmt.Value] = declaredType
	}
	if exprType.Kind == "" && declaredType.Kind == FunctionType {
		if fnType, resolved := a.resolveFunctionValueInitializer(declaredType, stmt.Value); resolved {
			exprType = fnType
		}
	}
	if exprType.Kind == "" && declaredType.Kind == ResultType {
		if resultType, resolved := a.resolveResultValueInitializer(declaredType, stmt.Value); resolved {
			exprType = resultType
		}
	}
	if exprType.Kind == "" {
		exprType, _ = a.inferExpressionWithExpected(stmt.Value, declaredType)
	}
	if exprType.Kind == InvalidType {
		return
	}

	if defined && typeCarriesReferenceOrigin(declaredType) && typeCarriesReferenceOrigin(exprType) {
		declaredType = referenceTypeWithOrigin(declaredType, exprType)
		a.updateReferenceSymbolOrigin(stmt.Name.Value, declaredType)
	}

	if a.checkCompileTimeContractExpression(declaredType, stmt.Value) {
		return
	}

	if a.checkInitializerType(declaredType, exprType, stmt.Value) && defined {
		referenceOrigin, hasReferenceOrigin := a.localReferenceOriginForTransfer(stmt.Value)
		if !a.validateLetOwnership(stmt) {
			return
		}
		a.applyLetOwnership(stmt)
		a.bindBorrowHoldersFromExpression(stmt.Value, stmt.Name.Value)
		a.markLocalRefContainerFromValue(stmt.Name.Value, stmt.Value)
		if hasReferenceOrigin {
			a.localRefContainers[stmt.Name.Value] = referenceOrigin
		}
		a.recordBoundCallableIdentity(stmt.Name.Value, stmt.Value)
		a.recordResultConstruction(stmt)
		a.setConstInt(stmt.Name.Value, stmt.Value)
		a.recordFloatingBindingConstant(stmt.Name.Value, stmt.Value)
	}
}
