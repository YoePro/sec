package sema

import (
	"math/big"
	"strconv"
	"strings"

	"sec/internal/ast"
)

func constantIntegerValue(expr ast.Expression) (*big.Int, bool) {
	switch expr := expr.(type) {
	case *ast.IntegerLiteral:
		value, ok := new(big.Int).SetString(expr.Token.Lexeme, 10)
		return value, ok
	case *ast.PrefixExpression:
		if expr.Operator != "-" {
			return nil, false
		}
		value, ok := constantIntegerValue(expr.Right)
		if !ok {
			return nil, false
		}
		return value.Neg(value), true
	case *ast.InfixExpression:
		left, ok := constantIntegerValue(expr.Left)
		if !ok {
			return nil, false
		}

		right, ok := constantIntegerValue(expr.Right)
		if !ok {
			return nil, false
		}

		value := new(big.Int)
		switch expr.Operator {
		case "+":
			return value.Add(left, right), true
		case "-":
			return value.Sub(left, right), true
		case "*":
			return value.Mul(left, right), true
		default:
			return nil, false
		}
	default:
		return nil, false
	}
}

func (a *Analyzer) integerConstantValue(expr ast.Expression) (*big.Int, bool) {
	switch expr := expr.(type) {
	case *ast.Identifier:
		value, ok := a.constInts[expr.Value]
		if !ok {
			return nil, false
		}
		return new(big.Int).Set(value), true
	case *ast.ConversionExpression:
		return a.integerConstantValue(expr.Value)
	case *ast.CallExpression:
		if a.isExplicitConversionExpression(expr) {
			return a.integerConstantValue(expr.Arguments[0])
		}
		return nil, false
	case *ast.PrefixExpression:
		if expr.Operator != "-" {
			return nil, false
		}
		value, ok := a.integerConstantValue(expr.Right)
		if !ok {
			return nil, false
		}
		return value.Neg(value), true
	case *ast.InfixExpression:
		left, ok := a.integerConstantValue(expr.Left)
		if !ok {
			return nil, false
		}

		right, ok := a.integerConstantValue(expr.Right)
		if !ok {
			return nil, false
		}

		value := new(big.Int)
		switch expr.Operator {
		case "+":
			return value.Add(left, right), true
		case "-":
			return value.Sub(left, right), true
		case "*":
			return value.Mul(left, right), true
		default:
			return nil, false
		}
	default:
		return constantIntegerValue(expr)
	}
}

func isNumericLiteral(expr ast.Expression) bool {
	switch expr := expr.(type) {
	case *ast.IntegerLiteral, *ast.FloatLiteral:
		return true
	case *ast.PrefixExpression:
		return expr.Operator == "-" && isNumericLiteral(expr.Right)
	default:
		return false
	}
}

func decimalLiteralValue(expr ast.Expression) (DecimalValue, bool) {
	negative := false
	lexeme := ""

	switch expr := expr.(type) {
	case *ast.IntegerLiteral, *ast.FloatLiteral:
		lexeme = expr.TokenLiteral()
	case *ast.PrefixExpression:
		if expr.Operator != "-" {
			return DecimalValue{}, false
		}

		value, ok := decimalLiteralValue(expr.Right)
		if !ok {
			return DecimalValue{}, false
		}

		value.Int64 = -value.Int64
		return value, true
	default:
		return DecimalValue{}, false
	}

	if strings.HasPrefix(lexeme, "-") {
		negative = true
		lexeme = strings.TrimPrefix(lexeme, "-")
	}

	parts := strings.Split(lexeme, ".")
	if len(parts) > 2 {
		return DecimalValue{}, false
	}

	digits := parts[0]
	scale := 0
	if len(parts) == 2 {
		digits += parts[1]
		scale = len(parts[1])
	}

	if scale > 255 {
		return DecimalValue{}, false
	}

	if digits == "" {
		return DecimalValue{}, false
	}

	intValue, err := strconv.ParseInt(digits, 10, 64)
	if err != nil {
		return DecimalValue{}, false
	}

	if negative {
		intValue = -intValue
	}

	return DecimalValue{Int64: intValue, Scale: uint8(scale)}, true
}

func (a *Analyzer) assignmentIntegerValue(name string, stmt *ast.AssignmentStatement) (*big.Int, bool) {
	right, ok := a.integerConstantValue(stmt.Value)
	if !ok {
		return nil, false
	}

	if stmt.Operator == "=" {
		return right, true
	}

	current, ok := a.constInts[name]
	if !ok {
		return nil, false
	}

	result := new(big.Int)
	switch stmt.Operator {
	case "+=":
		return result.Add(current, right), true
	case "-=":
		return result.Sub(current, right), true
	case "*=":
		return result.Mul(current, right), true
	default:
		return nil, false
	}
}

func (a *Analyzer) setConstInt(name string, expr ast.Expression) {
	value, ok := a.integerConstantValue(expr)
	if !ok {
		delete(a.constInts, name)
		return
	}

	a.constInts[name] = new(big.Int).Set(value)
}

func (a *Analyzer) updateAssignedConstInt(name string, stmt *ast.AssignmentStatement) {
	value, ok := a.assignmentIntegerValue(name, stmt)
	if !ok {
		delete(a.constInts, name)
		return
	}

	a.constInts[name] = new(big.Int).Set(value)
}
