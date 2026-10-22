package semantic

import (
	"sec/internal/ast"
	"sec/internal/ast/walk"
	"sec/internal/sema"
)

// validateOwnershipLoweringCapabilities checks represented P17 prerequisites
// before any IR is constructed. Selecting a newer package retains predecessor
// capabilities; it does not make absent ownership/cleanup operations available.
// Only resolved facts of the requested module participate, without re-inference
// or another ownership checker. Individual producers replace these gates as
// their complete Semantic IR contracts become executable.
// Rules: rules/mlir/packages/sec-mlir-dialect_package17.md — §§4,17–18,38,79,128,135;
// rules/compiler/compiler_pipeline.md — §§32–34.
func validateOwnershipLoweringCapabilities(program *ast.Program, analyzer *sema.Analyzer, module string, maxPackage uint8) error {
	if maxPackage < 17 {
		return nil
	}
	current := ""
	for _, statement := range program.Statements {
		if declaration, ok := statement.(*ast.ModuleStatement); ok {
			current = declaration.Path
			continue
		}
		if current != module {
			continue
		}
		if err := walk.Inspect(statement, func(node any) error {
			feature := ""
			var at Location
			switch node := node.(type) {
			case *ast.LetStatement:
				if _, ok := analyzer.ResolvedOwnedCleanupInitializationOf(node); ok {
					feature = "P17 owned-value initialization and cleanup registration (P17-28/P17-33)"
					at = location(node.Token)
				}
			case *ast.AssignmentStatement:
				if _, ok := analyzer.ResolvedOwnedCleanupAssignmentOf(node); ok {
					feature = "P17 owned-value replacement/reinitialization (P17-28)"
					at = location(node.Token)
				}
			case *ast.DeferStatement:
				if _, ok := analyzer.ResolvedDeferPlanOf(node); ok {
					feature = "P17 deferred cleanup registration and execution (P17-33)"
					at = location(node.Token)
				}
			}
			if feature != "" {
				return &UnsupportedFeatureError{Feature: feature, Package: maxPackage, Location: at}
			}
			if expression, ok := node.(ast.Expression); ok {
				if plan, ok := analyzer.ResolvedTransferPlanOf(expression); ok &&
					(plan.Action == sema.TransferCopySemantic || !sema.TriviallyDestructible(plan.Source.Type)) {
					return &UnsupportedFeatureError{Feature: "P17 non-trivial ownership transfer (P17-26/P17-27)", Package: maxPackage, Location: location(expressionToken(expression))}
				}
				at := expressionToken(expression)
				if len(analyzer.ResolvedOwnedCleanupCancellationsAt(at.File, at.Line, at.Column)) != 0 {
					return &UnsupportedFeatureError{Feature: "P17 ownership termination and cleanup cancellation (P17-29/P17-33)", Package: maxPackage, Location: location(at)}
				}
			}
			return nil
		}); err != nil {
			return err
		}
	}
	return nil
}
