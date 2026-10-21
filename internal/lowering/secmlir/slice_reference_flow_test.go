package secmlir

import (
	"bytes"
	"fmt"
	"os/exec"
	"strings"
	"testing"

	"sec/internal/ir/semantic"
	"sec/internal/testsupport"
)

// Corrupting only a source-derived element borrow's pin dependency must stop
// schema emission before any bytes are returned, even after valid functions.
// Rules: rules/compiler/semantic_ir.md — §126.17–19,23;
// rules/mlir/packages/sec-mlir-dialect_package16.md — §§85–86,116;
// rules/compiler/compiler_pipeline.md — §§32–34.
func TestPackage16EmitRejectsForgedElementBorrow(t *testing.T) {
	for _, width := range []uint16{32, 64} {
		t.Run(fmt.Sprint(width), func(t *testing.T) {
			m := sliceEmitModule(t, "slice_source_index.sec", width)
			if output, err := EmitWithOptions(m, testPlan(width), EmitOptions{DialectVersion: 12}); err != nil || len(output) == 0 {
				t.Fatal("source baseline", err)
			}
			changed := false
			for _, fn := range m.Functions {
				for _, block := range fn.Blocks {
					for i := range block.Operations {
						op := &block.Operations[i]
						if !changed && (op.Kind == semantic.OpReferenceBorrowShared || op.Kind == semantic.OpReferenceBorrowMutable) && op.Reference != nil {
							op.Reference.Pinned = !op.Reference.Pinned
							changed = true
						}
					}
				}
			}
			if !changed {
				t.Fatal("fixture lost element borrow coverage")
			}
			before := semantic.Format(m)
			output, err := EmitWithOptions(m, testPlan(width), EmitOptions{DialectVersion: 12})
			if output != nil || err == nil || !strings.Contains(err.Error(), "slice element borrow changes") {
				t.Fatal("forged borrowing emitted bytes or lost owning error", err)
			}
			if semantic.Format(m) != before {
				t.Fatal("failed verification mutated source IR")
			}
		})
	}
}

// A source-built module is explicitly transformed into a white-box CFG
// forwarding case. It must verify and round-trip through all four real passes;
// this is represented-IR evidence, not new source phi/signature production.
// Rules: rules/compiler/semantic_ir.md — §§66,126.23;
// rules/mlir/packages/sec-mlir-dialect_package16.md — §§83–86,141.
func TestPackage16EmitForwardedSliceFacts(t *testing.T) {
	tool := testsupport.RequireSecMLIROptPath(t)
	for _, width := range []uint16{32, 64} {
		t.Run(fmt.Sprint(width), func(t *testing.T) {
			m := sliceEmitModule(t, "slice_source_length.sec", width)
			var fn *semantic.Function
			for _, candidate := range m.Functions {
				if candidate.Name == "Property" {
					fn = candidate
					break
				}
			}
			if fn == nil {
				t.Fatal("fixture lost Property function")
			}
			values := map[semantic.ValueID]semantic.Value{}
			maxValue, maxBlock := semantic.ValueID(0), semantic.BlockID(0)
			add := func(v semantic.Value) {
				values[v.ID] = v
				if v.ID > maxValue {
					maxValue = v.ID
				}
			}
			for _, p := range fn.Parameters {
				add(p.Value)
			}
			for _, block := range fn.Blocks {
				if block.ID > maxBlock {
					maxBlock = block.ID
				}
				for _, p := range block.Parameters {
					add(p)
				}
				for _, op := range block.Operations {
					for _, v := range op.Results {
						add(v)
					}
				}
			}
			changed := false
			for _, block := range fn.Blocks {
				for index, op := range block.Operations {
					if op.Kind != semantic.OpSliceLength {
						continue
					}
					parameter := values[op.Operands[0]]
					parameter.ID = maxValue + 1
					tail := append([]semantic.Operation(nil), block.Operations[index:]...)
					tail[0].Operands = []semantic.ValueID{parameter.ID}
					next := &semantic.Block{ID: maxBlock + 1, Parameters: []semantic.Value{parameter}, Operations: tail}
					block.Operations = append(append([]semantic.Operation(nil), block.Operations[:index]...), semantic.Operation{Kind: semantic.OpBranch, Successors: []semantic.BranchTarget{{Block: next.ID, Arguments: []semantic.ValueID{op.Operands[0]}}}})
					fn.Blocks = append(fn.Blocks, next)
					changed = true
					break
				}
				if changed {
					break
				}
			}
			if !changed {
				t.Fatal("fixture lost length observation")
			}
			before := semantic.Format(m)
			output, err := EmitWithOptions(m, testPlan(width), EmitOptions{DialectVersion: 12})
			if err != nil {
				t.Fatal(err)
			}
			if before != semantic.Format(m) {
				t.Fatal("forwarded emission mutated input")
			}
			args := []string{"--sec-verify-slice-guards", "--sec-verify-places", "--sec-verify-reference-guards", "--sec-verify-borrow-semantics", "-"}
			cmd := exec.Command(tool, args...)
			cmd.Stdin = bytes.NewReader(output)
			canonical, err := cmd.CombinedOutput()
			if err != nil {
				t.Fatalf("forwarded facts failed real passes: %v\n%s", err, canonical)
			}
			round := exec.Command(tool, args...)
			round.Stdin = bytes.NewReader(canonical)
			again, err := round.CombinedOutput()
			if err != nil || !bytes.Equal(canonical, again) {
				t.Fatalf("forwarded round trip: %v\n%s", err, again)
			}
		})
	}
}
