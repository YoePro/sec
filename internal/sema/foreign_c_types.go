package sema

import (
	"fmt"
	"math/big"
	"strings"

	"sec/internal/ast"
	"sec/internal/diagnostics"
	"sec/internal/layout"
	"sec/internal/lexer"
)

// registerCFundamentalTypes installs every compiler-known c:: fundamental
// scalar resolved through the active target C ABI model. Each is a distinct
// Sec type: representation equality with a Sec scalar never creates type
// identity, so ordinary Sec rules already require explicit conversion while
// still permitting representable literal shaping.
//
// Rules:
//   - rules/platform/ffi.md — §5 "Fundamental C ABI scalar family"; §7; §8; §52
//   - rules/platform/abi.md — § 19 "C scalar representation"
func (a *Analyzer) registerCFundamentalTypes(model layout.CABIModel) {
	if !model.Defined() {
		return
	}
	a.cABIModel = model
	for _, name := range layout.CFundamentalTypeNames() {
		scalar, ok := model.Fundamental(name)
		if !ok {
			continue
		}
		a.types[scalar.Name] = cScalarType(scalar)
	}
}

func cScalarType(scalar layout.CScalar) Type {
	switch scalar.Kind {
	case layout.CScalarSigned:
		typ := targetSignedIntegerType(scalar.Name, scalar.ValueBits)
		typ.Intrinsic = true
		return typ
	case layout.CScalarUnsigned:
		typ := targetUnsignedIntegerType(scalar.Name, scalar.ValueBits)
		typ.Intrinsic = true
		return typ
	case layout.CScalarFloat:
		return Type{Name: scalar.Name, Kind: FloatType, FloatBits: int(scalar.ValueBits), BitWidth: int64(scalar.StorageBits), Intrinsic: true}
	default:
		return Type{Name: scalar.Name, Kind: BoolType, BitWidth: int64(scalar.StorageBits), Intrinsic: true}
	}
}

// isForeignCScalar reports whether typ is a compiler-known c:: scalar.
func isForeignCScalar(typ Type) bool {
	return strings.HasPrefix(typ.Name, "c::") && typ.Intrinsic
}

// canInitializeForeignCScalar keeps c:: scalars nominally distinct from
// every other scalar, including Sec bool for c::bool. Only literal shaping
// crosses that boundary implicitly; everything else needs an explicit
// conversion.
//
// Rules:
//   - rules/platform/ffi.md — §7 "C scalar types and Sec scalar types remain distinct"; §8 literal shaping
func canInitializeForeignCScalar(target, value Type, literal bool) (bool, bool) {
	if !isForeignCScalar(target) && !isForeignCScalar(value) {
		return false, false
	}
	if target.Name == value.Name {
		return true, true
	}
	if !isForeignCScalar(target) || !literal {
		return false, true
	}
	if target.Kind == BoolType {
		return value.Kind == BoolType, true
	}
	return isNumericType(target) && isNumericType(value), true
}

// reportUnresolvedForeignType gives c:: and c:: names their owning
// diagnostics instead of the generic unknown-type error.
//
// Rules:
//   - rules/platform/ffi.md — §5, §6, §52 "Sema requirements"; §53
func (a *Analyzer) reportUnresolvedForeignType(name string, token lexer.Token) bool {
	if !strings.HasPrefix(name, "c::") {
		return false
	}
	// MD-020 §2.2, §2.8: one segment names a compiler-known fundamental C
	// type resolved through the active C ABI model; more segments name a
	// binding of the selected platform C binding environment (MD-021).
	if strings.Count(name, "::") == 1 {
		if !a.cABIModel.Defined() {
			a.addErrorAtTokenWithMetadata(token, diagnostics.ForeignCABIModelUnavailable,
				"Select a target whose CompilationPlan defines a C ABI model.",
				"%s requires the active target's C ABI model, but the current target defines none", name)
			return true
		}
		a.addErrorAtTokenWithMetadata(token, diagnostics.ForeignUnknownCFundamentalType,
			"Use one of the fundamental C ABI scalars ("+strings.Join(cFundamentalSpellings(), ", ")+") or a binding name such as c::stddef::size_t.",
			"%s is not a compiler-known fundamental C ABI type", name)
		return true
	}
	a.addErrorAtTokenWithMetadata(token, diagnostics.ForeignUnresolvedCBindingType,
		"c:: binding types are supplied by the selected target/library binding environment, which is not available yet.",
		"%s cannot be resolved: no C binding environment defines it for the active target", name)
	return true
}

func cFundamentalSpellings() []string {
	names := layout.CFundamentalTypeNames()
	spellings := make([]string, len(names))
	for index, name := range names {
		spellings[index] = "c::" + name
	}
	return spellings
}

// isBooleanLiteral reports a direct true/false literal, the boolean form of
// literal shaping for c::bool.
func isBooleanLiteral(expr ast.Expression) bool {
	_, ok := expr.(*ast.BooleanLiteral)
	return ok
}

// shapeForeignCharacterLiteral shapes a single-quoted character literal into
// c::char, c::schar, or c::uchar when the decoded Unicode scalar's code-point
// number is representable by that target's C ABI integer range. No text
// encoding is applied and there is no first-byte fallback; plain c::char
// takes its signedness from the active ABI model. It reports handled=false
// for any other expression or target, so typed rune/char values still need
// an explicit conversion. An unrepresentable literal reports S1138 and is
// otherwise treated as handled so no second mismatch is reported.
//
// Rules:
//   - rules/platform/ffi.md — §8 "Literal shaping"
//   - rules/corrections/applied/md012-md022-conversion-errors-ffi-literals-correction-20261008.md — §2.2–2.4, §2.6
func (a *Analyzer) shapeForeignCharacterLiteral(expr ast.Expression, target Type) (accepted bool, handled bool) {
	literal, ok := expr.(*ast.CharLiteral)
	if !ok || !isForeignCScalar(target) || target.MaxInteger == nil {
		return false, false
	}
	switch target.Name {
	case "c::char", "c::schar", "c::uchar":
	default:
		return false, false
	}
	scalar, valid := characterLiteralScalar(literal)
	if !valid {
		return false, false
	}
	if big.NewInt(int64(scalar)).Cmp(target.MaxInteger) > 0 {
		a.addErrorAtTokenWithMetadata(literal.Token, diagnostics.CharLiteralOutOfRange,
			fmt.Sprintf("Shaping uses the code-point number U+%04X (%d), never an encoding; %s holds %s..%s on the active C ABI. Use a character in that range or an explicit integer value.", scalar, scalar, target.Name, target.MinInteger, target.MaxInteger),
			"character literal %s does not fit %s", literal.Token.Lexeme, target.Name)
		a.expressionTypes[expr] = Type{Kind: InvalidType}
		// The focused diagnostic replaces the generic type mismatch.
		return true, true
	}
	a.expressionTypes[expr] = target
	return true, true
}
