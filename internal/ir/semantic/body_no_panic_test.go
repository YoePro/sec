package semantic

import (
	"strings"
	"testing"
)

const bodyNoPanicSource = `module main

type Counter struct {
    _value: int,
}

impl Counter {
    @noPanic
    free {
    }

    property Value: int {
        @noPanic
        get {
            return self._value
        }

        set value {
            self._value = value
        }
    }

    property Other: int {
        get {
            return self._value
        }

        @noPanic
        set value {
            self._value = value
        }
    }
}

fn Use() int {
    let mut counter := Counter { _value: 1 }
    counter.Other = counter.Value
    return counter.Other
}
`

// The verified @noPanic contract on a property accessor or a custom free is
// carried on the Semantic IR function built from that body, and only on it:
// annotating a getter does not annotate its setter, and the reverse.
//
// Rules:
//   - rules/foundations/attributes.md — "Callable-body attachment (MD-030)"
//   - rules/corrections/applied/md030-no-panic-attribute-placement-correction-20261017.md — §§ 9(3), 9(5)
//   - rules/compiler/semantic_ir.md — § 55(3) noPanic effect facts
func TestCallableBodyNoPanicContractInSemanticIR(t *testing.T) {
	module, err := analyzedModule(t, bodyNoPanicSource, 17)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	if err := Verify(module); err != nil {
		t.Fatalf("Verify: %v\n%s", err, Format(module))
	}
	got := map[string]bool{}
	for _, fn := range module.Functions {
		got[string(fn.ID)] = fn.NoPanic
		if fn.NoPanic && fn.NoPanicSource != "written" {
			t.Errorf("%s noPanic source = %q", fn.ID, fn.NoPanicSource)
		}
	}
	for id, want := range map[string]bool{
		"main::Counter.Value.get()":    true,
		"main::Counter.Other.set(int)": true,
		"main::@free:Counter()":        true,
		"main::Counter.Other.get()":    false,
		"main::Use()":                  false,
	} {
		if noPanic, ok := got[id]; !ok || noPanic != want {
			t.Errorf("%s noPanic = %v (built %v), want %v\n%s", id, noPanic, ok, want, Format(module))
		}
	}
	if text := Format(module); strings.Count(text, "noPanic(written)") != 3 {
		t.Errorf("want three printed noPanic(written) contracts:\n%s", text)
	}
}
