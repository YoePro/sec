package main

import (
	"os"
	"strings"
	"testing"
)

// Callable hover presents each reachable non-panic effect with its canonical
// cause chain from the shared call graph: introduced here, or via the
// shortest call path with the introducing site; a callable without such an
// effect gets no effect line.
//
// Rules:
//   - rules/compiler/semantic_ir.md — § 109(1)–(3)
//   - rules/tooling/lsp.md — § 24.4(2)
func TestEffectCauseChainHover(t *testing.T) {
	data, err := os.ReadFile("../../testdata/tooling/effect_cause_chains.sec")
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	uri := "file:///tmp/sec-lsp-effect-causes/main.sec"
	hover := func(declaration string) string {
		t.Helper()
		offset := strings.Index(text, declaration) + len("fn ")
		result, ok := hoverForSource(uri, text, offsetPosition(text, offset))
		if !ok {
			t.Fatalf("no hover for %s", declaration)
		}
		return result.Contents.Value
	}
	for declaration, want := range map[string]string{
		"fn Leaf":    "Effect `memory-write`: here at 8:",
		"fn Wrapper": "Effect `memory-write` via `Wrapper` -> `Leaf` (introduced at",
		"fn Outer":   "Effect `memory-write` via `Outer` -> `Wrapper` -> `Leaf` (introduced at",
	} {
		if contents := hover(declaration); !strings.Contains(contents, want) {
			t.Errorf("%s hover lacks %q:\n%s", declaration, want, contents)
		}
	}
	if contents := hover("fn Plain"); strings.Contains(contents, "Effect `") {
		t.Errorf("Plain hover invents an effect:\n%s", contents)
	}
}
