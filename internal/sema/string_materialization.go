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

// StringMaterializationFailureMode is the source-selected failure of one runtime
// string materialization; no type, target or build setting changes it.
// Rules: rules/foundations/operators.md — "Runtime allocation and failure policy" (MD-039).
type StringMaterializationFailureMode string

const (
	// StringMaterializationTry reports failure as StringError under a protecting try.
	StringMaterializationTry StringMaterializationFailureMode = "try"
	// StringMaterializationPanic is ordinary `+` or `+=`: failure panics with
	// StringMaterializationFailure.
	StringMaterializationPanic StringMaterializationFailureMode = "panic"
)

// ResolvedStringAppend is one compound `string +=`: a transactional
// materialization whose right operand is its value expression.
type ResolvedStringAppend struct {
	Plan StringConcatPlan
	// Fallible marks a Result[string, StringError] right operand.
	Fallible bool
}

// ResolvedStringAppendOf returns the append assigning value.
func (a *Analyzer) ResolvedStringAppendOf(value ast.Expression) (ResolvedStringAppend, bool) {
	if a == nil || value == nil {
		return ResolvedStringAppend{}, false
	}
	resolved, ok := a.stringAppends[value]
	return resolved, ok
}

// recordStringAppend publishes the plan of a `string +=` with the same context,
// site and effects as `+`. It is keyed by the right operand, the one expression
// every assignment target form shares.
func (a *Analyzer) recordStringAppend(value ast.Expression, fallible bool) {
	a.stringAppends[value] = ResolvedStringAppend{Fallible: fallible, Plan: StringConcatPlan{
		Runtime:    true,
		Allocation: StringMaterializationAllocation{Context: a.activeAllocationContext(), FailureType: a.types["StringError"]},
	}}
	a.stringAppendSites[value] = stringMaterializationSite{callable: a.currentCallable, reachable: a.callGraphPathReachable && !a.summaryPass, token: expressionToken(value)}
}

// stringMaterializationSite remembers where a runtime plan was recorded so
// its allocation and panic effects and required-try diagnostic can be published
// once the maximal plan is final.
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
	facts := a.AllocationCapabilities()
	origin := StorageOriginUnknown
	if facts.HasActiveArenaContext() {
		origin = StorageOriginArena
	}
	return AllocationContext{Available: facts.HasActiveArenaContext(), Origin: origin, Profile: facts.Profile}
}

// storeStringConcatPlan records a maximal plan and classifies it: a plan whose
// segments are all constant text is compile-time folded static data; any other
// plan is one runtime materialization in the active context.
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

// reportStringMaterializations publishes, once every maximal plan is final, the
// allocation effect of each runtime string materialization and its source-selected
// failure mode: a protected plan reports StringError and an ordinary one panics. A
// plan with an unhandled Result operand or an interpolation still requires try,
// and every mode requires a valid allocation context.
//
// Rules:
//   - rules/foundations/operators.md — "Runtime allocation and failure policy" (MD-039)
//   - rules/corrections/applied/missing-decisions-md001-md009-correction-20261003.md — §§ 5.5–5.6, 5.18–5.19
//   - rules/memory/allocation.md — § 10 "Allocation failure", § 22(4), § 24 "Allocation effects"
func (a *Analyzer) reportStringMaterializations() {
	roots := make([]ast.Expression, 0, len(a.stringMaterializationSites))
	for root := range a.stringMaterializationSites {
		if a.runtimeStringMaterialization(root) {
			roots = append(roots, root)
		}
	}
	sortExpressionsBySite(roots, a.stringMaterializationSites)
	for _, root := range roots {
		site := a.stringMaterializationSites[root]
		plan := a.stringConcatPlans[root]
		plan.Failure = a.reportStringMaterialization(site, plan, a.protectedStringMaterializations[root])
		a.stringConcatPlans[root] = plan
		if plan.Failure != "" || a.protectedStringMaterializations[root] || !a.stringPlanRequiresTry(plan) {
			continue
		}
		message := "runtime string interpolation may fail to allocate and requires try"
		if !plan.Interpolated {
			message = "this concatenation has an unhandled Result[string, StringError] operand and requires try; a Result is never unwrapped implicitly"
		}
		a.addErrorAtTokenWithMetadata(site.token, diagnostics.StringMaterializationRequiresTry,
			"Write `try` before the expression to propagate StringError, or add a handler such as `try ... { Err(_) => fallback }`.",
			"%s", message)
	}
	values := make([]ast.Expression, 0, len(a.stringAppendSites))
	for value := range a.stringAppendSites {
		values = append(values, value)
	}
	sortExpressionsBySite(values, a.stringAppendSites)
	for _, value := range values {
		resolved := a.stringAppends[value]
		resolved.Plan.Failure = a.reportStringMaterialization(a.stringAppendSites[value], resolved.Plan, a.protectedStringAppends[value])
		a.stringAppends[value] = resolved
	}
}

// stringPlanRequiresTry reports a plan that has no ordinary panic mode.
func (a *Analyzer) stringPlanRequiresTry(plan StringConcatPlan) bool {
	if plan.Interpolated {
		return true
	}
	for _, segment := range plan.Segments {
		if segment.Kind == StringConcatFallibleText {
			return true
		}
	}
	return false
}

// reportStringMaterialization publishes one site's effects and returns its mode,
// or "" when a required try is missing. A missing context is reported here, so the
// returned mode only stops a second diagnostic for that plan.
func (a *Analyzer) reportStringMaterialization(site stringMaterializationSite, plan StringConcatPlan, protected bool) StringMaterializationFailureMode {
	if site.reachable {
		a.callGraph.addArenaEffect(site.callable, ArenaEffectSite{Kind: ArenaEffectAllocate, Source: site.token, MayAllocate: true})
	}
	if !plan.Allocation.Context.Available && a.stringSiteDemanded(site) {
		if !a.AllocationCapabilities().ProfileKnown {
			profile := plan.Allocation.Context.Profile
			if profile == "" {
				profile = "unresolved"
			}
			a.addErrorAtTokenWithMetadata(site.token, diagnostics.StringMaterializationWithoutAllocationContext,
				"Select a resolved allocation profile with a valid Arena context, or use compile-time text so no runtime allocation is required.",
				"runtime string materialization needs an allocation context, but allocation capabilities for target profile %q are unresolved", profile)
			return StringMaterializationTry
		}
		a.addErrorAtTokenWithMetadata(site.token, diagnostics.StringMaterializationWithoutAllocationContext,
			"Build the text from compile-time constants so it folds to static data, or select a target profile that provides an allocation context.",
			"runtime string materialization needs an allocation context, but target profile %q provides none", plan.Allocation.Context.Profile)
		return StringMaterializationTry
	}
	if protected {
		return StringMaterializationTry
	}
	if a.stringPlanRequiresTry(plan) {
		return ""
	}
	if site.reachable {
		a.callGraph.addEffect(site.callable, EffectSite{Kind: EffectMayPanicStringMaterialization, Source: site.token, PanicReasonIDs: []diagnostics.PanicReasonID{diagnostics.PanicReasonStringMaterializationFailure}})
	}
	return StringMaterializationPanic
}

// stringSiteDemanded excludes a core body no program module reaches: core is
// shared by every target, and a text helper that a no-allocation build never calls
// must not fail that build. Its allocation and panic effects stay published for
// the callers that do reach it (rules/memory/allocation.md § 17(9)).
func (a *Analyzer) stringSiteDemanded(site stringMaterializationSite) bool {
	seen := map[CallableID]bool{site.callable: true}
	pending := []CallableID{site.callable}
	for len(pending) > 0 {
		id := pending[0]
		pending = pending[1:]
		node, known := a.callGraph.Node(id)
		if !known || node.Module != "core" {
			return true
		}
		for _, incoming := range a.callGraph.Incoming(id) {
			if !seen[incoming.Caller] {
				seen[incoming.Caller] = true
				pending = append(pending, incoming.Caller)
			}
		}
	}
	return false
}

func sortExpressionsBySite(roots []ast.Expression, sites map[ast.Expression]stringMaterializationSite) {
	sort.SliceStable(roots, func(left, right int) bool {
		l, r := sites[roots[left]].token, sites[roots[right]].token
		if l.File != r.File {
			return l.File < r.File
		}
		if l.Line != r.Line {
			return l.Line < r.Line
		}
		return l.Column < r.Column
	})
}
