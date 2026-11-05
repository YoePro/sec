package sema

import (
	"math/big"
	"strconv"
	"unicode/utf8"

	"sec/internal/ast"
	"sec/internal/diagnostics"
)

// ResolvedStringRangePlan is a shared UTF-8 borrow in rune coordinates, not
// an array of stored runes or an owned string. Range preserves source/endpoint
// evaluation order and RangeError precedence; byte translation belongs to P23.
// Rules: rules/collections/collections.md — §7.10; rules/compiler/semantic_ir.md
// — §17(9–10); rules/corrections/applied/MD-126-borrowed-string-ranges-rune-iteration-applied-20261105.md — §§2–5.
type ResolvedStringRangePlan struct {
	Range     ResolvedSliceCreationPlan
	ViewPlace Place
}

// inferStringRange checks the type-directed implicit shared borrow, resolving
// source then endpoints once and retaining ultimate storage rather than a copy.
// Rules: rules/collections/collections.md — §§7.4–7.5,7.10;
// rules/memory/borrowing.md — §5.5; rules/types/types.md — StringView (MD-126).
func (a *Analyzer) inferStringRange(expr *ast.SliceExpression, sourceType Type) (Type, expressionValue) {
	delete(a.stringRanges, expr)
	invalid := func() (Type, expressionValue) {
		return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}
	}
	before := len(a.errors)
	for _, endpoint := range []ast.Expression{expr.Start, expr.End} {
		if endpoint == nil {
			continue
		}
		typ, _ := a.inferExpression(endpoint)
		if typ.Kind == InvalidType {
			return invalid()
		}
		if !isIntegerType(typ) {
			a.addErrorAtTokenWithMetadata(expressionToken(endpoint), diagnostics.InvalidSliceRange, "String ranges use zero-based rune indexes; supply integer endpoints.", "slice bound must be an integer, got %s", typeDisplayName(typ))
			return invalid()
		}
	}
	source, ok := a.sliceSourcePlace(expr.Left, a.expressionTypes[expr.Left])
	if !ok || !source.Addressable {
		a.addErrorAtTokenWithMetadata(expr.Token, diagnostics.InvalidRangeSource, "Store the string in a live binding before borrowing a range; a view cannot extend a temporary's lifetime.", "string range requires live source storage")
		return invalid()
	}
	if sourceType.Kind == StringViewType {
		source = a.canonicalSlicePlace(expr.Left, source)
	}
	if a.checkBorrowCreationPlace(source, false, expr.Token) {
		return invalid()
	}
	plan := ResolvedStringRangePlan{Range: ResolvedSliceCreationPlan{
		EvaluationOrder: sliceEvaluationOrder(expr.Start != nil, expr.End != nil), Mode: SliceShared,
		SourceKind: SliceSourceString, ElementType: a.types["rune"], SourcePlace: source,
		SourceSliceType: a.expressionTypes[expr.Left], Start: a.resolvedSliceEndpoint(expr.Start), End: a.resolvedSliceEndpoint(expr.End),
		EndExclusive: expr.Exclusive, CheckKind: SliceRangeRuntimeCheck, ErrorType: a.types["RangeError"],
	}}
	if sourceType.Kind == StringViewType {
		plan.Range.SourceKind = SliceSourceStringView
		plan.Range.SourceLength = staticSliceExtent(source)
	} else {
		plan.Range.SourceLength = a.knownStringRuneLength(expr.Left)
	}
	facts := SliceRangeEndpoints{Start: plan.Range.Start.Constant, End: plan.Range.End.Constant, StartOmitted: expr.Start == nil, EndOmitted: expr.End == nil, Inclusive: !expr.Exclusive, Length: plan.Range.SourceLength}
	start, end, classification := normalizeSliceRange(facts)
	if classification.Failure != SliceRangeValid {
		token := expr.Token
		if classification.Cause == SliceRangeCauseNegativeStart {
			token = expressionToken(expr.Start)
		}
		if classification.Cause == SliceRangeCauseNegativeEnd {
			token = expressionToken(expr.End)
		}
		a.addErrorAtTokenWithMetadata(token, diagnostics.InvalidSliceRange, "Use non-negative rune indexes within Len. '..' includes the upper rune; '..<' excludes it and permits an empty range at Len.", "invalid string range: RangeError.%s (%s)", classification.Failure, classification.Cause)
		return invalid()
	}
	if start != nil && end != nil {
		plan.Range.StaticRange = &StaticSliceRange{Start: start, EndExclusive: end}
		if plan.Range.SourceLength != nil {
			plan.Range.CheckKind, plan.Range.ProofKind = SliceRangeProvenSafe, "constant"
		}
	}
	if expr.Start == nil && expr.End == nil {
		plan.Range.CheckKind, plan.Range.ProofKind = SliceRangeProvenSafe, "source-open-bound"
	}
	projection := a.slicePlaceProjectionWithLength(expr, sourceType, plan.Range.SourceLength)
	plan.ViewPlace = cloneSlicePlanPlace(source)
	plan.ViewPlace.Projections = appendSliceProjection(plan.ViewPlace.Projections, projection)
	for i := range plan.ViewPlace.AlternativeOrigins {
		plan.ViewPlace.AlternativeOrigins[i].Projections = appendSliceProjection(plan.ViewPlace.AlternativeOrigins[i].Projections, projection)
	}
	result := a.types["StringView"]
	name, token, local, storage, generation := a.referenceOriginForExpression(expr.Left)
	result.ReferenceOriginName, result.ReferenceOriginToken, result.ReferenceOriginLocal = name, token, local
	result.ReferenceOriginStorage, result.ReferenceOriginGeneration = storage, generation
	result.ReferenceOriginMatchScoped = a.referenceOriginMatchScopedForExpression(expr.Left)
	result.ReferenceOriginAlternatives = a.referenceOriginAlternativesForExpression(expr.Left)
	plan.ViewPlace.Type, plan.ViewPlace.Mutable, plan.ViewPlace.PartialMoveSafe = result, false, false
	if len(a.errors) != before {
		return invalid()
	}
	if a.stringRanges == nil {
		a.stringRanges = map[*ast.SliceExpression]ResolvedStringRangePlan{}
	}
	a.stringRanges[expr] = plan
	a.recordReferencePlan(expr, plan.ViewPlace, false, true, expr.Token)
	plan.Range.ReferenceFacts = a.resolvedReferencePlans[expr]
	plan.Range.BorrowID, plan.Range.LifetimeID = plan.Range.ReferenceFacts.Facts.Borrow, plan.Range.ReferenceFacts.Facts.Lifetime
	plan.Range.DynamicRange = projection.DynamicRange
	a.stringRanges[expr] = plan
	if !a.summaryPass {
		a.callGraph.removeEffect(a.currentCallable, EffectMayPanicBounds, expr.Token)
	}
	if !a.summaryPass && a.callGraphPathReachable && plan.Range.CheckKind == SliceRangeRuntimeCheck {
		a.callGraph.addEffect(a.currentCallable, EffectSite{Kind: EffectMayPanicBounds, Source: expr.Token, PanicReasonIDs: []diagnostics.PanicReasonID{diagnostics.PanicReasonBoundsFailure}})
	}
	return result, expressionValue{Display: expr.String()}
}

// ResolvedStringRangeOf returns detached source/rune-range/provenance facts.
// It never evaluates operands, allocates a borrow or turns unknown bounds safe.
// Rules: rules/compiler/semantic_ir.md — §17(9–10); rules/collections/collections.md — §7.10.
func (a *Analyzer) ResolvedStringRangeOf(expr *ast.SliceExpression) (ResolvedStringRangePlan, bool) {
	if a == nil {
		return ResolvedStringRangePlan{}, false
	}
	plan, ok := a.stringRanges[expr]
	if !ok {
		return ResolvedStringRangePlan{}, false
	}
	plan.Range = cloneSliceCreationPlan(plan.Range)
	plan.ViewPlace = cloneSlicePlanPlace(plan.ViewPlace)
	return plan, true
}

// knownStringRuneLength retains only constant scalar counts, never byte counts
// or a guess about mutable text. All other lengths need the runtime range guard.
// Rules: rules/collections/collections.md — §7.10; rules/types/types.md — string.
func (a *Analyzer) knownStringRuneLength(expr ast.Expression) *big.Int {
	if id, ok := expr.(*ast.Identifier); ok {
		if symbol, found := a.symbols[id.Value]; found {
			if symbol.Mutable {
				return nil
			}
			return clonePlaceConstantIndex(symbol.stringRuneLength)
		}
	}
	if value, outcome := a.semanticCompileTimeConstant(expr); outcome == compileTimeEvaluated && value.Kind == StringType {
		return big.NewInt(int64(utf8.RuneCountInString(value.String)))
	}
	return nil
}

// recordStringBindingLength snapshots an accepted immutable string initializer
// in its ordinary symbol scope so shadowing/branch restoration keep proofs local.
// Rules: rules/collections/collections.md — §7.10; rules/types/types.md — string.
func (a *Analyzer) recordStringBindingLength(stmt *ast.LetStatement, errorsBefore int) {
	if stmt == nil || stmt.Name == nil || stmt.Mutable || stmt.Value == nil || len(a.errors) != errorsBefore {
		return
	}
	symbol, exists := a.symbols[stmt.Name.Value]
	if !exists || symbol.Type.Kind != StringType {
		return
	}
	symbol.stringRuneLength = a.knownStringRuneLength(stmt.Value)
	a.symbols[stmt.Name.Value] = symbol
}

// bindStringViewBorrow gives every copied/returned view its own shared holder
// edge. Ending one view cannot release the dependency held by a surviving copy.
// Rules: rules/memory/borrowing.md — §5.5; MD-126 — §4(2,5–7).
func (a *Analyzer) bindStringViewBorrow(holder string, expr ast.Expression) bool {
	if holder == "" || expr == nil || a.expressionTypes[expr].Kind != StringViewType {
		return false
	}
	if origin, ok := a.directReferenceOrigin(expr); ok {
		a.registerBorrowFromReturnedOrigin(holder, origin, expressionToken(expr))
	}
	return true
}

// borrowStringViewIterable retains the one-time borrowed iterable throughout
// the loop, including ref-mut calls in the body, without borrowing each rune.
// Rules: rules/control-flow/flowcontrol_for.md — §§8,12,19; MD-126 — §5.
func (a *Analyzer) borrowStringViewIterable(stmt *ast.ForStatement) func() {
	if stmt.Iterable == nil || dereferenceType(a.expressionTypes[stmt.Iterable]).Kind != StringViewType {
		return func() {}
	}
	holder := "$string-view-loop:" + stmt.Token.File + ":" + strconv.Itoa(stmt.Token.Line) + ":" + strconv.Itoa(stmt.Token.Column)
	if origin, ok := a.directReferenceOrigin(stmt.Iterable); ok {
		a.registerBorrowFromReturnedOrigin(holder, origin, stmt.Token)
	}
	return func() { a.endBorrowsHeldBy(holder) }
}
