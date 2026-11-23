package sema

import "sec/internal/ast"

// GenericTypeArgument is a declaration-ordered binding supplied by semantic
// specialization. It is presentation data, never a new instantiation demand.
// Rules: rules/declarations/generics.md — §§25–26;
// rules/tooling/lsp.md — §18(2).
type GenericTypeArgument struct {
	Name string
	Type Type
}

// GenericCallTypeArgumentsOf returns the selected callable's complete concrete
// scalar substitution in declaration order (owner before method parameters).
// Pack sequences have their own facts; failed, symbolic and ordinary calls
// supply no specialization.
// Rules: rules/declarations/generics.md — §§18–20,25–26,29;
// rules/tooling/lsp.md — §18(2).
func (a *Analyzer) GenericCallTypeArgumentsOf(call *ast.CallExpression) ([]GenericTypeArgument, bool) {
	target, ok := a.ResolvedCallTarget(call)
	if !ok {
		return nil, false
	}
	template, ok := a.genericCallableTemplate(target.Function)
	if !ok {
		return nil, false
	}
	names := append([]string(nil), a.types[template.ImplTarget].GenericParameters...)
	names = append(names, template.GenericParameters...)
	if len(names) == 0 {
		return nil, false
	}
	arguments := make([]GenericTypeArgument, 0, len(names))
	for _, name := range names {
		typ, ok := target.Function.GenericSubstitution[name]
		if !ok || typ.Kind == "" || typ.Kind == InvalidType || typeMentionsGenericParameter(typ) {
			return nil, false
		}
		arguments = append(arguments, GenericTypeArgument{Name: name, Type: semanticSnapshotType(typ)})
	}
	return arguments, true
}

// GenericTypeArgumentsOf projects a concrete nominal instance's arguments onto
// its registered declaration, preserving parameter order and module identity.
// This read-only query never resolves a source spelling or instantiates a type.
// Rules: rules/declarations/generics.md — §§3–4,25–26,30;
// rules/tooling/lsp.md — §18(2).
func (a *Analyzer) GenericTypeArgumentsOf(typ Type) ([]GenericTypeArgument, bool) {
	if !typ.Named && typ.Kind == ReferenceType && typ.Element != nil {
		return a.GenericTypeArgumentsOf(*typ.Element)
	}
	if a == nil || len(typ.TypeArgs) == 0 || typeMentionsGenericParameter(typ) {
		return nil, false
	}
	template, ok := a.types[typ.Name]
	if !ok || template.Module != typ.Module || len(template.GenericParameters) != len(typ.TypeArgs) {
		return nil, false
	}
	arguments := make([]GenericTypeArgument, len(typ.TypeArgs))
	for i, value := range typ.TypeArgs {
		if value.Kind == "" || value.Kind == InvalidType {
			return nil, false
		}
		arguments[i] = GenericTypeArgument{Name: template.GenericParameters[i], Type: semanticSnapshotType(value)}
	}
	return arguments, true
}
