package sema

import (
	"sec/internal/ast"
	"sec/internal/diagnostics"
	"sec/internal/lexer"
)

// inferCompilerKnownFill resolves contextual fixed-array, owning-array and
// string construction. Array elements must support ordinary infallible copying,
// independently of the destination extent or requested runtime count.
// Rules: rules/compiler/compiler_known_members.md — "Contextual fill";
// rules/collections/collections.md — §§5,6,7.9;
// rules/memory/copy_move.md — §§3.1–3.4 copy classification.
func (a *Analyzer) inferCompilerKnownFill(expr *ast.CallExpression, expected Type, hasExpected bool) (Type, expressionValue, bool) {
	result := expressionValue{Display: expr.String()}
	if len(expr.GenericArguments) > 0 {
		a.addErrorAtToken(expr.Token, "fill uses target context and does not accept generic arguments")
		return Type{Kind: InvalidType}, result, true
	}
	if !hasExpected || expected.Kind == InvalidType || expected.Kind == "" {
		a.addErrorAtToken(expr.Token, "fill requires an explicit array or string target type")
		return Type{Kind: InvalidType}, result, true
	}
	if expected.Kind == ArrayType && expected.Element != nil {
		if arrayShapeOf(expected) == ArrayShapeFixed {
			if !a.checkCompilerKnownCallArity(expr, "fill", 1, 1) {
				return Type{Kind: InvalidType}, result, true
			}
			valueType, _ := a.inferExpressionWithExpected(expr.Arguments[0], *expected.Element)
			if !a.canInitialize(*expected.Element, valueType, expr.Arguments[0]) {
				a.addErrorAtToken(expressionToken(expr.Arguments[0]), "fill value must be %s, got %s", typeDisplayName(*expected.Element), typeDisplayName(valueType))
				return Type{Kind: InvalidType}, result, true
			}
			if a.rejectNonCopyableFillElement(*expected.Element, expr.Token, "fill") {
				return Type{Kind: InvalidType}, result, true
			}
			if !a.summaryPass {
				if a.fixedArrayFillCalls == nil {
					a.fixedArrayFillCalls = map[*ast.CallExpression]bool{}
				}
				a.fixedArrayFillCalls[expr] = true
			}
			return expected, result, true
		}
		if !a.checkCompilerKnownCallArity(expr, "fill", 2, 2) {
			return Type{Kind: InvalidType}, result, true
		}
		valueType, _ := a.inferExpressionWithExpected(expr.Arguments[0], *expected.Element)
		if !a.canInitialize(*expected.Element, valueType, expr.Arguments[0]) {
			a.addErrorAtToken(expressionToken(expr.Arguments[0]), "fill value must be %s, got %s", typeDisplayName(*expected.Element), typeDisplayName(valueType))
			return Type{Kind: InvalidType}, result, true
		}
		if a.rejectNonCopyableFillElement(*expected.Element, expr.Token, "fill") {
			return Type{Kind: InvalidType}, result, true
		}
		if !a.compilerKnownCountArgument(expr.Arguments[1], "fill") {
			return Type{Kind: InvalidType}, result, true
		}
		resultType := compilerKnownResult(expected, a.types["CollectionError"])
		a.recordDynamicArrayFill(expr, expected, resultType)
		return resultType, result, true
	}
	if expected.Kind == StringType {
		if !a.checkCompilerKnownCallArity(expr, "fill", 2, 2) {
			return Type{Kind: InvalidType}, result, true
		}
		fragmentType, _ := a.inferExpressionWithExpected(expr.Arguments[0], expected)
		if fragmentType.Kind != StringType && fragmentType.Kind != CharType && fragmentType.Kind != RuneType {
			a.addErrorAtToken(expressionToken(expr.Arguments[0]), "string fill fragment must be string, char, or rune, got %s", typeDisplayName(fragmentType))
			return Type{Kind: InvalidType}, result, true
		}
		if !a.compilerKnownCountArgument(expr.Arguments[1], "fill") {
			return Type{Kind: InvalidType}, result, true
		}
		return compilerKnownResult(expected, a.types["CollectionError"]), result, true
	}
	a.addErrorAtToken(expr.Token, "fill target must be a fixed array, owning dynamic array, or string, got %s", typeDisplayName(expected))
	return Type{Kind: InvalidType}, result, true
}

// rejectNonCopyableFillElement enforces repeated-copy availability for both
// contextual construction and mutable-slice replacement.
// Rules: rules/compiler/compiler_known_members.md — "Contextual fill";
// rules/collections/collections.md — §7.9 "Slice Fill";
// rules/memory/copy_move.md — §§3.1–3.4 copy classification.
func (a *Analyzer) rejectNonCopyableFillElement(element Type, token lexer.Token, operation string) bool {
	classification := CopyClassificationOf(element)
	if classification == CopyTrivial || classification == CopySemantic {
		return false
	}
	a.addErrorAtTokenWithMetadata(token, diagnostics.FillElementNotCopyable,
		"fill evaluates its source once and copies it into each element. Construct move-only elements individually; a zero extent or count does not make this copy operation available.",
		"%s requires a copyable element type, got %s", operation, typeDisplayName(element))
	return true
}

// IsFixedArrayFillCall reports a resolved contextual fixed-array fill(value):
// the call's type is the target fixed array and its one argument the element
// value copied into every element.
// Rules: rules/compiler/compiler_known_members.md — "Contextual fill".
func (a *Analyzer) IsFixedArrayFillCall(call *ast.CallExpression) bool {
	return a != nil && a.fixedArrayFillCalls[call]
}

// AllocationDynamicArrayFill constructs `try fill(value, count)` into an
// owning dynamic array: a fresh owner grown through the active allocation
// context.
const AllocationDynamicArrayFill AllocationOperationKind = "dynamic-array-fill"

// ResolvedDynamicArrayFillPlan is the read-only plan of one contextual
// `try fill(value, count)` whose target is an owning dynamic array. Owner is
// the compiler-internal temporary owner the copies are appended to, with its
// own storage and backing domain; Allocation is its growth plan. The
// temporary never escapes: success moves it into Ok, failure destroys the
// initialized prefix and ends/reclaims its backing before
// Err(CollectionError.AllocationFailed).
type ResolvedDynamicArrayFillPlan struct {
	Owner      DynamicArrayOwner
	Allocation ResolvedAllocationOperationPlan
	ResultType Type
}

// recordDynamicArrayFill publishes the plan of a resolved dynamic-array fill.
// Rules: rules/compiler/compiler_known_members.md — § 50(2);
// rules/collections/collections.md — § 17;
// rules/mlir/packages/sec-mlir-dialect_package18.md — §§ 20-37.
func (a *Analyzer) recordDynamicArrayFill(call *ast.CallExpression, target, resultType Type) {
	if a.summaryPass || resultType.Kind != ResultType {
		return
	}
	array, dynamic := ResolveDynamicArrayPlan(target)
	if !dynamic {
		return
	}
	site := call.Token
	key := "dynamic-array-fill:" + referenceSiteKey(site)
	owner := DynamicArrayOwner{
		Place: Place{
			Root: "fill.result", RootToken: site, RootID: a.placeRootID(site), RootKind: PlaceRootLocal,
			StorageIdentity: a.placeIdentities.storageIdentity(key), StorageOrigin: StorageOriginAutomatic,
			AddressSpace: AddressSpaceOrdinary, Type: array.ArrayType, Mutable: true, Addressable: true,
		},
		OwnerOrigin: StorageOriginAutomatic,
	}
	owner.OwnerStorage = owner.Place.StorageIdentity
	owner.BackingDomain = a.placeIdentities.invalidationDomain("dynamic-array-backing:" + key)
	context := a.activeAllocationContext()
	plan := ResolvedAllocationOperationPlan{
		Operation: AllocationDynamicArrayFill, ArrayType: array.ArrayType, ElementType: array.ElementType,
		MayAllocate: true, Context: context, StorageOrigin: context.Origin,
		Fallible: true, FailureType: builtinType("CollectionError"), AllocationFailureType: builtinType("AllocationError"),
		Source: site, Receiver: &owner,
	}
	a.resolveAllocationIdentities(&plan, call)
	if a.allocationOperationPlans == nil {
		a.allocationOperationPlans = map[*ast.CallExpression]ResolvedAllocationOperationPlan{}
	}
	a.allocationOperationPlans[call] = plan
	if a.dynamicArrayFills == nil {
		a.dynamicArrayFills = map[*ast.CallExpression]ResolvedDynamicArrayFillPlan{}
	}
	a.dynamicArrayFills[call] = ResolvedDynamicArrayFillPlan{Owner: owner, Allocation: plan, ResultType: semanticSnapshotType(resultType)}
}

// ResolvedDynamicArrayFillOf reads the plan of a dynamic-array fill call; it
// allocates and infers nothing. The result is detached.
// Rules: rules/compiler/compiler_known_members.md — § 50(2).
func (a *Analyzer) ResolvedDynamicArrayFillOf(call *ast.CallExpression) (ResolvedDynamicArrayFillPlan, bool) {
	if a == nil || call == nil {
		return ResolvedDynamicArrayFillPlan{}, false
	}
	plan, ok := a.dynamicArrayFills[call]
	if !ok {
		return ResolvedDynamicArrayFillPlan{}, false
	}
	plan.Owner.Place = clonePlace(plan.Owner.Place)
	plan.Allocation.Receiver = &plan.Owner
	return plan, true
}
