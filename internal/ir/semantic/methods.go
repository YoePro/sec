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
// receiver (reference_receiver.go). Method-owned generic parameters register a
// template for demand-driven specialization instead of an executable body.
// Rules: rules/declarations/functions.md §38A(2),(9);
// rules/compiler/monomorphization.md §§9.1,9.3.
func (b *builder) buildStaticMethods(impl *ast.ImplStatement) error {
	if impl == nil || impl.Target == nil {
		return nil
	}
	// The methods of a generic implementation are templates; each selected
	// call on a concrete instance demands one specialization
	// (generic_specialization.go; monomorphization.md § 9.3).
	if len(impl.Target.TypeArgs) != 0 {
		for _, member := range impl.Members {
			decl, ok := member.(*ast.FunctionDeclaration)
			if !ok || decl == nil || decl.Name == nil {
				continue
			}
			if resolved, ok := b.analyzer.ResolvedMethodForDeclaration(impl.Target.Name, decl); ok {
				b.registerGenericTemplate(decl, resolved)
			}
		}
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
		// Method-owned generics are templates even when the owner is concrete.
		// Register them without building an unspecialized executable body.
		// Rules: rules/declarations/functions.md §38A(2),(9);
		// rules/compiler/monomorphization.md §§9.1,9.3.
		if len(resolved.GenericParameters) != 0 {
			b.registerGenericTemplate(decl, resolved)
			continue
		}
		if !resolved.Static {
			// A hardware instance reaches this member through its demanded
			// hardware variant; a hardware-only member has no software body.
			b.registerGenericTemplate(decl, resolved)
			if hardwareOnly, _, ok := b.analyzer.RegisterMemberRequirement(registerMemberKey(resolved)); ok && hardwareOnly {
				continue
			}
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
// without generic parameters of its own. A method of a generic implementation
// reaches calls only as an instance specialized from its receiver's arguments.
func staticMethodLowerable(function sema.Function) bool {
	return function.Static && len(function.GenericParameters) == 0
}
