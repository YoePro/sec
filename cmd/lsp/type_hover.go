package main

import (
	"fmt"
	"strings"

	"sec/internal/sema"
)

// typedHover presents compiler-resolved type, contract and aggregate metadata.
// Rules: rules/tooling/lsp.md — "Hover", "One semantic source of truth".
func typedHover(rng lspRange, name string, typ sema.Type) hoverResult {
	contents := fmt.Sprintf("```sec\n%s: %s\n```", name, lspTypeName(typ))
	contents += unitQuantityHoverSuffix(typ)
	// rules/tooling/lsp.md "Hover" lists contracts; they are resolved Sema
	// facts, including inherited contracts, never re-parsed from source.
	if contracts := sema.ContractDisplays(typ); len(contracts) > 0 {
		contents += "\n\nContracts: `" + strings.Join(contracts, "` `") + "`"
	}
	// rules/declarations/lambda-functions.md: callable authority is a resolved
	// Sema fact. The LSP presents it and never re-parses fn/mut fn/-> fn syntax.
	if capability, ok := sema.CallableCapabilityFactOf(typ); ok {
		contents += fmt.Sprintf("\n\nCallable capability: `%s`\n\nInvocation requires: %s.", capability.Spelling, capability.InvocationRequirement)
		if capability.ConsumesCallable {
			contents += "\n\nA successful invocation consumes the callable value."
		}
	}
	contents += shapedFactsHoverSuffix(typ)
	contents += collectionFactsHoverSuffix(typ)
	if fact, atomic := sema.AtomicTypeFactsOf(typ); atomic {
		contents += atomicFactsHoverSuffix(fact)
	}
	if shape, associatedEnum := compareExchangeResultHoverContents(typ, nil); associatedEnum {
		contents += "\n\n" + shape
	}
	if value, source, ok := sema.DefaultValuePreview(typ, 8); ok {
		contents += fmt.Sprintf("\n\nDefault: `%s`\n\nSource: `%s`", value, source)
	} else {
		contents += "\n\nDefault: _none_"
	}
	return hoverResult{
		Contents: markupContent{Kind: "markdown", Value: contents},
		Range:    rng,
	}
}

// shapedFactsHoverSuffix presents the shaped facts Sema resolved for a
// binding's type: rank, shape, Len, whether the shape is static or a runtime
// fact of the value, and canonical dense contiguity. It never invents facts
// Sema has not resolved.
//
// Rules:
//   - rules/tooling/lsp.md — "Shaped values"
func shapedFactsHoverSuffix(typ sema.Type) string {
	facts, ok := sema.ShapedFactsOf(typ)
	if !ok {
		return ""
	}
	if !facts.StaticShape {
		return fmt.Sprintf("\n\nShaped `%s`: Rank `%s` · Shape known at run time from the viewed storage", facts.Family, facts.Rank)
	}
	return fmt.Sprintf("\n\nShaped `%s`: Rank `%s` · Shape `%s` (static) · Len `%s` · Strides `%s` · IsContiguous `true`",
		facts.Family, facts.Rank, facts.Shape, facts.Len, facts.Strides)
}
