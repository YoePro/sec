package semantic

import (
	"errors"
	"strings"
	"testing"
)

// A try whose protected expression holds several language-defined checks
// lowers the ordered failure set as first-failure control flow: every checked
// operator and runtime fixed-array index fails into one typed failure edge
// instead of panicking, a Result carrier root is tested after its operands,
// and the edge propagates through the enclosing Result or dispatches the
// local handlers.
//
// Rules:
//   - rules/errors/errorhandling.md — §11 "Compiler-internal failure sets", §§15-16, §35
//   - rules/errors/runtime_checks.md — "What try converts"
func TestFailureSetTryLowersFirstFailureControlFlow(t *testing.T) {
	for name, test := range map[string]struct {
		source       string
		want         []string
		failureEdges int
		forbidPanics []string
	}{
		"nested arithmetic propagation": {`module main

fn Calc(a: int, b: int, c: int) Result[int, ArithmeticError] {
    let value := try a + b * c
    return Ok(value)
}
`, []string{"int.binary-checked multiply", "int.binary-checked add", "arithmetic-error.from-reason", "result.err"}, 2, []string{"fail.arithmetic"}},
		"nested arithmetic handlers": {`module main

fn Calc(a: int, b: int, c: int) int {
    let value := try a + b * c {
        Err(_) => 0
    }
    return value
}
`, []string{"[handler=ok index=-1]", "[handler=err-catch-all index=0"}, 2, []string{"fail.arithmetic"}},
		"carrier after its operands": {`module main

fn Half(value: int) Result[int, ArithmeticError] {
    return Ok(value)
}

fn Calc(a: int, b: int) Result[int, ArithmeticError] {
    let value := try Half(a + b)
    return Ok(value)
}
`, []string{"int.binary-checked add", "call.direct", "result.is-err", "result.unwrap-err"}, 2, []string{"fail.arithmetic"}},
		"nested fixed-array indexes": {`module main

fn Pick(values: int[3], index: int) Result[int, IndexError] {
    let value := try values[values[index]]
    return Ok(value)
}
`, []string{"array.index-in-bounds", "OutOfBounds"}, 2, []string{"fail.bounds"}},
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
		for _, forbidden := range test.forbidPanics {
			if strings.Contains(text, forbidden) {
				t.Errorf("%s: protected check lowered as panic %q:\n%s", name, forbidden, text)
			}
		}
		edges := 0
		for _, function := range module.Functions {
			for _, block := range function.Blocks {
				for _, op := range block.Operations {
					if op.Kind == OpBranch && len(op.Successors) == 1 && len(op.Successors[0].Arguments) == 1 && op.TryHandlerKind == "" {
						edges++
					}
				}
			}
		}
		if edges != test.failureEdges {
			t.Errorf("%s: %d routed failure edges, want %d:\n%s", name, edges, test.failureEdges, text)
		}
	}
}

// A heterogeneous failure set needs widening to the open error root, which
// Semantic IR does not represent yet; its handlers are rejected instead of
// lowered with a lost error identity or a panic. (Heterogeneous propagation
// can only target Result[T, error], whose signature is already rejected.)
func TestFailureSetTryRejectsHeterogeneousSets(t *testing.T) {
	for name, test := range map[string]struct{ source, want string }{
		"handlers": {`module main

fn Pick(values: int[3], index: int) int {
    let value := try values[index] + 1 {
        Err(_) => 0
    }
    return value
}
`, "handlers over a heterogeneous failure set (error widening)"},
	} {
		module, err := analyzedModule(t, test.source, 14)
		var unsupported *UnsupportedFeatureError
		if module != nil || !errors.As(err, &unsupported) || unsupported.Feature != test.want {
			t.Errorf("%s: module=%t err=%v, want %q", name, module != nil, err, test.want)
		}
	}
}
