package sema

import (
	"math/big"

	"sec/internal/ast"
)

// Explicit low-bit extraction `bit[N](value)` (MD-046/MD-056). The operation
// selects bits 0..N-1 of an integer's fixed-width two's-complement
// representation: value mod 2^N as the unique non-negative remainder. It is
// pure, infallible, allocation-free and endian-independent, needs neither try
// nor unsafe, never extends a narrower source, and returns the smallest
// existing unsigned type holding N bits with the proven interval 0..2^N-1.
//
// Rules:
//   - rules/types/types.md — "Explicit low-bit extraction"
//   - rules/declarations/registers.md — § 4, § 12
//   - rules/foundations/grammar.md — BitExtractionExpression
//   - rules/corrections/applied/md046-md056-bit-extraction-correction-20261012.md — §§ 2-6, 9

// MaximumBitExtractionWidth is the largest N of `bit[N](value)`.
const MaximumBitExtractionWidth = 256

// ResolvedBitExtraction is the immutable fact of one valid extraction.
type ResolvedBitExtraction struct {
	Width       int64
	SourceType  Type
	SourceWidth int64
	ResultType  Type
}

// ResolvedBitExtractionOf returns the resolved extraction of expr.
func (a *Analyzer) ResolvedBitExtractionOf(expr *ast.BitExtractionExpression) (ResolvedBitExtraction, bool) {
	if a == nil || expr == nil {
		return ResolvedBitExtraction{}, false
	}
	fact, ok := a.resolvedBitExtractions[expr]
	return fact, ok
}

func (a *Analyzer) inferBitExtractionExpression(expr *ast.BitExtractionExpression) (Type, expressionValue) {
	display := expressionValue{Display: expr.String()}
	invalid := Type{Kind: InvalidType}
	width, constant := a.integerConstantValue(expr.Width)
	if !constant {
		a.inferExpression(expr.Width)
		a.addErrorAtTokenWithMetadata(expressionToken(expr.Width), "",
			"Write the number of low bits to extract as a compile-time integer constant, for example bit[8](value).",
			"bit[N] extraction width must be a compile-time integer constant")
		a.inferExpression(expr.Value)
		return invalid, display
	}
	if width.Sign() <= 0 || width.Cmp(big.NewInt(MaximumBitExtractionWidth)) > 0 {
		a.addErrorAtTokenWithMetadata(expressionToken(expr.Width), "",
			"bit[N](value) extracts 1 through 256 low bits; choose a width in that range.",
			"bit[%s] extraction width must be in 1..256", width.String())
		a.inferExpression(expr.Value)
		return invalid, display
	}
	sourceType, _ := a.inferExpression(expr.Value)
	if sourceType.Kind == InvalidType {
		return invalid, display
	}
	sourceWidth, reason := a.bitExtractionSourceWidth(sourceType)
	if reason != "" {
		a.addErrorAtTokenWithMetadata(expressionToken(expr.Value), "",
			"bit[N](value) reads the bits of a primitive integer, byte or char. Convert a named, unit-bearing or other value explicitly to a primitive integer first; extraction never strips its contracts or unit.",
			"bit[%s] cannot extract bits from %s: %s", width.String(), typeDisplayName(sourceType), reason)
		return invalid, display
	}
	n := width.Int64()
	if n > sourceWidth {
		a.addErrorAtTokenWithMetadata(expr.Token, "",
			"bit[N](value) never extends its source. Extract at most the source width, or convert the value to a wider integer first when value preservation is intended.",
			"bit[%d] exceeds the %d-bit width of %s", n, sourceWidth, typeDisplayName(sourceType))
		return invalid, display
	}
	result := a.bitExtractionResultType(n)
	if a.resolvedBitExtractions == nil {
		a.resolvedBitExtractions = map[*ast.BitExtractionExpression]ResolvedBitExtraction{}
	}
	a.resolvedBitExtractions[expr] = ResolvedBitExtraction{Width: n, SourceType: sourceType, SourceWidth: sourceWidth, ResultType: result}
	return result, display
}

// bitExtractionSourceWidth returns the representation width of an accepted
// source, or why the source is not accepted.
func (a *Analyzer) bitExtractionSourceWidth(typ Type) (int64, string) {
	if typ.Named || typ.Unit != "" || !typ.Dimension.IsZero() || len(typ.Contracts) > 0 {
		return 0, "named, constrained and unit-bearing values need an explicit conversion first"
	}
	switch typ.Kind {
	case CharType:
		return 8, ""
	case IntType, UintType:
		if typ.Name == "int" || typ.Name == "uint" {
			// The target-sized width comes from the CompilationPlan's types.
			if selected := a.types[typ.Name]; selected.BitWidth > 0 {
				typ.BitWidth = selected.BitWidth
			}
			if typ.BitWidth <= 0 {
				return 0, "the width of " + typ.Name + " is unavailable without a selected target"
			}
			return typ.BitWidth, ""
		}
		if typ.BitWidth > 0 {
			return typ.BitWidth, ""
		}
		if typ.MaxInteger == nil {
			return 0, "its width is unknown"
		}
		bits := int64(typ.MaxInteger.BitLen())
		if typ.Kind == IntType {
			bits++
		}
		return bits, ""
	}
	return 0, "only primitive integers, byte and char are accepted"
}

// bitExtractionResultType is the smallest existing unsigned type holding n
// bits.
func (a *Analyzer) bitExtractionResultType(n int64) Type {
	name := "uint256"
	switch {
	case n <= 8:
		name = "uint8"
	case n <= 16:
		name = "uint16"
	case n <= 32:
		name = "uint32"
	case n <= 64:
		name = "uint64"
	case n <= 128:
		name = "uint128"
	}
	return a.types[name]
}

// bitExtractionInterval is the proven interval 0..2^N-1 of an extraction.
func (a *Analyzer) bitExtractionInterval(expr *ast.BitExtractionExpression) (*big.Int, *big.Int, bool) {
	fact, ok := a.resolvedBitExtractions[expr]
	if !ok {
		return nil, nil, false
	}
	maximum := new(big.Int).Lsh(big.NewInt(1), uint(fact.Width))
	return big.NewInt(0), maximum.Sub(maximum, big.NewInt(1)), true
}

// bitExtractionFitsRegisterField reports an explicit extraction whose width
// does not exceed a plain (unit-free) register field width, so its proven
// interval fits the field (registers.md § 12, MD-046 § 6).
func (a *Analyzer) bitExtractionFitsRegisterField(member *ast.MemberExpression, value ast.Expression) bool {
	extraction, ok := value.(*ast.BitExtractionExpression)
	if !ok || member == nil || member.Property == nil {
		return false
	}
	fact, ok := a.resolvedBitExtractions[extraction]
	if !ok {
		return false
	}
	objectType := dereferenceType(a.expressionTypes[member.Object])
	if objectType.Kind != RegisterType {
		return false
	}
	for _, field := range objectType.RegisterFields {
		if field.Name == member.Property.Value {
			return field.Unit == "" && field.Type.Unit == "" && fact.Width <= field.Width
		}
	}
	return false
}
