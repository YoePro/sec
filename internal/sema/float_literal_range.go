package sema

import (
	"sort"

	"sec/internal/ast"
	"sec/internal/diagnostics"
	"sec/internal/sema/constant"
)

// validateResolvedFloatLiterals checks exact source spelling at the final
// contextual binary width, after overload selection and operator shaping.
// Checking during initial inference would wrongly reject a g literal first
// inferred as platform float32 and subsequently shaped to explicit float64.
// Decimal sources of runtime conversions keep their decimal type and are
// deliberately excluded. Sorting gives diagnostics stable source order.
// Rules: rules/mlir/packages/sec-mlir-dialect_package20.md — §§11–19;
// rules/corrections/md066-md067-md068-decimal-correction-20261028.md — §3(7).
func (a *Analyzer) validateResolvedFloatLiterals() {
	literals := make([]ast.Expression, 0)
	for expr, typ := range a.expressionTypes {
		if typ.Kind != FloatType {
			continue
		}
		switch expr.(type) {
		case *ast.IntegerLiteral, *ast.FloatLiteral:
			literals = append(literals, expr)
		}
	}
	sort.Slice(literals, func(i, j int) bool {
		left, right := expressionToken(literals[i]), expressionToken(literals[j])
		if left.File != right.File {
			return left.File < right.File
		}
		if left.Line != right.Line {
			return left.Line < right.Line
		}
		return left.Column < right.Column
	})
	for _, literal := range literals {
		typ := a.expressionTypes[literal]
		if _, err := constant.FloatLiteralBits(literal.TokenLiteral(), uint16(typ.FloatBits)); err != nil {
			a.addErrorAtTokenWithMetadata(expressionToken(literal), diagnostics.FloatLiteralOutOfRange,
				"Use a representable literal or a wider numeric type. Decimal literals retain their exact spelling without a binary floating-point conversion.",
				"floating literal %s exceeds the finite range of %s (binary%d)", literal.TokenLiteral(), typeDisplayName(typ), typ.FloatBits)
		}
	}
}
