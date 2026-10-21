package semantic

import (
	"math/big"
	"strings"
	"testing"
)

// Malformed detached provenance cannot silently become discardable MLIR facts.
// Each mutation starts from a source-built, verified module; storage-fact
// mutation must not also mutate the operation's independent P15 metadata.
// Rules: rules/compiler/semantic_ir.md — §126.2–4,7–8,26;
// rules/mlir/packages/sec-mlir-dialect_package16.md — §§25–30,44,91.
func TestSliceValueFactsRejectContradictions(t *testing.T) {
	for _, test := range []struct {
		name   string
		change func(*SliceFacts)
	}{
		{"length SSA", func(f *SliceFacts) { f.RuntimeLength++ }},
		{"element identity", func(f *SliceFacts) { f.Element = 0 }},
		{"authority", func(f *SliceFacts) { f.Mutable = !f.Mutable }},
		{"source Place", func(f *SliceFacts) { f.OriginPlaces[0]++ }},
		{"storage identity detached", func(f *SliceFacts) { f.Reference.OriginStorages[0]++ }},
		{"epoch", func(f *SliceFacts) { f.Reference.EpochDependency++ }},
		{"missing bound", func(f *SliceFacts) { f.StaticRange.Start = nil }},
		{"negative bound", func(f *SliceFacts) { f.StaticRange.Start = big.NewInt(-1) }},
		{"reversed bound", func(f *SliceFacts) { f.StaticRange.Start = new(big.Int).Add(f.StaticRange.EndExclusive, big.NewInt(1)) }},
		{"static and dynamic", func(f *SliceFacts) { f.DynamicRange = 1 }},
	} {
		t.Run(test.name, func(t *testing.T) {
			program, a := sliceBoundaryInput(t, "semantic_ir/slice_source_reborrow.sec")
			m, err := Build(program, a, BuildOptions{RequestedModule: "main", MaxPackage: 16})
			if err != nil {
				t.Fatal(err)
			}
			if err := Verify(m); err != nil {
				t.Fatal(err)
			}
			changed := false
			for _, fn := range m.Functions {
				for _, block := range fn.Blocks {
					for i := range block.Operations {
						facts := block.Operations[i].Slice
						if !changed && facts != nil && facts.StaticRange != nil && len(facts.OriginPlaces) > 0 {
							test.change(facts)
							changed = true
						}
					}
				}
			}
			if !changed {
				t.Fatal("missing provenance mutation target")
			}
			if err := Verify(m); err == nil || !strings.Contains(err.Error(), "slice value facts") {
				t.Fatal("invalid provenance accepted", err)
			}
		})
	}
}
