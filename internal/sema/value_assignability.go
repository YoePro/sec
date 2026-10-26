package sema

// ValueAssignable reports whether an already typed value of type value may
// initialize a storage site of type target under the language's ordinary
// assignability: identical types, the one-way concrete-error-to-error
// widening, reference and unit-quantity rules, and the contextual conversions
// Sema accepts without a source-visible conversion. Literal shaping is not
// included, since no literal is involved. Unknown or invalid types are never
// assignable, so tools never invent a relation.
//
// Rules: rules/types/types.md — "Type identity", "Assignability";
// rules/errors/errorhandling.md — error widening.
func (a *Analyzer) ValueAssignable(target, value Type) bool {
	if a == nil || target.Kind == InvalidType || value.Kind == InvalidType || target.Kind == "" || value.Kind == "" ||
		value.Kind == VoidType || value.Kind == NeverType {
		return false
	}
	return a.canInitializeUnrecorded(target, value, nil)
}
