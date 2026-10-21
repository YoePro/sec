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

// Forward actual source-derived slice/index and normalized tuple operands in
// an explicit white-box CFG transformation. Exact proofs must survive verified
// emission and all four real passes, independently on 32/64 scalar plans.
// Rules: rules/compiler/semantic_ir.md — §§20,66,126.23;
// rules/mlir/packages/sec-mlir-dialect_package16.md — §§79–86,141.
func TestPackage16EmitForwardedSliceProofs(t *testing.T) {
	tool := testsupport.RequireSecMLIROptPath(t)
	for _, width := range []uint16{32, 64} {
		for _, tc := range []struct {
			name, file string
			kind       semantic.OpKind
		}{
			{"index", "slice_source_index.sec", semantic.OpSliceElementPlace},
			{"reborrow", "slice_source_reborrow.sec", semantic.OpSliceReborrowShared},
		} {
			t.Run(fmt.Sprintf("%d/%s", width, tc.name), func(t *testing.T) {
				m := sliceEmitModule(t, tc.file, width)
				edge := forwardEmittedSliceProof(t, m, tc.kind)
				before := semantic.Format(m)
				out, err := EmitWithOptions(m, testPlan(width), EmitOptions{DialectVersion: 12})
				if err != nil {
					t.Fatal(err)
				}
				if before != semantic.Format(m) {
					t.Fatal("emission changed original SSA")
				}
				for _, scalar := range []bool{false, true} {
					args := []string{"--sec-verify-slice-guards", "--sec-verify-places", "--sec-verify-reference-guards", "--sec-verify-borrow-semantics", "-"}
					if scalar {
						args = append([]string{"--sec-resolve-scalar-layout"}, args...)
					}
					cmd := exec.Command(tool, args...)
					cmd.Stdin = bytes.NewReader(out)
					parsed, err := cmd.CombinedOutput()
					if err != nil {
						t.Fatalf("forwarded proof: %v\n%s", err, parsed)
					}
					round := exec.Command(tool, args...)
					round.Stdin = bytes.NewReader(parsed)
					again, err := round.CombinedOutput()
					if err != nil || !bytes.Equal(parsed, again) {
						t.Fatalf("proof round trip: %v\n%s", err, again)
					}
				}
				if tc.kind == semantic.OpSliceReborrowShared {
					edge.Arguments[1], edge.Arguments[2] = edge.Arguments[2], edge.Arguments[1]
					out, err := EmitWithOptions(m, testPlan(width), EmitOptions{DialectVersion: 12})
					if out != nil || err == nil || !strings.Contains(err.Error(), "same normalized range start/length tuple") {
						t.Fatal("forged tuple emitted partial bytes or lost owner", err)
					}
				}
			})
		}
	}
}

// forwardEmittedSliceProof retains source facts while forwarding every operand
// of one proof-consuming operation through typed block arguments. Attached
// runtime-length facts follow the transported length SSA; no proof is invented.
// Rules: rules/compiler/semantic_ir.md — §§66,126.2–4,126.7–8,126.23.
func forwardEmittedSliceProof(t *testing.T, m *semantic.Module, kind semantic.OpKind) *semantic.BranchTarget {
	t.Helper()
	for _, fn := range m.Functions {
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
		for _, block := range fn.Blocks {
			for index, op := range block.Operations {
				if op.Kind != kind {
					continue
				}
				tail := append([]semantic.Operation(nil), block.Operations[index:]...)
				tail[0].Operands = make([]semantic.ValueID, len(op.Operands))
				params := make([]semantic.Value, len(op.Operands))
				for i, id := range op.Operands {
					v := values[id]
					maxValue++
					v.ID = maxValue
					params[i] = v
					tail[0].Operands[i] = v.ID
				}
				if op.Slice != nil {
					facts := op.Slice.Clone()
					tail[0].Slice = &facts
					tail[0].Slice.RuntimeLength = tail[0].Operands[2]
				}
				next := &semantic.Block{ID: maxBlock + 1, Parameters: params, Operations: tail}
				branch := semantic.Operation{Kind: semantic.OpBranch, Successors: []semantic.BranchTarget{{Block: next.ID, Arguments: append([]semantic.ValueID(nil), op.Operands...)}}}
				block.Operations = append(append([]semantic.Operation(nil), block.Operations[:index]...), branch)
				fn.Blocks = append(fn.Blocks, next)
				return &block.Operations[len(block.Operations)-1].Successors[0]
			}
		}
	}
	t.Fatal("fixture lost proof consumer", kind)
	return nil
}
