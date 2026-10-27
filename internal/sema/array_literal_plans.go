package sema

import (
	"math/big"

	"sec/internal/ast"
)

// resolveOrdinaryArrayElement resolves an owning element exactly once and
// publishes the selected transfer rather than treating every value as fresh.
// Spread remains copying in resolveArrayLiteralPlan, never implicit move-out.
// Successful element transfers commit before evaluating the next element;
// a later failure must clean the prepared components (destruction.md §14.2).
// Rules: rules/collections/collections.md — § 5.5–5.6;
// rules/mlir/packages/sec-mlir-dialect_package17.md — §§ 15, 17–18, 65–67.
func (a *Analyzer) resolveOrdinaryArrayElement(element ast.Expression, expected Type) (Type, ResolvedArrayTransferAction) {
	a.constructionOwnershipDepth++
	defer func() { a.constructionOwnershipDepth-- }()
	var typ Type
	if expected.Kind != InvalidType && expected.Kind != "" {
		typ, _ = a.inferExpressionWithExpected(element, expected)
	} else {
		typ, _ = a.inferExpression(element)
	}
	if typ.Kind == InvalidType {
		return typ, ""
	}
	if prefix, move := element.(*ast.PrefixExpression); move && prefix.Operator == "<-" {
		a.markExplicitMoveSource(prefix.Right)
		return typ, ArrayTransferMove
	}
	if _, reusable := a.resolvePlace(element); reusable {
		mode := ast.OwnershipCopy
		if a.terminalReturnDepth > 0 && requiresOwnershipTransfer(typ) {
			mode = ast.OwnershipMove
		}
		if !a.validateNamedOwnershipSource(mode, element, expressionToken(element), false, false, TransferSiteConstruction) {
			return Type{Kind: InvalidType}, ""
		}
		if mode == ast.OwnershipMove {
			a.markMoveSource(element)
			return typ, ArrayTransferMove
		}
		switch CopyClassificationOf(typ) {
		case CopyTrivial:
			// Preserve P14's ordinary trivial-element encoding. The separate
			// transfer plan retains the reusable source and copy provenance.
			return typ, ArrayTransferConstructDirect
		case CopySemantic:
			return typ, ArrayTransferCopySemantic
		}
	}
	return typ, ArrayTransferConstructDirect
}

// resolveArrayLiteralPlan is the authoritative source-order literal analysis
// required by SEC-MLIR Package 14 sections 18-21. It evaluates each source
// expression once and retains one compact fact per element or spread.
func (a *Analyzer) resolveArrayLiteralPlan(expr *ast.ArrayLiteral, expected Type) (ResolvedArrayLiteralPlan, bool) {
	plan := ResolvedArrayLiteralPlan{ElementType: expected, Length: new(big.Int), Entries: make([]ResolvedArrayLiteralEntry, 0, len(expr.Elements))}
	for sourceIndex, element := range expr.Elements {
		if segment, ok := element.(*ast.RangeExpression); ok {
			entry, ok := a.resolveRangeSegment(segment, expected)
			if !ok {
				return ResolvedArrayLiteralPlan{}, false
			}
			entry.SourceIndex = sourceIndex
			plan.Entries = append(plan.Entries, entry)
			plan.Length.Add(plan.Length, entry.Length)
			continue
		}
		if spread, ok := element.(*ast.SpreadExpression); ok {
			sourceType, _ := a.inferExpression(spread.Value)
			if sourceType.Kind == InvalidType {
				return ResolvedArrayLiteralPlan{}, false
			}
			sourceDisplay := typeDisplayName(sourceType)
			concreteSource := dereferenceType(sourceType)
			sourceLength, fixedSource := exactFixedArrayLength(concreteSource)
			if concreteSource.Kind != ArrayType || concreteSource.Element == nil || !fixedSource {
				a.addErrorAtToken(spread.Token, "cannot spread %s into fixed-array literal; expansion count is not known at compile time", sourceDisplay)
				return ResolvedArrayLiteralPlan{}, false
			}
			action, supported := arraySpreadTransferAction(CopyClassificationOf(*concreteSource.Element))
			if !supported {
				a.addErrorAtToken(spread.Token, "cannot spread %s into fixed-array literal; indexed expansion would require unsupported consuming element reads", sourceDisplay)
				return ResolvedArrayLiteralPlan{}, false
			}
			plan.Entries = append(plan.Entries, ResolvedArrayLiteralEntry{
				SourceIndex: sourceIndex,
				Kind:        ArrayLiteralSpread,
				Type:        concreteSource,
				Length:      new(big.Int).Set(sourceLength),
				Action:      action,
			})
			plan.Length.Add(plan.Length, sourceLength)
			continue
		}
		elementType, action := a.resolveOrdinaryArrayElement(element, expected)
		if elementType.Kind == InvalidType {
			return ResolvedArrayLiteralPlan{}, false
		}
		plan.Entries = append(plan.Entries, ResolvedArrayLiteralEntry{
			SourceIndex: sourceIndex,
			Kind:        ArrayLiteralElement,
			Type:        elementType,
			Length:      big.NewInt(1),
			Action:      action,
		})
		plan.Length.Add(plan.Length, big.NewInt(1))
	}
	return plan, true
}
