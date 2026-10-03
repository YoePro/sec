package sema

import (
	"strings"
	"testing"
)

// The owning Thread[T] exposes its exact CamelCase identity, name, status,
// observer creation, and deferred start, and the copyable ThreadObserver[T]
// exposes only identity, name, and status. Legacy lowercase spellings and
// owner-only operations on an observer are not part of the surface.
//
// Rules:
//   - rules/concurrency/threads.md — § 26 "Exact public Thread[T] surface", § 40 "ThreadObserver[T]", § 41
func TestThreadV2SourceSurface(t *testing.T) {
	analyzer, errors := analyzeSourceWithAnalyzer(t, `module main

fn Work() void {
}

fn Run() void {
    let worker := spawn thread Work()
    let id: ThreadID := worker.ID
    let name: string := worker.Name
    let status: ThreadStatus := worker.Status
    let observer := worker.Observe()
    let observed: ThreadStatus := observer.Status
    let observedName: string := observer.Name
    let started: Result[void, ThreadStartError] := worker.Start()
    discard id
    discard name
    discard status
    discard observed
    discard observedName
    discard started
    detach worker
}
`)
	assertSemaErrors(t, errors, nil)
	if got := typeDisplayName(analyzer.completionSymbols["observer"].Type); got != "ThreadObserver[void]" {
		t.Fatalf("observer type = %q, want ThreadObserver[void]", got)
	}

	errors = analyzeSource(t, `
module main

fn Work() void {
}

fn Run() void {
    let worker := spawn thread Work()
    let observer := worker.Observe()
    let legacy := worker.status
    let started := observer.Start()
    discard legacy
    discard started
    detach worker
}
`)
	if len(errors) < 2 {
		t.Fatalf("errors = %v, want legacy lowercase member and observer Start rejected", errors)
	}
	joined := ""
	for _, err := range errors {
		joined += err.Message + "\n"
	}
	if !strings.Contains(joined, "status") || !strings.Contains(joined, "Start") {
		t.Fatalf("errors = %v, want diagnostics naming worker.status and observer.Start", errors)
	}
}
