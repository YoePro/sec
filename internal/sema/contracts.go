package sema

import (
	"math/big"

	"sec/internal/ast"
	"sec/internal/diagnostics"
	"sec/internal/lexer"
)

func (a *Analyzer) typeFromDeclaration(stmt *ast.TypeDeclStatement, baseType Type) Type {
	return a.typeFromDeclarationWithName(stmt.Name.Value, stmt, baseType)
}

func (a *Analyzer) typeFromDeclarationWithName(name string, stmt *ast.TypeDeclStatement, baseType Type) Type {
	typ := baseType
	typ.Name = name
	typ.Module = a.currentModule
	typ.Named = true
	typ.Declared = true
	if hasAttribute(stmt.Attributes, "noCopy") {
		typ.ExplicitlyNonCopyable = true
		typ.NoCopyPolicyOrigin = name
	} else if typ.ExplicitlyNonCopyable && typ.NoCopyPolicyOrigin == "" {
		typ.NoCopyPolicyOrigin = baseType.Name
	}
	typ.Underlying = baseType.Name
	typ.Contracts = append([]Contract(nil), baseType.Contracts...)
	typ.GenericParameters = genericParameterNameValues(stmt.GenericParameters)

	if stmt.BaseType != nil && stmt.BaseType.Unit != "" {
		typ.Unit = stmt.BaseType.Unit
		typ.Dimension = a.parseDimension(stmt.BaseType.Unit)
	}
	if stmt.AssignedType != nil && stmt.AssignedType.Unit != "" {
		typ.Unit = stmt.AssignedType.Unit
		typ.Dimension = a.parseDimension(stmt.AssignedType.Unit)
	}

	typ = a.applyContracts(typ, stmt.Contract)
	if stmt.Default != nil {
		constant, ok := defaultConstantFromExpression(stmt.Default)
		if !ok {
			typ.InvalidExplicitDefault = true
			a.addErrorAtTokenWithMetadata(stmt.DefaultToken, diagnostics.InvalidExplicitDefault, "use an allocation-free compile-time constant", "default for %s must be a compile-time primitive constant", name)
		} else if !defaultConstantSatisfies(typ, constant) || !defaultConstantCompatible(typ, constant) {
			typ.InvalidExplicitDefault = true
			a.addErrorAtTokenWithMetadata(expressionToken(stmt.Default), diagnostics.InvalidExplicitDefault, "choose a value satisfying every type contract", "default value %s is invalid for %s", stmt.Default.String(), name)
		} else {
			typ.ExplicitDefault = &constant
		}
	}
	return typ
}

func flattenASTContracts(contract ast.Contract) []ast.Contract {
	if contract == nil {
		return nil
	}
	if list, ok := contract.(*ast.ContractList); ok {
		return list.Contracts
	}
	return []ast.Contract{contract}
}

func (a *Analyzer) applyContracts(typ Type, contractNode ast.Contract) Type {
	for _, contract := range flattenASTContracts(contractNode) {
		typ = a.applyContract(typ, contract)
	}
	a.checkContractSetConsistency(typ, contractNode)
	a.checkMembershipContractValues(typ, contractNode)
	return typ
}

func (a *Analyzer) checkMembershipContractValues(typ Type, contractNode ast.Contract) {
	for _, contract := range flattenASTContracts(contractNode) {
		membership, ok := contract.(*ast.MembershipContract)
		if !ok {
			continue
		}
		for _, expression := range membership.Values {
			constant, ok := defaultConstantFromExpression(expression)
			if !ok || !defaultConstantCompatible(typ, constant) {
				continue
			}
			if !defaultConstantSatisfies(typ, constant) {
				a.addErrorAtTokenWithMetadata(
					expressionToken(expression),
					diagnostics.InvalidMembershipValue,
					"remove the value or change the other type contracts",
					"membership value %s violates another contract on %s",
					expression.String(),
					typeDisplayName(typ),
				)
			}
		}
	}
}

func (a *Analyzer) applyContract(typ Type, contractNode ast.Contract) Type {
	switch contract := contractNode.(type) {
	case *ast.RangeContract:
		if !contractAppliesToType("range", typ) {
			a.addErrorAtToken(contract.Token, "range contract does not apply to %s", contractApplicabilityTypeName(typ))
			return typ
		}
		return applyRangeContract(typ, contract)
	case *ast.MembershipContract:
		if !contractAppliesToType("in", typ) {
			a.addErrorAtToken(contract.Token, "in contract does not apply to %s", contractApplicabilityTypeName(typ))
			return typ
		}
		membership := MembershipContract{}
		for _, value := range contract.Values {
			constant, ok := defaultConstantFromExpression(value)
			if !ok {
				a.addErrorAtToken(expressionToken(value), "membership values must be compile-time primitive constants")
				continue
			}
			if !defaultConstantCompatible(typ, constant) {
				a.addErrorAtToken(expressionToken(value), "membership value %s is incompatible with %s", value.String(), typeDisplayName(typ))
				continue
			}
			duplicate := false
			for _, previous := range membership.Values {
				if defaultConstantsEqual(constant, previous) {
					duplicate = true
					break
				}
			}
			if duplicate {
				a.addErrorAtToken(expressionToken(value), "duplicate membership value %s", value.String())
				continue
			}
			membership.Values = append(membership.Values, constant)
		}
		typ.Contracts = append(typ.Contracts, membership)
		return typ
	case *ast.MarkerContract:
		if !contractAppliesToType(contract.Name, typ) {
			a.addErrorAtToken(contract.Token, "%s contract does not apply to %s", contract.Name, contractApplicabilityTypeName(typ))
			return typ
		}
		if contract.Name == "multipleOf" {
			multiple := MultipleOfContract{}
			if value, ok := constantIntegerValue(contract.Value); ok {
				multiple.Value = new(big.Int).Set(value)
				if value.Sign() == 0 {
					a.addErrorAtToken(expressionToken(contract.Value), "multipleOf contract divisor must not be zero")
				}
			}
			typ.Contracts = append(typ.Contracts, multiple)
			return typ
		}
		typ.Contracts = append(typ.Contracts, MarkerContract{Name: contract.Name})
		return typ
	default:
		return typ
	}
}

func contractApplicabilityTypeName(typ Type) string {
	if typ.Named && typ.Underlying != "" {
		return typ.Underlying
	}
	return typeDisplayName(typ)
}

func applyRangeContract(typ Type, contract *ast.RangeContract) Type {
	rangeContract := RangeContract{Exclusive: contract.Exclusive}

	if contract.Min != nil {
		if min, ok := constantIntegerValue(contract.Min); ok {
			rangeContract.Min = new(big.Int).Set(min)
		}
	}

	if contract.Max != nil {
		if max, ok := constantIntegerValue(contract.Max); ok {
			rangeContract.Max = new(big.Int).Set(max)
		}
	}
	if typ.Kind == DecimalType || typ.Kind == FloatType {
		if exact, lexeme, ok := exactNumericConstant(contract.Min); ok {
			rangeContract.ExactMin = exact
			rangeContract.MinLexeme = lexeme
		}
		if exact, lexeme, ok := exactNumericConstant(contract.Max); ok {
			rangeContract.ExactMax = exact
			rangeContract.MaxLexeme = lexeme
		}
	}

	typ.Contracts = append(typ.Contracts, rangeContract)

	return typ
}

func (a *Analyzer) checkContractSetConsistency(typ Type, contractNode ast.Contract) {
	if contractNode == nil || !isIntegerType(typ) {
		return
	}

	var token lexer.Token
	var rangeContract *RangeContract
	var multiple *big.Int
	hasOdd := false
	hasEven := false

	for _, astContract := range flattenASTContracts(contractNode) {
		switch contract := astContract.(type) {
		case *ast.RangeContract:
			token = contract.Token
		case *ast.MarkerContract:
			token = contract.Token
		}
	}

	for _, contract := range typ.Contracts {
		switch contract := contract.(type) {
		case RangeContract:
			c := contract
			rangeContract = &c
		case MultipleOfContract:
			if contract.Value != nil && contract.Value.Sign() != 0 {
				multiple = new(big.Int).Abs(contract.Value)
			}
		case MarkerContract:
			switch contract.Name {
			case "odd":
				hasOdd = true
			case "even":
				hasEven = true
			}
		}
	}

	if hasOdd && hasEven {
		a.addErrorAtToken(token, "contracts odd and even cannot be combined")
		return
	}
	if hasOdd && multiple != nil && new(big.Int).Mod(multiple, big.NewInt(2)).Sign() == 0 {
		a.addErrorAtToken(token, "contracts multipleOf %s and odd cannot be combined because every multiple is even", multiple.String())
		return
	}
	if rangeContract == nil {
		return
	}
	if !integerRangeHasSatisfyingValue(rangeContract, multiple, hasOdd, hasEven) {
		a.addErrorAtToken(token, "contracts cannot be satisfied together for %s", typeDisplayName(typ))
	}
}

func integerRangeHasSatisfyingValue(contract *RangeContract, multiple *big.Int, odd bool, even bool) bool {
	if contract == nil || contract.Min == nil || contract.Max == nil {
		return true
	}
	min := new(big.Int).Set(contract.Min)
	max := new(big.Int).Set(contract.Max)
	if contract.Exclusive {
		max.Sub(max, big.NewInt(1))
	}
	if min.Cmp(max) > 0 {
		return false
	}

	step := big.NewInt(1)
	if multiple != nil && multiple.Sign() > 0 {
		step = new(big.Int).Set(multiple)
	}
	first := firstMultipleAtOrAbove(min, step)
	for value := first; value.Cmp(max) <= 0; value.Add(value, step) {
		if odd && new(big.Int).Mod(value, big.NewInt(2)).Sign() == 0 {
			continue
		}
		if even && new(big.Int).Mod(value, big.NewInt(2)).Sign() != 0 {
			continue
		}
		return true
	}
	return false
}

func firstMultipleAtOrAbove(min *big.Int, step *big.Int) *big.Int {
	if step.Sign() <= 0 {
		return new(big.Int).Set(min)
	}
	remainder := new(big.Int).Mod(min, step)
	if remainder.Sign() == 0 {
		return new(big.Int).Set(min)
	}
	return new(big.Int).Add(min, new(big.Int).Sub(step, remainder))
}

func contractAppliesToType(name string, typ Type) bool {
	switch name {
	case "range":
		return isNumericType(typ)
	case "in":
		return isScalarContractType(typ)
	case "multipleOf":
		return isIntegerType(typ)
	case "odd", "even":
		return isIntegerType(typ)
	case "notEmpty":
		return typ.Kind == StringType || isCollectionContractType(typ)
	case "unique":
		return isCollectionContractType(typ)
	case "finite":
		return typ.Kind == FloatType || typ.Kind == DecimalType
	default:
		return false
	}
}

func isScalarContractType(typ Type) bool {
	switch typ.Kind {
	case BoolType, StringType, CharType, RuneType, IntType, UintType, FloatType, DecimalType, EnumType:
		return true
	default:
		return false
	}
}

func isCollectionContractType(typ Type) bool {
	if typ.Kind == ArrayType || typ.Kind == SliceType {
		return true
	}
	switch typ.Name {
	case "Vec", "Set", "Map", "list", "set", "map", "vector", "matrix", "tensor", "tensor_view":
		return true
	default:
		return false
	}
}

func (a *Analyzer) checkIntegerExpressionRange(typ Type, expr ast.Expression) bool {
	value, ok := a.integerConstantValue(expr)
	if !ok {
		return false
	}

	return a.checkIntegerValueRange(typ, value, expressionToken(expr))
}

func (a *Analyzer) checkIntegerAssignmentRange(symbol Symbol, stmt *ast.AssignmentStatement) bool {
	if hasContracts(symbol.Type) && !a.isContractCheckableExpression(stmt.Value) {
		return false
	}

	result, ok := a.assignmentIntegerValue(symbol.Name, stmt)
	if !ok {
		return false
	}

	return a.checkIntegerValueRange(symbol.Type, result, expressionToken(stmt.Value))
}

func (a *Analyzer) isContractCheckableExpression(expr ast.Expression) bool {
	if isUntypedNumericExpression(expr) {
		return true
	}

	return a.isExplicitConversionExpression(expr)
}

func (a *Analyzer) checkIntegerLiteralRange(typ Type, expr ast.Expression) bool {
	value, ok := a.integerConstantValue(expr)
	if !ok {
		return false
	}

	return a.checkIntegerValueRange(typ, value, expressionToken(expr))
}

func (a *Analyzer) checkContractLiteralBounds(typ Type, contractNode ast.Contract) {
	for _, contract := range flattenASTContracts(contractNode) {
		switch contract := contract.(type) {
		case *ast.RangeContract:
			if contract.Min != nil {
				a.checkIntegerLiteralRange(typ, contract.Min)
			}
			if contract.Max != nil {
				a.checkIntegerLiteralRange(typ, contract.Max)
			}
		case *ast.MarkerContract:
			if contract.Name == "multipleOf" && contract.Value != nil {
				a.checkIntegerLiteralRange(typ, contract.Value)
			}
		}
	}
}

func (a *Analyzer) checkIntegerValueRange(typ Type, value *big.Int, token lexer.Token) bool {
	if value == nil {
		return false
	}

	overflow := false
	switch typ.Kind {
	case IntType:
		if typ.MinInteger == nil || typ.MaxInteger == nil {
			return false
		}
		overflow = value.Cmp(typ.MinInteger) < 0 || value.Cmp(typ.MaxInteger) > 0
	case UintType:
		if typ.MinInteger == nil || typ.MaxInteger == nil {
			return false
		}
		overflow = value.Sign() < 0 || value.Cmp(typ.MinInteger) < 0 || value.Cmp(typ.MaxInteger) > 0
	case EnumType:
		if typ.BitWidth <= 0 || typ.MinInteger == nil || typ.MaxInteger == nil {
			return false
		}
		overflow = value.Sign() < 0 || value.Cmp(typ.MinInteger) < 0 || value.Cmp(typ.MaxInteger) > 0
	}

	if overflow {
		if typ.Kind == EnumType && typ.BitWidth > 0 {
			a.addErrorAtToken(token, "value %s does not fit in %d-bit enum %s", value.String(), typ.BitWidth, typ.Name)
			return true
		}
		a.addErrorAtToken(token, "value %s overflows %s", value.String(), typ.Name)
		return true
	}

	for _, contract := range typ.Contracts {
		switch contract := contract.(type) {
		case RangeContract:
			violatesMin := contract.Min != nil && value.Cmp(contract.Min) < 0
			violatesMax := false
			if contract.Max != nil {
				if contract.Exclusive {
					violatesMax = value.Cmp(contract.Max) >= 0
				} else {
					violatesMax = value.Cmp(contract.Max) > 0
				}
			}
			if violatesMin || violatesMax {
				a.addErrorAtToken(
					token,
					"value %s violates range contract %s %s",
					value.String(),
					typ.Name,
					formatRangeContract(typ),
				)
				return true
			}
		case MultipleOfContract:
			if contract.Value == nil || contract.Value.Sign() == 0 {
				continue
			}
			if new(big.Int).Mod(value, contract.Value).Sign() != 0 {
				a.addErrorAtToken(token, "value %s violates multipleOf contract %s %s", value.String(), typ.Name, contract.Value.String())
				return true
			}
		case MarkerContract:
			switch contract.Name {
			case "odd":
				if new(big.Int).Mod(value, big.NewInt(2)).Sign() == 0 {
					a.addErrorAtToken(token, "value %s violates odd contract %s", value.String(), typ.Name)
					return true
				}
			case "even":
				if new(big.Int).Mod(value, big.NewInt(2)).Sign() != 0 {
					a.addErrorAtToken(token, "value %s violates even contract %s", value.String(), typ.Name)
					return true
				}
			}
		default:
			continue
		}
	}

	return false
}

func formatRangeContract(typ Type) string {
	for _, contract := range typ.Contracts {
		rangeContract, ok := contract.(RangeContract)
		if !ok {
			continue
		}

		min := ""
		if rangeContract.Min != nil {
			min = rangeContract.Min.String()
		}

		max := ""
		if rangeContract.Max != nil {
			max = rangeContract.Max.String()
		}

		operator := ".."
		if rangeContract.Exclusive {
			operator = "..<"
		}

		return min + operator + max
	}

	return ""
}
