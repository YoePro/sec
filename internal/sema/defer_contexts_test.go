package sema

import "testing"

// defer is valid in ordinary and static functions and methods, lambdas,
// property accessors and init; rejection inside free is covered by
// frontend.destruction.
//
// Rules:
//   - rules/control-flow/defer.md — § 3 "Invocation scope", § 4 "free"
func TestDeferIsPermittedInEveryCallableBody(t *testing.T) {
	errors := analyzeSourceRaw(t, `module main

type Counter struct {
    value: int,
}

impl Counter {
    init(start: int) {
        defer {
            self.value = start
        }
        value = 0
    }

    property Doubled: int {
        get {
            defer {
            }
            return value * 2
        }
    }

    static fn Make() Counter {
        defer {
        }
        return Counter { value: 1 }
    }

    fn Read() int {
        defer {
        }
        return value
    }
}

fn Plain() int {
    defer {
    }
    let f := fn() int {
        defer {
        }
        return 1
    }
    return f()
}
`)
	if len(errors) != 0 {
		t.Fatalf("defer must be valid in every callable body: %v", errors)
	}
}
