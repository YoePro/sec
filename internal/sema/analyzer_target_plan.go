package sema

import "sec/internal/layout"

// NewAnalyzerWithScalarPlan derives target-sized source integer bounds from the
// authoritative plan defined by rules/types/types.md and rules/memory/layout.md.
// correction5.md forbids Sema from guessing widths from architecture strings.
func NewAnalyzerWithScalarPlan(plan layout.ResolvedScalarPlan) *Analyzer {
	return NewAnalyzerWithScalarPlanAndDepth(plan, AnalysisStandard)
}

// NewAnalyzerWithScalarPlanAndDepth combines the authoritative target scalar
// plan with an analysis budget. Depth selects resources only; the language and
// every mandatory proof stay identical at each depth.
//
// Rules:
//   - rules/compiler/compiler_pipeline.md — § 65(2) analysis-only mode uses the same plan/Sema facts
//   - rules/compiler/compiler_analysis.md — § 62 analysis modes
//   - rules/memory/allocation.md — §22; rules/platform/target_profiles.md — §31
func NewAnalyzerWithScalarPlanAndDepth(plan layout.ResolvedScalarPlan, depth AnalysisDepth) *Analyzer {
	analyzer := NewAnalyzerWithDepth(depth)
	analyzer.SetAllocationProfile(plan.Profile)
	analyzer.referencePolicy = plan.References
	if plan.PointerWidthBits != 32 && plan.PointerWidthBits != 64 {
		return analyzer
	}
	analyzer.types["int"] = targetSignedIntegerType("int", plan.PointerWidthBits)
	analyzer.types["uint"] = targetUnsignedIntegerType("uint", plan.PointerWidthBits)
	// MD-014: plain float follows the platform width exactly like int and
	// uint; there is no separate float-width policy.
	// Rules: rules/corrections/applied/missing-decisions-md010-md014-correction-20261003.md — §§ 6.3–6.9
	platformFloat := analyzer.types["float"]
	platformFloat.FloatBits = int(plan.PointerWidthBits)
	analyzer.types["float"] = platformFloat
	analyzer.types["ProcessID"] = processIDType(plan.PointerWidthBits)
	analyzer.targetUintWidthBits = plan.PointerWidthBits
	analyzer.scalarPlanWidthBits = plan.PointerWidthBits
	analyzer.registerCFundamentalTypes(plan.CABI)
	analyzer.addressDomains = append([]layout.AddressDomain(nil), plan.AddressDomains...)
	return analyzer
}

// AddressDomain returns the selected plan's hardware address domain with that
// identity; an analyzer built without a plan has none (MD-111 § 1(6)).
func (a *Analyzer) AddressDomain(identity string) (layout.AddressDomain, bool) {
	if a == nil {
		return layout.AddressDomain{}, false
	}
	for _, domain := range a.addressDomains {
		if domain.Identity == identity {
			return domain, true
		}
	}
	return layout.AddressDomain{}, false
}

// ScalarPlanWidthBits is the selected CompilationPlan's pointer width, or
// zero when the analyzer was built without a plan. Plan-specific exactness
// proofs record it (MD-104).
func (a *Analyzer) ScalarPlanWidthBits() uint16 {
	if a == nil {
		return 0
	}
	return a.scalarPlanWidthBits
}
