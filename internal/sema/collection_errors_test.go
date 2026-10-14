package sema

import (
	"os"
	"strings"
	"testing"
)

// The canonical core family has exactly three structural cases and remains a
// distinct Error type, never an implicit alias for AllocationError.
// Rules: rules/collections/collections.md — §16; rules/library/core-library.md — §20.
func TestCanonicalCollectionErrorFamily(t *testing.T) {
	first := newBuiltinTypes()["CollectionError"]
	second := newBuiltinTypes()["CollectionError"]
	if first.Kind != EnumType || first.Underlying != "uint" || !first.ErrorAssignable || strings.Join(first.EnumValues, ",") != "AllocationFailed,CapacityExceeded,SizeOverflow" {
		t.Fatalf("wrong public family: %+v", first)
	}
	for index, name := range first.EnumValues {
		if value := first.EnumConsts[name].Value; value == nil || value.Int64() != int64(index) {
			t.Fatalf("inconsistent case metadata: %s", name)
		}
	}
	first.EnumValues[0] = "Changed"
	first.EnumConsts["AllocationFailed"].Value.SetInt64(99)
	if second.EnumValues[0] != "AllocationFailed" || second.EnumConsts["AllocationFailed"].Value.Int64() != 0 {
		t.Fatal("fresh builtin registrations share mutable case metadata")
	}
	data, err := os.ReadFile("../../testdata/sema/collection_errors_invalid.sec")
	if err != nil {
		t.Fatal(err)
	}
	_, errors := analyzeSourceWithAnalyzer(t, string(data))
	if len(errors) != 2 || !strings.Contains(errors[0].Message, "Missing") || !strings.Contains(errors[1].Message, "AllocationError") {
		t.Fatalf("invalid case or implicit error-domain conversion accepted: %v", errors)
	}
}
