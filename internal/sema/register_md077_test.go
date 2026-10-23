package sema

import (
	"strings"
	"testing"
)

const md077DetachedHeader = `
module main

enum Mode bit[2] {
    Off = 0,
    On = 1,
}

type Header register[32] msb-first {
    Flag: bit,
    Small: bit[2],
    Wide: bit[12],
    Mode: Mode,
    _: bit[15],
}
`

// MD-077 §§ 5–6 and § 16(2)–(3): detached register values are ordinary
// values; a bit field reads as bool, bit[N] (N >= 2) as the smallest
// standard unsigned carrier with a proven 0..2^N-1 interval, a bit-backed
// enum field keeps its nominal type, a proven-fitting value (field-to-field
// or literal) assigns, and explicit bit[N](value) extracts low bits.
func TestMD077DetachedFieldCarriersAndProvenWrites(t *testing.T) {
	input := md077DetachedHeader + `
fn Carriers(raw: uint32) bool {
    let h := Header(raw)
    let f: bool := h.Flag
    let s: uint8 := h.Small
    let w: uint16 := h.Wide
    let m: Mode := h.Mode
    discard s
    discard w
    discard m
    return f
}

fn ProvenFit(raw: uint32, other: uint32) void {
    let a := Header(raw)
    let mut b := Header(other)
    b.Small = a.Small
    b.Wide = a.Wide
    b.Small = 3
}

fn Lossy(raw: uint32, value: uint16) void {
    let mut b := Header(raw)
    b.Small = bit[2](value)
}
`
	assertSemaErrors(t, analyzeSourceRaw(t, input), nil)
}

// MD-077 § 6(5) and § 17(7): an unproven runtime value is never silently
// truncated into a field, and a known out-of-range constant is rejected.
func TestMD077DetachedFieldWritesNeverTruncate(t *testing.T) {
	input := md077DetachedHeader + `
fn Unproven(raw: uint32, value: uint8) void {
    let mut b := Header(raw)
    b.Small = value
}

fn OutOfRange(raw: uint32) void {
    let mut b := Header(raw)
    b.Small = 4
}
`
	errors := analyzeSourceRaw(t, input)
	want := []string{"the value is not proven to fit 0..3", "value 4 does not fit register field Small (bit[2], 0..3)"}
	if len(errors) != len(want) {
		t.Fatalf("errors = %v", errors)
	}
	for index, expected := range want {
		if !strings.Contains(errors[index].Error(), expected) {
			t.Fatalf("error %d = %q, want %q", index, errors[index].Error(), expected)
		}
	}
}

const md077Hardware = `
module main

type InterruptStatus register[8] {
    Ready: bit read-only,
    Pending: bit write-one-clear,
    _: bit[6],
}

type OtherStatus register[8] {
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

    static fn AcknowledgeFirst() void {
        first.Pending = true
    }
}

impl OtherStatus {
    fn ClearPending() void {
        self.Pending = true
    }
}

@address(0x40001000)
let mut first: InterruptStatus

@address(0x40001004)
let mut second: InterruptStatus

@address(0x40001008)
let mut other: OtherStatus
`

// MD-077 §§ 2, 4 and § 16(4), (7), (11): an @address binding denotes a
// HardwareInstance[T] whose public surface is impl T; one type-level impl
// serves several instances; ref of the instance is ref HardwareInstance[T];
// inside impl T the physical fields resolve (also of a global binding).
func TestMD077HardwareInstancesUseTheImplAPI(t *testing.T) {
	analyzer, errors := analyzeSourceWithAnalyzerRaw(t, md077Hardware+`
fn Inspect(device: ref HardwareInstance[InterruptStatus]) bool {
    return device.IsReady
}

fn Clear(device: ref mut HardwareInstance[InterruptStatus]) void {
    device.ClearPending()
}

fn Use() bool {
    first.ClearPending()
    second.ClearPending()
    Clear(second)
    let view := ref first
    return Inspect(first) && view.IsReady
}
`)
	assertSemaErrors(t, errors, nil)
	symbol := analyzer.symbols["first"]
	if symbol.Type.Kind != HardwareInstanceType || typeDisplayName(symbol.Type) != "HardwareInstance[InterruptStatus]" {
		t.Fatalf("binding type = %s, want HardwareInstance[InterruptStatus]", typeDisplayName(symbol.Type))
	}
}

// MD-077 § 17(1)–(3), (10), (11): a binding without impl, public physical
// field access, physical bit borrows (through the instance and through self),
// cross-typing between HardwareInstance[T], T and a same-layout register type,
// a detached copy of the instance, and HardwareInstance of a non-register
// are all rejected.
func TestMD077HardwareInstanceMisuseIsRejected(t *testing.T) {
	input := md077Hardware + `
type Bare register[8] {
    Flag: bit,
    _: bit[7],
}

@address(0x4000100C)
let bare: Bare

fn Detached(value: ref InterruptStatus) bool {
    return true
}

fn Instance(value: ref HardwareInstance[OtherStatus]) bool {
    return true
}

fn Misuse(snapshot: InterruptStatus) bool {
    first.Pending = true
    let bits := ref mut first.Pending
    let copy: InterruptStatus := first
    let detached := Detached(first)
    return Instance(first)
}

fn NotARegister(value: HardwareInstance[int]) void {
}
`
	errors := analyzeSourceRaw(t, input)
	want := []string{
		"hardware-bound register type Bare has no type-owned impl",
		"HardwareInstance[T] requires a register type, got int",
		"register field Pending of hardware instance first is not public",
		"cannot borrow physical register field first.Pending",
		"cannot initialize InterruptStatus with HardwareInstance[InterruptStatus]",
		"argument 1 to Detached must be ref InterruptStatus, got HardwareInstance[InterruptStatus]",
		"argument 1 to Instance must be ref HardwareInstance[OtherStatus], got HardwareInstance[InterruptStatus]",
	}
	joined := ""
	for _, err := range errors {
		joined += err.Error() + "\n"
	}
	for _, expected := range want {
		if !strings.Contains(joined, expected) {
			t.Fatalf("missing %q in:\n%s", expected, joined)
		}
	}
	if len(errors) != len(want) {
		t.Fatalf("got %d errors, want %d:\n%s", len(errors), len(want), joined)
	}

	// A field borrow of `self` is valid on a software register; the member
	// is rejected where it is used on a hardware instance (MD-077 § 4(1)).
	inside := analyzeSourceRaw(t, md077Hardware+`
impl extends OtherStatus {
    fn BorrowBits() bool {
        let bits := ref self.Ready
        return *bits
    }

    fn Indirect() bool {
        return BorrowBits()
    }
}

fn Software(value: OtherStatus) bool {
    return value.BorrowBits() && value.Indirect()
}

fn Hardware() bool {
    return other.Indirect()
}
`)
	if len(inside) != 1 || !strings.Contains(inside[0].Error(), "OtherStatus.Indirect cannot be used on hardware instance of OtherStatus: it borrows a register field of its receiver") {
		t.Fatalf("place-only member on a hardware instance = %v", inside)
	}
}

// registers.md § 11.5 and hardware-register-access.md § 9.4: the effective
// shadow policy is the field override, else the register default, else the
// global noshadow default.
func TestRegisterShadowPolicyInheritance(t *testing.T) {
	analyzer, errors := analyzeSourceWithAnalyzerRaw(t, `
module main

type Configuration register[16] shadow {
    Mode: bit[3],
    CurrentInput: bit read-only noshadow,
    _: bit[12],
}

type Plain register[8] {
    Live: bit[4],
    Kept: bit[4] shadow,
}
`)
	assertSemaErrors(t, errors, nil)
	want := map[string][]RegisterShadowPolicy{
		"Configuration": {RegisterShadowed, RegisterNoShadow, RegisterShadowed},
		"Plain":         {RegisterNoShadow, RegisterShadowed},
	}
	for name, policies := range want {
		register := analyzer.types[name]
		for index, policy := range policies {
			if got := register.RegisterFields[index].Shadow; got != policy {
				t.Fatalf("%s.%s shadow = %q, want %q", name, register.RegisterFields[index].Name, got, policy)
			}
		}
	}
}

const md077Requests = `
module main

type Control register[8] {
    Ready: bit read-only,
    Mode: bit[2],
    Pending: bit write-one-clear,
    Enable: bit write-zero-set,
    _: bit[3],
}

type Events register[8] {
    Ready: bit read-only,
    Count: bit[3] read-clear,
    _: bit[4],
}

impl Control {
    property IsReady: bool {
        get {
            return self.Ready
        }
    }

    fn Acknowledge() void {
        self.Pending = true
        Enable = 0
        self.Mode = 2
    }

    fn Clear(flag: bool) void {
        self.Pending = flag
    }

    fn Bump() void {
        self.Pending |= true
    }

    static fn Direct() void {
        control.Pending = false
        control.Pending |= true
    }
}

impl Events {
    fn IsReady() bool {
        return self.Ready
    }

    fn Drain() uint8 {
        return self.Count
    }
}

@address(0x40002000)
let mut control: Control

@address(0x40002004)
let mut events: Events
`

// hardware-register-access.md §§ 4, 6.2-6.3, 13.1 and MD-077 § 18(4): a field
// step on a hardware instance or impl receiver is a logical request derived
// from the field contract; requests invalid on hardware are rejected on a
// hardware instance and make an impl member software-only (MD-112 for
// non-trigger special writes).
func TestMD077HardwareRequests(t *testing.T) {
	analyzer, errors := analyzeSourceWithAnalyzerRaw(t, md077Requests+`
fn Use(value: Control, snapshot: Events) bool {
    control.Acknowledge()
    let drained := events.Drain()
    discard drained
    value.Clear(true)
    snapshot.IsReady()
    control.Clear(true)
    control.Bump()
    return control.IsReady && events.IsReady()
}
`)
	want := []string{
		"hardware register access writes a value other than the trigger constant to write-one-clear register field Pending",
		"register field Pending with write-one-clear semantics cannot use compound assignment",
		"Control.Clear cannot be used on hardware instance of Control: it writes a value other than the trigger constant to write-one-clear register field Pending",
		"Control.Bump cannot be used on hardware instance of Control: it uses compound assignment on write-one-clear register field Pending",
		"Events.IsReady cannot be used on hardware instance of Events: it observes Ready, which would also perform the destructive read-clear read of Count",
	}
	joined := ""
	for _, err := range errors {
		joined += err.Error() + "\n"
	}
	for _, expected := range want {
		if !strings.Contains(joined, expected) {
			t.Fatalf("missing %q in:\n%s", expected, joined)
		}
	}
	if len(errors) != len(want) {
		t.Fatalf("got %d errors, want %d:\n%s", len(errors), len(want), joined)
	}
	requests := map[string]ResolvedRegisterHardwareRequest{}
	for member, plan := range analyzer.resolvedRegisterFields {
		if plan.Request != nil {
			requests[plan.Member+"/"+member.Property.Value] = *plan.Request
		}
	}
	for identifier, use := range analyzer.implicitReceiverUses {
		if use.RegisterField != nil && use.RegisterField.Request != nil {
			requests[use.RegisterField.Member+"/"+identifier.Value] = *use.RegisterField.Request
		}
	}
	for key, expected := range map[string]ResolvedRegisterHardwareRequest{
		"Control.IsReady.get/Ready":   {Kind: RegisterRequestObserve, Shadow: RegisterNoShadow},
		"Control.Acknowledge/Pending": {Kind: RegisterRequestAction, Effect: "clear", Shadow: RegisterNoShadow},
		"Control.Acknowledge/Enable":  {Kind: RegisterRequestAction, Effect: "set", Shadow: RegisterNoShadow},
		"Control.Acknowledge/Mode":    {Kind: RegisterRequestWriteValue, Shadow: RegisterNoShadow},
		"Events.Drain/Count":          {Kind: RegisterRequestDestructiveObserve, Effect: "clear", Shadow: RegisterNoShadow},
	} {
		if got, ok := requests[key]; !ok || got != expected {
			t.Fatalf("request %s = %+v (%v), want %+v; all: %+v", key, got, ok, expected, requests)
		}
	}
}

// An implicit field name in a register impl is `self.Field` and is checked
// against the field, not its carrier.
func TestImplicitRegisterFieldWritesUseFieldRules(t *testing.T) {
	errors := analyzeSourceRaw(t, `
module main

type H register[8] {
    Small: bit[2],
    Flag: bit,
    _: bit[5],
}

impl H {
    fn Set() void {
        Small = 3
        Flag = 1
        Small = 9
    }
}
`)
	if len(errors) != 1 || !strings.Contains(errors[0].Error(), "value 9 does not fit register field Small (bit[2], 0..3)") {
		t.Fatalf("implicit field writes = %v", errors)
	}
}

// hardware-register-access.md §§ 7–8 and MD-077 § 14: Read(), Write() and
// Write(value) are compiler-known operations reachable only inside impl T on
// a hardware receiver; they make a member hardware-only, and Read/Write are
// reserved member names in a register impl (owner decision 2026-10-23).
func TestMD077RegisterReadWriteOperations(t *testing.T) {
	analyzer, errors := analyzeSourceWithAnalyzerRaw(t, `
module main

type Control register[8] shadow {
    Mode: bit[2],
    Ready: bit read-only noshadow,
    _: bit[5],
}

type Plain register[8] {
    Value: bit[8],
}

impl Control {
    fn Snapshot() Control {
        return self.Read()
    }

    fn Commit() void {
        Write()
    }

    fn Replace(value: Control) void {
        self.Write(value)
    }

    fn Wrapped() Control {
        return Snapshot()
    }

    static fn Global() Control {
        return control.Read()
    }

    fn Read() Control {
        return self
    }
}

impl Plain {
    fn Commit() void {
        self.Write()
    }

    fn Local(value: Plain) Plain {
        return value.Read()
    }
}

@address(0x40003000)
let mut control: Control

@address(0x40003004)
let mut plain: Plain

fn Use(value: Control) Control {
    let snapshot := control.Snapshot()
    control.Commit()
    discard control.Read()
    return value.Wrapped()
}
`)
	want := []string{
		"Read is reserved in impl Control for the compiler-known hardware register operation Read()",
		"Write() commits pending shadow write intents, but register Plain has no writable shadow field",
		"Read() is a hardware register operation; value is a software Plain value",
		"Read() of hardware instance control is not public; it is reached only inside impl Control",
		"Control.Wrapped requires a hardware instance receiver: it calls Read()",
	}
	joined := ""
	for _, err := range errors {
		joined += err.Error() + "\n"
	}
	for _, expected := range want {
		if !strings.Contains(joined, expected) {
			t.Fatalf("missing %q in:\n%s", expected, joined)
		}
	}
	if len(errors) != len(want) {
		t.Fatalf("got %d errors, want %d:\n%s", len(errors), len(want), joined)
	}
	kinds := map[RegisterOperationKind]RegisterRootKind{}
	for _, operation := range analyzer.resolvedRegisterOperations {
		kinds[operation.Kind] = operation.Root
	}
	if len(kinds) != 3 || kinds[RegisterOperationCommit] != RegisterRootImplReceiver || kinds[RegisterOperationWriteWhole] != RegisterRootImplReceiver {
		t.Fatalf("resolved register operations = %+v", analyzer.resolvedRegisterOperations)
	}
}
