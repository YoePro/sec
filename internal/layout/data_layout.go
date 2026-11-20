package layout

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
)

// dataLayoutSpec is one primitive alignment entry of an LLVM data layout:
// a bit width and its ABI alignment in bytes.
type dataLayoutSpec struct {
	widthBits uint64
	abiBytes  uint64
}

// llvmDefaultIntegerSpecs and llvmDefaultFloatSpecs are LLVM's documented
// defaults that a data-layout string overrides (llvm/lib/IR/DataLayout.cpp:
// i8:8:8, i16:16:16, i32:32:32, i64:32:64; f16, f32, f64, f128 natural).
// Rules: MD-141 correction § 3(5) — the producer applies the toolchain's
// documented default-resolution algorithm.
var (
	llvmDefaultIntegerSpecs = []dataLayoutSpec{{8, 1}, {16, 2}, {32, 4}, {64, 4}}
	llvmDefaultFloatSpecs   = []dataLayoutSpec{{16, 2}, {32, 4}, {64, 8}, {128, 16}}
)

// ScalarAlignmentsFromLLVMDataLayout normalizes the scalar ABI alignments of
// an LLVM target data-layout string into typed facts for every Sec storage
// representation: IntegerStorage(8..256) and BinaryFloatStorage(32, 64).
//
// Integer widths follow LLVM's lookup: the entry of the smallest specified
// width >= the requested width, or the largest specified width when none is
// larger (so i128 on a layout without an i128 entry uses its i64 alignment).
// Floats require an exact entry, which the defaults always provide for 32
// and 64 bits. Non-alignment components (endianness, mangling, pointers,
// native widths, stack, function-pointer and vector entries) are ignored;
// a malformed integer or float entry is an error, never guessed.
// Rules: rules/memory/layout.md — § 47.2; rules/platform/target_profiles.md — § 40;
// MD-141 correction §§ 3(4)–3(6).
func ScalarAlignmentsFromLLVMDataLayout(dataLayout string) (ScalarAlignments, error) {
	integers := append([]dataLayoutSpec(nil), llvmDefaultIntegerSpecs...)
	floats := append([]dataLayoutSpec(nil), llvmDefaultFloatSpecs...)
	for _, component := range strings.Split(dataLayout, "-") {
		if len(component) < 2 || (component[0] != 'i' && component[0] != 'f') || component[1] < '0' || component[1] > '9' {
			continue
		}
		spec, err := parseDataLayoutSpec(component)
		if err != nil {
			return nil, err
		}
		if component[0] == 'i' {
			integers = upsertDataLayoutSpec(integers, spec)
		} else {
			floats = upsertDataLayoutSpec(floats, spec)
		}
	}
	facts := ScalarAlignments{}
	for _, width := range []uint16{8, 16, 32, 64, 128, 256} {
		facts[StorageRepresentation{IntegerStorage, width}] = integerAlignment(integers, uint64(width))
	}
	for _, width := range []uint16{32, 64} {
		alignment, found := exactAlignment(floats, uint64(width))
		if !found {
			return nil, fmt.Errorf("data layout %q has no binary%d alignment", dataLayout, width)
		}
		facts[StorageRepresentation{BinaryFloatStorage, width}] = alignment
	}
	for representation, alignment := range facts {
		if alignment == 0 || alignment&(alignment-1) != 0 {
			return nil, fmt.Errorf("data layout %q gives %s alignment %d, not a positive power of two", dataLayout, representation, alignment)
		}
	}
	return facts, nil
}

// parseDataLayoutSpec reads "iN:abi[:pref]" or "fN:abi[:pref]" with bit
// counts; only the ABI alignment is the required alignment (MD-141 § 3(1)).
// Rules: MD-141 correction § 3(5).
func parseDataLayoutSpec(component string) (dataLayoutSpec, error) {
	parts := strings.Split(component[1:], ":")
	if len(parts) < 2 || len(parts) > 3 {
		return dataLayoutSpec{}, fmt.Errorf("data layout entry %q is not <width>:<abi>[:<pref>]", component)
	}
	width, err := strconv.ParseUint(parts[0], 10, 32)
	if err != nil || width == 0 {
		return dataLayoutSpec{}, fmt.Errorf("data layout entry %q has an invalid width", component)
	}
	abi, err := strconv.ParseUint(parts[1], 10, 32)
	if err != nil || abi == 0 || abi%8 != 0 {
		return dataLayoutSpec{}, fmt.Errorf("data layout entry %q has an ABI alignment that is not a whole number of bytes", component)
	}
	return dataLayoutSpec{widthBits: width, abiBytes: abi / 8}, nil
}

// upsertDataLayoutSpec replaces or inserts spec, keeping width order.
// Rules: MD-141 correction § 3(5).
func upsertDataLayoutSpec(specs []dataLayoutSpec, spec dataLayoutSpec) []dataLayoutSpec {
	for index := range specs {
		if specs[index].widthBits == spec.widthBits {
			specs[index] = spec
			return specs
		}
	}
	specs = append(specs, spec)
	sort.Slice(specs, func(i, j int) bool { return specs[i].widthBits < specs[j].widthBits })
	return specs
}

// integerAlignment is LLVM's DataLayout::getIntegerAlignment lookup.
// Rules: MD-141 correction § 3(5).
func integerAlignment(specs []dataLayoutSpec, width uint64) uint64 {
	for _, spec := range specs {
		if spec.widthBits >= width {
			return spec.abiBytes
		}
	}
	return specs[len(specs)-1].abiBytes
}

// exactAlignment returns the alignment of exactly width bits.
// Rules: MD-141 correction § 3(5).
func exactAlignment(specs []dataLayoutSpec, width uint64) (uint64, bool) {
	for _, spec := range specs {
		if spec.widthBits == width {
			return spec.abiBytes, true
		}
	}
	return 0, false
}
