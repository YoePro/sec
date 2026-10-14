package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func tryHoverFixture(t *testing.T, name string) (string, string) {
	t.Helper()
	path := filepath.Join("../../testdata/lsp/try_hover", name)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	// Each fixture is its own module source set; sibling invalid cases must
	// not be assembled into a valid hover test's module.
	isolated := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(isolated, data, 0o644); err != nil {
		t.Fatal(err)
	}
	return uriFromPath(isolated), string(data)
}

// Coverage uses the same analyzed snapshot as ordinary hover and diagnostics.
// Rules: rules/tooling/lsp.md — "Hover"; rules/errors/errorhandling.md — §§9,12,16,19.
func TestTryHoverAdditionalPartialHandlersAndOperations(t *testing.T) {
	cases := []struct {
		file, function, root string
		wants                []string
	}{
		{"partial.sec", "Partial", "Read(value)", []string{"Handler coverage: `partial`", "Resolved handlers: `1`", "Handler 1: `Err(Missing)`", "Unhandled errors: `propagated to Result[int, ReadError]`"}},
		{"partial.sec", "Multiple", "Read(value)", []string{"Handler coverage: `partial`", "Resolved handlers: `2`", "Handler 1: `Err(Missing)`", "Handler 2: `Err(Failed)`", "Unhandled errors: `propagated to Result[int, ReadError]`"}},
		{"partial.sec", "GuardedThenPartial", "Read(value)", []string{"Handler coverage: `partial`", "Resolved handlers: `2`", "where …", "Handler 2: `Err(Missing)`", "Unhandled errors: `propagated to Result[int, ReadError]`"}},
		{"partial.sec", "Guarded", "Read(value)", []string{"Handler coverage: `partial`", "where …", "Unhandled errors: `propagated to Result[int, ReadError]`"}},
		{"partial.sec", "Mixed", "Read(value)", []string{"Handler coverage: `exhaustive`", "Resolved handlers: `3`", "Handler 1:", "Handler 2: `Err(Missing)`", "Handler 3: `Err(_)`"}},
		{"partial.sec", "Widen", "Read(value)", []string{"Handler coverage: `partial`", "Unhandled errors: `propagated to Result[int, error]`", "Error identity: `ReadError` widened to `error`"}},
		{"operations.sec", "Add", "+ right", []string{"Success value: `int`", "ArithmeticError", "Failure handling: `propagated`", "Result[int, ArithmeticError]"}},
		{"operations.sec", "Divide", "/ right", []string{"Success value: `int`", "ArithmeticError", "Handler coverage: `exhaustive`"}},
		{"operations.sec", "Index", "[index]", []string{"Success value: `int`", "IndexError", "Failure handling: `propagated`"}},
		{"operations.sec", "RecoverIndex", "[index]", []string{"Success value: `int`", "IndexError", "Handler coverage: `exhaustive`"}},
		{"operations.sec", "Convert", "int8(value)", []string{"Success value: `int8`", "ConversionError", "Failure handling: `propagated`"}},
		{"operations.sec", "RecoverConversion", "int8(value)", []string{"Success value: `int8`", "ConversionError", "Handler coverage: `exhaustive`"}},
	}
	for _, tc := range cases {
		t.Run(tc.function, func(t *testing.T) {
			uri, source := tryHoverFixture(t, tc.file)
			if diagnostics := analyze(uri, source); len(diagnostics) != 0 {
				t.Fatal(diagnostics)
			}
			start := strings.Index(source, "fn "+tc.function+"(")
			keyword := start + strings.Index(source[start:], "try ")
			operand := keyword + strings.Index(source[keyword:], tc.root)
			var keywordText string
			for i, offset := range []int{keyword, operand} {
				h, ok := hoverForSource(uri, source, offsetPosition(source, offset))
				if !ok {
					t.Fatalf("no hover at offset %d", offset)
				}
				for _, want := range tc.wants {
					if !strings.Contains(h.Contents.Value, want) {
						t.Fatalf("missing %q:\n%s", want, h.Contents.Value)
					}
				}
				if i == 0 {
					keywordText = h.Contents.Value
				} else if h.Contents.Value != keywordText {
					t.Fatalf("keyword and operand differ:\n%s\n%s", keywordText, h.Contents.Value)
				}
			}
			if tc.function == "Partial" {
				argument := keyword + strings.Index(source[keyword:], "Read(value)") + len("Read(")
				h, ok := hoverForSource(uri, source, offsetPosition(source, argument))
				if !ok || strings.Contains(h.Contents.Value, "### `try`") {
					t.Fatal(h, ok)
				}
			}
		})
	}
}

// Invalid source must not advertise successful handling or propagation; a
// valid neighboring try remains useful while the document has other errors.
// Rules: rules/tooling/lsp.md — "Unknown facts", "Hover";
// rules/errors/errorhandling.md — §§9,12.1–12.3,16.
func TestTryHoverRejectsInvalidOperandsAndErrorChannels(t *testing.T) {
	for _, file := range []string{"operands_invalid.sec", "channels_invalid.sec"} {
		t.Run(file, func(t *testing.T) {
			uri, source := tryHoverFixture(t, file)
			if reported := analyze(uri, source); len(reported) == 0 {
				t.Fatal("invalid fixture accepted")
			}
			start := 0
			for {
				next := strings.Index(source[start:], "try ")
				if next < 0 {
					break
				}
				keyword := start + next
				valid := file == "channels_invalid.sec" && keyword > strings.Index(source, "fn ValidNeighbor(")
				for _, offset := range []int{keyword, keyword + len("try ")} {
					h, ok := hoverForSource(uri, source, offsetPosition(source, offset))
					if valid {
						if !ok || !strings.Contains(h.Contents.Value, "Handler coverage: `exhaustive`") {
							t.Fatal(h, ok)
						}
					} else if ok && strings.Contains(h.Contents.Value, "### `try`") {
						t.Fatalf("invalid try at %d published valid-looking facts:\n%s", offset, h.Contents.Value)
					}
				}
				start = keyword + len("try ")
			}
		})
	}
}
