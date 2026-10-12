package semantic

import (
	"errors"
	"strings"
	"testing"
)

const optionTryFixture = `module main

fn Find(flag: bool) Option[int] {
    if flag {
        return Some(4)
    }
    return None
}
`

// try over Option[T] lowers through the canonical P11 union representation:
// one evaluation, a Some test, a copy-trivial payload unwrap on success, and
// on absence either a None return through the compatible Option or the
// source-ordered None handlers with guards, recovery-value merge and residual
// None propagation. No Result operation appears.
//
// Rules:
//   - rules/errors/errorhandling.md — §10.2, §12.2, §§15-16, §19, §21, §35
//   - rules/compiler/semantic_ir.md — § 24 "Result and Option"
func TestOptionTryLowersThroughUnionRepresentation(t *testing.T) {
	for name, test := range map[string]struct {
		source string
		want   []string
	}{
		"propagation": {optionTryFixture + `
fn Next(flag: bool) Option[int] {
    let value := try Find(flag)
    return Some(value + 1)
}
`, []string{"union.is-variant %1 variant=#0", "union.unwrap-payload %1 variant=#0[copy-trivial]", "union.construct variant=#1 : !2"}},
		"exhaustive handler": {optionTryFixture + `
fn Value(flag: bool) int {
    let value := try Find(flag) {
        None => 0
    }
    return value
}
`, []string{"[handler=ok index=-1]", "[handler=none index=0"}},
		"guarded handler with residual propagation": {optionTryFixture + `
fn Guarded(flag: bool, fallback: bool) Option[int] {
    let value := try Find(flag) {
        None where fallback => 7
    }
    return Some(value)
}
`, []string{"[handler=none index=0", "guarded", "[handler=residual index=-1"}},
		"block recovery value": {optionTryFixture + `
fn Block(flag: bool) int {
    let value := try Find(flag) {
        None => {
            let base := 3
            base + 1
        }
    }
    return value
}
`, []string{"[handler=none index=0"}},
	} {
		module, err := analyzedModule(t, test.source, 14)
		if err != nil {
			t.Fatalf("%s: build: %v", name, err)
		}
		if err := Verify(module); err != nil {
			t.Fatalf("%s: verify: %v\n%s", name, err, Format(module))
		}
		text := Format(module)
		for _, want := range test.want {
			if !strings.Contains(text, want) {
				t.Errorf("%s: module lacks %q:\n%s", name, want, text)
			}
		}
		if strings.Contains(text, "result.") {
			t.Errorf("%s: Option absence was lowered through Result operations:\n%s", name, text)
		}
	}
}

// Option try needs the P12 Option-through-union lowering; earlier packages
// keep an explicit boundary.
func TestOptionTryRequiresPackage12(t *testing.T) {
	module, err := analyzedModule(t, optionTryFixture+`
fn Next(flag: bool) Option[int] {
    let value := try Find(flag)
    return Some(value)
}
`, 11)
	var unsupported *UnsupportedFeatureError
	if module != nil || !errors.As(err, &unsupported) || unsupported.Feature != "Option try" {
		t.Fatalf("module=%t err=%v, want the package-12 Option try boundary", module != nil, err)
	}
}

// The verifier keeps the Option carrier distinct: None and Err handlers never
// share one try merge, and a merge has at most one unguarded None handler.
func TestOptionTryVerifierRejectsMixedCarrierHandlers(t *testing.T) {
	module, err := analyzedModule(t, optionTryFixture+`
fn Value(flag: bool) int {
    let value := try Find(flag) {
        None => 0
    }
    return value
}
`, 14)
	if err != nil {
		t.Fatal(err)
	}
	mutated := false
	for _, function := range module.Functions {
		for _, block := range function.Blocks {
			for index := range block.Operations {
				if block.Operations[index].TryHandlerKind == TryHandlerOK {
					block.Operations[index].TryHandlerKind = TryHandlerErrCatchAll
					mutated = true
				}
			}
		}
	}
	if !mutated {
		t.Fatal("fixture has no try success edge to mutate")
	}
	if err := Verify(module); err == nil || !strings.Contains(err.Error(), "mixes Option None and Result Err handlers") {
		t.Fatalf("verify = %v, want mixed-carrier rejection", err)
	}
}
