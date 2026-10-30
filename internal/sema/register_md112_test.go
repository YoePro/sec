package sema

import (
	"strings"
	"testing"
)

// MD-112: specialized hardware writes are per-bit action requests. Trigger
// and neutral constants of all six behaviors, multibit masks, all-neutral
// no-ops and runtime values are legal and resolve to the logical action
// mask; detached registers and ordinary fields are unaffected.
// Rules: rules/declarations/registers.md — § 11.3 (Amendment MD-112);
// rules/corrections/applied/MD-112-specialized-register-write-non-trigger-and-multibit-correction-20261030.md — §§ 2–4, 8(2).
func TestMD112SpecializedWriteActionMasks(t *testing.T) {
	analyzer, errors := analyzeSourceWithAnalyzerRaw(t, `
module main

type Actions register[16] {
    OneClear: bit write-one-clear,
    OneSet: bit write-one-set,
    OneToggle: bit write-one-toggle,
    ZeroClear: bit write-zero-clear,
    ZeroSet: bit write-zero-set,
    ZeroToggle: bit write-zero-toggle,
    Pending: bit[4] write-one-clear,
    Errors: bit[4] write-zero-clear,
    Mode: bit[2],
}

@address(0x40003000)
let mut actions: Actions

impl Actions {
    fn Trigger() void {
        self.OneClear = true
        self.OneSet = 1
        self.OneToggle = true
        self.ZeroClear = false
        self.ZeroSet = 0
        self.ZeroToggle = false
    }

    fn Neutral() void {
        self.OneClear = false
        self.OneSet = 0
        self.OneToggle = false
        self.ZeroClear = true
        self.ZeroSet = 1
        self.ZeroToggle = true
    }

    fn Multibit() void {
        self.Pending = 0b0101
        self.Errors = 0b1010
        self.Pending = 0
        self.Errors = 0b1111
        self.Mode = 2
    }

    fn Runtime(flag: bool, bits: uint8) void {
        self.OneToggle = flag
        self.Pending = bit[4](bits)
    }
}

fn Use() void {
    actions.Trigger()
    actions.Neutral()
    actions.Multibit()
    actions.Runtime(true, 3)
}

fn Detached(value: Actions) Actions {
    let mut copy := value
    copy.Pending = 0b0011
    copy.OneClear = false
    return copy
}
`)
	if len(errors) != 0 {
		t.Fatal(errors)
	}
	type key struct {
		line  int
		field string
	}
	requests := map[key]ResolvedRegisterHardwareRequest{}
	for member, plan := range analyzer.resolvedRegisterFields {
		if plan.Request != nil {
			requests[key{member.Property.Token.Line, member.Property.Value}] = *plan.Request
		}
		if plan.Root == RegisterRootDetached && plan.Request != nil {
			t.Fatalf("detached write got a hardware request: %+v", plan)
		}
	}
	action := func(effect, mask string) ResolvedRegisterHardwareRequest {
		return ResolvedRegisterHardwareRequest{Kind: RegisterRequestAction, Effect: effect, Shadow: RegisterNoShadow, ActionMask: mask}
	}
	noop := func(effect, mask string) ResolvedRegisterHardwareRequest {
		return ResolvedRegisterHardwareRequest{Kind: RegisterRequestNoOp, Effect: effect, Shadow: RegisterNoShadow, ActionMask: mask}
	}
	for at, want := range map[key]ResolvedRegisterHardwareRequest{
		{21, "OneClear"}: action("clear", "1"), {22, "OneSet"}: action("set", "1"), {23, "OneToggle"}: action("toggle", "1"),
		{24, "ZeroClear"}: action("clear", "1"), {25, "ZeroSet"}: action("set", "1"), {26, "ZeroToggle"}: action("toggle", "1"),
		{30, "OneClear"}: noop("clear", "0"), {31, "OneSet"}: noop("set", "0"), {32, "OneToggle"}: noop("toggle", "0"),
		{33, "ZeroClear"}: noop("clear", "0"), {34, "ZeroSet"}: noop("set", "0"), {35, "ZeroToggle"}: noop("toggle", "0"),
		{39, "Pending"}: action("clear", "0101"), {40, "Errors"}: action("clear", "0101"),
		{41, "Pending"}: noop("clear", "0000"), {42, "Errors"}: noop("clear", "0000"),
		{43, "Mode"}:      {Kind: RegisterRequestWriteValue, Shadow: RegisterNoShadow},
		{47, "OneToggle"}: {Kind: RegisterRequestAction, Effect: "toggle", Shadow: RegisterNoShadow, ValueDependent: true},
		{48, "Pending"}:   {Kind: RegisterRequestAction, Effect: "clear", Shadow: RegisterNoShadow, ValueDependent: true},
	} {
		if got, ok := requests[at]; !ok || got != want {
			t.Errorf("line %d %s = %+v (%v), want %+v", at.line, at.field, got, ok, want)
		}
	}
	if len(requests) != 19 {
		t.Errorf("requests = %+v", requests)
	}

	invalid := analyzeSourceRaw(t, `
module main

type Status register[8] {
    Pending: bit[4] write-one-clear,
    _: bit[4],
}

impl Status {
    fn Bad() void {
        self.Pending = 16
        self.Pending |= 1
    }
}
`)
	joined := ""
	for _, err := range invalid {
		joined += err.Message + "\n"
	}
	if len(invalid) != 2 || !strings.Contains(joined, "compound assignment") {
		t.Fatalf("errors = %s", joined)
	}
}
