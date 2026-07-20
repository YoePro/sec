package sema

import (
	"math/big"

	"sec/internal/ast"
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

	return applyRangeContract(typ, stmt.Contract)
}

func applyRangeContract(typ Type, contractNode ast.Contract) Type {
	contract, ok := contractNode.(*ast.RangeContract)
	if !ok {
		return typ
	}
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

	typ.Contracts = append(typ.Contracts, rangeContract)

	return typ
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

func (a *Analyzer) checkIntegerValueRange(typ Type, value *big.Int, token lexer.Token) bool {
	if value == nil {
		return false
	}

	overflow := false
	switch typ.Kind {
	case IntType:
		if typ.MinInt == nil || typ.MaxInt == nil {
			return false
		}
		min := big.NewInt(*typ.MinInt)
		max := big.NewInt(*typ.MaxInt)
		overflow = value.Cmp(min) < 0 || value.Cmp(max) > 0
	case UintType:
		if typ.MaxUint == nil || typ.MinUint == nil {
			return false
		}
		min := new(big.Int).SetUint64(*typ.MinUint)
		max := new(big.Int).SetUint64(*typ.MaxUint)
		overflow = value.Sign() < 0 || value.Cmp(min) < 0 || value.Cmp(max) > 0
	}

	if overflow {
		a.addErrorAtToken(token, "value %s overflows %s", value.String(), typ.Name)
		return true
	}

	for _, contract := range typ.Contracts {
		rangeContract, ok := contract.(RangeContract)
		if !ok {
			continue
		}

		violatesMin := rangeContract.Min != nil && value.Cmp(rangeContract.Min) < 0
		violatesMax := false
		if rangeContract.Max != nil {
			if rangeContract.Exclusive {
				violatesMax = value.Cmp(rangeContract.Max) >= 0
			} else {
				violatesMax = value.Cmp(rangeContract.Max) > 0
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
