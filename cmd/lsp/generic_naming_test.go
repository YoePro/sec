package main

import (
	"os"
	"testing"

	"sec/internal/diagnostics"
)

func TestAnalyzePublishesGenericParameterNamingDiagnostics(t *testing.T) {
	source, err := os.ReadFile("../../testdata/names/generic_capitalization_invalid.sec")
	if err != nil {
		t.Fatal(err)
	}
	count := 0
	for _, diagnostic := range analyze("file:///tmp/generic_capitalization_invalid.sec", string(source)) {
		if diagnostic.Code != diagnostics.InvalidGenericParameterName {
			continue
		}
		count++
		if diagnostic.Severity != 1 || diagnostic.Range.End.Character <= diagnostic.Range.Start.Character {
			t.Errorf("missing error severity or name range: %+v", diagnostic)
		}
	}
	if count != 9 {
		t.Fatalf("got %d naming diagnostics, want 9", count)
	}
}
