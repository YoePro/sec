package sema

import (
	"strings"
	"testing"
)

// `join worker` resolves an owned thread's lifecycle and preserves the
// handle; a second join, a join on only one path, a join of a task and a
// join through a reference are rejected (threads.md § 34).
func TestThreadJoinStatement(t *testing.T) {
	prelude := "module main\n\nfn Calculate() int {\n    return 42\n}\n\n"
	valid := analyzeSourceRaw(t, prelude+`fn Joined() uint {
    let worker := spawn thread Calculate()
    join worker
    let id := worker.ID
    return 0
}

fn TaskJoined() void {
    let work := spawn Calculate()
    join work
}

fn Branches(flag: bool) void {
    let worker := spawn thread Calculate()
    if flag {
        join worker
    } else {
        join worker
    }
}
`)
	for _, err := range valid {
		if strings.Contains(err.Message, "join") || strings.Contains(err.Message, "unresolved") {
			t.Fatalf("unexpected join diagnostic: %v", valid)
		}
	}
	cases := map[string]struct{ body, want string }{
		"second join":  {"fn F() void {\n    let worker := spawn thread Calculate()\n    join worker\n    join worker\n}", "thread worker was already joined"},
		"one path":     {"fn F(flag: bool) void {\n    let worker := spawn thread Calculate()\n    if flag {\n        join worker\n    }\n}", "owned thread worker is unresolved at scope exit"},
		"task twice":   {"fn F() void {\n    let work := spawn Calculate()\n    join work\n    join work\n}", "task work was already joined"},
		"not a handle": {"fn F(value: int) void {\n    join value\n}", "join requires Task[T] or Thread[T], got int"},
	}
	for name, test := range cases {
		t.Run(name, func(t *testing.T) {
			errors := analyzeSourceRaw(t, prelude+test.body+"\n")
			for _, err := range errors {
				if strings.Contains(err.Message, test.want) {
					return
				}
			}
			t.Fatalf("no diagnostic containing %q; got %v", test.want, errors)
		})
	}
}

// A selectable join branch (threads.md § 35, tasks.md § 17(13)–(16)) commits
// the ordinary join in its own branch; a select without default continues
// only through a committed branch (select.md §§ 4(4), 35), so joining in every
// branch resolves the handles. One handle may be used by one branch only.
func TestSelectJoinBranch(t *testing.T) {
	errors := analyzeSourceRaw(t, `module main

fn Calculate() int {
    return 42
}

fn Selected() void {
    let worker := spawn thread Calculate()
    let work := spawn Calculate()
    select {
        join worker => {
            join work
        }
        join work => {
            join worker
        }
    }
}

fn Twice() void {
    let worker := spawn thread Calculate()
    select {
        join worker => {
        }
        join worker => {
        }
    }
    join worker
}

fn JoinAndAwait() void {
    let work := spawn Calculate()
    select {
        join work => {
        }
        result := await work => {
        }
    }
}

fn Unresolved() void {
    let worker := spawn thread Calculate()
    select {
        join worker => {
        }
        default => {
        }
    }
}
`)
	want := []string{
		"thread worker is used by more than one branch in the same select",
		"thread worker was already joined at 23:9",
		"task work is used by more than one branch in the same select",
		"owned thread worker is unresolved at scope exit",
	}
	if len(errors) != len(want) {
		t.Fatalf("diagnostics = %v", errors)
	}
	for i, message := range want {
		if !strings.Contains(errors[i].Message, message) {
			t.Errorf("diagnostic %d = %q, want %q", i, errors[i].Message, message)
		}
	}
}

// Non-waiting operations are never selectable (select.md §§ 13, 17(4), 54(3)).
func TestSelectRejectsNonWaitingOperations(t *testing.T) {
	for name, operation := range map[string]string{
		"TryReceive": "r := rx.TryReceive()",
		"TrySend":    "s := tx.TrySend(1)",
	} {
		t.Run(name, func(t *testing.T) {
			errors := analyzeSourceRaw(t, "module main\n\nfn F(rx: ref mut Receiver[int], tx: ref mut Sender[int]) void {\n    select {\n        "+operation+" => {\n        }\n        default => {\n        }\n    }\n}\n")
			for _, err := range errors {
				if strings.Contains(err.Message, "operation is not selectable") {
					return
				}
			}
			t.Fatalf("%s must not be selectable; got %v", name, errors)
		})
	}
}
