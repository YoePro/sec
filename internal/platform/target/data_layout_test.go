package target

import (
	"os/exec"
	"regexp"
	"strings"
	"testing"

	"sec/internal/layout"
)

func representation(class layout.StorageClass, width uint16) layout.StorageRepresentation {
	return layout.StorageRepresentation{Class: class, WidthBits: width}
}

// MD-141 § 4(1-2): every registered target with an LLVM triple resolves a
// complete fact set for every storage representation from its maintained
// data layout; a planned target without a triple stays unresolved instead of
// receiving invented alignments.
func TestEveryTargetHasScalarAlignmentFacts(t *testing.T) {
	for _, definition := range Definitions() {
		facts, err := definition.ScalarAlignments()
		if err != nil {
			t.Fatalf("%s: %v", definition.LLVMTriple, err)
		}
		if definition.LLVMTriple == "" {
			if facts != nil || definition.Status != Planned {
				t.Errorf("%s/%s without a triple has facts %v (status %s)", definition.OS, definition.Arch, facts, definition.Status)
			}
			continue
		}
		if len(facts) != 8 {
			t.Errorf("%s has %d scalar alignment facts, want 8: %v", definition.LLVMTriple, len(facts), facts)
		}
	}
	if facts, err := (Definition{LLVMTriple: "unknown-unknown-unknown"}).ScalarAlignments(); facts != nil || err != nil {
		t.Errorf("an unrecorded triple got facts %v, %v", facts, err)
	}
}

// MD-141 § 4: concrete per-target values, checked independently per
// architecture rather than by pointer width.
func TestTargetScalarAlignmentValues(t *testing.T) {
	integer := func(width uint16) layout.StorageRepresentation { return representation(layout.IntegerStorage, width) }
	for triple, want := range map[string]map[layout.StorageRepresentation]uint64{
		"x86_64-pc-linux-gnu":           {integer(8): 1, integer(32): 4, integer(64): 8, integer(128): 16, integer(256): 16},
		"aarch64-unknown-linux-gnu":     {integer(8): 1, integer(16): 2, integer(64): 8, integer(128): 16},
		"armv7-unknown-linux-gnueabihf": {integer(64): 8, integer(128): 8, integer(256): 8},
		"wasm32-unknown-unknown":        {integer(64): 8, integer(128): 16},
		"riscv64-unknown-linux-gnu":     {integer(64): 8, integer(128): 16},
	} {
		definition := definitionByTriple(t, triple)
		facts, err := definition.ScalarAlignments()
		if err != nil {
			t.Fatal(err)
		}
		for key, alignment := range want {
			if facts[key] != alignment {
				t.Errorf("%s %s = %d, want %d", triple, key, facts[key], alignment)
			}
		}
		if facts[representation(layout.BinaryFloatStorage, 32)] != 4 || facts[representation(layout.BinaryFloatStorage, 64)] != 8 {
			t.Errorf("%s float facts = %v", triple, facts)
		}
	}
}

// MD-141 § 4(3): the maintained table is a machine-checked contract against
// the toolchain named by DataLayoutProvenance.
func TestTargetDataLayoutsMatchClang(t *testing.T) {
	clang, err := exec.LookPath("clang")
	if err != nil {
		t.Skip("clang not found; the data-layout table is not re-verified in this run")
	}
	version, _ := exec.Command(clang, "--version").Output()
	if !strings.Contains(string(version), "clang version 21.") {
		t.Skipf("clang is not the 21.x toolchain of %q", DataLayoutProvenance)
	}
	pattern := regexp.MustCompile(`target datalayout = "([^"]+)"`)
	for _, definition := range Definitions() {
		if definition.LLVMTriple == "" {
			continue
		}
		want, ok := definition.DataLayout()
		if !ok {
			t.Errorf("%s has no recorded data layout", definition.LLVMTriple)
			continue
		}
		output, err := exec.Command(clang, "-target", definition.LLVMTriple, "-S", "-emit-llvm", "-x", "c", "/dev/null", "-o", "-").Output()
		if err != nil {
			t.Errorf("%s: clang: %v", definition.LLVMTriple, err)
			continue
		}
		match := pattern.FindSubmatch(output)
		if match == nil || string(match[1]) != want {
			t.Errorf("%s data layout = %q, recorded %q", definition.LLVMTriple, match, want)
		}
	}
}

// MD-141/142 end to end: a definition's resolver places decimal128 with the
// target's own i128 alignment, so ARMv7 and x86-64 differ.
func TestTargetLayoutResolverUsesTargetFacts(t *testing.T) {
	decimal128 := layout.LayoutRequirement{TypeIdentity: "decimal128", Kind: layout.LayoutScalar, Scalar: layout.ScalarDecimal, WidthBits: 128}
	for triple, want := range map[string][2]int64{
		"x86_64-pc-linux-gnu":           {32, 16},
		"armv7-unknown-linux-gnueabihf": {24, 8},
	} {
		resolver, err := definitionByTriple(t, triple).LayoutResolver()
		if err != nil {
			t.Fatal(err)
		}
		resolved := resolver.Resolve(decimal128)
		if !resolved.Complete() || resolved.SizeBytes.Int64() != want[0] || resolved.AlignBytes.Int64() != want[1] {
			t.Errorf("%s decimal128 = %s size=%v align=%v %v", triple, resolved.Completeness, resolved.SizeBytes, resolved.AlignBytes, resolved.Missing)
		}
	}
}

func definitionByTriple(t *testing.T, triple string) Definition {
	t.Helper()
	for _, definition := range Definitions() {
		if definition.LLVMTriple == triple {
			return definition
		}
	}
	t.Fatalf("no registered target %s", triple)
	return Definition{}
}
