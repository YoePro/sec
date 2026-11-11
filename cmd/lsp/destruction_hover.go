package main

import (
	"fmt"
	"strings"

	"sec/internal/lexer"
	"sec/internal/sema"
)

// destructionHover presents the destruction responsibility of the owned
// binding declared at the cursor: its destruction classification and custom
// free, the cleanup owner, release authority for compiler-known resources,
// the validated terminal uses that end all or part of the obligation (and so
// the partial state they leave), pending deferred uses, and the expected
// terminal action. It reads ResolvedDestructionAt only; exit-path resolution
// stays with cleanup planning, so the terminal action is stated per recorded
// site rather than claimed for every path.
// Rules: rules/memory/destruction.md — §§ 3, 6.1, 9–12, 33; rules/tooling/lsp.md — "Hover".
func destructionHover(analyzer *sema.Analyzer, token lexer.Token) string {
	if analyzer == nil {
		return ""
	}
	fact, ok := analyzer.ResolvedDestructionAt(token.File, token.Line, token.Column)
	if !ok {
		return ""
	}
	registration := fact.Registration
	name := "`" + registration.Place.Root + "`"
	lines := []string{}
	plan, planned := registration.Destruction.Plan(registration.Destruction.Root)
	if planned {
		lines = append(lines, "- classification: "+destructionClassification(plan))
		if plan.Kind == sema.DestructionResource {
			lines = append(lines, fmt.Sprintf("- release authority: the `%s` resource contract", plan.Resource))
		}
	}
	start := "after successful initialization"
	if fact.Origin == "parameter" {
		start = "on entry to the callable"
	}
	lines = append(lines, fmt.Sprintf("- cleanup owner: %s (%s); the obligation starts %s", name, fact.Origin, start))
	whole := []string{}
	for _, terminal := range fact.Terminals {
		reason := terminal.Reason
		if reason == "" {
			reason = "ended"
		}
		if len(terminal.Place.Projections) > 0 {
			lines = append(lines, fmt.Sprintf("- partial state: `%s` %s at %d:%d; the remaining parts are still destroyed", terminal.Place.String(), reason, terminal.Source.Line, terminal.Source.Column))
			continue
		}
		whole = append(whole, fmt.Sprintf("%s at %d:%d", reason, terminal.Source.Line, terminal.Source.Column))
	}
	for _, use := range fact.DeferredUses {
		lines = append(lines, fmt.Sprintf("- pending deferred use at %d:%d; the defer runs before %s is destroyed", use.Line, use.Column, name))
	}
	if len(whole) == 0 {
		lines = append(lines, "- expected terminal action: destroyed at the end of its scope")
	} else {
		lines = append(lines, fmt.Sprintf("- expected terminal action: ownership ends where %s is %s; on a path that does not reach such a use it is destroyed at the end of its scope", name, strings.Join(whole, ", ")))
	}
	return "\n\n**Destruction**\n\n" + strings.Join(lines, "\n")
}

func destructionClassification(plan sema.ResolvedDestructionPlan) string {
	switch plan.Kind {
	case sema.DestructionStruct:
		fields := []string{}
		for _, step := range plan.Fields {
			fields = append(fields, "`"+step.Field+"`")
		}
		text := "struct"
		if plan.CustomFree {
			text += "; runs its custom `free` first"
		}
		if len(fields) > 0 {
			step := "; destroys"
			if plan.CustomFree {
				step = "; then destroys"
			}
			text += step + " owned fields in reverse declaration order: " + strings.Join(fields, ", ")
		}
		return text
	case sema.DestructionFixedArray:
		return fmt.Sprintf("fixed array; destroys %s initialized elements in reverse index order", plan.ElementCount)
	case sema.DestructionVariant:
		family := plan.VariantFamily
		if family == "" {
			family = "variant"
		}
		return family + "; destroys only the active, initialized payload"
	case sema.DestructionResource:
		return "compiler-known resource; released through its own contract"
	case sema.DestructionUnsupported:
		return "no destruction plan in the compilation track yet: " + plan.Unsupported
	}
	return string(plan.Kind)
}
