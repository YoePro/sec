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

@address(0x40021000)
let status: Status

fn InvalidDeclarationMove() Status {
	let moved :<- status
	return moved
}

fn InvalidReturnMove() Status {
	return <-status
}

fn InvalidProjectedMove() void {
	let moved :<- status.Value
	discard moved
}

fn ValidSnapshotMove() Status {
	let snapshot := status
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
