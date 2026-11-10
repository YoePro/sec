package sema

import "testing"

// A borrowed-backing Arena ends its backing borrow at implicit destruction on
// every continuing path (both if arms, loop iterations, an early return inside
// the Arena scope, match arms, sequential re-borrowing), and every escape that
// would let the Arena outlive its backing (stored into a returned aggregate,
// returned while backed by a parameter, consumed by a call) is rejected.
//
// Rules:
//   - rules/memory/arena.md — § 10 "Borrowed fixed Arena", § 4.2(3)
//   - rules/memory/destruction.md — scope-exit destruction
func TestArenaBackingBorrowEndsOnEveryPathAndCannotEscape(t *testing.T) {
	assertSemaErrors(t, analyzeSourceRaw(t, `
module main

fn ConditionalScope(c: bool) byte {
    let mut buffer: byte[2] := [0u, 0u]
    if c {
        let mut arena := Arena.FromBuffer(buffer)
        arena.Reset()
    } else {
        let mut other := Arena.FromBuffer(buffer)
        other.Reset()
    }
    return buffer[0]
}

fn LoopScope(n: int) byte {
    let mut buffer: byte[2] := [0u, 0u]
    let mut i := 0
    while i < n {
        let mut arena := Arena.FromBuffer(buffer)
        arena.Reset()
        i += 1
    }
    return buffer[0]
}

fn EarlyReturn(c: bool) byte {
    let mut buffer: byte[2] := [0u, 0u]
    if true {
        let mut arena := Arena.FromBuffer(buffer)
        if c {
            return 1u
        }
        arena.Reset()
    }
    return buffer[0]
}

fn MatchScope(v: Option[int]) byte {
    let mut buffer: byte[2] := [0u, 0u]
    match v {
        Some(_) => {
            let mut arena := Arena.FromBuffer(buffer)
            arena.Reset()
        }
        None => {}
    }
    return buffer[0]
}

fn Reinit() byte {
    let mut buffer: byte[2] := [0u, 0u]
    let mut first := Arena.FromBuffer(buffer)
    first.Release()
    let mut second := Arena.FromBuffer(buffer)
    second.Release()
    return buffer[0]
}
`), nil)
	assertSemaErrors(t, analyzeSourceRaw(t, `
module main

type Holder struct {
    arena: Arena,
}

fn EscapeIntoStruct() Holder {
    let mut buffer: byte[2] := [0u, 0u]
    let arena := Arena.FromBuffer(buffer)
    return Holder { arena: <-arena }
}

fn EscapeViaParam(buffer: ref mut byte[]) Arena {
    return Arena.FromBuffer(buffer)
}

fn Consume(arena: Arena) void {}

fn EscapeViaConsume() void {
    let mut buffer: byte[2] := [0u, 0u]
    let arena := Arena.FromBuffer(buffer)
    Consume(<-arena)
}
`), []string{
		"cannot move arena arena out of its binding while it controls borrowed backing buffer at 11:30, previous declaration at 10:23",
		"cannot return Arena.FromBuffer(...): its borrowed backing must outlive the Arena at 15:17",
		"cannot move arena arena out of its binding while it controls borrowed backing buffer at 23:15, previous declaration at 22:23",
	})
}
