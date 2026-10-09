package sema

// refinedCompilerKnownType preserves the canonical trusted-core refinement of
// compiler-owned result/parameter identities, including nested error channels.
// The registry supplies ownership; ordinary declarations cannot replace it.
// Rules: rules/library/core-library.md — compiler/core ownership;
// rules/types/types.md — Type identity; compiler_known_members.md — privileged core declarations.
func (a *Analyzer) refinedCompilerKnownType(typ Type) Type {
	if builtin := builtinType(typ.Name); builtin.Kind != "" && builtin.Kind == typ.Kind && len(typ.TypeArgs) == 0 && len(typ.ConstArgs) == 0 && typ.Element == nil {
		if resolved, found := a.types[typ.Name]; found && resolved.Kind == typ.Kind {
			return resolved
		}
	}
	if len(typ.TypeArgs) > 0 {
		arguments := make([]Type, len(typ.TypeArgs))
		for i, argument := range typ.TypeArgs {
			arguments[i] = a.refinedCompilerKnownType(argument)
		}
		typ.TypeArgs = arguments
	}
	if typ.Element != nil {
		element := a.refinedCompilerKnownType(*typ.Element)
		typ.Element = &element
	}
	return typ
}
