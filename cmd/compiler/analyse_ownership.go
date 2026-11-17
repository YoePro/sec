package main

import (
	"fmt"
	"strings"

	"sec/internal/lexer"
	"sec/internal/sema"
)

// addOwnership presents the ownership transitions and Place provenance Sema
// resolved, from the same facts as the LSP ownership and place-use hovers:
// every accepted move with its source Place, owning destination and the
// source state after it, and every read at which part of the owner is
// unavailable with the moved parts and where they were moved. Copies are
// omitted; they change no availability.
// Rules: rules/compiler/semantic_ir.md — § 109(1)–(3) "ownership/availability transition";
// rules/memory/copy_move.md — §§ 8–11, 29(1)–(2).
func (r *analyseReport) addOwnership(analyzer *sema.Analyzer, inSelection func(lexer.Token) bool) {
	graph := analyzer.CallGraph()
	callableName := func(id sema.CallableID) string {
		if node, ok := graph.Node(id); ok && node.Name != "" {
			return node.Name
		}
		return string(id)
	}
	var lines []string
	for _, plan := range analyzer.ResolvedTransferPlans() {
		at := lexer.Token{File: plan.File, Line: plan.Line, Column: plan.Column}
		if plan.Action != sema.TransferMove || !inSelection(at) {
			continue
		}
		source := "fresh value"
		if !plan.SourceFresh && plan.Source.String() != "" {
			source = plan.Source.String()
		}
		verb := "move"
		if plan.Partial {
			verb = "partial move"
		}
		destination := string(plan.Destination.Site)
		switch plan.Destination.Site {
		case sema.TransferSiteArgument:
			destination = fmt.Sprintf("argument %d of %s", plan.Destination.ArgumentIndex+1, callableName(plan.Destination.Callable))
		default:
			if plan.Destination.Place != nil && plan.Destination.Place.String() != "" {
				destination += " " + plan.Destination.Place.String()
			}
		}
		line := fmt.Sprintf("%s: %s of %s to %s at %s", callableName(plan.Destination.Context), verb, source, destination, analysePosition(at))
		if !plan.SourceFresh && plan.OwnershipAfter == sema.TransferMoved {
			line += "; source moved"
		}
		lines = append(lines, line)
	}
	for _, use := range analyzer.ResolvedPartialPlaceUses() {
		at := lexer.Token{File: use.File, Line: use.Line, Column: use.Column}
		if !inSelection(at) {
			continue
		}
		parts := make([]string, 0, len(use.UnavailableParts))
		for _, part := range use.UnavailableParts {
			state := "moved"
			if part.Conditional {
				state = "conditionally moved"
			} else if part.Reasons != "" {
				state = part.Reasons
			}
			parts = append(parts, fmt.Sprintf("%s %s at %s", part.Place, state, analysePosition(part.At)))
		}
		lines = append(lines, fmt.Sprintf("read of %s at %s with unavailable parts: %s", use.Place.String(), analysePosition(at), strings.Join(parts, ", ")))
	}
	if len(lines) == 0 {
		return
	}
	section := r.section("ownership")
	section.lines = append(section.lines, lines...)
}
