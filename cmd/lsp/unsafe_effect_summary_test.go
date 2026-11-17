package main

import (
	"os"
	"strings"
	"testing"
)

// Unsafe hover retains the canonical effects of safe wrappers and separately
// reports unresolved domains; ordinary safety never becomes an effect guarantee.
// Rules: rules/memory/unsafe.md §§13–15,52(1); rules/tooling/lsp.md §10.2.
func TestUnsafeCanonicalEffectsHover(t *testing.T) {
	data, err := os.ReadFile("../../testdata/analysis/unsafe_effects.sec")
	if err != nil {
		t.Fatal(err)
	}
	source := string(data)
	for _, tc := range []struct {
		name string
		want []string
	}{
		{"VolatileLeaf", []string{"Caller-facing safety: unsafe", "Effects remain active through unsafe: `MayAccessVolatile`"}},
		{"SafeWrapper", []string{"Caller-facing safety: safe", "Effects remain active through unsafe: `MayAccessVolatile`"}},
		{"MixedEffects", []string{"Effects remain active through unsafe: `MayAccessVolatile`", "Effects unknown through unsafe:", "`MayIO`", "no effect-freedom proof"}},
		{"ForeignWrapper", []string{"Effects unknown through unsafe:", "`MayIO`", "no effect-freedom proof"}},
	} {
		offset := strings.Index(source, "fn "+tc.name+"(")
		hover, ok := hoverForSource("file:///tmp/sec-unsafe-effects/main.sec", source, offsetPosition(source, offset+3))
		if !ok {
			t.Fatalf("no hover for %s", tc.name)
		}
		for _, want := range tc.want {
			if !strings.Contains(hover.Contents.Value, want) {
				t.Errorf("%s missing %q:\n%s", tc.name, want, hover.Contents.Value)
			}
		}
	}
}
