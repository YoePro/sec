package sema

import (
	"strings"

	"sec/internal/ast"
	"sec/internal/diagnostics"
	"sec/internal/layout"
	"sec/internal/lexer"
)

// registerCFundamentalTypes installs every compiler-known C:: fundamental
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

// isForeignCScalar reports whether typ is a compiler-known C:: scalar.
func isForeignCScalar(typ Type) bool {
	return strings.HasPrefix(typ.Name, "C::") && typ.Intrinsic
}

// canInitializeForeignCScalar keeps C:: scalars nominally distinct from
// every other scalar, including Sec bool for C::bool. Only literal shaping
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

// reportUnresolvedForeignType gives C:: and c:: names their owning
// diagnostics instead of the generic unknown-type error.
//
// Rules:
//   - rules/platform/ffi.md — §5, §6, §52 "Sema requirements"; §53
func (a *Analyzer) reportUnresolvedForeignType(name string, token lexer.Token) bool {
	switch {
	case strings.HasPrefix(name, "C::"):
		if !a.cABIModel.Defined() {
			a.addErrorAtTokenWithMetadata(token, diagnostics.ForeignCABIModelUnavailable,
				"Select a target whose CompilationPlan defines a C ABI model.",
				"%s requires the active target's C ABI model, but the current target defines none", name)
			return true
		}
		a.addErrorAtTokenWithMetadata(token, diagnostics.ForeignUnknownCFundamentalType,
			"Use one of the fundamental C ABI scalars: "+strings.Join(cFundamentalSpellings(), ", ")+".",
			"%s is not a compiler-known fundamental C ABI type", name)
		return true
	case strings.HasPrefix(name, "c::"):
		a.addErrorAtTokenWithMetadata(token, diagnostics.ForeignUnresolvedCBindingType,
			"c:: binding types are supplied by the selected target/library binding environment, which is not available yet.",
			"%s cannot be resolved: no C binding environment defines it for the active target", name)
		return true
	}
	return false
}

func cFundamentalSpellings() []string {
	names := layout.CFundamentalTypeNames()
	spellings := make([]string, len(names))
	for index, name := range names {
		spellings[index] = "C::" + name
	}
	return spellings
}

// isBooleanLiteral reports a direct true/false literal, the boolean form of
// literal shaping for C::bool.
func isBooleanLiteral(expr ast.Expression) bool {
	_, ok := expr.(*ast.BooleanLiteral)
	return ok
}
