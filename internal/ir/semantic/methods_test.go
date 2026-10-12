package semantic

import (
	"errors"
	"strings"
	"testing"
)

// A static method has no receiver: it lowers as an ordinary function under
// its qualified Target.Name identity, and calls to it, including from other
// methods, are direct calls. Instance methods and static methods of generic
// implementations stay explicit boundaries.
//
// Rules:
//   - rules/declarations/impl.md — §§ 11, 13
//   - rules/compiler/semantic_ir.md — §§ 60, 61
func TestStaticMethodsLowerAsDirectCalls(t *testing.T) {
	module, err := analyzedModule(t, `module main

type Counter struct { value: int }

impl Counter {
    static fn Maximum() int {
        return 100
    }

    static fn Clamp(value: int) int {
        if value > Counter.Maximum() {
            return Counter.Maximum()
        }
        return value
    }
}

fn Use(x: int) int {
    return Counter.Clamp(x)
}
`, 14)
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	if err := Verify(module); err != nil {
		t.Fatalf("verify: %v\n%s", err, Format(module))
	}
	text := Format(module)
	for _, want := range []string{`func "main::Counter.Maximum()"`, `func "main::Counter.Clamp(int)"`, `call.direct "main::Counter.Maximum()"()`, `call.direct "main::Counter.Clamp(int)"(%0[copy-trivial])`} {
		if !strings.Contains(text, want) {
			t.Errorf("module lacks %q:\n%s", want, text)
		}
	}
}

func TestMethodLoweringBoundaries(t *testing.T) {
	for name, test := range map[string]struct{ source, want string }{
		"instance method": {`module main

type Counter struct { value: int }

impl Counter {
    fn Value() int {
        return value
    }
}

fn Use(counter: Counter) int {
    return counter.Value()
}
`, "instance method call"},
	} {
		module, err := analyzedModule(t, test.source, 14)
		var unsupported *UnsupportedFeatureError
		if module != nil || !errors.As(err, &unsupported) || unsupported.Feature != test.want {
			t.Errorf("%s: module=%t err=%v, want %q", name, module != nil, err, test.want)
		}
	}
}
