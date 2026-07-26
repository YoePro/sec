package sema

import "testing"

func TestReturnReferenceToParameterIsAllowed(t *testing.T) {
	input := `
module main

fn Identity(value: ref int) ref int {
    return value
}
`

	errors := analyzeSourceRaw(t, input)
	assertSemaErrors(t, errors, nil)
}

func TestReturnReferenceToLocalIsRejected(t *testing.T) {
	input := `
module main

fn Invalid() ref int {
    let value := 10
    return ref value
}
`

	errors := analyzeSourceRaw(t, input)
	assertSemaErrors(t, errors, []string{
		"function Invalid cannot return reference to local variable value at 6:12, previous declaration at 5:9",
	})
}

func TestReturnLocalReferenceVariableIsRejected(t *testing.T) {
	input := `
module main

fn Invalid() ref int {
    let value := 10
    let view := ref value
    return view
}
`

	errors := analyzeSourceRaw(t, input)
	assertSemaErrors(t, errors, []string{
		"function Invalid cannot return reference to local variable value at 7:12, previous declaration at 5:9",
	})
}

func TestReturnExplicitLocalReferenceVariableIsRejected(t *testing.T) {
	input := `
module main

fn Invalid() ref int {
    let value := 10
    let view: ref int := ref value
    return view
}
`

	errors := analyzeSourceRaw(t, input)
	assertSemaErrors(t, errors, []string{
		"function Invalid cannot return reference to local variable value at 7:12, previous declaration at 5:9",
	})
}

func TestResultOkReferenceToLocalIsRejected(t *testing.T) {
	input := `
module main

enum IOError {
    Invalid,
}

fn Invalid() Result[ref int, IOError] {
    let value := 10
    return Ok(ref value)
}
`

	errors := analyzeSourceRaw(t, input)
	assertSemaErrors(t, errors, []string{
		"function Invalid cannot return reference to local variable value at 10:15, previous declaration at 9:9",
	})
}

func TestReturnStructContainingLocalReferenceIsRejected(t *testing.T) {
	input := `
module main

type View struct {
    value: ref int,
}

fn Invalid() View {
    let value := 10
    return View{
        value: ref value,
    }
}
`

	errors := analyzeSourceRaw(t, input)
	assertSemaErrors(t, errors, []string{
		"function Invalid cannot return value containing reference to local variable value at 10:12, previous declaration at 9:9",
	})
}

func TestReturnLambdaCapturingLocalReferenceIsRejected(t *testing.T) {
	input := `
module main

fn Invalid() fn() int {
    let value := 10
    let view := ref value
    return capture(view) fn() int {
        return value
    }
}
`

	errors := analyzeSourceRaw(t, input)
	assertSemaErrors(t, errors, []string{
		"function Invalid cannot return lambda capturing reference to local variable value at 7:20, previous declaration at 5:9",
	})
}

func TestReturnSliceIntoLocalArrayIsRejected(t *testing.T) {
	input := `
module main

fn Invalid() ref byte[] {
    let local: byte[4] := [1, 2, 3, 4]
    return ref local[..]
}
`

	errors := analyzeSourceRaw(t, input)
	assertSemaErrors(t, errors, []string{
		"function Invalid cannot return reference to local variable local at 6:12, previous declaration at 5:9",
	})
}

func TestStoreLocalReferenceIntoCallerOwnedFieldIsRejected(t *testing.T) {
	input := `
module main

type Holder struct {
    view: ref int,
}

fn Invalid(target: ref mut Holder) void {
    let value := 10
    target.view = ref value
}
`

	errors := analyzeSourceRaw(t, input)
	assertSemaErrors(t, errors, []string{
		"cannot store reference to local variable value into target at 10:19, previous declaration at 9:9",
	})
}

func TestReturnLocalAggregateAfterStoredLocalReferenceIsRejected(t *testing.T) {
	input := `
module main

type Holder struct {
    view: ref int,
}

fn Invalid() Holder {
    let value := 10
    let mut holder := Holder {
        view: ref value,
    }
    return holder
}
`

	errors := analyzeSourceRaw(t, input)
	assertSemaErrors(t, errors, []string{
		"function Invalid cannot return value containing reference to local variable value at 13:12, previous declaration at 9:9",
	})
}

func TestReturnIndirectLambdaCapturingLocalReferenceIsRejected(t *testing.T) {
	input := `
module main

fn Invalid() fn() int {
    let value := 10
    let view := ref value
    let callback := capture(view, value) fn() int {
        return value
    }
    return callback
}
`

	errors := analyzeSourceRaw(t, input)
	assertSemaErrors(t, errors, []string{
		"function Invalid cannot return value containing reference to local variable value at 10:12, previous declaration at 5:9",
	})
}

func TestArenaResetInvalidatesAllocatedSlice(t *testing.T) {
	input := `
module main

fn Invalid() Result[void, AllocationError] {
    let mut mem: Arena
    let storage := try mem.Alloc[byte](16)
    mem.Reset()
    let length := storage.len
    return Ok()
}
`

	errors := analyzeSourceRaw(t, input)
	assertSemaErrors(t, errors, []string{
		"cannot use storage after arena mem was reset at 8:19, previous declaration at 6:24",
	})
}

func TestArenaResetInContinuingBranchInvalidatesAllocatedSlice(t *testing.T) {
	input := `
module main

fn Invalid(condition: bool) Result[void, AllocationError] {
    let mut mem: Arena
    let storage := try mem.Alloc[byte](16)
    if condition {
        mem.Reset()
    }
    let length := storage.len
    return Ok()
}
`

	errors := analyzeSourceRaw(t, input)
	assertSemaErrors(t, errors, []string{
		"cannot use storage after arena mem was reset at 10:19, previous declaration at 6:24",
	})
}

func TestArenaResetInReturningBranchDoesNotInvalidateContinuingPath(t *testing.T) {
	input := `
module main

fn Valid(condition: bool) Result[void, AllocationError] {
    let mut mem: Arena
    let storage := try mem.Alloc[byte](16)
    if condition {
        mem.Reset()
        return Ok()
    }
    let length := storage.len
    return Ok()
}
`

	errors := analyzeSourceRaw(t, input)
	assertSemaErrors(t, errors, nil)
}

func TestArenaResetInForLoopInvalidatesAllocatedSlice(t *testing.T) {
	input := `
module main

fn Invalid() Result[void, AllocationError] {
    let mut mem: Arena
    let storage := try mem.Alloc[byte](16)
    for i in 0..1 {
        mem.Reset()
    }
    let length := storage.len
    return Ok()
}
`

	errors := analyzeSourceRaw(t, input)
	assertSemaErrors(t, errors, []string{
		"cannot use storage after arena mem was reset at 10:19, previous declaration at 6:24",
	})
}

func TestArenaResetInWhileLoopInvalidatesAllocatedSlice(t *testing.T) {
	input := `
module main

fn Invalid(condition: bool) Result[void, AllocationError] {
    let mut mem: Arena
    let storage := try mem.Alloc[byte](16)
    while condition {
        mem.Reset()
        break
    }
    let length := storage.len
    return Ok()
}
`

	errors := analyzeSourceRaw(t, input)
	assertSemaErrors(t, errors, []string{
		"cannot use storage after arena mem was reset at 11:19, previous declaration at 6:24",
	})
}

func TestArenaResetBeforeLoopBreakInvalidatesAllocatedSlice(t *testing.T) {
	input := `
module main

fn Invalid() Result[void, AllocationError] {
    let mut mem: Arena
    let storage := try mem.Alloc[byte](16)
    while true {
        mem.Reset()
        break
    }
    let length := storage.len
    return Ok()
}
`

	errors := analyzeSourceRaw(t, input)
	assertSemaErrors(t, errors, []string{
		"cannot use storage after arena mem was reset at 11:19, previous declaration at 6:24",
	})
}
