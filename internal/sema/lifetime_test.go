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

func TestAggregateReferenceProvenanceIsFieldSensitive(t *testing.T) {
	input := `
module main

type Views struct {
    local: ref int,
    external: ref int,
}

fn Valid(external: ref int) ref int {
    let value := 10
    let views := Views {
        local: ref value,
        external: external,
    }
    return views.external
}
`

	errors := analyzeSourceRaw(t, input)
	assertSemaErrors(t, errors, nil)
}

func TestAggregateReferenceProvenanceRejectsOnlyLocalField(t *testing.T) {
	input := `
module main

type Views struct {
    local: ref int,
    external: ref int,
}

fn Invalid(external: ref int) ref int {
    let value := 10
    let views := Views {
        local: ref value,
        external: external,
    }
    return views.local
}
`

	errors := analyzeSourceRaw(t, input)
	assertSemaErrors(t, errors, []string{
		"function Invalid cannot return reference to local variable value at 15:17, previous declaration at 10:9",
	})
}

func TestNestedAggregateReferenceProvenanceIsFieldSensitive(t *testing.T) {
	input := `
module main

type View struct {
    value: ref int,
}

type Pair struct {
    local: View,
    external: View,
}

fn Valid(external: ref int) ref int {
    let value := 10
    let pair := Pair {
        local: View { value: ref value },
        external: View { value: external },
    }
    return pair.external.value
}
`

	errors := analyzeSourceRaw(t, input)
	assertSemaErrors(t, errors, nil)
}

func TestAggregateFieldAssignmentReplacesReferenceProvenance(t *testing.T) {
	input := `
module main

type View struct {
    value: ref int,
}

fn Valid(external: ref int) ref int {
    let value := 10
    let mut view := View { value: ref value }
    view.value = external
    return view.value
}
`

	errors := analyzeSourceRaw(t, input)
	assertSemaErrors(t, errors, nil)
}

func TestAggregateFieldProvenanceJoinsAcrossBranches(t *testing.T) {
	input := `
module main

type View struct {
    value: ref int,
}

fn Invalid(condition: bool, external: ref int) ref int {
    let local := 10
    let mut view := View { value: external }
    if condition {
        view.value = ref local
    }
    return view.value
}
`

	errors := analyzeSourceRaw(t, input)
	assertSemaErrors(t, errors, []string{
		"function Invalid cannot return reference to local variable local at 14:16, previous declaration at 9:9",
	})
}

func TestAggregateCopyPreservesFieldSensitiveReferenceProvenance(t *testing.T) {
	input := `
module main

type Views struct {
    local: ref int,
    external: ref int,
}

fn Valid(external: ref int) ref int {
    let value := 10
    let views := Views {
        local: ref value,
        external: external,
    }
    let copied := views
    return copied.external
}
`

	errors := analyzeSourceRaw(t, input)
	assertSemaErrors(t, errors, nil)
}

func TestReferenceArrayProvenanceIsElementSensitiveForConstantIndex(t *testing.T) {
	input := `
module main

fn Valid(external: ref int) ref int {
    let value := 10
    let views := [ref value, external]
    return views[1]
}
`

	errors := analyzeSourceRaw(t, input)
	assertSemaErrors(t, errors, nil)
}

func TestReferenceArrayDynamicIndexJoinsElementProvenance(t *testing.T) {
	input := `
module main

fn Invalid(index: uint, external: ref int) ref int {
    let value := 10
    let views := [ref value, external]
    return views[index]
}
`

	errors := analyzeSourceRaw(t, input)
	assertSemaErrors(t, errors, []string{
		"function Invalid cannot return reference to local variable value at 7:17, previous declaration at 5:9",
	})
}

func TestAggregateFieldProvenanceJoinsAcrossLoopBackedges(t *testing.T) {
	input := `
module main

type View struct {
    value: ref int,
}

fn Invalid(condition: bool, external: ref int) ref int {
    let local := 10
    let mut view := View { value: external }
    while condition {
        view.value = ref local
        break
    }
    return view.value
}
`

	errors := analyzeSourceRaw(t, input)
	assertSemaErrors(t, errors, []string{
		"function Invalid cannot return reference to local variable local at 15:16, previous declaration at 9:9",
	})
}

func TestReferenceArrayElementAssignmentReplacesOnlyThatOrigin(t *testing.T) {
	input := `
module main

fn Valid(external: ref int) ref int {
    let value := 10
    let mut views := [ref value, ref value]
    views[1] = external
    return views[1]
}
`

	errors := analyzeSourceRaw(t, input)
	assertSemaErrors(t, errors, nil)
}

func TestInterproceduralReferenceSummaryRejectsLocalArgument(t *testing.T) {
	input := `
module main

fn Invalid() ref int {
    let value := 10
    return Identity(ref value)
}

fn Identity(value: ref int) ref int {
    return value
}
`

	errors := analyzeSourceRaw(t, input)
	assertSemaErrors(t, errors, []string{
		"function Invalid cannot return reference to local variable value at 6:12, previous declaration at 5:9",
	})
}

func TestInterproceduralReferenceSummaryAllowsCallerOwnedArgument(t *testing.T) {
	input := `
module main

fn Forward(value: ref int) ref int {
    return Identity(value)
}

fn Identity(value: ref int) ref int {
    return value
}
`

	errors := analyzeSourceRaw(t, input)
	assertSemaErrors(t, errors, nil)
}

func TestInterproceduralReferenceSummaryComposesTransitively(t *testing.T) {
	input := `
module main

fn Invalid() ref int {
    let value := 10
    return Forward(ref value)
}

fn Forward(value: ref int) ref int {
    return Identity(value)
}

fn Identity(value: ref int) ref int {
    return value
}
`

	errors := analyzeSourceRaw(t, input)
	assertSemaErrors(t, errors, []string{
		"function Invalid cannot return reference to local variable value at 6:12, previous declaration at 5:9",
	})
}

func TestInterproceduralProjectedReferenceSummary(t *testing.T) {
	input := `
module main

type Pair struct {
    first: int,
    second: int,
}

fn Invalid() ref int {
    let pair := Pair { first: 1, second: 2 }
    return First(ref pair)
}

fn First(pair: ref Pair) ref int {
    return ref pair.first
}
`

	errors := analyzeSourceRaw(t, input)
	assertSemaErrors(t, errors, []string{
		"function Invalid cannot return reference to local variable pair at 11:12, previous declaration at 10:9",
	})
}

func TestInterproceduralSummaryJoinsMultipleParameters(t *testing.T) {
	input := `
module main

fn Invalid(condition: bool, external: ref int) ref int {
    let local := 10
    return Select(condition, ref local, external)
}

fn Select(condition: bool, first: ref int, second: ref int) ref int {
    if condition {
        return first
    }
    return second
}
`

	errors := analyzeSourceRaw(t, input)
	assertSemaErrors(t, errors, []string{
		"function Invalid cannot return reference to local variable local at 6:12, previous declaration at 5:9",
	})
}

func TestInterproceduralAggregateSummaryIsFieldSensitive(t *testing.T) {
	input := `
module main

type Views struct {
    local: ref int,
    external: ref int,
}

fn Valid(local: ref int, external: ref int) ref int {
    let views := MakeViews(local, external)
    return views.external
}

fn MakeViews(local: ref int, external: ref int) Views {
    return Views { local: local, external: external }
}
`

	errors := analyzeSourceRaw(t, input)
	assertSemaErrors(t, errors, nil)
}

func TestInterproceduralAggregateSummaryRejectsLocalField(t *testing.T) {
	input := `
module main

type Views struct {
    local: ref int,
    external: ref int,
}

fn Invalid(external: ref int) ref int {
    let local := 10
    let views := MakeViews(ref local, external)
    return views.local
}

fn MakeViews(local: ref int, external: ref int) Views {
    return Views { local: local, external: external }
}
`

	errors := analyzeSourceRaw(t, input)
	assertSemaErrors(t, errors, []string{
		"function Invalid cannot return reference to local variable local at 12:17, previous declaration at 10:9",
	})
}

func TestInterproceduralMethodSummaryInstantiatesReceiver(t *testing.T) {
	input := `
module main

type Pair struct {
    first: int,
    second: int,
}

impl Pair {
    fn First() ref int {
        return ref self.first
    }
}

fn Invalid() ref int {
    let pair := Pair { first: 1, second: 2 }
    return pair.First()
}
`

	errors := analyzeSourceRaw(t, input)
	assertSemaErrors(t, errors, []string{
		"function Invalid cannot return reference to local variable pair at 17:16, previous declaration at 16:9",
	})
}

func TestInterproceduralReturnedReferenceKeepsGrantingBorrowActive(t *testing.T) {
	input := `
module main

fn Identity(value: ref int) ref int {
    return value
}

fn Invalid() void {
    let mut value := 10
    let view := Identity(ref value)
    value = 20
}
`

	errors := analyzeSourceRaw(t, input)
	assertSemaErrors(t, errors, []string{
		"cannot assign to value while it is shared borrowed at 11:5, previous declaration at 10:17",
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
    let mut mem: Arena := Arena {}
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
    let mut mem: Arena := Arena {}
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
    let mut mem: Arena := Arena {}
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
    let mut mem: Arena := Arena {}
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
    let mut mem: Arena := Arena {}
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
    let mut mem: Arena := Arena {}
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
