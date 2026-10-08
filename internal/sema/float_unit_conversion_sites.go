package sema

import (
	"sort"

	"sec/internal/lexer"
)

// FloatingUnitConversionSites exposes the deterministic source sites whose
// proven conversions require a lowering operation rather than numeric erasure.
// Rules: rules/compiler/semantic_ir.md — Unit semantics and conversion plans;
// rules/corrections/applied/semantic-ir-units-correction-20260818.md — Erasure boundary.
func (a *Analyzer) FloatingUnitConversionSites() []lexer.Token {
	var sites []lexer.Token
	if a == nil {
		return sites
	}
	for expr, plan := range a.unitConversionPlans {
		if plan.Proof == UnitConversionFloatingValueRange {
			sites = append(sites, expressionToken(expr))
		}
	}
	sort.Slice(sites, func(i, j int) bool {
		if sites[i].File != sites[j].File {
			return sites[i].File < sites[j].File
		}
		if sites[i].Line != sites[j].Line {
			return sites[i].Line < sites[j].Line
		}
		return sites[i].Column < sites[j].Column
	})
	return sites
}
