package sema

import "testing"

func TestSpawnAwaitAndTaskType(t *testing.T) {
	input := `
module main

fn Calculate() int {
    return 42
}

fn Run() int {
    let work := spawn Calculate()
    let value := await work
    return value
}
`

	errors := analyzeSourceRaw(t, input)
	assertSemaErrors(t, errors, nil)
}

func TestSpawnPreservesResultReturnType(t *testing.T) {
	input := `
module main

enum IOError {
    Invalid,
}

fn Load() Result[int, IOError] {
    return Ok(1)
}

fn Run() void {
    let work := spawn Load()
    let result: Result[int, IOError] := await work
    return
}
`

	errors := analyzeSourceRaw(t, input)
	assertSemaErrors(t, errors, nil)
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
