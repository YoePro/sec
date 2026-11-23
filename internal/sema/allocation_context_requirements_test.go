package sema

import (
	"testing"

	"sec/internal/lexer"
	"sec/internal/parser"
)

// P19 §§ 52-58, 63, 77: RequiresAllocationContext is distinct from
// MayAllocate. An ambient allocation (T[] Append, owned Arena creation) and
// its synchronous callers require context; allocation through an explicit
// Arena, pure code and a spawner whose worker allocates do not (a spawned
// task does not inherit the context); the program entry is a root.
func TestAllocationContextRequirements(t *testing.T) {
	source := `module main

fn Grow() Result[void, CollectionError] {
    let mut values: int[]
    try values.Append(1)
    return Ok()
}

fn Middle() Result[void, CollectionError] {
    return Grow()
}

fn Owned() void {
    let arena := Arena.WithCapacity(64u)
}

fn Explicit() Result[int, AllocationError] {
    let mut buffer: byte[64]
    let mut arena := Arena.FromBuffer(ref mut buffer[..])
    let value := try arena.New[int]()
    *value = 3
    return Ok(0)
}

fn Spawner() void {
    let worker := try spawn Owned() { Err(_) => { return } }
    detach worker
}

fn Pure() int {
    return 1
}

fn main() int {
    discard Middle()
    discard Explicit()
    Spawner()
    return Pure()
}
`
	p := parser.New(lexer.NewWithFile(source, "context.sec"))
	program := p.ParseProgram()
	if len(p.Errors()) != 0 {
		t.Fatal(p.Errors())
	}
	a := NewAnalyzer()
	a.SetAllocationProfile("hosted")
	if errs := a.Analyze(program); len(errs) != 0 {
		t.Fatal(errs)
	}
	want := map[string]bool{"Grow": true, "Middle": true, "Owned": true, "Explicit": false, "Spawner": false, "Pure": false, "main": true}
	for name, required := range want {
		functions := a.functions[name]
		if len(functions) != 1 {
			t.Fatalf("%s: %d declarations", name, len(functions))
		}
		if got := a.RequiresAllocationContextAt(functions[0].Token); got != required {
			t.Errorf("%s requires context = %v, want %v", name, got, required)
		}
	}
	if kind := a.CallRootKindAt(a.functions["main"][0].Token); kind != CallRootProgramEntry {
		t.Fatalf("main root kind = %q", kind)
	}
	if kind := a.CallRootKindAt(a.functions["Owned"][0].Token); kind != CallRootTaskEntry {
		t.Fatalf("spawned Owned root kind = %q", kind)
	}
}
