package semantic

import (
	"fmt"
	"math/big"

	"sec/internal/ast"
)

// buildBitExtraction lowers `bit[N](value)` from Sema's resolved extraction as
// one int.bit-extract: the operand is evaluated once and the result is the
// unsigned carrier Sema selected for N. The operation is pure and infallible,
// so it has no failure edge.
//
// Rules:
//   - rules/types/types.md — "Explicit low-bit extraction"
//   - rules/corrections/applied/md046-md056-bit-extraction-correction-20261012.md — §§ 2-4
func (fb *functionBuilder) buildBitExtraction(expr *ast.BitExtractionExpression) (builtValue, error) {
	fact, ok := fb.owner.analyzer.ResolvedBitExtractionOf(expr)
	if !ok {
		return builtValue{}, fb.unsupported("unresolved bit extraction", expr.Token)
	}
	sourceType, err := fb.owner.internType(fact.SourceType)
	if err != nil {
		return builtValue{}, err
	}
	resultType, err := fb.owner.internType(fact.ResultType)
	if err != nil {
		return builtValue{}, err
	}
	source, err := fb.buildExpr(expr.Value, sourceType)
	if err != nil {
		return builtValue{}, err
	}
	return fb.result(Operation{Kind: OpIntBitExtract, Operands: []ValueID{source.id}, Integer: big.NewInt(fact.Width), Location: location(expr.Token)}, resultType), nil
}

// verifyBitExtract checks an int.bit-extract: an integer, byte or char
// operand at least N bits wide, 1 <= N <= 256, and the smallest unsigned
// result carrier holding N bits.
func verifyBitExtract(types *TypeTable, op Operation, values map[ValueID]Value) error {
	if len(op.Operands) != 1 || len(op.Results) != 1 || op.Integer == nil || !op.Integer.IsInt64() {
		return fmt.Errorf("invalid int.bit-extract")
	}
	width := op.Integer.Int64()
	if width < 1 || width > 256 {
		return fmt.Errorf("int.bit-extract width %d is outside 1..256", width)
	}
	source, ok := types.Lookup(values[op.Operands[0]].Type)
	if !ok || (source.Kind != TypeInt && source.Kind != TypeUint && source.Kind != TypeByte && source.Kind != TypeChar) {
		return fmt.Errorf("int.bit-extract operand is not an integer, byte or char")
	}
	sourceWidth := int64(source.BitWidth)
	if source.Kind == TypeByte || source.Kind == TypeChar {
		sourceWidth = 8
	}
	if sourceWidth > 0 && width > sourceWidth {
		return fmt.Errorf("int.bit-extract width %d exceeds the %d-bit operand", width, sourceWidth)
	}
	result, ok := types.Lookup(op.Results[0].Type)
	if !ok || result.Kind != TypeUint || result.Signed {
		return fmt.Errorf("int.bit-extract result is not an unsigned integer")
	}
	carrier := int64(8)
	for carrier < width {
		carrier *= 2
	}
	if int64(result.BitWidth) != carrier {
		return fmt.Errorf("int.bit-extract result width %d, want the %d-bit carrier for %d bits", result.BitWidth, carrier, width)
	}
	return nil
}
