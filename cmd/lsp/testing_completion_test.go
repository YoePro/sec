package main

import (
	"strings"
	"testing"
)

// `testing.` inside a test body completes exactly the compiler-known testing
// surface, one item per overload, and nothing else.
func TestTestingNamespaceCompletionListsOnlyTestingMembers(t *testing.T) {
	source := "module sample\n\nfn Helper() int {\n    return 1\n}\n\ntest \"members\" {\n    let value := Helper()\n    testing."
	text := source + "\n}\n"
	items := completeSource("file:///sample_test.sec", text, len(source))
	var details []string
	for _, item := range items {
		if !strings.HasPrefix(item.Detail, "testing.") {
			t.Fatalf("unexpected completion %q (%s)", item.Label, item.Detail)
		}
		details = append(details, item.Detail)
	}
	want := []string{
		"testing.Pass()",
		"testing.Fail(message: string)",
		"testing.Skip(message: string)",
		"testing.Log(message: string)",
		"testing.Expect(condition: bool)",
		"testing.Expect(condition: bool, message: string)",
		"testing.Require(condition: bool)",
		"testing.Require(condition: bool, message: string)",
		"testing.ExpectEqual(expected, actual)",
		"testing.ExpectEqual(expected, actual, message: string)",
		"testing.RequireEqual(expected, actual)",
		"testing.RequireEqual(expected, actual, message: string)",
	}
	if strings.Join(details, "\n") != strings.Join(want, "\n") {
		t.Fatalf("testing completions:\n%s", strings.Join(details, "\n"))
	}

	filtered := completeSource("file:///sample_test.sec", source+"Ex\n}\n", len(source+"Ex"))
	if len(filtered) != 4 {
		t.Fatalf("prefix Ex: want the four Expect/ExpectEqual overloads, got %+v", filtered)
	}
	if outside := completeSource("file:///sample.sec", "module sample\n\nfn Use() {\n    testing.\n}\n", len("module sample\n\nfn Use() {\n    testing.")); len(outside) != 0 {
		t.Fatalf("testing members offered outside a test: %+v", outside)
	}
}
