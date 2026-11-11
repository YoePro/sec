package main

import (
	"fmt"
	"strings"

	"sec/internal/lexer"
	"sec/internal/sema"
)

// ownershipOperationHover presents the transfer Sema resolved for the source
// expression at the cursor: the copy classification of the transferred type,
// the operation (trivial copy, semantic copy or move), the owning destination
// and, for an argument, the selected parameter, plus the source state after
// the transfer. It reads ResolvedTransferPlans only; it never re-derives
// ownership and offers no edit.
// Rules: rules/memory/copy_move.md — §§ 3, 8–11, 29(1)–(2); rules/tooling/lsp.md — "Hover".
func ownershipOperationHover(analyzer *sema.Analyzer, token lexer.Token) string {
	if analyzer == nil {
		return ""
	}
	sections := []string{}
	for _, plan := range analyzer.ResolvedTransferPlans() {
		if plan.File != token.File || plan.Line != token.Line {
			continue
		}
		// A projected source is keyed by its member operator; the field
		// name right after it names the same transferred Place.
		projected := len(plan.Source.Projections) > 0 && token.Column == plan.Column+1
		if plan.Column != token.Column && !projected {
			continue
		}
		sections = append(sections, ownershipTransferLines(analyzer, plan))
	}
	if len(sections) == 0 {
		return ""
	}
	return "\n\n**Ownership**\n\n" + strings.Join(sections, "\n\n")
}

func ownershipTransferLines(analyzer *sema.Analyzer, plan sema.ResolvedTransferPlan) string {
	lines := []string{}
	source := plan.Source.String()
	if plan.SourceFresh || source == "" {
		source = "fresh value"
	} else {
		source = "`" + source + "`"
	}
	lines = append(lines, fmt.Sprintf("- operation: %s of %s", ownershipActionSpelling(plan.Action, plan.Partial), source))
	if plan.Source.Type.Name != "" {
		lines = append(lines, fmt.Sprintf("- copy classification: %s (`%s`)", sema.CopyClassificationOf(plan.Source.Type), plan.Source.Type.Name))
	}
	lines = append(lines, "- destination: "+ownershipDestinationSpelling(analyzer, plan))
	if !plan.SourceFresh {
		switch plan.OwnershipAfter {
		case sema.TransferMoved:
			lines = append(lines, "- source after: moved; it is unavailable until reinitialized")
		default:
			lines = append(lines, "- source after: still owned")
		}
	}
	return strings.Join(lines, "\n")
}

func ownershipActionSpelling(action sema.TransferAction, partial bool) string {
	switch action {
	case sema.TransferCopyTrivial:
		return "trivial copy"
	case sema.TransferCopySemantic:
		return "semantic copy"
	case sema.TransferMove:
		if partial {
			return "partial move"
		}
		return "move"
	}
	return string(action)
}

func ownershipDestinationSpelling(analyzer *sema.Analyzer, plan sema.ResolvedTransferPlan) string {
	destination := plan.Destination
	switch destination.Site {
	case sema.TransferSiteArgument:
		parameter := "argument"
		if destination.Parameter.Lexeme != "" {
			parameter = "parameter `" + destination.Parameter.Lexeme + "`"
		}
		if plan.Action == sema.TransferMove {
			parameter = "consuming " + parameter
		}
		if graph := analyzer.CallGraph(); graph != nil {
			if node, ok := graph.Node(destination.Callable); ok && node.Name != "" {
				return fmt.Sprintf("%s of `%s` (argument %d)", parameter, node.Name, destination.ArgumentIndex+1)
			}
		}
		return parameter
	case sema.TransferSiteReturn:
		return "return value"
	case sema.TransferSiteCapture:
		return "closure environment capture"
	case sema.TransferSiteConstruction:
		return "owning construction payload"
	case sema.TransferSiteBranch:
		return "match result"
	}
	if destination.Place != nil && destination.Place.Root != "" {
		return fmt.Sprintf("%s `%s`", destination.Site, destination.Place.String())
	}
	return string(destination.Site)
}
