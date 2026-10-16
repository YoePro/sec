package sema

import (
	"math/big"

	"sec/internal/ast"
	"sec/internal/lexer"
)

// inferMatrixMultiplyExpression resolves the static matrix/matrix or
// matrix/vector form through ordinary scalar product and accumulation rules.
// Rules: rules/collections/shaped-types.md — §§22.1–22.4;
// rules/corrections/applied/correction27-20260823.md — "Bug 2".
func (a *Analyzer) inferMatrixMultiplyExpression(expr *ast.InfixExpression, leftType Type, rightType Type) (Type, expressionValue) {
	if !a.summaryPass {
		delete(a.resolvedShapedMultiplies, expr)
	}
	invalid := func(format string, args ...any) (Type, expressionValue) {
		a.addErrorAtToken(expr.Token, format, args...)
		return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}
	}

	if leftType.Name != "matrix" || len(leftType.TypeArgs) != 1 || len(leftType.ConstArgs) != 2 {
		return invalid("left operand of x must be matrix, got %s", typeDisplayName(leftType))
	}
	if (rightType.Name != "matrix" && rightType.Name != "vector") || len(rightType.TypeArgs) != 1 {
		return invalid("right operand of x must be matrix or vector, got %s", typeDisplayName(rightType))
	}
	// rules/collections/shaped-types.md; correction27.md: x derives its
	// element and accumulator types from ordinary scalar multiplication and
	// addition. Equal operand element types are not a language requirement.
	scalarProduct := *expr
	scalarProduct.Operator = "*"
	productType, _ := a.inferScalarOperatorType(&scalarProduct, leftType.TypeArgs[0], rightType.TypeArgs[0])
	if productType.Kind == InvalidType {
		return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}
	}
	scalarSum := *expr
	scalarSum.Operator = "+"
	accumulatorType, _ := a.inferScalarOperatorType(&scalarSum, productType, productType)
	if accumulatorType.Kind == InvalidType || !sameConcreteType(productType, accumulatorType) {
		return invalid("matrix multiplication product type %s is not stable under accumulation", typeDisplayName(productType))
	}

	inner := leftType.ConstArgs[1]
	switch rightType.Name {
	case "matrix":
		if len(rightType.ConstArgs) != 2 {
			return invalid("right operand of x must have two matrix dimensions, got %s", typeDisplayName(rightType))
		}
		if inner != rightType.ConstArgs[0] {
			return invalid("matrix multiplication inner dimensions differ: %d and %d", inner, rightType.ConstArgs[0])
		}
		result := leftType
		result.TypeArgs = []Type{productType}
		result.ConstArgs = []int64{leftType.ConstArgs[0], rightType.ConstArgs[1]}
		return a.finishShapedMultiply(expr, leftType, rightType, productType, accumulatorType, result)
	case "vector":
		if len(rightType.ConstArgs) != 1 {
			return invalid("right operand of x must have one vector dimension, got %s", typeDisplayName(rightType))
		}
		if inner != rightType.ConstArgs[0] {
			return invalid("matrix-vector multiplication inner dimensions differ: %d and %d", inner, rightType.ConstArgs[0])
		}
		result := rightType
		result.TypeArgs = []Type{productType}
		result.ConstArgs = []int64{leftType.ConstArgs[0]}
		return a.finishShapedMultiply(expr, leftType, rightType, productType, accumulatorType, result)
	default:
		return invalid("right operand of x must be matrix or vector, got %s", typeDisplayName(rightType))
	}
}

// validateShapedStaticType implements the static extent proof from
// rules/collections/shaped-types.md (correction27.md). The product is computed
// with arbitrary precision, checked against the target uint plan, and retained
// on the resolved type for later semantic consumers.
// Rules: rules/collections/shaped-types.md — §§4–5,29;
// rules/corrections/applied/correction27-20260823.md — "Preserve the proven product".
func (a *Analyzer) validateShapedStaticType(token lexer.Token, typ *Type, minConstArgs, maxConstArgs int) bool {
	if !a.validateCompilerKnownTypeArity(token, *typ, 1, minConstArgs, maxConstArgs) {
		return false
	}
	product := big.NewInt(1)
	for _, extent := range typ.ConstArgs {
		if extent == 0 {
			product.SetInt64(0)
			break
		}
		product.Mul(product, big.NewInt(extent))
	}
	uintType := a.types["uint"]
	representation, _, representable := a.integerRepresentation(uintType)
	maximum := representation.MaxInteger
	if !representable || maximum == nil {
		maximum = new(big.Int).SetUint64(^uint64(0))
	}
	if product.Cmp(maximum) > 0 {
		a.addErrorAtToken(token, "%s static element count %s overflows target uint", typ.Name, product.String())
		return false
	}
	typ.StaticElementCount = new(big.Int).Set(product)
	return true
}
