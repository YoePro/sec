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
		// A discarded whole binding cancels through discardOwnedBinding.
		discarded := map[ast.Expression]bool{}
		if err := walk.Inspect(statement, func(node any) error {
			feature := ""
			var at Location
			switch node := node.(type) {
			case *ast.DiscardStatement:
				if node.Name != nil {
					discarded[node.Name] = true
				}
				if identifier, ok := node.Value.(*ast.Identifier); ok {
					discarded[identifier] = true
				}
			case *ast.CallExpression:
				// A pack forwarded as an argument moves element by element in
				// buildPackForward, which names its own boundary for elements
				// that need ownership.
				for _, argument := range node.Arguments {
					// `<-values...` parses as a move of the spread or a spread
					// of the move; both forward the pack.
					var source ast.Expression = argument
					if move, ok := source.(*ast.PrefixExpression); ok && move.Operator == "<-" {
						if spread, isSpread := move.Right.(*ast.SpreadExpression); isSpread {
							discarded[move], discarded[spread] = true, true
							source = spread.Value
						}
					}
					if spread, ok := source.(*ast.SpreadExpression); ok {
						discarded[spread] = true
						source = spread.Value
						if move, ok := source.(*ast.PrefixExpression); ok && move.Operator == "<-" {
							discarded[move] = true
							source = move.Right
						}
					}
					if _, forwarded := argument.(*ast.SpreadExpression); forwarded || source != argument {
						discarded[source] = true
					}
				}
				// Result.Ok()/Err() consume their receiver through buildTryOperand.
				if member, ok := node.Callee.(*ast.MemberExpression); ok && member.Property != nil {
					site := member.Property.Token
					if known, ok := analyzer.CompilerKnownMemberAt(site.File, site.Line, site.Column); ok && (known.ID == "CKM-RESULT-OK" || known.ID == "CKM-RESULT-ERR") {
						discarded[member.Property] = true
						if root, chain := memberRoot(member.Object); root != nil {
							discarded[root] = true
							for _, step := range chain {
								discarded[step] = true
								if step.Property != nil {
									discarded[step.Property] = true
								}
							}
						}
					}
				}
			case *ast.TryExpression:
				// A tried owner cancels through buildTryOperand; a tried field
				// leaves its owner registered.
				if root, chain := memberRoot(node.Expression); root != nil {
					discarded[root] = true
					for _, step := range chain {
						discarded[step] = true
						if step.Property != nil {
							discarded[step.Property] = true
						}
					}
				}
			case *ast.MemberExpression:
				// A moved stored field keeps its owner registered (moveOwnedField).
				if plan, planned := analyzer.ResolvedTransferPlanOf(node); planned && fieldMoveLowerable(plan) {
					discarded[node] = true
					if node.Property != nil {
						discarded[node.Property] = true
					}
				}
			case *ast.IfStatement:
				if node.OptionBinding != nil && node.OptionBinding.Binding != nil {
					discarded[node.OptionBinding.Binding] = true
				}
			case *ast.MatchExpression:
				// A moved payload binding cancels through prepareOwnedMatchPayload.
				for _, arm := range node.Arms {
					if identifier := matchArmBindingIdentifier(arm.Pattern.Expression()); identifier != nil {
						discarded[identifier] = true
					}
				}
			case *ast.LetStatement:
				// Root-scope immutable owned locals lower through
				// cleanup.track-owned (P17-35); the builder rejects the
				// remaining forms at their own sites.
				// Owned locals, mutable ones in address-taken storage, lower
				// through cleanup.track-owned (P17-35, P17-28).
			case *ast.AssignmentStatement:
				// Whole-local replacement/reinitialization lowers through
				// own.replace-place / own.initialize-place (P17-28); the
				// builder rejects conditional and partial previous values.
			case *ast.ExpressionStatement:
				_, call := node.Expression.(*ast.CallExpression)
				if typ, ok := analyzer.ResolvedTypeOf(node.Expression); ok && !call && typ.Kind == sema.ArrayType && !sema.TriviallyDestructible(typ) {
					if _, fixed := sema.FixedArrayLength(typ); fixed {
						feature = "P17 owned fixed-array temporary cleanup (P17-40)"
						at = location(node.Token)
					}
				}
			}
			if feature != "" {
				return &UnsupportedFeatureError{Feature: feature, Package: maxPackage, Location: at}
			}
			if expression, ok := node.(ast.Expression); ok {
				plan, planned := analyzer.ResolvedTransferPlanOf(expression)
				// A whole owned local moved out by a return or into a new
				// local lowers to own.move + cleanup.cancel (P17-35).
				wholeLocalMove := planned && (ownedLocalMoveLowerable(plan) || fieldMoveLowerable(plan))
				if planned && !wholeLocalMove &&
					(plan.Action == sema.TransferCopySemantic || !sema.TriviallyDestructible(plan.Source.Type)) {
					return &UnsupportedFeatureError{Feature: "P17 non-trivial ownership transfer (P17-26/P17-27)", Package: maxPackage, Location: location(expressionToken(expression))}
				}
				at := expressionToken(expression)
				if !wholeLocalMove && !discarded[expression] && len(analyzer.ResolvedOwnedCleanupCancellationsAt(at.File, at.Line, at.Column)) != 0 {
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

// ownedLocalMoveLowerable reports a transfer the P17 producers lower: a move
// of a whole local binding into a return, local, call or owning construction.
func ownedLocalMoveLowerable(plan sema.ResolvedTransferPlan) bool {
	return plan.Action == sema.TransferMove && len(plan.Source.Projections) == 0 && !plan.Partial &&
		(plan.Site == sema.TransferSiteReturn || plan.Site == sema.TransferSiteInitialization || plan.Site == sema.TransferSiteArgument || plan.Site == sema.TransferSiteConstruction)
}

// ArrayConstructionNeedsOwnership exposes the canonical type-table boundary
// to schema emitters: ordinary P14 array actions cannot erase P17 ownership.
// It reads verified type facts; it does not infer source copy semantics.
// Rules: rules/mlir/packages/sec-mlir-dialect_package17.md — §§ 65–67, 79;
// rules/mlir/dialect-versions/sec_mlir_dialect_v13.md — ownership actions.
func ArrayConstructionNeedsOwnership(module *Module, op Operation) bool {
	if op.Kind != OpArrayConstruct && op.Kind != OpArrayDefault {
		return false
	}
	return ownedCopyClass(module, op.ArrayElementType) != "trivial" ||
		!trivialDestruction(module, op.ArrayElementType) ||
		len(op.Results) == 1 && op.Results[0].Ownership == OwnershipOwned
}
