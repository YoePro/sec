package sema

import (
	"strings"
	"testing"

	"sec/internal/lexer"
	"sec/internal/parser"
)

// Execution-context requirements are inferred per callable and checked from
// every root (cancellation.md §§ 19–23, scheduling.md § 22): main has neither
// a logical task nor a cancellable execution, a spawned thread is cancellable
// but not a logical task, and a spawned task satisfies both. Reports land on
// main's first call on the path or on the operation itself.
func TestExecutionContextRequirements(t *testing.T) {
	errors := analyzeSourceRaw(t, `module main

fn Observe() bool {
    return Task.Current().CancelRequested
}

fn Stop() void {
    cancel
}

fn Wrapper() bool {
    return Observe()
}

fn TaskWork() int {
    if Observe() {
        Stop()
    }
    return 1
}

fn ThreadWork() int {
    Stop()
    if Observe() {
        return 0
    }
    return 1
}

fn main() void {
    let work := spawn TaskWork()
    await work
    let worker := spawn thread ThreadWork()
    join worker
    let direct := Task.Current()
    let viaWrapper := Wrapper()
    Stop()
}
`)
	want := []string{
		"because Observe uses Task.Current() at 4:17; ordinary program entry is not a logical task at 36:23",
		"because Stop uses cancel at 8:5; ordinary program entry is not a cancellable task or thread at 37:5",
		"Task.Current() requires a current logical task context; ordinary program entry is not a logical task at 35:24",
		"because Observe uses Task.Current() at 4:17; a spawned thread is a physical thread, not a logical task at 24:8",
	}
	if len(errors) != len(want) {
		t.Fatalf("diagnostics = %v", errors)
	}
	for _, expected := range want {
		found := false
		for _, err := range errors {
			found = found || strings.Contains(err.Error(), expected)
		}
		if !found {
			t.Errorf("missing %q in %v", expected, errors)
		}
	}
}

// A test body runs like ordinary program entry (testing.md § 9.7): no
// logical task and no cancellable execution unless the test spawns one.
func TestExecutionContextRequirementsInTests(t *testing.T) {
	source := "module main\n\nfn Observe() bool {\n    return Task.Current().CancelRequested\n}\n\ntest \"direct\" {\n    let observed := Observe()\n}\n\ntest \"spawned\" {\n    let work := spawn Observe()\n    let observed := await work\n}\n"
	p := parser.New(lexer.NewWithFile(source, "observe_test.sec"))
	program := p.ParseProgram()
	if len(p.Errors()) != 0 {
		t.Fatal(p.Errors())
	}
	errors := NewAnalyzer().Analyze(program)
	if len(errors) != 1 || !strings.Contains(errors[0].Message, "because Observe uses Task.Current() at 4:17; a test body is not a logical task") || errors[0].Line != 8 {
		t.Fatalf("diagnostics = %v", errors)
	}
}
