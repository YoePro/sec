package sema

import "sec/internal/ast"

// compilerKnownTicketMembers exposes revocation on its sole owning authority,
// with the complete concrete result shared by Sema and editor tooling.
// Rules: rules/concurrency/channels.md — §§18(1–4),30(1–5),62(5).
func compilerKnownTicketMembers(typ Type) []CompilerKnownMember {
	if typ.Name != "MessageTicket" || !typ.Intrinsic || len(typ.TypeArgs) != 1 {
		return nil
	}
	result := builtinChannelRevokeResultType()
	result.GenericParameters = nil
	result.TypeArgs = []Type{typ.TypeArgs[0]}
	result.UnionVariants[0].Payload = &result.TypeArgs[0]
	return []CompilerKnownMember{{
		ID: "CKM-MESSAGE-TICKET-REVOKE", Name: "Revoke", Kind: CompilerKnownMethod,
		Result: result, Signature: "fn Revoke() " + typeDisplayName(result),
		Receiver:          "owning " + typeDisplayName(typ),
		Documentation:     "Consumes this ticket to attempt revocation. Revoked(T) returns message ownership; Unavailable(MessageDisposition) reports Received, Expired or Discarded. The originating Sender is not required.",
		CallerObligations: []string{"requires an owned ticket; the ticket is unavailable after the call"},
		Rule:              "rules/concurrency/channels.md — §§18,30",
	}}
}

// inferTicketRevokeCall consumes exactly one owned ticket using canonical
// Place/borrow/receiver-move facts, independently of the originating sender.
// Rules: rules/concurrency/channels.md — §§18(2–5),30(1–5);
// rules/memory/ownership.md — ownership availability;
// rules/memory/borrowing.md — overlapping borrow conflicts.
func (a *Analyzer) inferTicketRevokeCall(expr *ast.CallExpression, member *ast.MemberExpression, receiver Type) (Type, expressionValue, bool) {
	value := expressionValue{Display: expr.String()}
	ticket := dereferenceType(receiver)
	if !a.checkCompilerKnownCallArity(expr, "MessageTicket.Revoke", 0, 0) {
		return Type{Kind: InvalidType}, value, true
	}
	if receiver.Kind == ReferenceType {
		a.addErrorAtTokenWithMetadata(member.Property.Token, "",
			"Revoke consumes its ticket; call it on the owning MessageTicket[T] rather than a reference.",
			"MessageTicket.Revoke requires an owned ticket, got a reference")
		return Type{Kind: InvalidType}, value, true
	}
	if place, reusable := a.resolvePlace(member.Object); reusable {
		if a.rejectOrdinaryMethodWholeSelfConsumption(place, member.Property.Token) || a.rejectCustomFreePartialMove(place, member.Property.Token) {
			return Type{Kind: InvalidType}, value, true
		}
		if !place.Addressable || len(place.Projections) > 0 && !place.PartialMoveSafe {
			a.addErrorAtToken(member.Property.Token, "MessageTicket.Revoke requires independently tracked owned ticket storage")
			return Type{Kind: InvalidType}, value, true
		}
		if a.checkBorrowedMovePlaceForAction(place, member.Property.Token, "consume") {
			return Type{Kind: InvalidType}, value, true
		}
		a.recordReceiverMove(expr, place)
		a.markPlaceUnavailable(place, member.Property.Token, "consumed by MessageTicket.Revoke()")
		if len(place.Projections) == 0 {
			a.endBorrowsHeldBy(place.Root)
		}
	}
	return a.intrinsicGenericType("ChannelRevokeResult", ticket.TypeArgs[0]), value, true
}
