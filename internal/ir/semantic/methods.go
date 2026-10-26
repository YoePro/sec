package semantic

import (
	"fmt"

	"sec/internal/ast"
	"sec/internal/sema"
)

// Methods declared in impl blocks (impl.md §§ 11, 13). A `static fn` member has
// no receiver, so it lowers as an ordinary Semantic IR function under its
// qualified `Target.Name` identity, and a call to it is a direct call to that
// resolved callable. Instance methods carry an implicit `self` receiver whose
// capability (shared, mutable or consuming) becomes an explicit receiver
// parameter; shared and mutable receivers need reference parameters
// (sec-mlir-dialect P15-47), so instance methods and their calls remain
// unsupported, as do static methods of generic implementations until
// specialization identities exist in Semantic IR.
//
// Rules:
//   - rules/declarations/impl.md — § 8 "self", § 11 "Methods", § 13 "Static members", § 25
//   - rules/compiler/semantic_ir.md — § 60 "Function representation", § 61 "Calls"

// buildStaticMethods builds every static method of one non-generic impl block
// and, from package 15, every instance method with a shared or mutable
// receiver (reference_receiver.go).
func (b *builder) buildStaticMethods(impl *ast.ImplStatement) error {
	if impl == nil || impl.Target == nil || len(impl.Target.TypeArgs) != 0 {
		return nil
	}
	for _, member := range impl.Members {
		if free, ok := member.(*ast.FreeDeclaration); ok && b.maxPackage >= ownershipLoweringPackage {
			if err := b.buildCustomFree(impl, free); err != nil {
				return err
			}
			continue
		}
		decl, ok := member.(*ast.FunctionDeclaration)
		if !ok || decl == nil || decl.Name == nil {
			continue
		}
		resolved, ok := b.analyzer.ResolvedMethodForDeclaration(impl.Target.Name, decl)
		if !ok {
			return fmt.Errorf("missing resolved method %s.%s", impl.Target.Name, decl.Name.Value)
		}
		if !resolved.Static && b.instanceMethodLowerable(resolved) {
			if err := b.buildResolvedFunction(decl, resolved); err != nil {
				return err
			}
			continue
		}
		if !resolved.Static || !staticMethodLowerable(resolved) {
			continue
		}
		if err := b.buildResolvedFunction(decl, resolved); err != nil {
			return err
		}
	}
	return nil
}

// staticMethodLowerable reports a static method that Semantic IR builds: one
// without generic parameters of its own or of its implementation.
func staticMethodLowerable(function sema.Function) bool {
	return function.Static && len(function.GenericParameters) == 0 && len(function.GenericSubstitution) == 0
}
