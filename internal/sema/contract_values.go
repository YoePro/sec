package sema

import (
	"math/big"

	"sec/internal/ast"
	"sec/internal/diagnostics"
	"sec/internal/lexer"
)

// checkCompileTimeContractExpression validates source values whose complete
// value is known while analyzing the destination boundary. Runtime values are
// deliberately left to the canonical fallible conversion path.
//
// Rules:
//   - rules/types/contracts.md — "Initialization and assignment"
//   - rules/types/contracts.md — "String and collection contracts"
func (a *Analyzer) checkCompileTimeContractExpression(typ Type, expr ast.Expression) bool {
	if a.checkEmptyListLiteralContracts(typ, expr) {
		return true
	}
	if a.checkIntegerExpressionRange(typ, expr) {
		return true
	}
	if a.checkNominalMembershipValue(typ, expr) {
		return true
	}
	return a.checkStringLiteralContracts(typ, expr)
}

// checkDeclaredContractExpression excludes plain representation checks of
// typed operations at call and return boundaries: an ordinary typed integer
// operation retains its S1023 warning and checked runtime semantics when the
// destination has no named contracts. An untyped integer literal or literal
// constant expression is instead shaped by the destination and must be
// representable by it, including the target-sized int and uint bounds.
//
// Rules:
//   - rules/types/types.md — "Unsuffixed literal inference", "Context shaping"
//   - rules/types/types.md — "int and uint" (target-selected width)
func (a *Analyzer) checkDeclaredContractExpression(typ Type, expr ast.Expression) bool {
	if !hasContracts(typ) {
		if isUntypedNumericExpression(expr) {
			return a.checkIntegerExpressionRange(typ, expr)
		}
		return false
	}
	return a.checkCompileTimeContractExpression(typ, expr)
}

// checkStringLiteralContracts checks every represented string contract without
// reinterpreting runtime strings as compile-time values. StringLiteral.Value is
// already escape-decoded by the parser. Len contracts count Unicode scalars;
// explicit ByteLen contracts count encoded UTF-8 bytes.
// Rules: rules/types/contracts.md — String and collection contracts (revision 2.1).
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
				a.addContractError(expressionToken(expr), contract, diagnostics.ValueViolatesContract, "use a value satisfying every contract of the named type", "string value %q violates in contract %s", literal.Value, typ.Name)
				return true
			}
		case LengthContract:
			if !stringLengthSatisfiesContract(literal.Value, contract) {
				a.addContractError(expressionToken(expr), contract, diagnostics.ValueViolatesContract, "use a value satisfying every contract of the named type", "string value %q violates %s contract %s %s", literal.Value, contract.Name, typ.Name, contract.Value.String())
				return true
			}
		case MarkerContract:
			if contract.Name == "notEmpty" && literal.Value == "" {
				a.addContractError(expressionToken(expr), contract, diagnostics.ValueViolatesContract, "use a value satisfying every contract of the named type", "string value %q violates notEmpty contract %s", literal.Value, typ.Name)
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
				a.addContractError(literal.Token, contract, diagnostics.ValueViolatesContract, "use a value satisfying every contract of the named type", "array literal length %s violates %s contract %s %s", length.String(), contract.Name, typeName, contract.Value.String())
				return true
			}
		case MarkerContract:
			switch contract.Name {
			case "notEmpty":
				if length.Sign() == 0 {
					a.addContractError(literal.Token, contract, diagnostics.ValueViolatesContract, "use a value satisfying every contract of the named type", "array literal length 0 violates notEmpty contract %s", typeName)
					return true
				}
			case "unique":
				duplicate, original, ok := a.duplicateArrayLiteralConstant(typ, literal)
				if ok {
					a.addContractError(expressionToken(literal.Elements[duplicate]), contract, diagnostics.ValueViolatesContract, "use a value satisfying every contract of the named type", "array literal element %d duplicates element %d under unique contract %s", duplicate+1, original+1, typeName)
					return true
				}
			}
		}
	}
	return false
}

// checkEmptyListLiteralContracts proves the canonical explicit empty collection
// shape at the destination boundary, including constrained named derivations.
// Unsupported or runtime collection expressions receive no length proof.
// Rules: rules/types/contracts.md — "String and collection contracts",
// "Initialization and assignment"; rules/collections/collections.md — §13.3;
// rules/types/default_values.md — "List defaults", "Defaults and contracts".
func (a *Analyzer) checkEmptyListLiteralContracts(typ Type, expr ast.Expression) bool {
	literal, ok := expr.(*ast.CollectionLiteral)
	if !ok || literal.Invalid || !isDefaultableEmptyListType(typ) {
		return false
	}
	return a.checkKnownCollectionLengthContracts(typ, expressionToken(expr), new(big.Int), "list literal")
}

// checkKnownCollectionLengthContracts validates one proven length in declared
// contract order, preserving the defining contract's related source location.
// Rules: rules/types/contracts.md — "Composition", "String and collection contracts",
// "Diagnostics"; rules/types/default_values.md — "Defaults and contracts".
func (a *Analyzer) checkKnownCollectionLengthContracts(typ Type, token lexer.Token, length *big.Int, description string) bool {
	typeName := typeDisplayName(typ)
	if typ.Named && typ.Name != "" {
		typeName = typ.Name
	}
	for _, contract := range typ.Contracts {
		switch contract := contract.(type) {
		case LengthContract:
			if !knownLengthSatisfiesContract(length, contract) {
				a.addContractError(token, contract, diagnostics.ValueViolatesContract, "use a value satisfying every contract of the named type", "%s length %s violates %s contract %s %s", description, length.String(), contract.Name, typeName, contract.Value.String())
				return true
			}
		case MarkerContract:
			if contract.Name == "notEmpty" && length.Sign() == 0 {
				a.addContractError(token, contract, diagnostics.ValueViolatesContract, "use a value satisfying every contract of the named type", "%s length 0 violates notEmpty contract %s", description, typeName)
				return true
			}
		}
	}
	return false
}
