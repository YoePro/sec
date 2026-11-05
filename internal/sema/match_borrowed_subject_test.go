package sema

import (
	"strings"
	"testing"

	"sec/internal/diagnostics"
)

// A match over a `ref`/`ref mut` subject matches the referent: payload bindings
// may copy a copyable payload or borrow with `ref` (and `ref mut` through a
// `ref mut` subject), but never move a payload out (flowcontrol_match.md § 14).
func TestMatchOverBorrowedSubject(t *testing.T) {
	errors := analyzeSourceRaw(t, `
module main

type Handle struct {
    id: int,
}

impl Handle {
    free {
    }
}

fn Take(handle: Handle) void {
}

fn Borrowed(value: ref Option[Handle]) int {
    match value {
        Some(ref handle) => {
            return handle.id
        }
        None => {
            return 0
        }
    }
}

fn Mutable(value: ref mut Option[Handle]) int {
    match value {
        Some(ref mut handle) => {
            handle.id = 3
            return handle.id
        }
        None => {
            return 0
        }
    }
}

fn Copyable(value: ref Option[int], result: ref Result[int, IndexError]) int {
    let first := match value {
        Some(number) => number
        None => 0
    }
    return match result {
        Ok(number) => number + first
        Err(_) => -1
    }
}

fn Moves(value: ref Option[Handle]) void {
    match value {
        Some(handle) => {
            Take(<-handle)
        }
        None => {
        }
    }
}

fn SharedMut(value: ref Option[Handle]) int {
    match value {
        Some(ref mut handle) => {
            return handle.id
        }
        None => {
            return 0
        }
    }
}
`)
	if len(errors) != 2 {
		t.Fatalf("errors = %v, want the move and the mutable borrow through ref", errors)
	}
	if errors[0].ID != diagnostics.UnionPayloadMoveStorage || !strings.Contains(errors[0].Message, "out of a borrowed match subject") {
		t.Fatalf("move error = %+v", errors[0])
	}
	if !strings.Contains(errors[1].Message, "cannot create mutable reference to immutable place") {
		t.Fatalf("ref mut error = %+v", errors[1])
	}
}
