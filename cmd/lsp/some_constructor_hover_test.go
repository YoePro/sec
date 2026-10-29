package main

import (
	"os"
	"strings"
	"testing"
)

// TestSomeConstructorHover shows the actual inferred/contextual constructor
// facts at each callee, including both distinct tokens in a nested expression.
// Rules: rules/types/types.md — "Contextual and self-inferred `Some(value)`";
// rules/tooling/lsp.md — "Error patterns, inactive arms and `Some`".
func TestSomeConstructorHover(t *testing.T) {
	data, err := os.ReadFile("../../testdata/tooling/some_hover/constructors.sec")
	if err != nil {
		t.Fatal(err)
	}
	source := string(data)
	for _, diagnostic := range analyze("", source) {
		if diagnostic.Severity == 1 {
			t.Fatalf("valid constructor fixture rejected: %+v", diagnostic)
		}
	}
	for _, test := range []struct {
		use, typ string
		shift    int
	}{
		{"Some(3)", "Option[int]", 0},
		{"Some(\"hello\")", "Option[string]", 0},
		{"Some(42)", "Option[int64]", 0},
		{"Some(Some(1))", "Option[Option[int]]", 0},
		{"Some(Some(1))", "Option[int]", len("Some(")},
		{"Some(None)", "Option[Option[int]]", 0},
		{"Some(7)", "Option[int]", 0},
		{"Some(43)", "Option[int64]", 0},
		{"Some(44)", "Option[int64]", 0},
	} {
		t.Run(test.use+test.typ, func(t *testing.T) {
			offset := strings.Index(source, test.use) + test.shift
			hover, ok := hoverForSource("", source, offsetPosition(source, offset+1))
			for _, want := range []string{"Compiler-known constructor: `" + test.typ + ".Some`", "Selected type: `" + test.typ + "`"} {
				if !ok || !strings.Contains(hover.Contents.Value, want) {
					t.Fatalf("want %s, got %+v (found %v)", want, hover, ok)
				}
			}
			if hover.Range != offsetsRange(source, offset, offset+len("Some")) {
				t.Fatalf("hover range includes payload or surrounding expression: %+v", hover.Range)
			}
		})
	}
}

// TestSomeConstructorHoverExcludesOrdinaryAndInvalidCalls prevents an invented
// constructor identity even when a user Some function itself returns Option.
// Rules: rules/types/types.md — "Contextual and self-inferred `Some(value)`";
// correction md029-md042-md102-pattern-reachability-some-correction-20261028.md — §§11(1),13.
func TestSomeConstructorHoverExcludesOrdinaryAndInvalidCalls(t *testing.T) {
	for _, file := range []string{"user.sec", "inference_invalid.sec"} {
		data, err := os.ReadFile("../../testdata/tooling/some_hover/" + file)
		if err != nil {
			t.Fatal(err)
		}
		source := string(data)
		hasError := false
		for _, diagnostic := range analyze("", source) {
			hasError = hasError || diagnostic.Severity == 1
		}
		if hasError != (file == "inference_invalid.sec") {
			t.Fatalf("unexpected fixture validity for %s: error=%v", file, hasError)
		}
		offset := strings.LastIndex(source, "Some(")
		hover, ok := hoverForSource("", source, offsetPosition(source, offset+1))
		if strings.Contains(hover.Contents.Value, "Compiler-known constructor") {
			t.Fatalf("invented Option construction in %s: %+v", file, hover)
		}
		if file == "user.sec" && (!ok || !strings.Contains(hover.Contents.Value, "fn Some")) {
			t.Fatalf("ordinary function hover lost: %+v, %v", hover, ok)
		}
	}
}
