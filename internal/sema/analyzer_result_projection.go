package sema

import (
	"sec/internal/ast"
	"sec/internal/diagnostics"
)

// inferConsumingResultProjection implements the owned Result.Ok()/Err()
// projection boundary. The retained payload is represented by the returned
// Option while the complete source Result becomes unavailable.
//
// Rules:
//   - rules/errors/errorhandling.md — §6.1 "Consuming owned projections" and §29 "Ownership and affine value semantics"
//   - rules/memory/ownership.md — ownership availability and path merge
//   - rules/corrections/applied/ownership-errorhandling-correction-20260824.md — "Consuming Result projections"
func (a *Analyzer) inferConsumingResultProjection(
	expr *ast.CallExpression,
	memberExpr *ast.MemberExpression,
	receiverType Type,
	resultType Type,
	member CompilerKnownMember,
) (Type, expressionValue, bool) {
	value := expressionValue{Display: expr.String()}
	if resultType.Kind != ResultType || len(resultType.TypeArgs) != 2 {
		return Type{}, expressionValue{}, false
	}
	if !a.checkCompilerKnownCallArity(expr, typeDisplayName(resultType)+"."+member.Name, 0, 0) {
		return Type{Kind: InvalidType}, value, true
	}
	if receiverType.Kind == ReferenceType {
		a.addErrorAtToken(memberExpr.Property.Token, "%s() consumes an owned Result; use %sRef to inspect a borrowed Result", member.Name, member.Name)
		return Type{Kind: InvalidType}, value, true
	}

	discardedPayload := resultType.TypeArgs[1]
	if member.Name == "Err" {
		discardedPayload = resultType.TypeArgs[0]
	}
	if !isDiscardableType(discardedPayload) {
		a.addErrorAtTokenWithMetadata(
			memberExpr.Property.Token,
			diagnostics.NonDiscardableValue,
			"handle the Result with match so both payloads are handled explicitly",
			"%s() cannot discard the alternate %s payload because it carries a non-discardable obligation",
			member.Name,
			typeDisplayName(discardedPayload),
		)
		return Type{Kind: InvalidType}, value, true
	}

	place, reusable := a.resolvePlace(memberExpr.Object)
	if reusable {
		if a.rejectOrdinaryMethodWholeSelfConsumption(place, memberExpr.Property.Token) {
			return Type{Kind: InvalidType}, value, true
		}
		if !place.Addressable {
			a.addErrorAtToken(memberExpr.Property.Token, "%s() requires an addressable owned Result receiver", member.Name)
			return Type{Kind: InvalidType}, value, true
		}
		if len(place.Projections) > 0 && !place.PartialMoveSafe {
			a.addErrorAtToken(memberExpr.Property.Token, "%s() cannot consume this Result projection because its storage is not independently tracked", member.Name)
			return Type{Kind: InvalidType}, value, true
		}
		if a.checkBorrowedMovePlaceForAction(place, memberExpr.Property.Token, "consume") {
			return Type{Kind: InvalidType}, value, true
		}
		a.markPlaceUnavailable(place, memberExpr.Property.Token, "consumed by Result."+member.Name+"()")
		if len(place.Projections) == 0 {
			a.endBorrowsHeldBy(place.Root)
		}
	}

	return member.Result, value, true
}
