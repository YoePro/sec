package collectionshape

// FirstDuplicate compares direct collection elements using the caller's resolved
// semantic equality. It requires neither hashing nor Go identity, and performs no
// recursive uniqueness check. The second index and then the first index select
// the earliest source-ordered duplicate. An empty or single-element collection
// never evaluates equality.
// Rules: rules/types/contracts.md — String and collection contracts (unique).
func FirstDuplicate[T any](values []T, equal func(T, T) bool) (duplicate, original int, found bool) {
	for i := 1; i < len(values); i++ {
		for j := 0; j < i; j++ {
			if equal(values[j], values[i]) {
				return i, j, true
			}
		}
	}
	return 0, 0, false
}
