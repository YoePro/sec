package sema

import (
	"math/big"

	"sec/internal/ast"
	"sec/internal/lexer"
)

func (a *Analyzer) typeFromDeclaration(stmt *ast.TypeDeclStatement, baseType Type) Type {
	typ := baseType
	typ.Name = stmt.Name.Value
	typ.Named = true
	typ.Declared = true
	typ.Underlying = baseType.Name
	typ.Contracts = append([]Contract(nil), baseType.Contracts...)

	if stmt.BaseType != nil && stmt.BaseType.Unit != "" {
		typ.Unit = stmt.BaseType.Unit
		typ.Dimension = parseDimension(stmt.BaseType.Unit)
	}
	if stmt.AssignedType != nil && stmt.AssignedType.Unit != "" {
		typ.Unit = stmt.AssignedType.Unit
		typ.Dimension = parseDimension(stmt.AssignedType.Unit)
	}

	contract, ok := stmt.Contract.(*ast.RangeContract)
	if !ok {
		return typ
	}

	rangeContract := RangeContract{Exclusive: contract.Exclusive}

	if contract.Min != nil {
		if min, ok := constantIntegerValue(contract.Min); ok {
			rangeContract.Min = new(big.Int).Set(min)
			switch typ.Kind {
			case IntType:
				if min.IsInt64() {
					minInt := min.Int64()
					typ.MinInt = &minInt
				}
			case UintType:
				if min.Sign() >= 0 && min.IsUint64() {
					minUint := min.Uint64()
					typ.MinUint = &minUint
				}
			}
		}
	}

	if contract.Max != nil {
		if max, ok := constantIntegerValue(contract.Max); ok {
			rangeContract.Max = new(big.Int).Set(max)
			switch typ.Kind {
			case IntType:
				if max.IsInt64() {
					maxInt := max.Int64()
					typ.MaxInt = &maxInt
				}
			case UintType:
				if max.Sign() >= 0 && max.IsUint64() {
					maxUint := max.Uint64()
					typ.MaxUint = &maxUint
				}
			}
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
	if hasContracts(symbol.Type) && !isContractCheckableExpression(stmt.Value) {
		return false
	}

	result, ok := a.assignmentIntegerValue(symbol.Name, stmt)
	if !ok {
		return false
	}

	return a.checkIntegerValueRange(symbol.Type, result, expressionToken(stmt.Value))
}

func isContractCheckableExpression(expr ast.Expression) bool {
	if isUntypedNumericExpression(expr) {
		return true
	}

	_, ok := expr.(*ast.ConversionExpression)
	return ok
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
		if typ.Named && hasContracts(typ) {
			a.addErrorAtToken(
				token,
				"value %s violates range contract %s %s",
				value.String(),
				typ.Name,
				formatRangeContract(typ),
			)
			return true
		}
		a.addErrorAtToken(token, "value %s overflows %s", value.String(), typ.Name)
	}
	return overflow
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
