package semantic

import (
	"os"
	"strings"
	"testing"
)

// Complete-extent returns retain parameter origins without a range projection.
// Rules: rules/mlir/packages/sec-mlir-dialect_package16.md — §§60–62,66–67.
func TestPackage16FullOpenReturns(t *testing.T) {
	source, err := os.ReadFile("../../../testdata/semantic_ir/slice_full_open_returns.sec")
	if err != nil {
		t.Fatal(err)
	}
	for _, width := range []uint16{32, 64} {
		module, err := buildPackage16Source(t, string(source), width)
		if err != nil {
			t.Fatal(err)
		}
		if err := Verify(module); err != nil {
			t.Fatal(err)
		}
		repeated, err := buildPackage16Source(t, string(source), width)
		if err != nil || Format(module) != Format(repeated) {
			t.Fatalf("unstable return facts: %v", err)
		}
		for _, name := range []string{"Shared", "Mutable", "Reduced", "Named", "Nested"} {
			fn := functionNamed(t, module, name)
			if len(fn.ReturnOrigins) != 1 || fn.ReturnOrigins[0].Kind != ReturnOriginParameter || fn.ReturnOrigins[0].Parameter != 0 || len(fn.ReturnOrigins[0].Path) != 0 {
				t.Fatalf("%s acquired an identity range projection: %v", name, fn.ReturnOrigins)
			}
			calls, reborrows := 0, 0
			for _, block := range fn.Blocks {
				for _, op := range block.Operations {
					if op.Kind == OpDirectCall {
						calls++
					}
					if op.Kind == OpSliceReborrowShared || op.Kind == OpSliceReborrowMutable {
						reborrows++
					}
				}
			}
			want := 1
			if name == "Named" {
				want = 0
			}
			if name == "Mutable" {
				want = 2
			}
			if calls != want || reborrows == 0 {
				t.Fatalf("%s: calls=%d reborrows=%d", name, calls, reborrows)
			}
		}
		// An outer full-open range cannot erase a genuine narrowed source.
		narrow, err := os.ReadFile("../../../testdata/semantic_ir/package16_unsupported/full_open_narrowed_return.sec")
		if err != nil {
			t.Fatal(err)
		}
		narrowed, err := buildPackage16Source(t, string(narrow), width)
		if err != nil || !strings.Contains(Format(narrowed), "returns-from=[param#0[1..<3]]") {
			t.Fatalf("erased narrowed origin: %v", err)
		}
	}
}
