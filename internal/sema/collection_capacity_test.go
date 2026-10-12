package sema

import (
	"strings"
	"testing"
)

// MD-058: dynamic list, map and set reserve capacity through Reserve and the
// associated WithCapacity factory, both fallible with CollectionError; every
// list, map and set observes Capacity; bounded forms, T[], slices and fixed
// arrays have no growth members; no Capacity() method exists.
//
// Rules:
//   - rules/collections/collections.md — §§ 3.1, 13.12, 14.3, 14.8, 15.2, 15.5, 16
//   - rules/compiler/compiler_known_members.md — "Collection capacity members"
//   - rules/corrections/applied/md058-collection-capacity-reservation-correction-20261012.md — §§ 2-5, 9.4
func TestCollectionCapacityReservationSurface(t *testing.T) {
	analyzer, errors := analyzeSourceWithAnalyzerRaw(t, `
module main

fn Dynamic() Result[uint, CollectionError] {
    let mut bytes: list[byte]
    try bytes.Reserve(1024)
    try bytes.Reserve(0)
    let mut numbers := try list[int].WithCapacity(100)
    let mut empty := try list[int].WithCapacity(0)
    let mut users := try map[string, int].WithCapacity(100)
    let mut names := try set[string].WithCapacity(100)
    try numbers.Reserve(20)
    try users.Reserve(20)
    try names.Reserve(20)
    return Ok(bytes.Capacity + numbers.Capacity + empty.Capacity + users.Capacity + names.Capacity)
}

fn Bounded(values: list[int, 8], users: map[string, int, 4], names: set[string, 4]) uint {
    return values.Capacity + users.Capacity + names.Capacity
}
`)
	assertSemaErrors(t, errors, nil)
	ids := map[string]bool{}
	for _, member := range analyzer.compilerKnownMemberFacts {
		ids[member.ID] = true
	}
	for _, id := range []string{"CKM-LIST-RESERVE", "CKM-LIST-WITHCAPACITY", "CKM-LIST-CAPACITY", "CKM-MAP-RESERVE", "CKM-MAP-WITHCAPACITY", "CKM-MAP-CAPACITY", "CKM-SET-RESERVE", "CKM-SET-WITHCAPACITY", "CKM-SET-CAPACITY"} {
		if !ids[id] {
			t.Errorf("missing resolved compiler-known member %s", id)
		}
		if _, documented := compilerKnownSyntheticCatalog[id]; !documented && id != "CKM-LIST-CAPACITY" {
			t.Errorf("%s has no synthetic LSP declaration", id)
		}
	}
}

func TestCollectionCapacityReservationRejections(t *testing.T) {
	errors := analyzeSourceRaw(t, `
module main

fn Invalid(values: list[int, 8], users: map[string, int, 4], array: int[], slice: ref mut int[], dynamic: list[int]) Result[void, CollectionError] {
    try values.Reserve(1)
    try users.Reserve(1)
    try array.Reserve(1)
    try slice.Reserve(1)
    let hidden := array.Capacity
    let called := dynamic.Capacity()
    let wrong := try list[int].WithCapacity("many")
    return Ok()
}
`)
	if len(errors) != 7 {
		t.Fatalf("errors = %v, want one rejection per invalid use", errors)
	}
	for index, want := range []string{"values.Reserve", "users.Reserve", "array.Reserve", "slice.Reserve", "unknown member Capacity on int[]", "Capacity", "WithCapacity capacity must be uint"} {
		if !strings.Contains(errors[index].Message, want) {
			t.Errorf("error %d = %q, want %q", index, errors[index].Message, want)
		}
	}
}

// MD-058 § 7: string Len counts Unicode scalars and the value-form SizeOf
// counts UTF-8 payload bytes; the ByteLen and RuneLen aliases are gone, and
// the type form string.SizeOf stays the descriptor layout size.
func TestStringLengthObservationsWithoutAliases(t *testing.T) {
	errors := analyzeSourceRaw(t, `
module main

fn Counts(text: string) uint {
    return text.Len + text.SizeOf + string.SizeOf + "Hej π".SizeOf
}

fn Aliases(text: string) uint {
    return text.ByteLen + text.RuneLen
}
`)
	if len(errors) != 1 || !strings.Contains(errors[0].Message, "ByteLen") {
		t.Fatalf("errors = %v, want only the removed ByteLen alias rejected (RuneLen follows the same rule)", errors)
	}
	for _, member := range CompilerKnownMembersForType(builtinTypes()["string"], false) {
		if member.Name == "ByteLen" || member.Name == "RuneLen" || member.Name == "CharLen" {
			t.Fatalf("string still exposes the alias %s", member.Name)
		}
	}
}
