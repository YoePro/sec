package sema

import (
	"sort"
	"strings"

	"sec/internal/ast"
	"sec/internal/diagnostics"
	"sec/internal/lexer"
)

// StringMaterializationAllocation is the frontend-resolved allocation fact of
// one runtime string materialization. Semantic IR and backends consume it and
// never select an allocator themselves.
//
// Rules:
//   - rules/corrections/applied/missing-decisions-md001-md009-correction-20261003.md — §§ 5.10–5.15
//   - rules/memory/allocation.md — § 5 "Default allocation model", § 17 "Strings", § 25 "Semantic IR"
type StringMaterializationAllocation struct {
	// Context is the canonical active allocation context; Sec 0.1 defines no
	// string-specific allocator or allocation domain.
	Context AllocationContext
	// FailureType is the canonical StringError of the ordinary failure
	// channel; an allocation failure is reported as StringError.Allocation.
	FailureType Type
}

// stringMaterializationSite remembers where a runtime plan was recorded so
// its allocation effect and missing-try diagnostic can be published once the
// maximal plan is final.
type stringMaterializationSite struct {
	callable  CallableID
	reachable bool
	token     lexer.Token
}

// activeAllocationContext resolves the canonical active allocation context of
// the selected target profile. A hosted profile provides the compiler-managed
// Arena context of § 5(1)–(3); noalloc provides none. No rule yet assigns an
// allocation profile to other target profiles such as freestanding
// (missing-decisions.yaml MD-034), so they provide no positive context.
//
// Rules:
//   - rules/memory/allocation.md — § 5 "Default allocation model", § 22 "Target and build profiles"
func (a *Analyzer) activeAllocationContext() AllocationContext {
	profile := a.targetProfile
	switch profile {
	case "", "hosted":
		if profile == "" {
			profile = "hosted"
		}
		return AllocationContext{Available: true, Origin: StorageOriginArena, Profile: profile}
	case "embedded-arena":
		return AllocationContext{Available: true, Origin: StorageOriginArena, Profile: profile}
	default:
		return AllocationContext{Available: false, Origin: StorageOriginUnknown, Profile: profile}
	}
}

// storeStringConcatPlan records a maximal plan and classifies it: a plan whose
// segments are all constant text is compile-time folded static data; any other
// plan is one fallible runtime materialization in the active context.
//
// Rules:
//   - rules/corrections/applied/missing-decisions-md001-md009-correction-20261003.md — §§ 5.1–5.8, 5.10–5.15
//   - rules/foundations/operators.md — "Maximal concatenation plan", "Compile-time concatenation"
func (a *Analyzer) storeStringConcatPlan(root ast.Expression, segments []StringConcatSegment) {
	plan := StringConcatPlan{Segments: segments}
	foldedText, folded := foldedStringConcatText(segments)
	plan.FoldedText = foldedText
	plan.Runtime = !folded
	if plan.Runtime {
		plan.Allocation = StringMaterializationAllocation{Context: a.activeAllocationContext(), FailureType: a.types["StringError"]}
		a.stringMaterializationSites[root] = stringMaterializationSite{callable: a.currentCallable, reachable: a.callGraphPathReachable && !a.summaryPass, token: expressionToken(root)}
	}
	a.stringConcatPlans[root] = plan
}

// foldedStringConcatText materializes the exact static-data value of a fully
// compile-time concatenation. Literal nodes already contain lexer-decoded Sec
// text, so folding preserves Unicode scalars and escape semantics without
// consulting host-language quoting rules.
//
// Rules:
//   - rules/foundations/operators.md — "Compile-time concatenation"
//   - rules/foundations/lexical_structure.md — §§13–15 character, string, and escape semantics
func foldedStringConcatText(segments []StringConcatSegment) (string, bool) {
	var folded strings.Builder
	for _, segment := range segments {
		switch {
		case segment.Kind == StringConcatConstantString:
			folded.WriteString(segment.Text)
		case segment.Expression != nil:
			switch literal := segment.Expression.(type) {
			case *ast.CharLiteral:
				folded.WriteString(literal.Value)
			case *ast.StringLiteral:
				folded.WriteString(literal.Value)
			default:
				return "", false
			}
		default:
			return "", false
		}
	}
	return folded.String(), true
}

// runtimeStringMaterialization reports whether expr is the root of a final
// runtime string materialization plan.
func (a *Analyzer) runtimeStringMaterialization(expr ast.Expression) bool {
	plan, ok := a.stringConcatPlans[expr]
	return ok && plan.Runtime
}

// reportStringMaterializations publishes, once every maximal plan is final,
// the allocation effect of each runtime string materialization and rejects
// those that no try protects or that have no valid allocation context.
//
// Rules:
//   - rules/corrections/applied/missing-decisions-md001-md009-correction-20261003.md — §§ 5.5–5.6, 5.18–5.19
//   - rules/memory/allocation.md — § 10 "Allocation failure", § 22(4), § 24 "Allocation effects"
func (a *Analyzer) reportStringMaterializations() {
	roots := make([]ast.Expression, 0, len(a.stringMaterializationSites))
	for root := range a.stringMaterializationSites {
		if a.runtimeStringMaterialization(root) {
			roots = append(roots, root)
		}
	}
	sort.SliceStable(roots, func(left, right int) bool {
		l, r := a.stringMaterializationSites[roots[left]].token, a.stringMaterializationSites[roots[right]].token
		if l.File != r.File {
			return l.File < r.File
		}
		if l.Line != r.Line {
			return l.Line < r.Line
		}
		return l.Column < r.Column
	})
	for _, root := range roots {
		site := a.stringMaterializationSites[root]
		plan := a.stringConcatPlans[root]
		if site.reachable {
			a.callGraph.addArenaEffect(site.callable, ArenaEffectSite{Kind: ArenaEffectAllocate, Source: site.token, MayAllocate: true})
		}
		if !plan.Allocation.Context.Available {
			a.addErrorAtTokenWithMetadata(site.token, diagnostics.StringMaterializationWithoutAllocationContext,
				"Build the text from compile-time constants so it folds to static data, or select a target profile that provides an allocation context.",
				"runtime string materialization needs an allocation context, but target profile %q provides none", plan.Allocation.Context.Profile)
			continue
		}
		if a.protectedStringMaterializations[root] {
			continue
		}
		a.addErrorAtTokenWithMetadata(site.token, diagnostics.StringMaterializationRequiresTry,
			"Write `try` before the expression to propagate StringError, or add a handler such as `try ... { Err(_) => fallback }`.",
			"runtime string concatenation or interpolation may fail to allocate and requires try")
	}
}
