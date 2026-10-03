package sema

import (
	"fmt"
	"math/big"

	"sec/internal/ast"
	"sec/internal/diagnostics"
	"sec/internal/lexer"
)

func (a *Analyzer) typeFromDeclaration(stmt *ast.TypeDeclStatement, baseType Type) Type {
	return a.typeFromDeclarationWithName(stmt.Name.Value, stmt, baseType)
}

func (a *Analyzer) typeFromDeclarationWithName(name string, stmt *ast.TypeDeclStatement, baseType Type) Type {
	typ := baseType
	typ.Name = name
	typ.Module = a.currentModule
	typ.Named = true
	typ.Declared = true
	if hasAttribute(stmt.Attributes, "noCopy") {
		typ.ExplicitlyNonCopyable = true
		typ.NoCopyPolicyOrigin = name
	} else if typ.ExplicitlyNonCopyable && typ.NoCopyPolicyOrigin == "" {
		typ.NoCopyPolicyOrigin = baseType.Name
	}
	typ.Underlying = baseType.Name
	typ.Contracts = append([]Contract(nil), baseType.Contracts...)
	typ.GenericParameters = genericParameterNameValues(stmt.GenericParameters)
	typ.GenericConstraints = a.resolvedGenericParameterConstraints(stmt.GenericParameters)

	if stmt.BaseType != nil && stmt.BaseType.Unit != "" {
		typ.Unit = stmt.BaseType.Unit
		typ.Dimension = a.parseDimension(stmt.BaseType.Unit)
	}
	if stmt.AssignedType != nil && stmt.AssignedType.Unit != "" {
		typ.Unit = stmt.AssignedType.Unit
		typ.Dimension = a.parseDimension(stmt.AssignedType.Unit)
	}

	typ = a.applyContracts(typ, stmt.Contract)
	if stmt.Default != nil {
		// MD-011: the default is an ordinary expression in a
		// SemanticCompileTimeRequiredContext.
		constant, outcome := a.semanticCompileTimeConstant(stmt.Default)
		ok := outcome == compileTimeEvaluated
		if outcome == compileTimeRequiresExecution {
			typ.InvalidExplicitDefault = true
			a.reportCompileTimeRequirement(stmt.Default, outcome, "default", diagnostics.InvalidExplicitDefault, "")
		} else if !ok {
			typ.InvalidExplicitDefault = true
			a.addErrorAtTokenWithMetadata(stmt.DefaultToken, diagnostics.InvalidExplicitDefault, "use an allocation-free compile-time constant", "default for %s must be a compile-time primitive constant", name)
		} else if !defaultRepresentable(typ, constant) {
			// rules/types/default_values.md, "Explicit type defaults" and
			// "Diagnostics": representability is checked before contracts.
			typ.InvalidExplicitDefault = true
			a.addErrorAtTokenWithMetadata(expressionToken(stmt.Default), diagnostics.DefaultNotRepresentable, "choose a value representable by "+contractApplicabilityTypeName(typ), "default value %s is not representable by %s", stmt.Default.String(), name)
		} else if violated, ok := firstViolatedContract(typ, constant); ok {
			// rules/types/contracts.md, "Explicit defaults" and "Diagnostics":
			// name the first violated contract in source order.
			typ.InvalidExplicitDefault = true
			a.addErrorAtTokenWithMetadata(expressionToken(stmt.Default), diagnostics.DefaultViolatesContract, fmt.Sprintf("%s requires %s; choose a value satisfying every type contract", name, describeContract(violated)), "default value %s is invalid for %s", stmt.Default.String(), name)
		} else {
			typ.ExplicitDefault = &constant
		}
	}
	return typ
}

func flattenASTContracts(contract ast.Contract) []ast.Contract {
	if contract == nil {
		return nil
	}
	if list, ok := contract.(*ast.ContractList); ok {
		return list.Contracts
	}
	return []ast.Contract{contract}
}

// rejectStorageSiteContract diagnoses the obsolete variable/field contract
// forms while leaving the parsed node intact for recovery and tooling. Type
// contracts belong exclusively to named type declarations in Sec 0.1.
//
// Rules:
//   - rules/types/contracts.md — "Status"
//   - rules/types/contracts.md — "Core rule"
func (a *Analyzer) rejectStorageSiteContract(contract ast.Contract, siteKind string, siteName string) bool {
	if contract == nil {
		return false
	}
	token := astContractToken(contract)
	a.addErrorAtTokenWithMetadata(
		token,
		diagnostics.StorageSiteContract,
		"declare a named constrained type and use that type at this storage site",
		"contracts belong to named types; %s %s cannot declare an inline contract",
		siteKind,
		siteName,
	)
	return true
}

// astContractToken returns the first source token of a parsed contract
// conjunction for focused contract diagnostics.
//
// Rule: rules/types/contracts.md — "Composition".
func astContractToken(contract ast.Contract) lexer.Token {
	switch contract := contract.(type) {
	case *ast.ContractList:
		return contract.Token
	case *ast.RangeContract:
		return contract.Token
	case *ast.MembershipContract:
		return contract.Token
	case *ast.MarkerContract:
		return contract.Token
	case *ast.RegexContract:
		return contract.Token
	default:
		return lexer.Token{}
	}
}

func (a *Analyzer) applyContracts(typ Type, contractNode ast.Contract) Type {
	for _, contract := range flattenASTContracts(contractNode) {
		typ = a.applyContract(typ, contract)
	}
	a.checkContractSetConsistency(typ, contractNode)
	a.checkLengthContractSetConsistency(typ, contractNode)
	a.checkMembershipContractValues(typ, contractNode)
	return typ
}

func (a *Analyzer) checkMembershipContractValues(typ Type, contractNode ast.Contract) {
	var token lexer.Token
	for _, contract := range flattenASTContracts(contractNode) {
		switch contract := contract.(type) {
		case *ast.RangeContract:
			token = contract.Token
		case *ast.MarkerContract:
			token = contract.Token
		case *ast.MembershipContract:
			token = contract.Token
		}
	}
	// rules/types/contracts.md, inherited conjunction; correction4.md requires
	// every semantic membership value, not only values in the current AST node,
	// to be checked against the complete derived contract set.
	for _, contract := range typ.Contracts {
		membership, ok := contract.(MembershipContract)
		if !ok {
			continue
		}
		for _, constant := range membership.Values {
			if !defaultConstantCompatible(typ, constant) {
				continue
			}
			if !defaultConstantSatisfies(typ, constant) {
				a.addErrorAtTokenWithMetadata(
					token,
					diagnostics.InvalidMembershipValue,
					"remove the value or change the other type contracts",
					"membership value %s violates another contract on %s",
					constant.Lexeme,
					typeDisplayName(typ),
				)
			}
		}
	}
}

func (a *Analyzer) applyContract(typ Type, contractNode ast.Contract) Type {
	switch contract := contractNode.(type) {
	case *ast.RangeContract:
		if !a.contractAppliesToType("range", typ) {
			a.addErrorAtTokenWithMetadata(contract.Token, diagnostics.InapplicableContract, "remove the contract or use a base type the contract applies to", "range contract does not apply to %s", contractApplicabilityTypeName(typ))
			return typ
		}
		return a.applyRangeContract(typ, contract)
	case *ast.MembershipContract:
		if !a.contractAppliesToType("in", typ) {
			a.addErrorAtTokenWithMetadata(contract.Token, diagnostics.InapplicableContract, "remove the contract or use a base type the contract applies to", "in contract does not apply to %s", contractApplicabilityTypeName(typ))
			return typ
		}
		membership := MembershipContract{}
		if len(contract.Values) == 0 {
			a.addErrorAtTokenWithMetadata(
				contract.Token,
				diagnostics.EmptyContractMembership,
				"add at least one permitted compile-time value to the in-contract",
				"in-contract for %s must contain at least one value",
				typeDisplayName(typ),
			)
			typ.Contracts = append(typ.Contracts, membership)
			return typ
		}
		membershipTokens := []lexer.Token{}
		for _, value := range contract.Values {
			constant, ok := enumMemberConstant(typ, value)
			if !ok {
				var outcome compileTimeOutcome
				constant, outcome = a.semanticCompileTimeConstant(value)
				ok = outcome == compileTimeEvaluated
				if outcome == compileTimeRequiresExecution {
					a.reportCompileTimeRequirement(value, outcome, "membership value", diagnostics.InvalidContractArgument, "")
					continue
				}
			}
			if !ok {
				a.addErrorAtTokenWithMetadata(expressionToken(value), diagnostics.InvalidContractArgument, "use a compile-time value or, for an enum-based type, one of its declared members", "membership value %s is not a compile-time constant of %s", value.String(), typeDisplayName(typ))
				continue
			}
			if !defaultConstantCompatible(typ, constant) {
				a.addErrorAtTokenWithMetadata(expressionToken(value), diagnostics.IncompatibleMembershipValue, "use a value of the named type's base type", "membership value %s is incompatible with %s", value.String(), typeDisplayName(typ))
				continue
			}
			duplicateIndex := -1
			for index, previous := range membership.Values {
				if defaultConstantsEqual(constant, previous) {
					duplicateIndex = index
					break
				}
			}
			if duplicateIndex >= 0 {
				a.addErrorAtTokenWithPreviousMetadata(
					expressionToken(value),
					membershipTokens[duplicateIndex],
					diagnostics.DuplicateContractMembershipValue,
					"remove the duplicate or replace it with a distinct permitted value",
					"duplicate membership value %s",
					value.String(),
				)
				continue
			}
			membership.Values = append(membership.Values, constant)
			membershipTokens = append(membershipTokens, expressionToken(value))
		}
		typ.Contracts = append(typ.Contracts, membership)
		return typ
	case *ast.MarkerContract:
		if !a.contractAppliesToType(contract.Name, typ) {
			a.addErrorAtTokenWithMetadata(contract.Token, diagnostics.InapplicableContract, "remove the contract or use a base type the contract applies to", "%s contract does not apply to %s", contract.Name, contractApplicabilityTypeName(typ))
			return typ
		}
		if contract.Name == "multipleOf" {
			// rules/types/contracts.md, "Integer contracts": multipleOf requires
			// a nonzero compile-time integer divisor; a divisor that cannot be
			// established is rejected instead of silently dropping the contract.
			value, outcome := a.semanticCompileTimeInteger(contract.Value)
			if outcome == compileTimeRequiresExecution {
				a.reportCompileTimeRequirement(contract.Value, outcome, "multipleOf divisor", diagnostics.InvalidContractArgument, "")
				return typ
			}
			if outcome != compileTimeEvaluated {
				if outcome != compileTimeAlreadyInvalid {
					a.addErrorAtTokenWithMetadata(expressionToken(contract.Value), diagnostics.InvalidContractArgument, "use a valid compile-time contract argument", "multipleOf contract divisor must be a compile-time integer")
				}
				return typ
			}
			if value.Sign() == 0 {
				a.addErrorAtTokenWithMetadata(expressionToken(contract.Value), diagnostics.InvalidContractArgument, "use a valid compile-time contract argument", "multipleOf contract divisor must not be zero")
			}
			typ.Contracts = append(typ.Contracts, MultipleOfContract{Value: new(big.Int).Set(value)})
			return typ
		}
		if isLengthContractName(contract.Name) {
			value, outcome := a.semanticCompileTimeInteger(contract.Value)
			if outcome == compileTimeRequiresExecution {
				a.reportCompileTimeRequirement(contract.Value, outcome, contract.Name+" value", diagnostics.InvalidContractArgument, "")
				return typ
			}
			if outcome != compileTimeEvaluated {
				a.addErrorAtTokenWithMetadata(expressionToken(contract.Value), diagnostics.InvalidContractArgument, "use a valid compile-time contract argument", "%s contract value must be a compile-time integer", contract.Name)
				return typ
			}
			if value.Sign() < 0 {
				a.addErrorAtTokenWithMetadata(expressionToken(contract.Value), diagnostics.InvalidContractArgument, "use a valid compile-time contract argument", "%s contract value must not be negative", contract.Name)
				return typ
			}
			typ.Contracts = append(typ.Contracts, LengthContract{
				Name:  contract.Name,
				Value: new(big.Int).Set(value),
			})
			return typ
		}
		typ.Contracts = append(typ.Contracts, MarkerContract{Name: contract.Name})
		return typ
	case *ast.RegexContract:
		return a.applyRegexContract(typ, contract)
	default:
		return typ
	}
}

// applyRegexContract validates the source-checkable parts of a `regex`
// contract: it applies only to string-like named types and requires a
// compile-time string pattern. Because the concrete regular-expression syntax
// and engine are not yet fixed, no value can be proven to satisfy the
// contract; the declaration is rejected with a focused diagnostic instead of
// silently dropping the contract.
//
// Rules:
//   - rules/types/contracts.md — "Applicability" (`regex` on string-like named types)
//   - rules/types/contracts.md — "String and collection contracts" (compile-time pattern; engine must be fixed before validation)
//   - missing-decisions.yaml — MD-010
func (a *Analyzer) applyRegexContract(typ Type, contract *ast.RegexContract) Type {
	if !a.contractAppliesToType("regex", typ) {
		a.addErrorAtTokenWithMetadata(contract.Token, diagnostics.InapplicableContract, "remove the contract or use a base type the contract applies to", "regex contract does not apply to %s", contractApplicabilityTypeName(typ))
		return typ
	}
	if _, invalid := contract.Pattern.(*ast.InvalidExpression); invalid || contract.Pattern == nil {
		// The parser already reported the missing or malformed pattern.
		return typ
	}
	pattern, outcome := a.semanticCompileTimeConstant(contract.Pattern)
	if outcome == compileTimeRequiresExecution {
		a.reportCompileTimeRequirement(contract.Pattern, outcome, "regex pattern", diagnostics.InvalidContractArgument, "")
		return typ
	}
	if outcome != compileTimeEvaluated || pattern.Kind != StringType {
		a.addErrorAtTokenWithMetadata(expressionToken(contract.Pattern), diagnostics.InvalidContractArgument, "use a valid compile-time contract argument", "regex contract pattern must be a compile-time string")
		return typ
	}
	typ.Contracts = append(typ.Contracts, RegexContract{Pattern: pattern.String})
	a.addErrorAtTokenWithMetadata(
		contract.Token,
		diagnostics.RegexContractUnavailable,
		"remove the regex contract until the Sec regular-expression syntax is defined",
		"regex contract on %s cannot be validated: the Sec regular-expression syntax and engine are not yet defined",
		typeDisplayName(typ),
	)
	return typ
}

func contractApplicabilityTypeName(typ Type) string {
	if typ.Named && typ.Underlying != "" {
		return typ.Underlying
	}
	return typeDisplayName(typ)
}

// applyRangeContract records the semantic range. Each bound is an ordinary
// expression evaluated in a SemanticCompileTimeRequiredContext (MD-011); a
// bound that cannot be established is diagnosed instead of being dropped.
//
// Rules:
//   - rules/corrections/applied/missing-decisions-md010-md014-correction-20261003.md — §§ 3.11–3.17
//   - rules/types/contracts.md — "Range contracts"
func (a *Analyzer) applyRangeContract(typ Type, contract *ast.RangeContract) Type {
	rangeContract := RangeContract{Exclusive: contract.Exclusive}
	exactBounds := typ.Kind == DecimalType || typ.Kind == FloatType
	bound := func(expr ast.Expression, position string) (*big.Int, *big.Rat, string, bool) {
		if expr == nil {
			return nil, nil, "", false
		}
		if exactBounds {
			exact, lexeme, outcome := a.semanticCompileTimeExact(expr)
			if outcome != compileTimeEvaluated {
				a.reportCompileTimeRequirement(expr, outcome, "range "+position+" bound", diagnostics.InvalidContractArgument, "use a value established at compile time")
				return nil, nil, "", false
			}
			var integer *big.Int
			if value, integerOutcome := a.semanticCompileTimeInteger(expr); integerOutcome == compileTimeEvaluated {
				integer = value
			}
			return integer, exact, lexeme, true
		}
		value, outcome := a.semanticCompileTimeInteger(expr)
		if outcome != compileTimeEvaluated {
			if constant, constantOutcome := a.semanticCompileTimeConstant(expr); constantOutcome == compileTimeEvaluated && constant.Integer == nil {
				// A non-integer constant bound on an integer type keeps the
				// existing range-domain diagnostics; nothing to record.
				return nil, nil, "", false
			}
			a.reportCompileTimeRequirement(expr, outcome, "range "+position+" bound", diagnostics.InvalidContractArgument, "use a value established at compile time")
			return nil, nil, "", false
		}
		return value, nil, "", true
	}
	if min, exact, lexeme, ok := bound(contract.Min, "lower"); ok {
		rangeContract.Min = min
		rangeContract.ExactMin = exact
		rangeContract.MinLexeme = lexeme
	}
	if max, exact, lexeme, ok := bound(contract.Max, "upper"); ok {
		rangeContract.Max = max
		rangeContract.ExactMax = exact
		rangeContract.MaxLexeme = lexeme
	}

	typ.Contracts = append(typ.Contracts, rangeContract)

	return typ
}

// checkContractSetConsistency rejects integer contract conjunctions with no
// representable value, including inherited contracts and the resolved target's
// bounds for int and uint.
// Rules: rules/types/contracts.md — Composition; Integer contracts;
// rules/types/types.md — `int` and `uint`;
// rules/corrections/applied/correction3-20260823.md — Required correction, 1–5.
func (a *Analyzer) checkContractSetConsistency(typ Type, contractNode ast.Contract) {
	if contractNode == nil || !isIntegerType(typ) {
		return
	}

	var token lexer.Token
	effectiveRange := intersectIntegerRangeContracts(nil, RangeContract{
		Min: typ.MinInteger,
		Max: typ.MaxInteger,
	})
	var multiple *big.Int
	hasOdd := false
	hasEven := false

	for _, astContract := range flattenASTContracts(contractNode) {
		switch contract := astContract.(type) {
		case *ast.RangeContract:
			token = contract.Token
		case *ast.MarkerContract:
			token = contract.Token
		}
	}

	for _, contract := range typ.Contracts {
		switch contract := contract.(type) {
		case RangeContract:
			effectiveRange = intersectIntegerRangeContracts(effectiveRange, contract)
		case MultipleOfContract:
			if contract.Value != nil && contract.Value.Sign() != 0 {
				divisor := new(big.Int).Abs(contract.Value)
				multiple = leastCommonMultiple(multiple, divisor)
			}
		case MarkerContract:
			switch contract.Name {
			case "odd":
				hasOdd = true
			case "even":
				hasEven = true
			}
		}
	}

	if hasOdd && hasEven {
		a.addErrorAtTokenWithMetadata(token, diagnostics.UnsatisfiableContractSet, "remove or relax one of the conflicting contracts", "contracts odd and even cannot be combined")
		return
	}
	if hasOdd && multiple != nil && new(big.Int).Mod(multiple, big.NewInt(2)).Sign() == 0 {
		a.addErrorAtTokenWithMetadata(token, diagnostics.UnsatisfiableContractSet, "remove or relax one of the conflicting contracts", "contracts multipleOf %s and odd cannot be combined because every multiple is even", multiple.String())
		return
	}
	if effectiveRange == nil {
		return
	}
	if !integerRangeHasSatisfyingValue(effectiveRange, multiple, hasOdd, hasEven) {
		a.addErrorAtTokenWithMetadata(token, diagnostics.UnsatisfiableContractSet, "remove or relax one of the conflicting contracts", "contracts cannot be satisfied together for %s", typeDisplayName(typ))
	}
}

// intersectIntegerRangeContracts implements the exact integer conjunction from
// rules/types/contracts.md. correction3.md requires every inherited and local
// range to contribute instead of retaining only the last declaration.
func intersectIntegerRangeContracts(current *RangeContract, next RangeContract) *RangeContract {
	normalized := RangeContract{}
	if next.Min != nil {
		normalized.Min = new(big.Int).Set(next.Min)
	}
	if next.Max != nil {
		normalized.Max = new(big.Int).Set(next.Max)
		if next.Exclusive {
			normalized.Max.Sub(normalized.Max, big.NewInt(1))
		}
	}
	if current == nil {
		return &normalized
	}
	result := RangeContract{}
	if current.Min != nil {
		result.Min = new(big.Int).Set(current.Min)
	}
	if normalized.Min != nil && (result.Min == nil || normalized.Min.Cmp(result.Min) > 0) {
		result.Min = new(big.Int).Set(normalized.Min)
	}
	if current.Max != nil {
		result.Max = new(big.Int).Set(current.Max)
	}
	if normalized.Max != nil && (result.Max == nil || normalized.Max.Cmp(result.Max) < 0) {
		result.Max = new(big.Int).Set(normalized.Max)
	}
	return &result
}

func leastCommonMultiple(left, right *big.Int) *big.Int {
	if left == nil {
		return new(big.Int).Set(right)
	}
	gcd := new(big.Int).GCD(nil, nil, left, right)
	return new(big.Int).Mul(new(big.Int).Quo(left, gcd), right)
}

// integerRangeHasSatisfyingValue proves nonempty range/divisibility/parity
// intersection with exact arithmetic, without enumerating the range.
// Rules: rules/types/contracts.md — Composition; Integer contracts;
// rules/corrections/applied/correction3-20260823.md — Algorithmic note.
func integerRangeHasSatisfyingValue(contract *RangeContract, multiple *big.Int, odd bool, even bool) bool {
	if contract == nil || contract.Min == nil || contract.Max == nil {
		return true
	}
	min := new(big.Int).Set(contract.Min)
	max := new(big.Int).Set(contract.Max)
	if contract.Exclusive {
		max.Sub(max, big.NewInt(1))
	}
	if min.Cmp(max) > 0 {
		return false
	}

	step := big.NewInt(1)
	if multiple != nil && multiple.Sign() > 0 {
		step = new(big.Int).Set(multiple)
	}
	first := firstMultipleAtOrAbove(min, step)
	if odd && even {
		return false
	}
	if odd && first.Bit(0) == 0 || even && first.Bit(0) != 0 {
		if step.Bit(0) == 0 {
			return false
		}
		first.Add(first, step)
	}
	return first.Cmp(max) <= 0
}

func firstMultipleAtOrAbove(min *big.Int, step *big.Int) *big.Int {
	if step.Sign() <= 0 {
		return new(big.Int).Set(min)
	}
	remainder := new(big.Int).Mod(min, step)
	if remainder.Sign() == 0 {
		return new(big.Int).Set(min)
	}
	return new(big.Int).Add(min, new(big.Int).Sub(step, remainder))
}

func (a *Analyzer) contractAppliesToType(name string, typ Type) bool {
	switch name {
	case "range":
		return isNumericType(typ)
	case "in":
		return isScalarContractType(typ)
	case "multipleOf":
		return isIntegerType(typ)
	case "odd", "even":
		return isIntegerType(typ)
	case "minLen", "maxLen", "exactLen", "notEmpty":
		return typ.Kind == StringType || a.isCollectionContractType(typ)
	case "unique":
		return a.isCollectionContractType(typ)
	case "finite":
		return typ.Kind == FloatType || typ.Kind == DecimalType
	case "regex":
		return typ.Kind == StringType
	default:
		return false
	}
}

func isScalarContractType(typ Type) bool {
	switch typ.Kind {
	case BoolType, StringType, CharType, RuneType, IntType, UintType, FloatType, DecimalType, EnumType:
		return true
	default:
		return false
	}
}

func (a *Analyzer) isCollectionContractType(typ Type) bool {
	seen := map[string]bool{}
	for {
		if typ.Kind == ArrayType || typ.Kind == SliceType || isCompilerKnownCollectionTypeName(typ.Name) {
			return true
		}
		if typ.Underlying == "" || seen[typ.Underlying] {
			return false
		}
		seen[typ.Underlying] = true
		if isCompilerKnownCollectionTypeName(typ.Underlying) {
			return true
		}
		underlying, ok := a.types[typ.Underlying]
		if !ok {
			return false
		}
		typ = underlying
	}
}

func isCompilerKnownCollectionTypeName(name string) bool {
	switch name {
	case "Vec", "Set", "Map", "list", "set", "map", "vector", "matrix", "tensor", "tensor_view":
		return true
	default:
		return false
	}
}

func (a *Analyzer) checkIntegerExpressionRange(typ Type, expr ast.Expression) bool {
	value, ok := a.integerConstantValue(expr)
	if !ok {
		return false
	}

	return a.checkIntegerValueRange(typ, value, expressionToken(expr))
}

func (a *Analyzer) checkIntegerAssignmentRange(symbol Symbol, stmt *ast.AssignmentStatement) bool {
	if hasContracts(symbol.Type) && !a.isContractCheckableExpression(stmt.Value) {
		return false
	}

	result, ok := a.assignmentIntegerValue(symbol.Name, stmt)
	if !ok {
		return false
	}

	return a.checkIntegerValueRange(symbol.Type, result, expressionToken(stmt.Value))
}

func (a *Analyzer) isContractCheckableExpression(expr ast.Expression) bool {
	if isUntypedNumericExpression(expr) {
		return true
	}

	return a.isExplicitConversionExpression(expr)
}

func (a *Analyzer) checkIntegerLiteralRange(typ Type, expr ast.Expression) bool {
	value, ok := a.integerConstantValue(expr)
	if !ok {
		return false
	}

	return a.checkIntegerValueRange(typ, value, expressionToken(expr))
}

func (a *Analyzer) checkContractLiteralBounds(typ Type, contractNode ast.Contract) {
	for _, contract := range flattenASTContracts(contractNode) {
		switch contract := contract.(type) {
		case *ast.RangeContract:
			if contract.Min != nil {
				a.checkIntegerLiteralRange(typ, contract.Min)
			}
			if contract.Max != nil {
				a.checkIntegerLiteralRange(typ, contract.Max)
			}
		case *ast.MarkerContract:
			if contract.Name == "multipleOf" && contract.Value != nil {
				a.checkIntegerLiteralRange(typ, contract.Value)
			}
		}
	}
}

func (a *Analyzer) checkIntegerValueRange(typ Type, value *big.Int, token lexer.Token) bool {
	if value == nil {
		return false
	}

	overflow := false
	switch typ.Kind {
	case IntType:
		if typ.MinInteger == nil || typ.MaxInteger == nil {
			return false
		}
		overflow = value.Cmp(typ.MinInteger) < 0 || value.Cmp(typ.MaxInteger) > 0
	case UintType:
		if typ.MinInteger == nil || typ.MaxInteger == nil {
			return false
		}
		overflow = value.Sign() < 0 || value.Cmp(typ.MinInteger) < 0 || value.Cmp(typ.MaxInteger) > 0
	case EnumType:
		if typ.BitWidth <= 0 || typ.MinInteger == nil || typ.MaxInteger == nil {
			return false
		}
		overflow = value.Sign() < 0 || value.Cmp(typ.MinInteger) < 0 || value.Cmp(typ.MaxInteger) > 0
	}

	if overflow {
		if typ.Kind == EnumType && typ.BitWidth > 0 {
			a.addErrorAtToken(token, "value %s does not fit in %d-bit enum %s", value.String(), typ.BitWidth, typ.Name)
			return true
		}
		a.addErrorAtToken(token, "value %s overflows %s", value.String(), typ.Name)
		return true
	}

	for _, contract := range typ.Contracts {
		switch contract := contract.(type) {
		case RangeContract:
			violatesMin := contract.Min != nil && value.Cmp(contract.Min) < 0
			violatesMax := false
			if contract.Max != nil {
				if contract.Exclusive {
					violatesMax = value.Cmp(contract.Max) >= 0
				} else {
					violatesMax = value.Cmp(contract.Max) > 0
				}
			}
			if violatesMin || violatesMax {
				a.addErrorAtTokenWithMetadata(
					token,
					diagnostics.ValueViolatesContract,
					"use a value satisfying every contract of the named type",
					"value %s violates range contract %s %s",
					value.String(),
					typ.Name,
					formatRangeContract(typ),
				)
				return true
			}
		case MultipleOfContract:
			if contract.Value == nil || contract.Value.Sign() == 0 {
				continue
			}
			if new(big.Int).Mod(value, contract.Value).Sign() != 0 {
				a.addErrorAtTokenWithMetadata(token, diagnostics.ValueViolatesContract, "use a value satisfying every contract of the named type", "value %s violates multipleOf contract %s %s", value.String(), typ.Name, contract.Value.String())
				return true
			}
		case MarkerContract:
			switch contract.Name {
			case "odd":
				if new(big.Int).Mod(value, big.NewInt(2)).Sign() == 0 {
					a.addErrorAtTokenWithMetadata(token, diagnostics.ValueViolatesContract, "use a value satisfying every contract of the named type", "value %s violates odd contract %s", value.String(), typ.Name)
					return true
				}
			case "even":
				if new(big.Int).Mod(value, big.NewInt(2)).Sign() != 0 {
					a.addErrorAtTokenWithMetadata(token, diagnostics.ValueViolatesContract, "use a value satisfying every contract of the named type", "value %s violates even contract %s", value.String(), typ.Name)
					return true
				}
			}
		default:
			continue
		}
	}

	return false
}

func formatRangeContract(typ Type) string {
	for _, contract := range typ.Contracts {
		rangeContract, ok := contract.(RangeContract)
		if !ok {
			continue
		}

		min := ""
		if rangeContract.Min != nil {
			min = rangeContract.Min.String()
		}

		max := ""
		if rangeContract.Max != nil {
			max = rangeContract.Max.String()
		}

		operator := ".."
		if rangeContract.Exclusive {
			operator = "..<"
		}

		return min + operator + max
	}

	return ""
}
