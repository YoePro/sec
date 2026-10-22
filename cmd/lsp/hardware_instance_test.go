package main

import (
	"strings"
	"testing"
)

const hardwareInstanceSource = `module main

type InterruptStatus register[8] {
    Ready: bit read-only,
    Pending: bit write-one-clear,
    _: bit[6],
}

impl InterruptStatus {
    property IsReady: bool {
        get {
            return self.Ready
        }
    }

    fn ClearPending() void {
        self.Pending = true
    }
}

@address(0x40001000)
let mut interruptStatus: InterruptStatus

fn Inspect(device: ref HardwareInstance[InterruptStatus]) bool {
    return device.IsReady
}

fn Acknowledge() void {
    interruptStatus.@@
}
`

// MD-077 §§ 2(2), 4(7): completion on a hardware instance offers the public
// members of `impl T`, never the physical register fields, and hover reports
// the binding's resolved instance type together with its register type.
func TestHardwareInstanceCompletionAndHover(t *testing.T) {
	offered := map[string]bool{}
	for _, label := range completionLabelsAt(t, hardwareInstanceSource, "@@") {
		offered[label] = true
	}
	if !offered["IsReady"] || !offered["ClearPending"] {
		t.Fatalf("completion lacks the public impl members: %v", offered)
	}
	if offered["Ready"] || offered["Pending"] {
		t.Fatalf("completion offers physical register fields: %v", offered)
	}
	source := strings.Replace(hardwareInstanceSource, "interruptStatus.@@", "interruptStatus.ClearPending()", 1)
	offset := strings.Index(source, "interruptStatus: InterruptStatus")
	result, ok := hoverForSource("", source, offsetPosition(source, offset))
	if !ok || !strings.Contains(result.Contents.Value, "HardwareInstance[InterruptStatus]") {
		t.Fatalf("hover does not report the resolved instance type: %+v, %v", result, ok)
	}
}
