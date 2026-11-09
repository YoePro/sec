package sema

import (
	"fmt"

	"sec/internal/ast"
)

// ResolvedMapIndexPlan is the frontend fact of one indexed map access
// `values[key]` (collections.md §§ 14.4–14.5). A lookup finds the existing
// entry's value Place; a missing key is the ordinary runtime index failure:
// panic.bounds without try and IndexError.OutOfBounds under a valid try
// (owner decision 2026-11-08). An upsert assignment never fails on a missing
// key: it inserts, and its structural growth fails with CollectionError, so it
// needs try. Executable map operations remain in lowering.semantic-ir-v2.
//
// Rules:
//   - rules/collections/collections.md — §§ 14.4 "Indexed lookup", 14.5 "Indexed assignment", 14.7
type ResolvedMapIndexPlan struct {
	MapType     Type
	KeyType     Type
	ValueType   Type
	UseKind     ArrayIndexUseKind
	Action      ResolvedArrayTransferAction
	CheckKind   ArrayIndexCheckKind
	FailureMode ArrayIndexFailureMode
	ErrorType   Type
	// Upsert marks an indexed assignment that inserts a missing key.
	Upsert          bool
	UpsertErrorType Type
	UpsertProtected bool
}

// isCompilerKnownMapType recognizes the canonical compiler-owned map family.
func isCompilerKnownMapType(typ Type) bool {
	return typ.Name == "map" && len(typ.TypeArgs) == 2
}

// inferMapIndexExpression checks the key against K and records a runtime
// checked lookup of the V entry Place.
func (a *Analyzer) inferMapIndexExpression(expr *ast.IndexExpression, mapType Type) (Type, expressionValue) {
	keyType, valueType := mapType.TypeArgs[0], mapType.TypeArgs[1]
	indexType, _ := a.inferExpressionWithExpected(expr.Index, keyType)
	if indexType.Kind == InvalidType {
		return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}
	}
	if !a.canInitialize(keyType, indexType, expr.Index) {
		a.addErrorAtToken(expressionToken(expr.Index), "%s key must be %s, got %s", typeDisplayName(mapType), typeDisplayName(keyType), typeDisplayName(indexType))
		return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}
	}
	a.recordSequenceIndexEffect(expr)
	if !a.summaryPass {
		a.storeMapIndexPlan(expr, ResolvedMapIndexPlan{
			MapType: mapType, KeyType: keyType, ValueType: valueType,
			UseKind: ArrayIndexRead, Action: arrayIndexTransferAction(valueType, ArrayIndexRead),
			CheckKind: ArrayIndexRuntimeCheck, FailureMode: ArrayIndexFailureOrdinary, ErrorType: a.indexErrorType(),
		})
	}
	if valueType.Kind == ReferenceType {
		valueType = a.referenceTypeWithOriginFromExpression(valueType, expr.Left)
	}
	return valueType, expressionValue{Display: expr.String()}
}

func (a *Analyzer) storeMapIndexPlan(expr *ast.IndexExpression, plan ResolvedMapIndexPlan) {
	if a.resolvedMapIndexPlans == nil {
		a.resolvedMapIndexPlans = map[*ast.IndexExpression]ResolvedMapIndexPlan{}
	}
	plan.MapType = semanticSnapshotType(plan.MapType)
	plan.KeyType = semanticSnapshotType(plan.KeyType)
	plan.ValueType = semanticSnapshotType(plan.ValueType)
	plan.ErrorType = cloneSlicePlanErrorType(plan.ErrorType)
	plan.UpsertErrorType = cloneSlicePlanErrorType(plan.UpsertErrorType)
	a.resolvedMapIndexPlans[expr] = plan
}

func (a *Analyzer) setMapIndexUse(expr *ast.IndexExpression, use ArrayIndexUseKind) {
	plan, ok := a.resolvedMapIndexPlans[expr]
	if !ok {
		return
	}
	plan.UseKind = use
	plan.Action = arrayIndexTransferAction(plan.ValueType, use)
	a.storeMapIndexPlan(expr, plan)
}

func (a *Analyzer) commitFallibleMapIndex(expr *ast.IndexExpression, errorType Type) {
	plan, ok := a.resolvedMapIndexPlans[expr]
	if !ok || plan.Upsert {
		return
	}
	plan.FailureMode = ArrayIndexFailureFallible
	plan.ErrorType = errorType
	a.storeMapIndexPlan(expr, plan)
}

// recordMapUpsert turns an indexed assignment into an upsert: no missing-key
// failure (the key is inserted), and a CollectionError growth failure that
// needs try (§ 14.5, § 14.7). It reports whether the upsert is valid.
func (a *Analyzer) recordMapUpsert(stmt *ast.AssignmentStatement, index *ast.IndexExpression, allowFallible bool) bool {
	plan, ok := a.resolvedMapIndexPlans[index]
	// `=` and the move form `<-` both upsert (§ 14.5); a compound operator
	// reads the existing entry and fails like a lookup.
	if !ok || stmt.Operator != "=" && stmt.Operator != "<-" {
		return true
	}
	a.callGraph.removeEffect(a.currentCallable, EffectMayPanicBounds, expressionToken(index))
	plan.Upsert = true
	plan.CheckKind, plan.FailureMode, plan.ErrorType = "", ArrayIndexFailureNone, Type{}
	plan.UpsertErrorType = builtinType("CollectionError")
	plan.UpsertProtected = allowFallible
	a.storeMapIndexPlan(index, plan)
	if !allowFallible {
		a.addErrorAtTokenWithMetadata(stmt.Token, "",
			fmt.Sprintf("Inserting a missing key may need to grow the map, which can fail with CollectionError. Write `try %s = ...` and propagate or handle the error.", index.String()),
			"map upsert %s = ... can fail with CollectionError and requires try", index.String())
		return false
	}
	return true
}

// ResolvedMapIndexPlanOf reads the plan of an indexed map access, detached.
func (a *Analyzer) ResolvedMapIndexPlanOf(expr *ast.IndexExpression) (ResolvedMapIndexPlan, bool) {
	if a == nil || expr == nil {
		return ResolvedMapIndexPlan{}, false
	}
	plan, ok := a.resolvedMapIndexPlans[expr]
	if !ok {
		return ResolvedMapIndexPlan{}, false
	}
	plan.MapType = semanticSnapshotType(plan.MapType)
	plan.KeyType = semanticSnapshotType(plan.KeyType)
	plan.ValueType = semanticSnapshotType(plan.ValueType)
	plan.ErrorType = cloneSlicePlanErrorType(plan.ErrorType)
	plan.UpsertErrorType = cloneSlicePlanErrorType(plan.UpsertErrorType)
	return plan, true
}
