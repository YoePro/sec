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

// registers.md § 11.5: hover shows the effective shadow policy Sema resolved
// (field override, register default, global noshadow).
func TestHoverShowsEffectiveRegisterShadowPolicy(t *testing.T) {
	source := `module main

type Configuration register[2] shadow {
    Mode: bit,
    Input: bit read-only noshadow,
}

fn Read(value: Configuration) bool {
    return value.Mode && value.Input
}
`
	for field, want := range map[string]string{"Mode": "Shadow: `shadow`", "Input": "Shadow: `noshadow`"} {
		offset := strings.LastIndex(source, field)
		result, ok := hoverForSource("", source, offsetPosition(source, offset))
		if !ok || !strings.Contains(result.Contents.Value, want) {
			t.Fatalf("%s hover = %+v, %v; want %q", field, result, ok, want)
		}
	}
}

// hardware-register-access.md § 33.3: hover shows Sema's logical hardware
// request of a register field step and the compiler-known Read(); completion
// after `self.` in a register impl offers Read/Write.
func TestHardwareRegisterRequestHoverAndCompletion(t *testing.T) {
	source := `module main

type Status register[8] {
    Ready: bit read-only,
    Pending: bit write-one-clear,
    _: bit[6],
}

impl Status {
    fn Clear() void {
        self.Pending = true
    }

    fn Snapshot() Status {
        return self.Read()
    }

    static fn Direct() bool {
        return status.Ready
    }
}

@address(0x40004000)
let mut status: Status
`
	for marker, want := range map[string]string{
		"Pending = true": "On a hardware instance: `action (clear) mask 0b1`. On a software value: an ordinary field.",
		"Ready\n":        "Hardware request: `observe`",
		"Read()":         "fn Read() Status",
	} {
		offset := strings.Index(source, marker)
		result, ok := hoverForSource("", source, offsetPosition(source, offset))
		if !ok || !strings.Contains(result.Contents.Value, want) {
			t.Fatalf("hover at %q = %+v, %v; want %q", marker, result, ok, want)
		}
	}
	completion := strings.Replace(source, "return self.Read()", "return self.Re", 1)
	items := completeSource("", completion, strings.Index(completion, "self.Re")+len("self.Re"))
	found := false
	for _, item := range items {
		found = found || item.Label == "Read"
	}
	if !found {
		t.Fatalf("self.Re completion = %+v, want Read", items)
	}
}
