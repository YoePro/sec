package sema

import "testing"

// A method needs a mutable receiver when it assigns a member inside a match
// arm, borrows a receiver member with `ref mut`, or binds a payload of a
// receiver-rooted match subject with `ref mut`; a method that only reads in a
// match keeps an immutable receiver.
//
// Rules:
//   - rules/memory/borrowing.md — § 7 "Mutability authority"
//   - rules/declarations/functions.md — instance method receivers
func TestReceiverMutabilityCountsMatchArmsAndMutableBorrows(t *testing.T) {
	analyzer, errors := analyzeSourceWithAnalyzerRaw(t, `
module main

type Point struct {
    x: int,
}

type Flag union {
    On(Point)
    Off
}

type Counter struct {
    count: int,
    flag: Flag,
}

impl Counter {
    fn InArm() void {
        match flag {
            On(point) => {
                count = 1
            }
            Off => {
            }
        }
    }

    fn MutRef() void {
        let p := ref mut count
    }

    fn PayloadMut() void {
        match self.flag {
            On(ref mut point) => {
                point.x = 7
            }
            Off => {
            }
        }
    }

    fn ReadOnly() int {
        match flag {
            On(ref point) => {
                return point.x
            }
            Off => {
                return count
            }
        }
    }
}
`)
	if len(errors) != 0 {
		t.Fatalf("unexpected errors: %+v", errors)
	}
	for name, want := range map[string]bool{"InArm": true, "MutRef": true, "PayloadMut": true, "ReadOnly": false} {
		if got := analyzer.implMethodReceiverMutable("Counter."+name, "Counter"); got != want {
			t.Errorf("%s receiver mutable = %v, want %v", name, got, want)
		}
	}
}
