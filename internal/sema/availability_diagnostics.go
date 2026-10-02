package sema

import (
	"sort"
	"strings"

	"sec/internal/diagnostics"
	"sec/internal/lexer"
)

// availabilityReasonSeparator joins the distinct UnavailableReasons retained
// for one Place after a control-flow join. The reasons stay provenance; they
// never form a distinct availability state.
//
// Rules:
//   - rules/memory/ownership.md — §6 "Unavailability reason"
const availabilityReasonSeparator = "|"

// joinAvailabilityReasons combines the reasons of every continuing path that
// left a Place unavailable, in a deterministic order and without duplicates.
func joinAvailabilityReasons(reasons []string) string {
	unique := map[string]bool{}
	for _, reason := range reasons {
		for _, part := range strings.Split(underlyingAvailabilityReason(reason), availabilityReasonSeparator) {
			if part != "" {
				unique[part] = true
			}
		}
	}
	parts := make([]string, 0, len(unique))
	for part := range unique {
		parts = append(parts, part)
	}
	sort.Strings(parts)
	return strings.Join(parts, availabilityReasonSeparator)
}

// availabilityReasonPhrase renders one or more retained reasons for a mentor
// diagnostic, for example "moved" or "discarded or moved".
func availabilityReasonPhrase(reason string) string {
	parts := strings.Split(underlyingAvailabilityReason(reason), availabilityReasonSeparator)
	phrases := make([]string, 0, len(parts))
	for _, part := range parts {
		switch part {
		case "":
			continue
		case "consumed by call", "consumed by method call":
			phrases = append(phrases, "consumed by a call")
		case "released":
			phrases = append(phrases, "released")
		default:
			phrases = append(phrases, part)
		}
	}
	if len(phrases) == 0 {
		return "moved"
	}
	return strings.Join(phrases, " or ")
}

func isMultipleAvailabilityReason(reason string) bool {
	return strings.Contains(underlyingAvailabilityReason(reason), availabilityReasonSeparator)
}

// reportConditionallyUnavailablePlace explains a Place that is unavailable
// on at least one possible continuing path but not proven unavailable on all.
//
// Rules:
//   - rules/memory/ownership.md — §5 "Conditional availability", §33.3 "Conditional availability"
//   - rules/memory/destruction.md — §11.1 "Conditional state"
func (a *Analyzer) reportConditionallyUnavailablePlace(place Place, movedKey string, token, movedAt lexer.Token, borrow bool) {
	subject := place.String()
	detail := "it was " + availabilityReasonPhrase(a.moveReasons[movedKey]) + " on one possible execution path"
	if movedKey != subject {
		detail = "sub-place " + movedKey + " was " + availabilityReasonPhrase(a.moveReasons[movedKey]) + " on one possible execution path"
	}
	help := "test `" + movedKey + " is available` before using it, or restructure the control flow so availability is known statically"
	if borrow {
		a.addErrorAtTokenWithPreviousMetadata(token, movedAt, diagnostics.ConditionallyUnavailableUse, help,
			"cannot borrow conditionally available place %s; %s", subject, detail)
		return
	}
	a.addErrorAtTokenWithPreviousMetadata(token, movedAt, diagnostics.ConditionallyUnavailableUse, help,
		"%s may no longer be available here; %s", subject, detail)
}

// reportDefinitelyUnavailablePlace explains a read of a Place that is
// unavailable on every continuing path, keeping the earlier ownership
// operation as related evidence.
//
// Rules:
//   - rules/memory/ownership.md — §6 "Unavailability reason", §33.2 "Use after move"
//   - rules/control-flow/discard.md — §§5, 26, 37
func (a *Analyzer) reportDefinitelyUnavailablePlace(place Place, movedKey string, token, movedAt lexer.Token) {
	reason := underlyingAvailabilityReason(a.moveReasons[movedKey])
	help := "use " + place.String() + " before that operation, borrow it instead when appropriate, or reinitialize a mutable binding before using it again"
	if isMultipleAvailabilityReason(reason) {
		a.addErrorAtTokenWithPreviousMetadata(token, movedAt, diagnostics.UseAfterMove, help,
			"value %s is no longer available; on every possible execution path it was %s", place.String(), availabilityReasonPhrase(reason))
		return
	}
	switch reason {
	case "discarded":
		a.addErrorAtTokenWithPreviousMetadata(token, movedAt, diagnostics.UseAfterDiscard,
			"reinitialize a mutable Place before using it, or keep the value until its last use",
			"value %s was discarded here and is no longer available", place.String())
	case "detached":
		a.addErrorAtTokenWithPreviousMetadata(token, movedAt, diagnostics.UseAfterMove, help,
			"value %s was detached here and is no longer available", place.String())
	case "released":
		a.addErrorAtTokenWithPreviousMetadata(token, movedAt, diagnostics.UseAfterMove,
			"use the arena's allocations before Release, or create a new Arena",
			"arena %s was released here and is no longer available", place.String())
	case "consumed by call":
		a.addErrorAtTokenWithPreviousMetadata(token, movedAt, diagnostics.UseAfterMove, help,
			"value %s was consumed by call here and is no longer available", place.String())
	default:
		a.addErrorAtTokenWithPreviousMetadata(token, movedAt, diagnostics.UseAfterMove, help,
			"use of moved value %s", place.String())
	}
}

// reportPartiallyUnavailablePlace explains a whole-value use of an aggregate
// whose sub-place is definitely unavailable.
//
// Rules:
//   - rules/memory/ownership.md — §5 "PartiallyAvailable", §14 "Partial moves"
//   - rules/memory/copy_move.md — §14(6)
func (a *Analyzer) reportPartiallyUnavailablePlace(place Place, movedKey string, token, movedAt lexer.Token, borrow bool) {
	help := "use the still-available sub-places individually, or reinitialize " + movedKey + " before using " + place.String() + " as a whole"
	if borrow {
		a.addErrorAtTokenWithPreviousMetadata(token, movedAt, diagnostics.PartiallyUnavailableUse, help,
			"cannot borrow partially available place %s; sub-place %s is unavailable", place.String(), movedKey)
		return
	}
	a.addErrorAtTokenWithPreviousMetadata(token, movedAt, diagnostics.PartiallyUnavailableUse, help,
		"cannot use partially moved value %s; place %s is unavailable", place.String(), movedKey)
}
