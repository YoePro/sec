package sema

import (
	"strings"
	"testing"

	"sec/internal/diagnostics"
)

// An unsafe function must not implicitly convert to a safe function value
// (unsafe.md §39(2)). FunctionType has no unsafe form, so every named-function
// value position rejects an unsafe target, also inside unsafe contexts.
func TestUnsafeFunctionValueIsRejected(t *testing.T) {
	const declarations = `module main

unsafe fn Raw(value: int) int {
    return value
}

unsafe extern "C" fn abs(value: int32) int32

fn Apply(callback: fn(int) int, value: int) int {
    return callback(value)
}

`
	for _, body := range []string{
		"let f: fn(int) int := Raw\ndiscard f",
		"let g := Raw\ndiscard g",
		"discard Apply(Raw, 1)",
		"unsafe {\n    let f: fn(int) int := Raw\n    discard f\n}",
		"let h := abs\ndiscard h",
	} {
		source := declarations + "fn main() void {\n" + indentLines(body) + "\n}\n"
		errors := analyzeSource(t, source)
		if len(errors) != 1 || errors[0].ID != diagnostics.UnsafeFunctionValueMismatch {
			t.Fatalf("%q errors = %v, want one %s", body, errors, diagnostics.UnsafeFunctionValueMismatch)
		}
		if !strings.Contains(errors[0].Message, "cannot be used as a function value") {
			t.Fatalf("%q message = %q", body, errors[0].Message)
		}
	}
	errors := analyzeSource(t, declarations+"fn main() void {\n    let h := abs\n    discard h\n}\n")
	if len(errors) != 1 || !strings.Contains(errors[0].Message, "unsafe extern function abs") {
		t.Fatalf("extern errors = %v", errors)
	}
}

// Direct unsafe calls and safe wrappers used as values remain valid
// (unsafe.md §39(3)).
func TestUnsafeFunctionCallsAndSafeWrapperValuesRemainValid(t *testing.T) {
	source := `module main

unsafe fn Raw(value: int) int {
    return value
}

fn Wrapped(value: int) int {
    unsafe {
        return Raw(value)
    }
}

fn Apply(callback: fn(int) int, value: int) int {
    return callback(value)
}

fn main() int {
    unsafe {
        discard Raw(1)
    }
    let f: fn(int) int := Wrapped
    discard f
    return Apply(Wrapped, 2)
}
`
	if errors := analyzeSource(t, source); len(errors) != 0 {
		t.Fatalf("errors = %v", errors)
	}
}

func indentLines(body string) string {
	lines := strings.Split(body, "\n")
	for index, line := range lines {
		lines[index] = "    " + line
	}
	return strings.Join(lines, "\n")
}

// Unsafe does not suppress effects: an unsafe block or an unsafe fn between a
// guaranteed function and a panic, allocation or blocking site leaves
// @noPanic, @noAlloc and @noBlock violated (unsafe.md §15(1), §15(4),
// §15(5), §16(1), §17(4), §18(1)).
func TestUnsafeDoesNotSuppressEffectGuarantees(t *testing.T) {
	source := `module main

type Message struct {
	value: int,
}

fn Loud() void {
	panic "boom"
}

fn Grow(buffer: ref mut byte[]) void {
	let mut arena := Arena.FromBuffer(buffer)
	let value := arena.New[int]()
	arena.Release()
}

unsafe fn RawLoud() void {
	Loud()
}

unsafe fn RawGrow(buffer: ref mut byte[]) void {
	Grow(buffer)
}

unsafe fn RawReceive(rx: Receiver[Message]) void {
	discard rx.Receive()
}

@noPanic
fn PanicBlock() void {
	unsafe {
		Loud()
	}
}

@noPanic
fn PanicFn() void {
	unsafe {
		RawLoud()
	}
}

@noAlloc
fn AllocBlock(buffer: ref mut byte[]) void {
	unsafe {
		Grow(buffer)
	}
}

@noAlloc
fn AllocFn(buffer: ref mut byte[]) void {
	unsafe {
		RawGrow(buffer)
	}
}

@noBlock
fn BlockBlock(rx: Receiver[Message]) void {
	unsafe {
		discard rx.Receive()
	}
}

@noBlock
fn BlockFn(rx: Receiver[Message]) void {
	unsafe {
		RawReceive(<-rx)
	}
}
`
	want := map[string]string{
		"PanicBlock": diagnostics.NoPanicViolation,
		"PanicFn":    diagnostics.NoPanicViolation,
		"AllocBlock": diagnostics.NoAllocViolation,
		"AllocFn":    diagnostics.NoAllocViolation,
		"BlockBlock": diagnostics.NoBlockViolation,
		"BlockFn":    diagnostics.NoBlockViolation,
	}
	got := map[string]string{}
	for _, err := range analyzeSourceRaw(t, source) {
		name := strings.Fields(strings.TrimPrefix(err.Message, "function "))[0]
		if _, expected := want[name]; !expected {
			t.Errorf("unexpected error: %s %s", err.ID, err.Message)
			continue
		}
		got[name] = err.ID
	}
	for name, id := range want {
		if got[name] != id {
			t.Errorf("%s diagnostic = %q, want %s", name, got[name], id)
		}
	}
}
