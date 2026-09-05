package sema

import "testing"

// rules/concurrency/tasks.md section 16: awaiting Task[T] consumes the handle
// and produces the complete TaskOutcome[T], never a flow-dependent bare T.
func TestSpawnAwaitAndTaskOutcomeType(t *testing.T) {
	input := `
module main

fn Calculate() int {
    return 42
}

fn Run() void {
    let work := spawn Calculate()
    let outcome := await work
}
`

	analyzer, errors := analyzeSourceWithAnalyzer(t, input)
	assertSemaErrors(t, errors, nil)
	outcome := analyzer.completionSymbols["outcome"].Type
	if got := typeDisplayName(outcome); got != "TaskOutcome[int]" {
		t.Fatalf("wrong await outcome type. got=%q", got)
	}
	if len(outcome.UnionVariants) != 4 || outcome.UnionVariants[0].Payload == nil || outcome.UnionVariants[0].Payload.Kind != IntType {
		t.Fatalf("TaskOutcome[int] variants = %+v, want Completed(int) plus three terminal alternatives", outcome.UnionVariants)
	}
}

// rules/concurrency/tasks.md sections 11-12: a task function's Result remains
// nested inside Completed and is not reclassified as task execution failure.
func TestSpawnAwaitPreservesNestedResultReturnType(t *testing.T) {
	input := `
module main

enum IOError error {
    Invalid,
}

fn Load() Result[int, IOError] {
    return Ok(1)
}

fn Run() void {
    let work := spawn Load()
    let outcome: TaskOutcome[Result[int, IOError]] := await work
    return
}
`

	errors := analyzeSourceRaw(t, input)
	assertSemaErrors(t, errors, nil)
}

func TestSpawnKindModifiersResolveHandleTypes(t *testing.T) {
	input := `
module main

fn Calculate() int {
    return 42
}

fn Run() void {
    let taskHandle := spawn task Calculate()
    let threadHandle := spawn thread Calculate()
    let outcome := await taskHandle
}
`

	analyzer, errors := analyzeSourceWithAnalyzer(t, input)
	assertSemaErrors(t, errors, []string{
		"owned thread threadHandle is unresolved at scope exit at 10:9",
	})

	if got := typeDisplayName(analyzer.completionSymbols["taskHandle"].Type); got != "Task[int]" {
		t.Fatalf("wrong task handle type. got=%q", got)
	}
	if got := typeDisplayName(analyzer.completionSymbols["threadHandle"].Type); got != "Thread[int]" {
		t.Fatalf("wrong thread handle type. got=%q", got)
	}
	if got := typeDisplayName(analyzer.completionSymbols["outcome"].Type); got != "TaskOutcome[int]" {
		t.Fatalf("wrong await outcome type. got=%q", got)
	}
}

// rules/concurrency/tasks.md section 12(9): void completion is payload-less,
// while cancellation, panic, and execution failure remain distinct variants.
func TestTaskOutcomeVoidHasPayloadlessCompletedVariant(t *testing.T) {
	input := `
module main

fn Work() void {
}

fn Run() void {
    let work := spawn Work()
    let outcome: TaskOutcome[void] := await work
}
`

	analyzer, errors := analyzeSourceWithAnalyzer(t, input)
	assertSemaErrors(t, errors, nil)
	outcome := analyzer.completionSymbols["outcome"].Type
	if len(outcome.UnionVariants) != 4 {
		t.Fatalf("TaskOutcome[void] variants = %+v, want four variants", outcome.UnionVariants)
	}
	if completed := outcome.UnionVariants[0]; completed.Name != "Completed" || completed.Payload != nil {
		t.Fatalf("TaskOutcome[void].Completed = %+v, want payload-less variant", completed)
	}
}

func TestDetachConsumesVoidTaskAndThreadHandles(t *testing.T) {
	input := `
module main

fn Work() void {
}

fn Run() void {
    let taskHandle := spawn Work()
    let threadHandle := spawn thread Work()
    detach taskHandle
    detach threadHandle
}
`

	errors := analyzeSourceRaw(t, input)
	assertSemaErrors(t, errors, nil)
}

func TestDetachNonVoidHandleRequiresDiscard(t *testing.T) {
	input := `
module main

fn Calculate() int {
    return 1
}

fn Invalid() void {
    let taskHandle := spawn Calculate()
    let threadHandle := spawn thread Calculate()
    detach taskHandle
    detach threadHandle
}
`

	errors := analyzeSourceRaw(t, input)
	assertSemaErrors(t, errors, []string{
		"detaching Task[int] with non-void result requires explicit discard at 11:12",
		"detaching Thread[int] with non-void result requires explicit discard at 12:12",
		"owned task taskHandle is unresolved at scope exit at 9:9",
		"owned thread threadHandle is unresolved at scope exit at 10:9",
	})
}

func TestDetachDiscardConsumesNonVoidHandle(t *testing.T) {
	input := `
module main

fn Calculate() int {
    return 1
}

fn Run() void {
    let taskHandle := spawn Calculate()
    let threadHandle := spawn thread Calculate()
    detach taskHandle discard
    detach threadHandle discard
}
`

	errors := analyzeSourceRaw(t, input)
	assertSemaErrors(t, errors, nil)
}

func TestDetachRejectsNonLifecycleValueAndUseAfterDetach(t *testing.T) {
	input := `
module main

fn Work() void {
}

fn Invalid() void {
    let value := 1
    detach value
    let taskHandle := spawn Work()
    detach taskHandle
    await taskHandle
}
`

	errors := analyzeSourceRaw(t, input)
	assertSemaErrors(t, errors, []string{
		"detach requires Task[T] or Thread[T], got int at 9:12",
		"value taskHandle was detached here and is no longer available at 12:11, previous declaration at 11:12",
	})
}

func TestSpawnResultMustBeOwned(t *testing.T) {
	input := `
module main

fn Work() void {
    return
}

fn Invalid() void {
    spawn Work()
}
`

	errors := analyzeSourceRaw(t, input)
	assertSemaErrors(t, errors, []string{
		"spawned task result must be owned, awaited, joined or detached at 9:5",
	})
}

func TestOwnedTaskMustBeResolvedAtScopeExit(t *testing.T) {
	input := `
module main

fn Work() int {
    return 1
}

fn Invalid() void {
    let work := spawn Work()
}
`

	errors := analyzeSourceRaw(t, input)
	assertSemaErrors(t, errors, []string{
		"owned task work is unresolved at scope exit at 9:9",
	})
}

func TestAwaitRequiresTask(t *testing.T) {
	input := `
module main

fn Invalid() int {
    let value := 1
    return await value
}
`

	errors := analyzeSourceRaw(t, input)
	assertSemaErrors(t, errors, []string{
		"await requires Task[T], got int at 6:18",
	})
}

func TestCancelOutsideCancellableContext(t *testing.T) {
	input := `
module main

fn Invalid() void {
    cancel
    let value := 1
}
`

	errors := analyzeSourceRaw(t, input)
	assertSemaErrors(t, errors, []string{
		"cancel is not valid outside a task or explicit thread context at 5:5",
		"unreachable code at 6:5",
	})
}

func TestCancelInsideOrdinaryLambdaIsRejected(t *testing.T) {
	input := `
module main

fn Invalid() void {
    let callback := fn() void {
        cancel
    }
    callback()
}
`

	errors := analyzeSourceRaw(t, input)
	assertSemaErrors(t, errors, []string{
		"cancel is not valid outside a task or explicit thread context at 6:9",
	})
}

func TestCancelInsideSpawnedLambdaIsAllowed(t *testing.T) {
	input := `
module main

fn Run() void {
    let work := spawn fn() void {
        cancel
    }
    await work
}
`

	errors := analyzeSourceRaw(t, input)
	assertSemaErrors(t, errors, nil)
}

func TestThreadHandleIsMoveOnlyAndNotDiscardable(t *testing.T) {
	input := `
module main

fn Invalid(thread: Thread[int]) void {
    discard thread
}
`

	errors := analyzeSourceRaw(t, input)
	assertSemaErrors(t, errors, []string{
		"cannot discard unresolved Thread[int]; await, join or detach it explicitly at 5:13",
	})
}

func TestThreadAndThreadLocalTypesResolve(t *testing.T) {
	input := `
module main

fn Use(thread: Thread[int], local: ThreadLocal[string], context: ThreadContext) void {
    let status: ThreadStatus := ThreadStatus.Running
    discard local
    discard context
}
`

	errors := analyzeSourceRaw(t, input)
	assertSemaErrors(t, errors, nil)
}

func TestStaticLetAndCompilerKnownConcurrencyTypes(t *testing.T) {
	input := `
module main

type State struct {
    running: bool,
    count: int,
}

static let Ready: Atomic[bool] := Atomic(false)
static let Requests: Atomic[uint64] := Atomic(uint64(0))
static let AppState: Mutex[State] := Mutex(State {
    running: false,
    count: 0,
})

fn Run() bool {
    Ready.store(true)
    Requests.fetchAdd(1u)
    let mut state := AppState.lock()
    state.running = true
    return Ready.load()
}
`

	errors := analyzeSourceRaw(t, input)
	assertSemaErrors(t, errors, nil)
}

func TestImplStaticLetIsAccessedThroughType(t *testing.T) {
	input := `
module main

type Counter struct {
    value: int,
}

impl Counter {
    static let Maximum: int := 100
}

fn ReadMaximum() int {
    return Counter.Maximum
}
`

	errors := analyzeSourceRaw(t, input)
	assertSemaErrors(t, errors, nil)
}

func TestAtomicRejectsUnsupportedElementType(t *testing.T) {
	input := `
module main

static let Bad: Atomic[string] := Atomic("no")
`

	errors := analyzeSourceRaw(t, input)
	assertSemaErrors(t, errors, []string{
		"type string is not supported by Atomic at 4:17",
	})
}
