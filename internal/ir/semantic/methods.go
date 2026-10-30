package semantic

import (
	"fmt"

	"sec/internal/ast"
	"sec/internal/sema"
)

// Methods declared in impl blocks retain their resolved qualified identity.
// Static methods have no receiver; from P15 ordinary instance methods take an
// explicit shared/mutable reference receiver. Ordinary methods never consume
// whole self (functions.md §22); generic implementations remain unsupported
// until concrete specialization identities and body facts exist in Semantic IR.
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
