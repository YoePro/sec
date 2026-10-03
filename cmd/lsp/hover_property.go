package main

import (
	"fmt"
	"strings"

	"sec/internal/sema"
)

// propertyHoverContents presents the property contract already resolved by
// Sema. In particular, a fallible setter's declared error type is public API
// and must not disappear behind field-like member syntax in editor tooling.
//
// Rules:
//   - rules/declarations/properties.md — §§ 4–6 getter and setter semantics
//   - rules/errors/errorhandling.md — § 24 fallible property setters; § 36 LSP requirements
//   - rules/tooling/lsp.md — "Hover", fallible-setter facts
func propertyHoverContents(property sema.Property) string {
	modifier := ""
	if property.Static {
		modifier = "static "
	}
	lines := []string{
		fmt.Sprintf("```sec\n%sproperty %s: %s\n```", modifier, property.Name, lspTypeName(property.Type)),
		"Accessors: `" + strings.Join(propertyAccessorLabels(property), "`, `") + "`",
	}
	if property.Fallible {
		if property.Error != nil {
			lines = append(lines, "Setter error: `"+lspTypeName(*property.Error)+"`")
		}
		lines = append(lines, "Assignment requires: `try`")
	}
	return strings.Join(lines, "\n\n")
}

func propertyCompletionDetail(property sema.Property) string {
	detail := lspTypeName(property.Type)
	accessors := propertyAccessorLabels(property)
	if len(accessors) > 0 {
		detail += " (" + strings.Join(accessors, ", ") + ")"
	}
	if property.Fallible && property.Error != nil {
		detail += " error " + lspTypeName(*property.Error)
	}
	return detail
}

func propertyAccessorLabels(property sema.Property) []string {
	labels := []string{}
	if property.HasGetter {
		labels = append(labels, "get")
	}
	if property.HasSetter {
		setter := "set"
		if property.Fallible {
			setter = "try set"
		}
		labels = append(labels, setter)
	}
	return labels
}
