package sema

import "testing"

// Every repeated range and multipleOf contract participates in one conjunction,
// independent of declaration order and of exclusive upper bounds.
//
// Rules:
//   - rules/types/contracts.md — "Integer contracts", unsatisfiable contract sets
func TestRepeatedIntegerContractsAreOrderIndependentConjunctions(t *testing.T) {
	errors := analyzeSource(t, `
type DisjointForward int range 0..10 range 20..30
type DisjointReversed int range 20..30 range 0..10
type Overlapping int range 0..10 range 5..20
type ExclusiveGap int range 0..<5 range 5..10
type NoLCMForward int range 1..10 multipleOf 4 multipleOf 6
type NoLCMInterleaved int multipleOf 6 range 1..10 multipleOf 4
type LCMReachable int range 1..12 multipleOf 4 multipleOf 6
type ZeroOnlyLCM int8 multipleOf 100 multipleOf 3
`)
	want := map[string]bool{
		"DisjointForward": true, "DisjointReversed": true, "ExclusiveGap": true,
		"NoLCMForward": true, "NoLCMInterleaved": true,
	}
	if len(errors) != len(want) {
		t.Fatalf("contract errors = %v, want exactly %d unsatisfiable sets", errors, len(want))
	}
	for name := range want {
		if !errorsContainMessage(errors, "contracts cannot be satisfied together for "+name) {
			t.Fatalf("missing unsatisfiable-set error for %s in %v", name, errors)
		}
	}
}
