package main

import (
	"fmt"
	"strings"

	"sec/internal/sema"
)

// tryFailureSetHoverLines lists the ordered compiler-internal failure set of a
// try: each protected operation with the error it can raise. The set is
// analysis information, never an inferred error type.
//
// Rules:
//   - rules/errors/errorhandling.md — §11 "Compiler-internal failure sets", §36 "LSP requirements"
func tryFailureSetHoverLines(failures []sema.TryFailurePoint) []string {
	lines := []string{}
	members := []string{}
	for _, failure := range failures {
		source := "protected carrier"
		if failure.Expression != nil && failure.Kind != sema.TryFailureCarrier {
			source = "`" + failure.Expression.String() + "`"
		}
		lines = append(lines, fmt.Sprintf("Failure source: %s (%s) may raise `%s`", source, failure.Kind, lspTypeName(failure.ErrorType)))
		name := lspTypeName(failure.ErrorType)
		duplicate := false
		for _, member := range members {
			duplicate = duplicate || member == name
		}
		if !duplicate {
			members = append(members, name)
		}
	}
	if len(members) > 1 {
		lines = append(lines, "Failure set: `"+strings.Join(members, "`, `")+"` (no common error type is inferred; `Err(_)` handles all of them)")
	}
	return lines
}

// tryHandlerHoverLines describes each resolved handler in source order.
// Rules: rules/errors/errorhandling.md — §§15–21,36; rules/tooling/lsp.md — "Hover".
func tryHandlerHoverLines(handlers []sema.ResolvedTryHandler) []string {
	lines := []string{}
	for _, handler := range handlers {
		pattern := "None"
		switch handler.PatternKind {
		case sema.TryHandlerErrVariant:
			pattern = "Err(" + handler.Variant + ")"
		case sema.TryHandlerErrCatchAll:
			pattern = "Err(_)"
			if handler.BindingName != "" {
				pattern = "Err(" + handler.BindingName + ": " + lspTypeName(handler.BindingType) + ")"
			}
		}
		if handler.Guarded {
			pattern += " where …"
		}
		// rules/errors/errorhandling.md §20: show whether the binding copies
		// the error payload or takes ownership of it.
		if handler.BindingName != "" {
			if handler.BindingCopies {
				pattern += " (copies payload)"
			} else {
				pattern += " (moves payload)"
			}
		}
		action := map[sema.ResolvedTryHandlerFlow]string{
			sema.TryHandlerProducesValue: "recovery value",
			sema.TryHandlerReturns:       "returns",
			sema.TryHandlerTerminates:    "terminates",
		}[handler.Flow]
		if action == "" {
			action = string(handler.Flow)
		}
		lines = append(lines, fmt.Sprintf("Handler %d: `%s` → %s", handler.SourceIndex+1, pattern, action))
	}
	return lines
}

// errorIdentityHoverLines states when a concrete error widens into the open
// error root on propagation; the concrete identity is retained.
//
// Rules:
//   - rules/errors/errorhandling.md — §2.1 "Error assignability", §36 "LSP requirements"
func errorIdentityHoverLines(resolved sema.ResolvedTry) []string {
	target := resolved.EnclosingResultType
	if target.Kind != sema.ResultType || len(target.TypeArgs) != 2 || target.TypeArgs[1].Kind != sema.ErrorRootType {
		return nil
	}
	if resolved.ErrorType.Kind == sema.ErrorRootType || resolved.ErrorType.Kind == "" {
		return nil
	}
	return []string{"Error identity: `" + lspTypeName(resolved.ErrorType) + "` widened to `error`; the concrete error identity is retained"}
}
