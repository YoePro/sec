package sema

import (
	"fmt"

	"sec/internal/ast"
	"sec/internal/ast/walk"
)

// walkMoveAwarePitfallBody keeps final-use advice inside whole-body budget
// admission and the existing dependency-qualified cache publication boundary.
// Rules: rules/analysis/pitfall_analysis.md — "Large by-value argument at its final use",
// "Analysis states", "Incremental analysis".
func (b *pitfallBuilder) walkMoveAwarePitfallBody(body *ast.BlockStatement) {
	b.inspectLastUseArgumentMoves(body)
	b.walkBlock(body)
}

// inspectLastUseArgumentMoves correlates selected copy contracts and binding
// uses in a complete callable body. Candidates are straight-line top-level
// invocations; nested control flow still contributes all possible later uses.
// Deferred/captured execution and unresolved aliases suppress advice rather
// than treating lexical last occurrence as a control-flow liveness proof.
// Rules: rules/analysis/pitfall_analysis.md — "Large by-value argument at its final use";
// rules/memory/copy_move.md — §§6.3,7,8.1–8.3; rules/memory/borrowing.md — §§21–22.
func (b *pitfallBuilder) inspectLastUseArgumentMoves(body *ast.BlockStatement) {
	evaluation := b.counts[PitfallLastUseArgumentCopy]
	if b.boundsOnly || evaluation == nil || evaluation.State == PitfallStateNotEvaluated || body == nil || len(b.analyzer.errors) != 0 {
		return
	}
	uses := map[BindingID][]*ast.Identifier{}
	blocked := map[BindingID]bool{}
	delayed := false
	blockUses := func(node any) {
		_ = walk.Inspect(node, func(node any) error {
			if id, ok := node.(*ast.Identifier); ok {
				if binding, ok := b.analyzer.ResolvedBindingOf(id); ok {
					blocked[binding.ID] = true
				}
			}
			return nil
		})
	}
	inspectParameters := func(function Function, args []ast.Expression, known bool) {
		for index, arg := range args {
			parameter, ok := functionParameterForArgument(function, index)
			if !known || !ok || parameter.Ref || parameter.MutableRef || parameter.Type.Kind == ReferenceType || function.Extern {
				blockUses(arg)
			}
		}
	}
	_ = walk.Inspect(body, func(node any) error {
		switch node := node.(type) {
		case *ast.Identifier:
			if binding, ok := b.analyzer.ResolvedBindingOf(node); ok {
				uses[binding.ID] = append(uses[binding.ID], node)
			}
		case *ast.DeferStatement, *ast.LambdaExpression, *ast.SpawnExpression:
			delayed = true
		case *ast.RefExpression:
			blockUses(node.Value)
		case *ast.MemberExpression:
			// Stored-field, getter, raw-pointer and receiver dependencies are
			// not reconstructed here from names or apparent readonly syntax.
			blockUses(node.Object)
		case *ast.CallExpression:
			target, known := b.analyzer.ResolvedCallTarget(node)
			inspectParameters(target.Function, node.Arguments, known)
		case *ast.NewExpression:
			target, known := b.analyzer.ResolvedConstructionOf(node)
			inspectParameters(target.Initializer, node.Arguments, known && !target.Implicit)
		}
		return nil
	})
	if delayed {
		return
	}
	for _, statement := range body.Statements {
		var expr ast.Expression
		switch statement := statement.(type) {
		case *ast.LetStatement:
			expr = statement.Value
		case *ast.ExpressionStatement:
			expr = statement.Expression
		case *ast.ReturnStatement:
			expr = statement.Value
		}
		if tried, ok := expr.(*ast.TryExpression); ok {
			expr = tried.Expression
		}
		var function Function
		var args []ast.Expression
		var known bool
		switch invocation := expr.(type) {
		case *ast.CallExpression:
			target, ok := b.analyzer.ResolvedCallTarget(invocation)
			function, args, known = target.Function, invocation.Arguments, ok && target.Kind != ResolvedForeignCall && target.Kind != ResolvedInterfaceMethodCall
		case *ast.NewExpression:
			target, ok := b.analyzer.ResolvedConstructionOf(invocation)
			function, args, known = target.Initializer, invocation.Arguments, ok && !target.Implicit
		}
		if known {
			for index, arg := range args {
				parameter, ok := functionParameterForArgument(function, index)
				id, bare := arg.(*ast.Identifier)
				if !ok || !bare || parameter.Ref || parameter.MutableRef || parameter.Consuming || parameter.Variadic || function.Extern {
					continue
				}
				binding, resolved := b.analyzer.ResolvedBindingOf(id)
				plan, copied := b.analyzer.ResolvedTransferPlanOf(id)
				if !resolved || blocked[binding.ID] || id.Value == "self" || !copied || plan.Site != TransferSiteArgument || plan.Action != TransferCopyTrivial || len(plan.Source.Projections) != 0 || !plan.Source.Addressable || plan.Source.AddressSpace != AddressSpaceOrdinary || (plan.Source.RootKind != PlaceRootLocal && plan.Source.RootKind != PlaceRootParameter) {
					continue
				}
				typ := binding.Type
				if (typ.Kind != StructType && typ.Kind != ArrayType) || CopyClassificationOf(typ) != CopyTrivial || !TriviallyDestructible(typ) || !sameConcreteType(parameter.Type, typ) {
					continue
				}
				size, known := estimatedTypeSizeBytes(typ, map[string]bool{})
				if !known || size < largeByValueParameterThresholdBytes {
					continue
				}
				last := true
				for _, use := range uses[binding.ID] {
					if use.Token.File != id.Token.File || use.Token.Line > id.Token.Line || use.Token.Line == id.Token.Line && use.Token.Column > id.Token.Column {
						last = false
						break
					}
				}
				if !last {
					continue
				}
				replacement := "<-" + id.Value
				b.add(PitfallFinding{
					Rule: PitfallLastUseArgumentCopy, Family: PitfallAPIUsage,
					Classification: PitfallLikelyMistake, Confidence: PitfallConfidenceHigh,
					Subject: PitfallSubject{Expression: id.Value, Source: id.Token}, OwningRule: "copy-move-and-borrowing",
					EvidenceFor: []PitfallEvidence{
						{Strength: PitfallEvidenceProof, Fact: "the selected ordinary value parameter copies this whole binding", Source: id.Token},
						{Strength: PitfallEvidenceStrong, Fact: fmt.Sprintf("estimated aggregate value copy is %d bytes (large-value policy threshold: %d)", size, largeByValueParameterThresholdBytes), Source: id.Token},
						{Strength: PitfallEvidenceStrong, Fact: "no later use of this binding or unresolved borrow/capture/defer dependency was found in the complete callable body", Source: id.Token},
					},
					Actions: []PitfallSuggestedAction{{Kind: PitfallSuggestedEdit, Title: "Consider moving " + id.Value + " instead of copying its final argument use", Replacement: replacement, Source: id.Token}},
				})
			}
		}
		if !b.analyzer.statementCanFallThrough(statement) {
			break
		}
	}
}
