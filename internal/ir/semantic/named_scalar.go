package semantic

import "sec/internal/sema"

// internNamedScalar retains nominal source identity over the resolved base
// type. Reconstructing scalar width from the declared name loses native-width
// and fixed-width facts, particularly for int/uint derivations.
// Rules: rules/types/types.md — Named types, "int and uint";
// rules/compiler/semantic_ir.md — nominal type identity;
// rules/memory/layout.md — §18(1–3).
func (b *builder) internNamedScalar(t sema.Type) (TypeID, error) {
	if len(t.Contracts) > 0 || t.Unit != "" || !t.Dimension.IsZero() {
		return 0, &UnsupportedFeatureError{Feature: "named type contracts or units", Package: b.maxPackage}
	}
	underlying, found := b.analyzer.Types()[t.Underlying]
	if t.NamedBase != nil {
		underlying, found = *t.NamedBase, true
	}
	if !found || t.Underlying == t.Name {
		return 0, &UnsupportedFeatureError{Feature: "unresolved scalar base for " + t.Name, Package: b.maxPackage}
	}
	baseID, err := b.internType(underlying)
	if err != nil {
		return 0, err
	}
	module, identity := b.semanticIdentity(t)
	args := make([]TypeID, 0, len(t.TypeArgs))
	if !t.FixedNamedArguments {
		for _, argument := range t.TypeArgs {
			id, err := b.internType(argument)
			if err != nil {
				return 0, err
			}
			args = append(args, id)
		}
	}
	return b.module.Types.Intern(Type{Kind: TypeNamed, Name: t.Name, Module: module, Identity: identity, Base: baseID, TypeArgs: args}), nil
}
