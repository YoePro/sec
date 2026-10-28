package main

import (
	"fmt"
	"strings"

	"sec/internal/ast"
	"sec/internal/sema"
)

// unitMetadataHover presents the value Sema resolved for a `<U>.Member`
// metadata read: its exact public type (the concrete enum for custom Kind,
// System and Origin), the value, whether it is an omission default, and for a
// decimal member the exact declared rational behind a projected Scale.
//
// Rules:
//   - rules/tooling/lsp.md — "Hover"
//   - rules/types/units.md — "Unit metadata value types" (1), (7), (10)
//   - rules/corrections/md062-unit-metadata-value-types-correction-20261027.md — § 12(9)
func unitMetadataHover(sourceRange lspRange, read sema.UnitMetadataRead) hoverResult {
	lines := []string{fmt.Sprintf("```sec\nunit metadata <%s>.%s: %s\n```", read.Unit, read.Member, lspTypeName(read.Type))}
	value := ""
	switch read.Member {
	case "Name", "LongName", "Symbol":
		value = fmt.Sprintf("`%q`", read.String)
	case "BaseUnit":
		value = fmt.Sprintf("`%t`", read.Bool)
	case "Dimension":
		value = fmt.Sprintf("`%s` (AxisCount `%d`, IsDimensionless `%t`)", sema.CanonicalDimensionDisplay(read.Dimension), len(read.Dimension.Base), read.Dimension.IsZero())
	case "Scale", "Offset", "LogBase", "LogFactor", "Reference":
		value = "`" + read.Decimal + "`"
		if read.Projected {
			value += fmt.Sprintf(" (nearest decimal of the exact `%s`, ties to even)", read.Exact.RatString())
		}
	default:
		value = "`" + read.Enum.EnumType.Name + "." + read.Enum.Name + "`"
		if read.Enum.EnumType.Module == "core" && (read.Enum.Name == "Unknown" || read.Enum.Name == "Unspecified" || read.Enum.Name == "None") {
			value += " (absence marker, not evidence of compatibility)"
		}
	}
	if read.Defaulted {
		value += " — omitted; default"
	}
	lines = append(lines, "Value: "+value)
	return hoverResult{Contents: markupContent{Kind: "markdown", Value: strings.Join(lines, "\n\n")}, Range: sourceRange}
}

// unitMetadataCompletionItems lists the metadata members readable on a named
// unit reference `<U>.`, with the types Sema assigns them. A member whose
// read is a compile-time error, such as an undeclared LongName, is omitted.
// Rules: rules/tooling/lsp.md — completions; rules/types/units.md — "Unit metadata value types" (1), (11).
func unitMetadataCompletionItems(analyzer *sema.Analyzer, target ast.Expression, prefix string) ([]completionItem, bool) {
	reference, ok := target.(*ast.UnitReferenceExpression)
	if !ok || reference.UnitExpression == nil || reference.UnitExpression.Kind != ast.UnitExpressionName {
		return nil, false
	}
	members, ok := analyzer.UnitMetadataMembers(reference.UnitExpression.Name)
	if !ok {
		return nil, false
	}
	items := []completionItem{}
	for _, member := range members {
		if member.Problem != "" || !strings.HasPrefix(member.Name, prefix) {
			continue
		}
		items = append(items, completionItem{Label: member.Name, Kind: completionKindProperty, Detail: "unit metadata " + lspTypeName(member.Type)})
	}
	return items, true
}
