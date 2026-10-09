package sema

import (
	"os"
	"testing"
)

// Arena.FromBuffer borrows mutable byte storage exclusively for the whole Arena
// lifetime: the backing is borrowed implicitly or through ref mut, local moves
// carry the borrow, Release and scope end release it, and any transfer the
// caller could not see (return, consuming call) is rejected. Arena operations
// also work through ref mut Arena, with allocations rooted in the caller.
//
// Rules:
//   - rules/memory/arena.md — § 10 "Borrowed fixed Arena", § 4.2(3), § 43, § 45(2), § 68(3)–(4)
//   - rules/analysis/escape_analysis.md — "Call-boundary semantic classification"
func TestArenaFromBufferBorrowsBackingForArenaLifetime(t *testing.T) {
	valid, err := os.ReadFile("../../testdata/sema/arena_from_buffer_valid.sec")
	if err != nil {
		t.Fatal(err)
	}
	assertSemaErrors(t, analyzeSourceRaw(t, string(valid)), nil)

	invalid, err := os.ReadFile("../../testdata/sema/arena_from_buffer_invalid.sec")
	if err != nil {
		t.Fatal(err)
	}
	assertSemaErrors(t, analyzeSourceRaw(t, string(invalid)), []string{
		"cannot read buffer[0] while it is mutably borrowed at 10:24, previous declaration at 9:27",
		"cannot read buffer while it is mutably borrowed at 18:40, previous declaration at 17:27",
		"cannot read buffer[0] while it is mutably borrowed at 27:24, previous declaration at 25:23",
		"cannot return arena arena: its backing buffer is borrowed here and must outlive the Arena at 35:12, previous declaration at 34:23",
		"cannot return Arena.FromBuffer(...): its borrowed backing must outlive the Arena at 39:17",
		"cannot move arena arena out of its binding while it controls borrowed backing buffer at 45:15, previous declaration at 44:23",
		"cannot create mutable reference to immutable variable buffer at 50:39",
		"Arena.FromBuffer requires mutable contiguous byte storage borrowed as ref mut byte[], such as a let mut byte[] or byte[N], got int[] at 56:39",
		"Arena.Release cannot consume arena arena through a borrowed reference at 61:5",
		"Arena.Reset requires mutable arena arena at 65:5",
		"cannot use values after arena arena was reset at 71:5, previous declaration at 69:23",
	})
}
