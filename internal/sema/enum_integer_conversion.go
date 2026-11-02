package sema

import (
	"math/big"
	"sec/internal/ast"
)

// enumToIntegerConversionResultType admits a direct integer result only when
// the shared exactness proof establishes representability for the active plan.
// Runtime member reads cannot borrow the value of a same-spelled enum case.
// Rules: rules/declarations/enums.md — §12.1;
// rules/types/types.md — "Exact integer conversion proof" (MD-104).
func (a *Analyzer) enumToIntegerConversionResultType(target Type, source Type, expr ast.Expression) Type {
	value, known := a.enumConstantValue(source, expr)
	if known && a.checkIntegerValueRange(target, value, expressionToken(expr)) {
		return Type{Kind: InvalidType}
	}
	if a.integerConversionProof(target, source, value).Proven() {
		return target
	}
	// MD-012/MD-104: intrinsic integer non-representability belongs to
	// ConversionError, independently of the arithmetic failure channel.
	errorType := a.types["ConversionError"]
	return Type{Name: "Result", Kind: ResultType, TypeArgs: []Type{target, errorType}}
}

// enumConstantValue reads the resolved numeric case identity, not the selector
// spelling. Unknown runtime values remain subject to the full enum domain.
// Rules: rules/declarations/enums.md — §§4,7,12.1;
// rules/types/types.md — "Exact integer conversion proof".
func (a *Analyzer) enumConstantValue(enumType Type, expr ast.Expression) (*big.Int, bool) {
	member, ok := expr.(*ast.MemberExpression)
	if !ok {
		return nil, false
	}
	resolved, ok := a.ResolvedEnumCaseOf(member)
	if !ok || resolved.Value == nil || !sameConcreteType(resolved.EnumType, enumType) {
		return nil, false
	}
	return resolved.Value, true
}

// closedEnumIntegerDomain bounds every unique numeric value class of a closed
// enum. Its declared inventory must be complete before aliases are normalized:
// the public class query intentionally skips unresolved members for tooling.
// Rules: rules/declarations/enums.md — §§7,9,12.1;
// rules/types/types.md — "Exact integer conversion proof" (MD-104).
func closedEnumIntegerDomain(source Type) (*big.Int, *big.Int, bool) {
	if source.Kind != EnumType || enumIsOpenBitDomain(source) || len(source.EnumValues) == 0 || len(source.EnumValues) != len(source.EnumConsts) {
		return nil, nil, false
	}
	declared := make(map[string]bool, len(source.EnumValues))
	for _, name := range source.EnumValues {
		value, found := source.EnumConsts[name]
		if !found || declared[name] || value.Name != name || value.Value == nil || value.StringValue != nil {
			return nil, nil, false
		}
		declared[name] = true
	}
	var minimum, maximum *big.Int
	for _, class := range EnumValueClasses(source) {
		value := class.IntegerValue
		if value == nil {
			return nil, nil, false
		}
		if minimum == nil || value.Cmp(minimum) < 0 {
			minimum = value
		}
		if maximum == nil || value.Cmp(maximum) > 0 {
			maximum = value
		}
	}
	if minimum == nil {
		return nil, nil, false
	}
	return new(big.Int).Set(minimum), new(big.Int).Set(maximum), true
}

// openEnumIntegerDomain derives the complete unsigned domain from the resolved
// width. Cached extrema and named cases cannot bound an open hardware enum.
// Rules: rules/declarations/enums.md — §§2,10,12.1;
// rules/types/types.md — "Exact integer conversion proof" (MD-104).
func openEnumIntegerDomain(source Type) (*big.Int, *big.Int, bool) {
	if !enumIsOpenBitDomain(source) || source.BitWidth < 1 || source.BitWidth > 256 {
		return nil, nil, false
	}
	maximum := new(big.Int).Sub(new(big.Int).Lsh(big.NewInt(1), uint(source.BitWidth)), big.NewInt(1))
	return big.NewInt(0), maximum, true
}
