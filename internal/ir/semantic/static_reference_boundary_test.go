package semantic

import (
	"errors"
	"strings"
	"testing"
)

// MD-081 § 9(3), § 12(6): a static initialized with `ref S` is valid source,
// but Semantic IR has no static-held borrow yet, so it rejects the static
// explicitly instead of approximating it; ordinary local references to
// statics still lower.
// Rules: rules/declarations/static.md — §§ 5, 27 (Amendment MD-081).
func TestStaticReferenceInitializerIsExplicitBoundary(t *testing.T) {
	module, err := analyzedModule(t, `module main

let Configuration: int := 42
let ConfigurationRef := ref Configuration

fn Take(value: ref int) int {
    return 0
}

fn Use() int {
    return Take(ConfigurationRef)
}
`, 15)
	var unsupported *UnsupportedFeatureError
	if module != nil || !errors.As(err, &unsupported) || !strings.Contains(unsupported.Feature, "static ConfigurationRef initialized with a reference") {
		t.Fatalf("module=%v err=%v", module != nil, err)
	}
	if _, err := analyzedModule(t, `module main

let Configuration: int := 42

fn Take(value: ref int) int {
    return 0
}

fn Use() int {
    let local := ref Configuration
    return Take(local)
}
`, 15); err != nil {
		t.Fatalf("local static reference regressed: %v", err)
	}
}
