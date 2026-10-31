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

@address(StatusEndpoint)
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

hardware region RegisterSpace {
    domain: "cpu-physical",
    kind: HardwareRegionKind.MMIO,
    base: 0x40020000,
    size: 0x10000,
    readable: true,
    writable: true,
}
hardware resource StatusState { width: 8, }
hardware endpoint StatusEndpoint {
    kind: HardwareEndpointKind.MemoryMapped,
    region: RegisterSpace,
    resource: StatusState,
    offset: 0x1000,
    size: 1,
    alignment: 1,
    read_widths: [8],
    write_widths: [8],
    read_footprint: HardwareFootprintKind.Whole,
    write_footprint: HardwareFootprintKind.Whole,
    read_effect: HardwareReadEffect.NonDestructive,
    write_operation: HardwareWriteOperation.Replace,
    ordering: HardwareOrdering.Volatile,
    completion: HardwareCompletion.Issued,
    atomicity: HardwareAtomicity.Unspecified,
    context: HardwareAccessContext.Current,
    fault: HardwareFaultContract.Platform,
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
