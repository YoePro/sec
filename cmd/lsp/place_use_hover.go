package main

import (
	"fmt"
	"strings"

	"sec/internal/lexer"
	"sec/internal/sema"
)

// placeUseHover presents the availability fact Sema recorded at an accepted
// read of a local Place: where the root's current value was moved in from,
// which parts of the enclosing owner are unavailable (partial aggregate
// state), and a consequence preview of consuming the Place at this read. It
// reads ResolvedPlaceUseAt only. The preview lists later uses in source order;
// it is information, not a proof that a move would be accepted, and no code
// action is derived from it (copy_move.md § 29(4)).
// Rules: rules/memory/copy_move.md — § 29(1)–(2), (4); rules/tooling/lsp.md — "Hover".
func placeUseHover(analyzer *sema.Analyzer, token lexer.Token) string {
	if analyzer == nil {
		return ""
	}
	use, ok := analyzer.ResolvedPlaceUseAt(token.File, token.Line, token.Column)
	if !ok {
		return ""
	}
	place := "`" + use.Place.String() + "`"
	lines := []string{}
	if len(use.UnavailableParts) == 0 {
		lines = append(lines, fmt.Sprintf("- availability: %s is available", place))
	} else {
		parts := make([]string, 0, len(use.UnavailableParts))
		for _, part := range use.UnavailableParts {
			state := "moved"
			if part.Conditional {
				state = "conditionally moved"
			}
			if part.Reasons != "" && !part.Conditional {
				state = part.Reasons
			}
			parts = append(parts, fmt.Sprintf("`%s` %s at %d:%d", part.Place, state, part.At.Line, part.At.Column))
		}
		lines = append(lines, fmt.Sprintf("- availability: %s is readable; owner `%s` is partially available (%s)", place, use.Place.Root, strings.Join(parts, ", ")))
	}
	if origin := use.Origin; origin != nil {
		lines = append(lines, fmt.Sprintf("- value origin: moved in from `%s` at %d:%d", origin.Source.String(), origin.Line, origin.Column))
	}
	if preview := placeUseConsequence(analyzer, use, token); preview != "" {
		lines = append(lines, preview)
	}
	return "\n\n**Availability**\n\n" + strings.Join(lines, "\n")
}

// placeUseConsequence previews consuming the Place at this read. Trivially
// copyable values are never consumed, and a read that already is a transfer
// site is described by the ownership section instead.
func placeUseConsequence(analyzer *sema.Analyzer, use sema.ResolvedPlaceUse, token lexer.Token) string {
	if sema.CopyClassificationOf(use.Place.Type) == sema.CopyTrivial {
		return ""
	}
	for _, plan := range analyzer.ResolvedTransferPlans() {
		if plan.File == token.File && plan.Line == token.Line && (plan.Column == token.Column || plan.Column+1 == token.Column) {
			return ""
		}
	}
	root := "`" + use.Place.Root + "`"
	text := ""
	if len(use.LaterUses) == 0 {
		text = fmt.Sprintf("- if moved here: no later use of %s follows in this callable in source order", root)
	} else {
		positions := make([]string, 0, len(use.LaterUses))
		for index, later := range use.LaterUses {
			if index == 4 {
				positions = append(positions, fmt.Sprintf("… %d more", len(use.LaterUses)-index))
				break
			}
			positions = append(positions, fmt.Sprintf("%d:%d", later.Line, later.Column))
		}
		noun := "use"
		if len(use.LaterUses) > 1 {
			noun = "uses"
		}
		text = fmt.Sprintf("- if moved here: %s becomes unavailable for %d later %s (%s) unless reinitialized first", root, len(use.LaterUses), noun, strings.Join(positions, ", "))
	}
	if use.RepeatedByLoop {
		text += "; the enclosing loop repeats this read, so the next iteration would observe the move"
	}
	return text
}
