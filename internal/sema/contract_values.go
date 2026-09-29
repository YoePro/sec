package sema

import (
	"math/big"

	"sec/internal/ast"
)

// checkCompileTimeContractExpression validates source values whose complete
// value is known while analyzing the destination boundary. Runtime values are
// deliberately left to the canonical fallible conversion path.
//
// Rules:
//   - rules/types/contracts.md — "Initialization and assignment"
//   - rules/types/contracts.md — "String and collection contracts"
func (a *Analyzer) checkCompileTimeContractExpression(typ Type, expr ast.Expression) bool {
	if a.checkIntegerExpressionRange(typ, expr) {
		return true
	}
	return a.checkStringLiteralContracts(typ, expr)
}

// checkDeclaredContractExpression excludes plain representation checks at new
// call and return boundaries. In particular, an ordinary typed integer
// operation retains its S1023 warning and checked runtime semantics when the
// destination has no named contracts.
func (a *Analyzer) checkDeclaredContractExpression(typ Type, expr ast.Expression) bool {
	if !hasContracts(typ) {
		return false
	}
	return a.checkCompileTimeContractExpression(typ, expr)
}

// checkStringLiteralContracts checks every represented string contract without
// reinterpreting runtime strings as compile-time values. StringLiteral.Value is
// already escape-decoded by the parser, so length uses the canonical UTF-8 byte
// count also exposed by string.Len.
func (a *Analyzer) checkStringLiteralContracts(typ Type, expr ast.Expression) bool {
	literal, ok := expr.(*ast.StringLiteral)
	if !ok || typ.Kind != StringType || len(typ.Contracts) == 0 {
		return false
	}
	constant := DefaultConstant{Kind: StringType, Lexeme: literal.Token.Lexeme, String: literal.Value}
	for _, contract := range typ.Contracts {
		switch contract := contract.(type) {
		case MembershipContract:
			member := false
			for _, allowed := range contract.Values {
				if defaultConstantsEqual(constant, allowed) {
					member = true
					break
				}
			}
			if !member {
				a.addErrorAtToken(expressionToken(expr), "string value %q violates in contract %s", literal.Value, typ.Name)
				return true
			}
		case LengthContract:
			if !stringLengthSatisfiesContract(literal.Value, contract) {
				a.addErrorAtToken(expressionToken(expr), "string value %q violates %s contract %s %s", literal.Value, contract.Name, typ.Name, contract.Value.String())
				return true
			}
		case MarkerContract:
			if contract.Name == "notEmpty" && literal.Value == "" {
				a.addErrorAtToken(expressionToken(expr), "string value %q violates notEmpty contract %s", literal.Value, typ.Name)
				return true
			}
		}
	}
	return false
}

// checkCompileTimeCallArgumentContracts applies the same literal proof after
// overload resolution has selected the exact parameter types. Contracts do not
// participate in overload ranking.
func (a *Analyzer) checkCompileTimeCallArgumentContracts(function Function, arguments []ast.Expression) bool {
	invalid := false
	for index, argument := range arguments {
		parameter, ok := functionParameterForArgument(function, index)
		if !ok {
			continue
		}
		if a.checkDeclaredContractExpression(parameter.Type, argument) {
			invalid = true
		}
	}
	return invalid
}

// checkArrayLiteralContracts consumes the exact compact literal length and
// compile-time direct element values in declared contract order. Unknown
// values and spreads receive no invented compile-time uniqueness proof.
//
// Rules:
//   - rules/types/contracts.md — "Composition"
//   - rules/types/contracts.md — "String and collection contracts"
//   - rules/types/contracts.md — "Initialization and assignment"
//   - rules/collections/collections.md — §5.5 "Array literals" and §5.6 "Spread in fixed-array literals"
func (a *Analyzer) checkArrayLiteralContracts(typ Type, literal *ast.ArrayLiteral, length *big.Int) bool {
	if typ.Kind != ArrayType || literal == nil || length == nil || length.Sign() < 0 || len(typ.Contracts) == 0 {
		return false
	}
	typeName := typeDisplayName(typ)
	if typ.Named && typ.Name != "" {
		typeName = typ.Name
	}
	for _, contract := range typ.Contracts {
		switch contract := contract.(type) {
		case LengthContract:
			if !knownLengthSatisfiesContract(length, contract) {
				a.addErrorAtToken(literal.Token, "array literal length %s violates %s contract %s %s", length.String(), contract.Name, typeName, contract.Value.String())
				return true
			}
		case MarkerContract:
			switch contract.Name {
			case "notEmpty":
				if length.Sign() == 0 {
					a.addErrorAtToken(literal.Token, "array literal length 0 violates notEmpty contract %s", typeName)
					return true
				}
			case "unique":
				duplicate, original, ok := duplicateArrayLiteralConstant(literal)
				if ok {
					a.addErrorAtToken(expressionToken(literal.Elements[duplicate]), "array literal element %d duplicates element %d under unique contract %s", duplicate+1, original+1, typeName)
					return true
				}
			}
		}
	}
	return false
}

// duplicateArrayLiteralConstant returns the first source-ordered pair whose
// direct elements have proven compile-time semantic equality. Spread contents
// and runtime expressions are intentionally opaque.
//
// Rules:
//   - rules/types/contracts.md — "String and collection contracts"
//   - rules/types/contracts.md — "Ordered membership" semantic equality
func duplicateArrayLiteralConstant(literal *ast.ArrayLiteral) (duplicate int, original int, ok bool) {
	if literal == nil {
		return 0, 0, false
	}
	type knownElement struct {
		index int
		value DefaultConstant
	}
	known := make([]knownElement, 0, len(literal.Elements))
	for index, element := range literal.Elements {
		if _, spread := element.(*ast.SpreadExpression); spread {
			continue
		}
		constant, constantOK := defaultConstantFromExpression(element)
		if !constantOK {
			continue
		}
		for _, previous := range known {
			if defaultConstantsEqual(constant, previous.value) {
				return index, previous.index, true
			}
		}
		known = append(known, knownElement{index: index, value: constant})
	}
	return 0, 0, false
}
