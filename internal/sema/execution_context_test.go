package sema

import (
	"strings"
	"testing"
)

// The current-execution surface (cancellation.md §§ 8–11): Task.Current()
// returns TaskContext and Thread.Current() ThreadContext, both with the bool
// CancelRequested observation; Task.Yield()/Thread.Yield() return void; a
// CancelRequested observation is never a selectable operation (select.md § 36).
func TestExecutionContextSurface(t *testing.T) {
	valid := analyzeSourceRaw(t, `module main

fn Observe() bool {
    let current := Task.Current()
    let physical := Thread.Current()
    Task.Yield()
    Thread.Yield()
    return current.CancelRequested || physical.CancelRequested
}
`)
	if len(valid) != 0 {
		t.Fatalf("unexpected errors: %v", valid)
	}
	cases := map[string]struct{ body, want string }{
		"arguments":  {"fn F() void {\n    let current := Task.Current(1)\n}", "Task.Current"},
		"lowercase":  {"fn F() bool {\n    return Thread.Current().cancelRequested\n}", "use the canonical CamelCase member `CancelRequested`"},
		"selectable": {"fn F() void {\n    select {\n        Task.Current().CancelRequested => {\n        }\n        default => {\n        }\n    }\n}", "operation is not selectable"},
	}
	for name, test := range cases {
		t.Run(name, func(t *testing.T) {
			errors := analyzeSourceRaw(t, "module main\n\n"+test.body+"\n")
			for _, err := range errors {
				if strings.Contains(err.Message, test.want) || strings.Contains(err.Help, test.want) {
					return
				}
			}
			t.Fatalf("no diagnostic containing %q; got %v", test.want, errors)
		})
	}
}
