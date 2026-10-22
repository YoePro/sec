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

	inside := analyzeSourceRaw(t, md077Hardware+`
impl extends OtherStatus {
    fn BorrowBits() void {
        let bits := ref mut self.Pending
    }
}
`)
	if len(inside) != 1 || !strings.Contains(inside[0].Error(), "cannot borrow physical register field self.Pending") {
		t.Fatalf("borrow of physical bits through self = %v", inside)
	}
}
