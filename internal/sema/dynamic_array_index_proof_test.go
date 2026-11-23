package sema

import (
	"sort"
	"testing"

	"sec/internal/lexer"
	"sec/internal/parser"
)

// package18 §§ 40-42: an index into a directly held owning T[] is proven
// only by a dominating `index < values.Len` / `len(values)` branch or
// assertion with no length-changing event since: a mutating member call
// (Append) or a mutable borrow of the owner revokes it, a try keeps the
// runtime check. (`ref T[]` parameters are slices with their own P16 plans.)
func TestDynamicArrayIndexProofs(t *testing.T) {
	source := `module main

fn Shrink(values: ref mut int[]) void {
    discard values
}

fn Through(values: ref int[], i: uint) int {
    if i < values.Len {
        return values[i]
    }
    return 0
}

fn F(i: uint) Result[int, IndexError] {
    let mut values: int[]
    discard values.Append(1)
    if i < values.Len {
        let a := values[i]
        discard values.Append(2)
        let b := values[i]
        return Ok(a + b)
    }
    if i < len(values) {
        Shrink(ref mut values)
        return Ok(values[i])
    }
    if i < values.Len {
        return Ok(try values[i])
    }
    assert i < values.Len
    return Ok(values[i])
}

fn main() int {
    return 0
}
`
	p := parser.New(lexer.NewWithFile(source, "proofs.sec"))
	program := p.ParseProgram()
	if len(p.Errors()) != 0 {
		t.Fatal(p.Errors())
	}
	a := NewAnalyzer()
	a.SetAllocationProfile("hosted")
	if errs := a.Analyze(program); len(errs) != 0 {
		t.Fatal(errs)
	}
	type row struct {
		line  int
		check ArrayIndexCheckKind
		proof ArrayIndexProofKind
	}
	var rows []row
	for expr, plan := range a.resolvedDynamicArrayIndexPlans {
		rows = append(rows, row{expr.Token.Line, plan.CheckKind, plan.ProofKind})
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].line < rows[j].line })
	want := []row{
		{18, ArrayIndexProvenSafe, ArrayIndexProofBranch},    // dominating branch
		{20, ArrayIndexRuntimeCheck, ""},                     // after Append
		{25, ArrayIndexRuntimeCheck, ""},                     // after ref mut
		{28, ArrayIndexRuntimeCheck, ""},                     // under try
		{31, ArrayIndexProvenSafe, ArrayIndexProofAssertion}, // assertion
	}
	if len(rows) != len(want) {
		t.Fatalf("rows = %+v", rows)
	}
	for index := range want {
		if rows[index] != want[index] {
			t.Errorf("row %d = %+v, want %+v", index, rows[index], want[index])
		}
	}
}
