package sema

import (
	"math/big"

	"sec/internal/ast"
	"sec/internal/diagnostics"
)

var (
	surrogateLow  = big.NewInt(0xD800)
	surrogateHigh = big.NewInt(0xDFFF)
)

// resolveRangeSegment resolves `lower..upper` or `lower..<upper` in an array
// literal or an Append argument to the inclusive compile-time bounds and the
// element count it contributes. The element type is the target element type
// when one exists, otherwise the bounds' common type; it must be a built-in
// integer type or rune without contracts. A descending segment is invalid, an
// exclusive segment with equal bounds is empty, and a rune segment may not
// cover a surrogate code point.
//
// Rules:
//   - rules/collections/collections.md — § 5.6a "Range segments in array literals", § 6.7 "Append"
//   - rules/foundations/operators.md — "Inclusive range", "Exclusive upper range"
//   - rules/types/types.md — rune holds Unicode scalar values only
func (a *Analyzer) resolveRangeSegment(segment *ast.RangeExpression, expected Type) (ResolvedArrayLiteralEntry, bool) {
	invalid := func(format string, args ...any) (ResolvedArrayLiteralEntry, bool) {
		a.addErrorAtTokenWithMetadata(segment.Token, diagnostics.ArrayRangeSegmentInvalid,
			"A range segment contributes every value from its lower to its upper bound; both bounds are compile-time integer or rune constants of the element type.",
			format, args...)
		return ResolvedArrayLiteralEntry{}, false
	}
	if segment.Start == nil || segment.End == nil {
		return invalid("range segment requires both bounds")
	}
	hasExpected := expected.Kind != InvalidType && expected.Kind != ""
	infer := func(bound ast.Expression) Type {
		if hasExpected {
			typ, _ := a.inferExpressionWithExpected(bound, expected)
			return typ
		}
		typ, _ := a.inferExpression(bound)
		return typ
	}
	lowerType, upperType := infer(segment.Start), infer(segment.End)
	if lowerType.Kind == InvalidType || upperType.Kind == InvalidType {
		return ResolvedArrayLiteralEntry{}, false
	}
	elementType := lowerType
	if hasExpected {
		elementType = expected
		for _, bound := range []struct {
			expr ast.Expression
			typ  Type
		}{{segment.Start, lowerType}, {segment.End, upperType}} {
			if !a.canInitialize(expected, bound.typ, bound.expr) {
				return invalid("range segment bound must be %s, got %s", typeDisplayName(expected), typeDisplayName(bound.typ))
			}
		}
	} else if !sameConcreteType(lowerType, upperType) {
		return invalid("range segment bounds must have one type, got %s and %s", typeDisplayName(lowerType), typeDisplayName(upperType))
	}
	isRune := elementType.Kind == RuneType
	if !isRune && !isIntegerType(elementType) {
		return invalid("range segment element type must be an integer type or rune, got %s", typeDisplayName(elementType))
	}
	if len(elementType.Contracts) > 0 {
		return invalid("range segment element type %s has contracts; write the values explicitly", typeDisplayName(elementType))
	}
	lower, lowerOK := a.rangeBoundValue(segment.Start, isRune)
	upper, upperOK := a.rangeBoundValue(segment.End, isRune)
	if !lowerOK || !upperOK {
		return invalid("range segment bounds must be compile-time constants")
	}
	if minimum, maximum, bounded := integerTypeInterval(elementType); bounded && !isRune {
		for _, bound := range []*big.Int{lower, upper} {
			if bound.Cmp(minimum) < 0 || bound.Cmp(maximum) > 0 {
				return invalid("range segment bound %s does not fit %s", bound.String(), typeDisplayName(elementType))
			}
		}
	}
	inclusiveUpper := new(big.Int).Set(upper)
	if segment.Exclusive {
		inclusiveUpper.Sub(inclusiveUpper, big.NewInt(1))
	}
	count := new(big.Int).Sub(inclusiveUpper, lower)
	count.Add(count, big.NewInt(1))
	if count.Sign() < 0 || (!segment.Exclusive && lower.Cmp(upper) > 0) {
		return invalid("range segment lower bound %s exceeds upper bound %s", lower.String(), upper.String())
	}
	if isRune && count.Sign() > 0 && lower.Cmp(surrogateHigh) <= 0 && inclusiveUpper.Cmp(surrogateLow) >= 0 {
		return invalid("rune range segment %s..%s covers surrogate code points U+D800..U+DFFF, which are not runes", lower.String(), upper.String())
	}
	return ResolvedArrayLiteralEntry{
		Kind:       ArrayLiteralRange,
		Type:       elementType,
		Length:     count,
		Action:     ArrayTransferConstructDirect,
		RangeLower: lower,
		RangeUpper: inclusiveUpper,
	}, true
}

// rangeBoundValue evaluates one compile-time range bound: a rune or character
// literal yields its Unicode scalar value, any other bound the shared
// compile-time integer evaluation.
func (a *Analyzer) rangeBoundValue(bound ast.Expression, isRune bool) (*big.Int, bool) {
	if isRune {
		switch literal := bound.(type) {
		case *ast.IntegerLiteral:
			if suffix := literal.Suffix(); suffix == "r" || suffix == "t" {
				return ast.ParseIntegerLiteralLexeme(literal.Token.Lexeme)
			}
		case *ast.CharLiteral:
			runes := []rune(literal.Value)
			if len(runes) == 1 {
				return big.NewInt(int64(runes[0])), true
			}
			return nil, false
		}
	}
	return a.integerConstantValue(bound)
}

// ResolvedRangeAppendOf returns the resolved range segment of an Append call
// whose argument is a range.
func (a *Analyzer) ResolvedRangeAppendOf(call *ast.CallExpression) (ResolvedArrayLiteralEntry, bool) {
	if a == nil || call == nil {
		return ResolvedArrayLiteralEntry{}, false
	}
	entry, ok := a.resolvedRangeAppends[call]
	return entry, ok
}
