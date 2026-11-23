package sema

import (
	"sort"

	"sec/internal/diagnostics"
	"sec/internal/lexer"
)

// checkFloatTargetCapabilities rejects float use on a plan whose known
// binary floating-point contract lacks a required P20 semantic, once per
// binary width at its first source use, instead of silently weakening Sec
// semantics. An unknown contract (no resolved plan or a planned target
// without a triple) is not a failure here: such plans cannot lower.
// Rules: rules/mlir/packages/sec-mlir-dialect_package20.md — §§ 6, 9, 83.
func (a *Analyzer) checkFloatTargetCapabilities() {
	capabilities := a.targetCapabilityPlan.Float
	if a.summaryPass || !capabilities.Known {
		return
	}
	first := map[int]lexer.Token{}
	for expr, typ := range a.expressionTypes {
		if typ.Kind != FloatType {
			continue
		}
		width := typ.FloatBits
		if width == 0 {
			width = int(a.targetCapabilityPlan.PointerWidthBits)
		}
		token := expressionToken(expr)
		if token.File == "" || token.Line == 0 {
			continue
		}
		if current, seen := first[width]; !seen || tokenBefore(token, current) {
			first[width] = token
		}
	}
	widths := make([]int, 0, len(first))
	for width := range first {
		widths = append(widths, width)
	}
	sort.Ints(widths)
	for _, width := range widths {
		if missing := capabilities.MissingText(width); missing != "" {
			a.addErrorAtTokenWithMetadata(first[width], diagnostics.FloatTargetCapability,
				"Select a target whose plan preserves IEEE-754 binary floating point, or avoid float on this target; Sec never weakens float semantics.",
				"target %s-%s cannot preserve binary%d floating point: missing %s", a.targetCapabilityPlan.TargetOS, a.targetCapabilityPlan.TargetArch, width, missing)
		}
	}
}
