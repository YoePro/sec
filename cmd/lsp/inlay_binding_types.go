package main

import (
	"sec/internal/ast"
	"sec/internal/lexer"
	"sec/internal/sema"
)

// inferredScopedBindingTypeHints projects declaration facts for loop and pattern
// bindings, including validated payload borrows and positive Option-if binding.
// It never derives types from container spelling, payload syntax or module-wide
// names, and leaves unresolved/discard/explicitly typed bindings unannotated.
// Rules: rules/tooling/lsp.md — "Inlay hints";
// rules/control-flow/flowcontrol_for.md — §§4,9,10;
// rules/control-flow/flowcontrol_match.md — §§9–12;
// rules/control-flow/flowcontrol_if.md — §13;
// rules/errors/errorhandling.md — §§15,20.
func inferredScopedBindingTypeHints(program *ast.Program, analyzer *sema.Analyzer, text string, inThisFile func(lexer.Token) bool) []inlayHint {
	hints := []inlayHint{}
	add := func(token lexer.Token, name string) {
		if name == "" || name == "_" || !inThisFile(token) || !validDefinitionTokenForHint(token) {
			return
		}
		binding, ok := analyzer.ResolvedBindingDeclaredAt(token.File, token.Line, token.Column)
		if !ok {
			// Match payload names currently navigate to the arm declaration;
			// follow that compiler-owned identity rather than look up the name.
			binding, ok = analyzer.ResolvedBindingAt(token.File, token.Line, token.Column)
		}
		if !ok || binding.ID == 0 || binding.Type.Kind == "" || binding.Type.Kind == sema.InvalidType {
			return
		}
		typ := lspTypeName(binding.Type)
		if typ == "" {
			return
		}
		hint := inlayHint{Position: tokenRange(text, token).End, Label: ": " + typ, Kind: inlayHintKindType, detail: name + ": " + typ}
		hint.declaration, _ = analyzer.ResolvedTypeDeclarationLocation(binding.Type)
		hints = append(hints, hint)
	}
	walkLSPNodes(program, func(node any) {
		switch node := node.(type) {
		case *ast.ForStatement:
			if node == nil {
				return
			}
			for _, binding := range node.Bindings {
				if !binding.Discard {
					add(binding.Token, binding.Name)
				}
			}
		case *ast.MatchPatternBinding:
			if node != nil && node.Name != nil {
				add(node.Name.Token, node.Name.Value)
			}
		case *ast.IfStatement:
			if node == nil || node.OptionBinding == nil || node.OptionBinding.Binding == nil {
				return
			}
			if _, ok := analyzer.ResolvedOptionIfBindingOf(node); !ok {
				return
			}
			binding := node.OptionBinding.Binding
			add(binding.Token, binding.Value)
		case *ast.TryHandler:
			if node == nil || node.Invalid {
				return
			}
			// The catch-all is Err(name). Narrowing patterns identify a concrete
			// variant and introduce no plain payload name to annotate.
			failure, ok := node.Pattern.(*ast.ErrExpression)
			if !ok {
				return
			}
			identifier, ok := failure.Value.(*ast.Identifier)
			if ok {
				add(identifier.Token, identifier.Value)
			}
		}
	})
	return hints
}
