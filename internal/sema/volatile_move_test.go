package sema

import (
	"strings"
	"testing"
)

// Rules:
//   - rules/memory/copy_move.md — §19.1 and §19.2
//   - rules/memory/ownership.md — §30
func TestExplicitMoveRejectsAddressedVolatileStorageButAllowsLocalSnapshot(t *testing.T) {
	errors := analyzeSourceRaw(t, `module main

type Status register[8] {
	Value: bit[8],
}

impl Status {
	fn Snapshot() Status {
		return self
	}

	static fn InvalidProjectedMove() void {
		let moved :<- status.Value
		discard moved
	}
}

@address(0x40021000)
let status: Status

fn InvalidDeclarationMove() void {
	let moved :<- status
	discard moved
}

fn InvalidReturnMove() HardwareInstance[Status] {
	return <-status
}

// MD-077: a detached snapshot comes from the type-owned impl; it is an
// ordinary value and may be moved.
fn ValidSnapshotMove() Status {
	let snapshot := status.Snapshot()
	let moved :<- snapshot
	return moved
}
`)
	if len(errors) != 3 {
		t.Fatalf("volatile storage move errors = %v", errors)
	}
	for _, err := range errors {
		if !strings.Contains(err.Message, "cannot move ownership out of volatile/MMIO storage status") ||
			!strings.Contains(err.Message, "local snapshot") {
			t.Fatalf("unexpected volatile storage move error: %v", err)
		}
	}
}
