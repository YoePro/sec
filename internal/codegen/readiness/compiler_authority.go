package readiness

import (
	"fmt"

	"sec/internal/ast"
	astwalk "sec/internal/ast/walk"
	"sec/internal/ir/semantic"
	"sec/internal/layout"
	"sec/internal/lexer"
	"sec/internal/sema"
)

// ValidateCompilerAuthority prevents raw backend APIs from granting compiler
// privilege by spelling alone. Reserved internal uses and impls targeting
// registered builtin identities trigger analysis; ordinary private helpers remain
// ordinary source functions. Sema
// checks loader provenance, owner file and signature on the current AST, using
// the same target plan as emission, including indirect uses of internal names.
// Rules: rules/compiler/compiler_known_members.md — Internal intrinsic operations,
// Internal core string-slice helper, Lookup order; rules/foundations/names_scopes_visibility.md — §19;
// rules/compiler/compiler_pipeline.md — Sema before lowering.
func ValidateCompilerAuthority(program *ast.Program, plan layout.ResolvedScalarPlan, backend string) error {
	names := map[string]bool{}
	for _, function := range sema.CompilerKnownFunctions() {
		if function.Internal {
			names[function.Name] = true
		}
	}
	directCallees := map[*ast.Identifier]bool{}
	var uses []*ast.Identifier
	var first *lexer.Token
	var analyzer *sema.Analyzer
	if err := astwalk.Inspect(program, func(node any) error {
		if impl, ok := node.(*ast.ImplStatement); ok && impl.Target != nil {
			if analyzer == nil {
				analyzer = sema.NewAnalyzerWithScalarPlan(plan)
			}
			if _, registered := analyzer.IntrinsicTypes()[impl.Target.Name]; registered && first == nil {
				token := impl.Target.Token
				first = &token
			}
		}
		if call, ok := node.(*ast.CallExpression); ok {
			if call.Function != nil {
				directCallees[call.Function] = true
			}
			if identifier, ok := call.Callee.(*ast.Identifier); ok {
				directCallees[identifier] = true
			}
		}
		if identifier, ok := node.(*ast.Identifier); ok && names[identifier.Value] {
			uses = append(uses, identifier)
			if first == nil {
				token := identifier.Token
				first = &token
			}
		}
		return nil
	}); err != nil {
		return err
	}
	if first == nil {
		return nil
	}
	if analyzer == nil {
		analyzer = sema.NewAnalyzerWithScalarPlan(plan)
	}
	if errs := analyzer.Analyze(program); len(errs) > 0 {
		failure := errs[0]
		return &semantic.UnsupportedFeatureError{
			Feature:  fmt.Sprintf("%s compiler authority validation: %s", backend, failure.Message),
			Location: semantic.Location{File: failure.File, Line: failure.Line, Column: failure.Column},
		}
	}
	// A validated reference is still unsupported when this legacy backend
	// has only direct inline emission and no callable helper representation.
	// This is a backend boundary, not a new source-language prohibition.
	for _, use := range uses {
		if !directCallees[use] {
			return &semantic.UnsupportedFeatureError{Feature: backend + " compiler-internal callable representation", Location: semantic.Location{File: use.Token.File, Line: use.Token.Line, Column: use.Token.Column}}
		}
	}
	return nil
}
