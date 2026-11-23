package sema

import (
	"testing"

	"sec/internal/ast"
	"sec/internal/lexer"
	"sec/internal/parser"
	"sec/internal/platform/target"
)

// P19 §§ 79–81, 155: the byte demand of a FromBuffer Arena is the peak over
// its Reset windows of count * size plus worst-case alignment padding
// (align - 1), with sequential sums, branch maxima plus continuation,
// constant-trip-count loop multiplication, interval upper bounds of a count,
// zero-element allocations and unknown loops. linux/amd64: int64 is 8/8,
// int32 4/4, byte 1/1.
func TestArenaDemandSummaries(t *testing.T) {
	source := `module main

fn Sequential() Result[int64, AllocationError] {
    let mut buffer: byte[256]
    let mut arena := Arena.FromBuffer(ref mut buffer[..])
    let a := try arena.New[int64]()
    let b := try arena.New[int64]()
    return Ok(0)
}

fn Padding() Result[int64, AllocationError] {
    let mut buffer: byte[256]
    let mut arena := Arena.FromBuffer(ref mut buffer[..])
    let values := try arena.Alloc[int32](4)
    return Ok(0)
}

fn Branch(flag: bool) Result[int64, AllocationError] {
    let mut buffer: byte[256]
    let mut arena := Arena.FromBuffer(ref mut buffer[..])
    if flag {
        let a := try arena.New[int64]()
    } else {
        let b := try arena.Alloc[byte](40)
    }
    let c := try arena.New[int64]()
    return Ok(0)
}

fn Loop() Result[int64, AllocationError] {
    let mut buffer: byte[256]
    let mut arena := Arena.FromBuffer(ref mut buffer[..])
    for i in 0..<4 {
        let a := try arena.New[int64]()
    }
    return Ok(0)
}

fn Bounded(count: uint) Result[int64, AllocationError] {
    let mut buffer: byte[256]
    let mut arena := Arena.FromBuffer(ref mut buffer[..])
    if count < 9 {
        let values := try arena.Alloc[byte](count)
    }
    return Ok(0)
}

fn Window() Result[int64, AllocationError] {
    let mut buffer: byte[256]
    let mut arena := Arena.FromBuffer(ref mut buffer[..])
    let a := try arena.New[int64]()
    let b := try arena.New[int64]()
    arena.Reset()
    let c := try arena.Alloc[byte](10)
    return Ok(0)
}

fn Zero() Result[int64, AllocationError] {
    let mut buffer: byte[256]
    let mut arena := Arena.FromBuffer(ref mut buffer[..])
    let values := try arena.Alloc[int32](0)
    return Ok(0)
}

fn Open(flag: bool) Result[int64, AllocationError] {
    let mut buffer: byte[256]
    let mut arena := Arena.FromBuffer(ref mut buffer[..])
    while flag {
        let a := try arena.New[int64]()
    }
    return Ok(0)
}

fn main() int {
    return 0
}
`
	definition, ok := target.Find(target.Target{OS: "linux", Arch: "amd64"})
	if !ok {
		t.Fatal("no linux/amd64 target")
	}
	plan, err := definition.ScalarPlan()
	if err != nil {
		t.Fatal(err)
	}
	p := parser.New(lexer.NewWithFile(source, "demand.sec"))
	program := p.ParseProgram()
	a := NewAnalyzerWithScalarPlan(plan)
	if errs := a.Analyze(program); len(errs) != 0 {
		t.Fatal(errs)
	}
	want := map[string]string{
		"Sequential": "30", "Padding": "19", "Branch": "55", "Loop": "60",
		"Bounded": "8", "Window": "30", "Zero": "0", "Open": "unknown",
	}
	for _, statement := range program.Statements {
		fn, ok := statement.(*ast.FunctionDeclaration)
		if !ok || fn.Name == nil {
			continue
		}
		expected, tested := want[fn.Name.Value]
		if !tested {
			continue
		}
		summaries := a.ArenaDemandSummariesOf(fn.Body)
		if len(summaries) != 1 {
			t.Fatalf("%s: %d summaries", fn.Name.Value, len(summaries))
		}
		got := summaries[0].Kind
		if summaries[0].Bytes != nil {
			got = summaries[0].Bytes.String()
		}
		if got != expected {
			t.Errorf("%s demand = %s (%s), want %s", fn.Name.Value, got, summaries[0].Expression, expected)
		}
		if summaries[0].CapacityBytes == nil || summaries[0].CapacityBytes.Int64() != 256 {
			t.Errorf("%s capacity = %v, want 256", fn.Name.Value, summaries[0].CapacityBytes)
		}
		delete(want, fn.Name.Value)
	}
	if len(want) != 0 {
		t.Fatalf("functions not summarized: %v", want)
	}
}
