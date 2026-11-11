package sema

import (
	"strings"
	"testing"
)

// Closure analysis consumes the ordinary ownership analysis for capture and
// callable copy/move legality: a `<-` capture is a move of the outer Place at
// construction (use after it, a second move, a move on a later loop iteration
// or past a conditional move, and a move under a live borrow are rejected),
// a consuming or move-only closure is itself move-only, and callable
// capability strength is checked where a closure flows into a parameter.
//
// Rules:
//   - rules/analysis/closure_analysis.md — § 4(2) ownership integration, § 24 callable Copy and Move
//   - rules/declarations/lambda-functions.md — §§ 16–17, 27–28
//   - rules/memory/copy_move.md
func TestClosureCaptureOwnershipIntegration(t *testing.T) {
	prelude := `module main

type Res struct {
    id: int,
}

impl Res {
    free {
    }
}

fn Run(f: fn() int) int {
    return f()
}

`
	cases := []struct {
		name string
		body string
		id   string
		want string
	}{
		{"use after move capture", `fn F() int {
    let r := Res { id: 1 }
    let take := capture(<-r) fn() Res {
        return r
    }
    let owned := take()
    return r.id + owned.id
}`, "S1088", "use of moved value r"},
		{"second move capture", `fn F() int {
    let r := Res { id: 1 }
    let a := capture(<-r) fn() int {
        return 0
    }
    let b := capture(<-r) fn() int {
        return 0
    }
    return 0
}`, "S1088", "use of moved value r"},
		{"move capture in loop", `fn F() int {
    let r := Res { id: 1 }
    let mut i := 0
    while i < 2 {
        let take := capture(<-r) fn() Res {
            return r
        }
        let o := take()
        i += 1
    }
    return 0
}`, "S1089", "may be unavailable on a later loop iteration"},
		{"conditional move capture", `fn F(c: bool) int {
    let r := Res { id: 1 }
    if c {
        let take := capture(<-r) fn() Res {
            return r
        }
        let o := take()
    }
    return r.id
}`, "S1089", "moved on one possible execution path"},
		{"move capture while borrowed", `fn F() int {
    let r := Res { id: 1 }
    let b := ref r
    let take := capture(<-r) fn() Res {
        return r
    }
    let o := take()
    return b.id
}`, "S1163", "cannot move r while it is borrowed"},
		{"consuming closure copy", `fn F() int {
    let r := Res { id: 1 }
    let take := capture(<-r) fn() Res {
        return r
    }
    let other := take
    let a := take()
    return 0
}`, "S1007", "cannot copy move-only value take"},
		{"use of explicitly moved closure", `fn F() int {
    let r := Res { id: 1 }
    let take := capture(<-r) fn() Res {
        return r
    }
    let other :<- take
    let o := take()
    return o.id
}`, "S1088", "use of moved value take"},
		{"mut fn argument to fn parameter", `fn F() int {
    let count := 0
    let mut next := capture(count) fn() int {
        count += 1
        return count
    }
    return Run(next)
}`, "", "must be fn() int, got mut fn() int"},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			errors := analyzeSourceRaw(t, prelude+test.body+"\n")
			for _, err := range errors {
				if strings.Contains(err.Message, test.want) && (test.id == "" || err.ID == test.id) {
					return
				}
			}
			t.Fatalf("no %s diagnostic containing %q; got %v", test.id, test.want, errors)
		})
	}

	// A consuming capture that is never invoked is still legal (the
	// environment owns the value and releases it with the closure), and an
	// explicit `:<-` move carries the callable to its destination (§ 24(4)).
	if errors := analyzeSourceRaw(t, prelude+`fn F() int {
    let r := Res { id: 1 }
    let take := capture(<-r) fn() Res {
        return r
    }
    return 0
}

fn G() int {
    let r := Res { id: 1 }
    let take := capture(<-r) fn() Res {
        return r
    }
    let other :<- take
    let o := other()
    return o.id
}
`); len(errors) != 0 {
		t.Fatalf("legal consuming closures: %v", errors)
	}
}
