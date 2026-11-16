package semantic

import (
	"fmt"

	"sec/internal/sema"
)

// methodSpecializationIdentity consumes Sema's complete specialization identity
// for scalar method-local generics after composing the enclosing substitution.
// The physical callable key cannot merge phantom owner arguments or overloads.
// Existing free-function and pack encodings keep their established symbols.
// Rules: rules/compiler/monomorphization.md — §§4,6.4,9.3,15.3;
// rules/compiler/generics_lowering.md — §§2–4;
// rules/compiler/semantic_ir.md — §75(2–5).
func (b *builder) methodSpecializationIdentity(target sema.Function, substitution map[string]sema.Type) (FunctionID, string, error) {
	template := b.specializations.templates[genericTemplateKey(target)]
	if template == nil || target.ImplTarget == "" || target.GenericTypePack != "" || len(template.GenericParameters) == 0 {
		return "", "", nil
	}
	target.GenericSubstitution = substitution
	identity, known := b.analyzer.InstantiationIdentityOf(target)
	if !known {
		return "", "", fmt.Errorf("missing canonical generic method specialization identity for %s", target.Name)
	}
	return FunctionID(fmt.Sprintf("%s::%s[instance=%s]", target.Module, qualifiedCallableName(target), identity.Key)), identity.Key, nil
}
