package semantic

import (
	"fmt"

	"sec/internal/ast"
	"sec/internal/ast/walk"
	"sec/internal/sema"
)

const sliceLoweringPackage = 16

// sliceLoweringPlans owns detached analysis-time decisions for the requested
// module. Source producers consume these snapshots instead of re-inferring
// bounds, ownership, origins or effects from syntax or target representation.
// Rules: rules/mlir/packages/sec-mlir-dialect_package16.md — §§33–37,51,115–116,128–129;
// rules/compiler/compiler_pipeline.md — §§32–34.
type sliceLoweringPlans struct {
	creations map[*ast.RefExpression]sema.ResolvedSliceCreationPlan
	indexes   map[*ast.IndexExpression]sema.ResolvedSliceIndexPlan
	lengths   map[ast.Expression]sema.ResolvedSliceLengthPlan
}

// prepareSliceLowering snapshots the requested module's creation/index/use
// facts before building any function. A missing semantic decision is an error,
// not permission to invent a proof or to fall back to P15 array/reference paths.
// Earlier package boundaries do not perform this P16 preflight.
// Rules: rules/mlir/packages/sec-mlir-dialect_package16.md — §§33–37,51,115–116,128–129;
// rules/compiler/semantic_ir.md — "Unsupported lowerings".
func (b *builder) prepareSliceLowering(program *ast.Program) error {
	if b.maxPackage < sliceLoweringPackage {
		return nil
	}
	plans := &sliceLoweringPlans{creations: map[*ast.RefExpression]sema.ResolvedSliceCreationPlan{}, indexes: map[*ast.IndexExpression]sema.ResolvedSliceIndexPlan{}, lengths: map[ast.Expression]sema.ResolvedSliceLengthPlan{}}
	current := ""
	for _, statement := range program.Statements {
		if module, ok := statement.(*ast.ModuleStatement); ok {
			current = module.Path
			continue
		}
		if current != b.module.Identity {
			continue
		}
		if err := walk.Inspect(statement, func(node any) error {
			if expr, expression := node.(ast.Expression); expression {
				if feature, deferred := b.deferredSliceCapability(expr); deferred {
					return &UnsupportedFeatureError{Feature: feature, Package: b.maxPackage, Location: location(expressionToken(expr))}
				}
				if plan, ok := b.analyzer.ResolvedSliceLengthPlanOf(expr); ok {
					if !sourceSliceType(plan.SourceType) || plan.ResultType.Kind != sema.UintType || plan.ResultType.Name != "uint" || (plan.CompilerKnownID != "CKM-LEN-SLICE" && plan.CompilerKnownID != "CKF-LEN") {
						return fmt.Errorf("incomplete resolved slice length plan at %s", formatLocation(location(expressionToken(expr))))
					}
					plans.lengths[expr] = plan
				}
			}
			switch expr := node.(type) {
			case *ast.RefExpression:
				if _, sliced := expr.Value.(*ast.SliceExpression); !sliced {
					return nil
				}
				plan, ok := b.analyzer.ResolvedSliceCreationPlanOf(expr)
				if !ok {
					return fmt.Errorf("missing resolved slice creation plan at %s", formatLocation(location(expr.Token)))
				}
				if plan.BorrowID == 0 || plan.LifetimeID == 0 || plan.BorrowID != plan.ReferenceFacts.Facts.Borrow || plan.LifetimeID != plan.ReferenceFacts.Facts.Lifetime || plan.CheckKind == "" || plan.CheckKind == sema.SliceRangeProvenSafe && plan.ProofKind == "" || plan.ElementType.Kind == sema.InvalidType || plan.ElementType.Kind == "" {
					return fmt.Errorf("incomplete resolved slice creation facts at %s", formatLocation(location(expr.Token)))
				}
				if !plan.HasValidEvaluationOrder() {
					return fmt.Errorf("invalid resolved slice operand evaluation order at %s", formatLocation(location(expr.Token)))
				}
				if !plan.ReferenceFacts.Facts.Checkable() {
					return &UnsupportedFeatureError{Feature: "uncheckable slice reference validity", Package: b.maxPackage, Location: location(expr.Token)}
				}
				plans.creations[expr] = plan
			case *ast.IndexExpression:
				plan, ok := b.analyzer.ResolvedSliceIndexPlanOf(expr)
				if !ok {
					source, resolved := b.analyzer.ResolvedTypeOf(expr.Left)
					if resolved && sourceSliceType(source) {
						return fmt.Errorf("missing resolved slice index plan at %s", formatLocation(location(expr.Token)))
					}
					return nil
				}
				if plan.UseKind == "" || plan.Action == "" || plan.CheckKind == "" || plan.IndexType.Kind == "" || plan.IndexType.Kind == sema.InvalidType || plan.ElementType.Kind == "" || plan.ElementType.Kind == sema.InvalidType {
					return fmt.Errorf("incomplete resolved slice index/use facts at %s", formatLocation(location(expr.Token)))
				}
				plans.indexes[expr] = plan
			}
			return nil
		}); err != nil {
			return err
		}
	}
	b.slicePlans = plans
	return nil
}

// sourceSliceType recognizes the resolved bounded-reference category without
// treating an owning dynamic array or direct reference to an array as a slice.
// Rules: rules/mlir/packages/sec-mlir-dialect_package16.md — §§31–32,75–76.
func sourceSliceType(t sema.Type) bool {
	return t.Kind == sema.SliceType || t.Kind == sema.ReferenceType && t.Element != nil && t.Element.Kind == sema.SliceType
}

// p16SliceExpression selects the P16 source route before P15 reference/array
// lowering. The operation producers remain explicit capability gates until
// their checklist items are implemented; a failure never yields a module.
// Rules: rules/mlir/packages/sec-mlir-dialect_package16.md — §§33–37,51,115–116,123–124,128–137;
// rules/compiler/compiler_pipeline.md — §§32–34.
func (fb *functionBuilder) p16SliceExpression(expr ast.Expression) (builtValue, bool, error) {
	if fb.owner.maxPackage < sliceLoweringPackage {
		return builtValue{}, false, nil
	}
	if fb.owner.slicePlans == nil {
		return builtValue{}, true, fmt.Errorf("missing package-16 slice lowering preflight")
	}
	if plan, ok := fb.owner.slicePlans.lengths[expr]; ok {
		value, err := fb.lowerSliceLength(expr, plan)
		return value, true, err
	}
	switch expr := expr.(type) {
	case *ast.TryExpression:
		if resolved, ok := fb.owner.analyzer.ResolvedTryOf(expr); ok && sourceSliceType(resolved.SuccessType) {
			value, err := fb.buildTryExpression(expr)
			return value, true, err
		}
	case *ast.RefExpression:
		if index, ok := expr.Value.(*ast.IndexExpression); ok {
			if plan, ok := fb.owner.slicePlans.indexes[index]; ok {
				value, err := fb.lowerSliceElementBorrow(expr, index, plan)
				return value, true, err
			}
		}
		if _, sliced := expr.Value.(*ast.SliceExpression); !sliced {
			break
		}
		plan, ok := fb.owner.slicePlans.creations[expr]
		if !ok {
			return builtValue{}, true, fmt.Errorf("missing prepared slice creation plan at %s", formatLocation(location(expr.Token)))
		}
		feature := "slice source kind without supported lowering"
		switch plan.SourceKind {
		case sema.SliceSourceFixedArray, sema.SliceSourceSlice:
			value, err := fb.lowerSliceCreation(expr, plan)
			return value, true, err
		case sema.SliceSourceDynamicArray:
			feature = "owning dynamic-array to slice lowering (P18)"
		}
		return builtValue{}, true, fb.unsupported(feature, expr.Token)
	case *ast.IndexExpression:
		if plan, ok := fb.owner.slicePlans.indexes[expr]; ok {
			value, err := fb.lowerSliceIndexRead(expr, plan)
			return value, true, err
		}
		if source, resolved := fb.owner.analyzer.ResolvedTypeOf(expr.Left); resolved && sourceSliceType(source) {
			return builtValue{}, true, fmt.Errorf("missing prepared slice index/use plan at %s", formatLocation(location(expr.Token)))
		}
	case *ast.SliceExpression:
		return builtValue{}, true, fb.unsupported("slice expression without a prepared explicit borrow", expr.Token)
	}
	if source, resolved := fb.owner.analyzer.ResolvedTypeOf(expr); resolved && sourceSliceType(source) {
		return builtValue{}, true, fb.unsupported("slice holder value transfer or projected origin (P16-36)", expressionToken(expr))
	}
	return builtValue{}, false, nil
}
