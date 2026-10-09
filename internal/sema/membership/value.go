// Package membership owns immutable nominal value identities for compile-time
// equality. It does not interpret source expressions or runtime memory.
package membership

// Value identifies a concrete enum/union type and one of its declared members.
// Owner includes concrete generic arguments; aliases use the declaration's
// identity. Variant names alone and source spellings are not semantic identity.
type Value struct {
	// Type is the actual source type, retained separately from the base value
	// class for checking operator relations before equality. TypeName omits
	// arguments only for checking the permitted named declaration ancestry.
	Type, TypeName string
	Owner          string
	Member         string
	// Key is the enum value class or union variant identity. Member retains
	// source spelling; different enum alias names may share the same Key.
	Key string
}

// Equal compares semantic type and member identities, never source text or
// memory addresses. Missing identities cannot prove nominal equality.
// Rules: rules/types/contracts.md — Ordered membership;
// rules/declarations/unions.md — §14 Equality.
func Equal(left, right Value) bool {
	return left.Owner != "" && left.Key != "" && left.Owner == right.Owner && left.Key == right.Key
}
