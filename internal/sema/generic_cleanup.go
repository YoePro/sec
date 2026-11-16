package sema

// mayRequireDestruction reports a value whose destruction Sema must track:
// one that is not trivially destructible, or one whose type depends on a
// generic parameter. A template carries such a value's destruction as a
// symbolic generic-dependent requirement (its plan is the unsupported generic
// template plan); each concrete specialization resolves the exact destruction
// semantics of the substituted type, and may elide a concrete no-op only after
// resolving it.
// Rules: rules/compiler/monomorphization.md — §§ 11.2, 11.4, 12.2–12.3.
func mayRequireDestruction(typ Type) bool {
	return !TriviallyDestructible(typ) || typeMentionsGenericParameter(typ)
}

// movesMatchBinding reports a payload binding that takes ownership: a move,
// or in a template a binding whose copyability is unproven, which template
// validity treats as a move (copy_move.md §§ 3–5; monomorphization.md § 11.3).
func movesMatchBinding(action ResolvedMatchBindingAction) bool {
	return action == MatchBindingMove || action == MatchBindingConditional
}
