package main

import (
	"strings"
	"testing"
)

// The optional trust report exposes canonical effects without changing unsafe
// call compatibility or turning foreign/open uncertainty into effect freedom.
// Rules: rules/memory/unsafe.md §§13–15,52(1); rules/tooling/cli.md §5.4.
func TestAnalyseUnsafeCanonicalEffects(t *testing.T) {
	out, stderr, code := runCLIForDiagnostics(t, "analyse", "--verbose", "../../testdata/analysis/unsafe_effects.sec")
	if code != 0 {
		t.Fatal(code, out, stderr)
	}
	for _, want := range []string{
		"SafeWrapper: caller-facing safety safe",
		"SafeWrapper: effects remain active through unsafe: MayAccessVolatile",
		"RecursiveEffects: effects remain active through unsafe: MayAccessVolatile",
		"MixedEffects: effects remain active through unsafe: MayAccessVolatile",
		"MixedEffects: effect unknown through unsafe: MayIO; no effect-freedom proof",
		"ForeignWrapper: effect unknown through unsafe: MayIO; no effect-freedom proof",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, "PlainUnsafe: effects remain active") || strings.Contains(out, "PlainSafe: effects remain active") {
		t.Fatal("safety marker invented an effect")
	}
}
