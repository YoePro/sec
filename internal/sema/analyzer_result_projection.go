package sema

import (
	"strings"

	"sec/internal/ast"
	"sec/internal/diagnostics"
	"sec/internal/lexer"
)

// reportConsumedResultProjectionUse keeps the Result-specific mentor
// diagnostic next to the projection semantics. The generic place tracker owns
// availability, while this layer can name the non-consuming alternative that
// the programmer can use when inspection was intended.
//
// Rules:
//   - rules/errors/errorhandling.md — §29 "Ownership and affine value semantics"
//   - rules/errors/errorhandling.md — §33 "Diagnostics"
func (a *Analyzer) reportConsumedResultProjectionUse(place Place, token, consumedAt lexer.Token, reason string) bool {
	const prefix = "consumed by Result."
	if !strings.HasPrefix(reason, prefix) || !strings.HasSuffix(reason, "()") {
		return false
	}
	projection := strings.TrimSuffix(strings.TrimPrefix(reason, prefix), "()")
	if projection != "Ok" && projection != "Err" {
		return false
	}
	a.addErrorAtTokenWithPrevious(
		token,
		consumedAt,
		"value %s was consumed by Result.%s() here and is no longer available; use %sRef to inspect without consuming, or do not use %s after the projection",
		place.String(),
		projection,
		projection,
		place.String(),
	)
	return true
}

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

// inferBorrowedResultProjection implements Result.OkRef/ErrRef as read-only
// projections whose Option payload borrows from the still-owned Result.
// Receiver provenance is retained so ordinary escape and borrow checks keep
// the Result alive and prevent conflicting mutation while the projection lives.
//
// Rules:
//   - rules/errors/errorhandling.md — §6.2 "Non-consuming borrowed projections"
//   - rules/errors/errorhandling.md — §29 "Ownership and affine value semantics"
//   - rules/memory/borrowing.md — shared-borrow lifetime and mutation exclusion
func (a *Analyzer) inferBorrowedResultProjection(
	expr *ast.MemberExpression,
	resultType Type,
	member CompilerKnownMember,
) (Type, bool) {
	if resultType.Kind != ResultType || len(resultType.TypeArgs) != 2 {
		return Type{}, false
	}
	receiver, ok := a.resolvePlace(expr.Object)
	if ok && receiver.Type.Kind == ReferenceType && receiver.Type.Element != nil {
		receiver = a.canonicalDereferencePlace(expr.Object, receiver, receiver.Type)
	}
	if !ok || !receiver.Addressable {
		a.addErrorAtToken(expr.Property.Token, "%s requires an addressable Result receiver whose lifetime can be borrowed", member.Name)
		return Type{Kind: InvalidType}, false
	}
	if a.checkBorrowCreationPlace(receiver, false, expr.Property.Token) {
		return Type{Kind: InvalidType}, false
	}

	payload := resultType.TypeArgs[0]
	if member.Name == "ErrRef" {
		payload = resultType.TypeArgs[1]
	}
	borrowed := compilerKnownSharedReference(payload)
	borrowed = a.referenceTypeWithOriginFromExpression(borrowed, expr.Object)
	result := compilerKnownOption(borrowed)

	origin := localOriginWithPlaces(localReferenceOrigin{
		Name:        borrowed.ReferenceOriginName,
		Token:       borrowed.ReferenceOriginToken,
		Local:       borrowed.ReferenceOriginLocal,
		MatchScoped: borrowed.ReferenceOriginMatchScoped,
	}, placeOriginAlternatives(receiver))
	container := localReferenceOrigin{Contained: map[string]localReferenceOrigin{".Some": origin}}
	recomputeContainedOriginSummary(&container)
	a.expressionReferenceOrigins[expr] = container
	return result, true
}
