package sema

import (
	"strings"

	"sec/internal/ast"
	"sec/internal/lexer"
)

// analyzeTryHandlerGuard checks a handler's `where` condition in the arm scope
// where the pattern binding is already visible. The guard runs before the
// handler is selected, so it may inspect but must not consume the binding.
//
// Rules:
//   - rules/errors/errorhandling.md — §19 "Guards", §20 "Handler ownership and guards"
//   - rules/errors/errorhandling.md — §22 "Handler failures are outside the protected set"
func (a *Analyzer) analyzeTryHandlerGuard(handler *ast.TryHandler, bindingName string) {
	guardType, _ := a.inferExpression(handler.Guard)
	if guardType.Kind != InvalidType && guardType.Kind != BoolType {
		a.addErrorAtToken(expressionToken(handler.Guard), "try handler guard must be bool, got %s", typeDisplayName(guardType))
	}
	if bindingName == "" {
		return
	}
	if token, moved := a.moved[bindingName]; moved {
		a.addErrorAtToken(token, "try handler guard must not consume %s before the handler is selected; inspect or borrow it in the guard and move it in the handler body", bindingName)
	}
}

// tryHandlerBlockValue returns the final expression statement of a handler
// block, which provides the recovery value in result position.
func tryHandlerBlockValue(block *ast.BlockStatement) (ast.Expression, bool) {
	if block == nil || len(block.Statements) == 0 {
		return nil, false
	}
	last := block.Statements[len(block.Statements)-1]
	statement, ok := last.(*ast.ExpressionStatement)
	if !ok || statement.Expression == nil {
		return nil, false
	}
	return statement.Expression, true
}

// analyzeTryHandlerBlockValue analyzes a handler block whose final expression
// is its recovery value: the preceding statements run in the handler scope and
// the final expression must be assignable to the try success type.
//
// Rules:
//   - rules/errors/errorhandling.md — §21 "Recovery values", §21.1 "Block handler result position"
func (a *Analyzer) analyzeTryHandlerBlockValue(handler *ast.TryHandler, value ast.Expression, successType Type) ResolvedTryHandlerFlow {
	block := handler.BlockBody
	prefix := &ast.BlockStatement{Token: block.Token, Tokens: block.Tokens, Statements: block.Statements[:len(block.Statements)-1]}
	a.analyzeBlockStatements(prefix)
	if len(prefix.Statements) > 0 && !blockCanFallThrough(prefix) {
		a.addErrorAtToken(expressionToken(value), "unreachable recovery value; the handler block already leaves control flow")
		if blockDefinitelyReturns(prefix) {
			return TryHandlerReturns
		}
		return TryHandlerTerminates
	}
	valueType, _ := a.inferExpressionWithExpected(value, successType)
	if valueType.Kind == InvalidType {
		return TryHandlerInvalidFlow
	}
	if !a.canInitialize(successType, valueType, value) {
		a.addErrorAtToken(expressionToken(value), "try handler must produce %s, got %s", typeDisplayName(successType), typeDisplayName(valueType))
		return TryHandlerInvalidFlow
	}
	return TryHandlerProducesValue
}

// rejectCarrierFamilyMismatch explains a match pattern from the wrong carrier
// family: Ok/Err against an Option subject or Some/None against a Result
// subject. Without it, None would bind as an ordinary catch-all name.
//
// Rules:
//   - rules/errors/errorhandling.md — §27.1 "Patterns are checked against the resolved subject type"
//   - rules/errors/errorhandling.md — §30 "Diagnostics must act as a mentor"
func (a *Analyzer) rejectCarrierFamilyMismatch(pattern ast.Expression, subjectType Type) bool {
	family, token := carrierPatternFamily(pattern)
	switch {
	case family == "Result" && subjectType.Kind == UnionType && subjectType.Name == "Option":
		a.addErrorAtToken(token, "Ok and Err patterns match Result values; %s uses Some(value) and None", typeDisplayName(subjectType))
		return true
	case family == "Option" && subjectType.Kind == ResultType:
		if _, shadowed := a.symbols["None"]; shadowed {
			return false
		}
		a.addErrorAtToken(token, "Some and None patterns match Option values; %s uses Ok(value) and Err(error)", typeDisplayName(subjectType))
		return true
	}
	return false
}

// carrierPatternFamily names the carrier whose variant a pattern spells, if any.
func carrierPatternFamily(pattern ast.Expression) (string, lexer.Token) {
	switch pattern := pattern.(type) {
	case *ast.OkExpression:
		return "Result", pattern.Token
	case *ast.ErrExpression:
		return "Result", pattern.Token
	case *ast.Identifier:
		if pattern.Value == "None" {
			return "Option", pattern.Token
		}
	case *ast.CallExpression:
		if callee, ok := pattern.Callee.(*ast.Identifier); ok && callee.Value == "Some" {
			return "Option", callee.Token
		}
		if pattern.Function != nil && pattern.Function.Value == "Some" {
			return "Option", pattern.Function.Token
		}
	}
	return "", lexer.Token{}
}

// openErrorNarrowingPrefix marks a match-pattern variant key that names a
// concrete error variant narrowed from an open error channel.
const openErrorNarrowingPrefix = "error:"

// concreteErrorVariantPrefix marks a match-pattern variant key that selects
// one variant of a closed concrete error channel Result[T, ConcreteError].
const concreteErrorVariantPrefix = "error-variant:"

// analyzeMatchErrorNarrowing validates `Err(ErrorType.Variant)` in a match.
// On an open Result[T, error] channel the concrete-variant form narrows and
// never covers the open error domain, so an Err fallback is still required.
// On a concrete channel it selects one variant of the closed error type and
// participates in closed exhaustiveness (MD-008).
//
// Rules:
//   - rules/control-flow/flowcontrol_match.md — "Open error narrowing", "Concrete error variants"
//   - rules/errors/errorhandling.md — §27.2 "Matching Result[T, error]"
//   - rules/corrections/applied/match-errorhandling-correction-20260824.md — "Open-domain exhaustiveness"
//   - rules/corrections/applied/missing-decisions-md001-md009-correction-20261003.md — § 9
func (a *Analyzer) analyzeMatchErrorNarrowing(member *ast.MemberExpression, errorType Type) (matchPatternInfo, bool) {
	if errorType.Kind != ErrorRootType {
		return a.analyzeMatchConcreteErrorVariant(member, errorType)
	}
	variantType, ok := a.inferMemberExpression(member)
	if !ok || variantType.Kind == InvalidType {
		return matchPatternInfo{}, false
	}
	if !variantType.ErrorAssignable {
		a.addErrorAtToken(member.Token, "Err(%s) must name a concrete error variant, got %s", member.String(), typeDisplayName(variantType))
		return matchPatternInfo{}, false
	}
	return matchPatternInfo{Variant: openErrorNarrowingPrefix + typeDisplayName(variantType) + "." + member.Property.Value}, true
}

// analyzeMatchConcreteErrorVariant resolves `Err(ConcreteError.Variant)` on a
// Result[T, ConcreteError] subject. The variant must belong to the subject's
// own closed error type. Enum error variants are keyed by their underlying
// value class, as ordinary enum match arms are; payload-less union error
// variants by name. A payload-carrying union variant would need a nested
// payload pattern, which Sec 0.1 does not define (MD-029).
//
// Rules:
//   - rules/control-flow/flowcontrol_match.md — "Concrete error variants"
//   - rules/corrections/applied/missing-decisions-md001-md009-correction-20261003.md — §§ 9.2–9.6, 9.11–9.13
func (a *Analyzer) analyzeMatchConcreteErrorVariant(member *ast.MemberExpression, errorType Type) (matchPatternInfo, bool) {
	if errorType.Kind != EnumType && errorType.Kind != UnionType {
		a.addErrorAtToken(member.Token, "Err(%s) requires a closed concrete error type; %s has no variants to select, so bind the payload with Err(name)", member.String(), typeDisplayName(errorType))
		return matchPatternInfo{}, false
	}
	if errorType.Kind == UnionType {
		owner, ownerOK := typePathFromExpression(member.Object)
		variant, variantOK := lookupUnionVariant(errorType, member.Property.Value)
		if !ownerOK || a.resolveTypeName(owner) != errorType.Name || !variantOK {
			a.addErrorAtToken(expressionToken(member), "Err(%s) must name a variant of %s", member.String(), typeDisplayName(errorType))
			return matchPatternInfo{}, false
		}
		if variant.Payload != nil || len(variant.PayloadFields) > 0 {
			a.addErrorAtToken(member.Property.Token,
				"Err(%s.%s) cannot select a payload-carrying error variant because Sec 0.1 has no nested payload pattern; bind the error with Err(name) and match it against %s.%s(...) in the arm body",
				typeDisplayName(errorType), variant.Name, typeDisplayName(errorType), variant.Name)
			return matchPatternInfo{}, false
		}
		a.bindDefinition(member.Property.Token, variant.Token)
		return matchPatternInfo{Variant: concreteErrorVariantPrefix + variant.Name, EnumCaseName: typeDisplayName(errorType) + "." + variant.Name}, true
	}
	patternType, ok := a.inferMemberExpression(member)
	if !ok || patternType.Kind == InvalidType {
		return matchPatternInfo{}, false
	}
	if !sameConcreteType(patternType, errorType) {
		a.addErrorAtToken(expressionToken(member), "Err(%s) must name a variant of %s, got %s", member.String(), typeDisplayName(errorType), typeDisplayName(patternType))
		return matchPatternInfo{}, false
	}
	enumCase, exists := errorType.EnumConsts[member.Property.Value]
	key, keyed := enumValueClassKey(enumCase)
	if !exists || !keyed {
		return matchPatternInfo{}, false
	}
	return matchPatternInfo{Variant: concreteErrorVariantPrefix + key, EnumCaseName: typeDisplayName(errorType) + "." + enumCase.Name}, true
}

// isErrorVariantMatchKey reports whether a match-pattern variant key selects
// one concrete error variant, from an open or a concrete error channel.
func isErrorVariantMatchKey(variant string) bool {
	return strings.HasPrefix(variant, openErrorNarrowingPrefix) || strings.HasPrefix(variant, concreteErrorVariantPrefix)
}

// concreteErrorDomainCovered reports whether unguarded Err(ConcreteError.Variant)
// arms cover every value of a closed concrete error type.
//
// Rules:
//   - rules/corrections/applied/missing-decisions-md001-md009-correction-20261003.md — §§ 9.4–9.6
func concreteErrorDomainCovered(errorType Type, seenVariants map[string]bool) bool {
	switch errorType.Kind {
	case EnumType:
		seen := map[string]string{}
		for variant := range seenVariants {
			if key, ok := strings.CutPrefix(variant, concreteErrorVariantPrefix); ok {
				seen[key] = key
			}
		}
		return len(seen) > 0 && enumDomainCovered(errorType, seen)
	case UnionType:
		if len(errorType.UnionVariants) == 0 {
			return false
		}
		for _, variant := range errorType.UnionVariants {
			if !seenVariants[concreteErrorVariantPrefix+variant.Name] {
				return false
			}
		}
		return true
	default:
		return false
	}
}

// missingConcreteErrorVariants lists the declared variants of a closed
// concrete error type that no unguarded arm covers, in declaration order.
func missingConcreteErrorVariants(errorType Type, seenVariants map[string]bool) []string {
	missing := []string{}
	switch errorType.Kind {
	case EnumType:
		for _, name := range errorType.EnumValues {
			enumCase, ok := errorType.EnumConsts[name]
			key, keyed := enumValueClassKey(enumCase)
			if ok && keyed && !seenVariants[concreteErrorVariantPrefix+key] {
				missing = append(missing, "Err("+typeDisplayName(errorType)+"."+name+")")
			}
		}
	case UnionType:
		for _, variant := range errorType.UnionVariants {
			if !seenVariants[concreteErrorVariantPrefix+variant.Name] {
				missing = append(missing, "Err("+typeDisplayName(errorType)+"."+variant.Name+")")
			}
		}
	}
	return missing
}

// concreteErrorVariantCount returns the number of selectable variants of a
// closed concrete error type.
func concreteErrorVariantCount(errorType Type) int {
	switch errorType.Kind {
	case EnumType:
		return len(errorType.EnumValues)
	case UnionType:
		return len(errorType.UnionVariants)
	default:
		return 0
	}
}
