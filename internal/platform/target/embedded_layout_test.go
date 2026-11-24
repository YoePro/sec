package target

import (
	"testing"

	"sec/internal/layout"
)

// Concrete embedded registrations resolve their own verified ABI layouts while
// preserving planned emission/link/run status. Architecture placeholders have no
// such plan, and cannot inherit layout facts from a host or an RTOS family.
// Rules: rules/platform/target_profiles.md — §40; rules/memory/layout.md — §47;
// MD-141 correction §§4,10(7),14.
func TestEmbeddedTargetLayoutCoverage(t *testing.T) {
	for _, test := range []struct {
		os, arch, triple string
		align            int64
	}{
		{"baremetal", "cortex-m0", "thumbv6m-none-eabi", 8},
		{"baremetal", "cortex-m3", "thumbv7m-none-eabi", 8},
		{"baremetal", "cortex-m4", "thumbv7em-none-eabi", 8},
		{"baremetal", "cortex-m7", "thumbv7em-none-eabi", 8},
		{"baremetal", "riscv32", "riscv32-unknown-elf", 8},
		{"freertos", "cortex-m4", "thumbv7em-none-eabi", 8},
		{"freertos", "cortex-m7", "thumbv7em-none-eabi", 8},
	} {
		t.Run(test.os+"/"+test.arch, func(t *testing.T) {
			definition, ok := Find(Target{OS: test.os, Arch: test.arch})
			if !ok || definition.LLVMTriple != test.triple {
				t.Fatalf("registration %+v", definition)
			}
			if definition.Status != Planned || definition.CanEmitLLVM || definition.CanLink || definition.CanRun {
				t.Fatalf("layout facts promoted execution support: %+v", definition)
			}
			plan, err := definition.ScalarPlan()
			if err != nil {
				t.Fatal(err)
			}
			if plan.PointerWidthBits != 32 || plan.Endianness != layout.LittleEndian || len(plan.Alignments) != 8 {
				t.Fatalf("plan %+v", plan)
			}
			resolver, err := definition.LayoutResolver()
			if err != nil {
				t.Fatal(err)
			}
			for _, width := range []uint16{8, 16, 32, 64, 128, 256} {
				req := layout.LayoutRequirement{TypeIdentity: "integer" + layout.StorageRepresentation{Class: layout.IntegerStorage, WidthBits: width}.String(), Kind: layout.LayoutScalar, Scalar: layout.ScalarUnsigned, WidthBits: width}
				resolved := resolver.Resolve(req)
				if !resolved.Complete() || resolved.SizeBytes.Int64() != int64(width/8) {
					t.Fatalf("%d-bit integer: %+v", width, resolved)
				}
			}
			decimal := resolver.Resolve(layout.LayoutRequirement{TypeIdentity: "decimal128", Kind: layout.LayoutScalar, Scalar: layout.ScalarDecimal, WidthBits: 128})
			if !decimal.Complete() || decimal.SizeBytes.Int64() != 24 || decimal.AlignBytes.Int64() != test.align || decimal.Fields[1].Offset.Int64() != 16 {
				t.Fatalf("decimal carrier %+v", decimal)
			}
		})
	}
	for _, os := range []string{"rtems", "zephyr"} {
		definition, ok := Find(Target{OS: os, Arch: "any"})
		if !ok {
			t.Fatal(os)
		}
		if definition.LLVMTriple != "" {
			t.Fatalf("guessed triple for %s", os)
		}
		if facts, err := definition.ScalarAlignments(); err != nil || facts != nil {
			t.Fatalf("guessed facts for %s: %v %v", os, facts, err)
		}
		if _, err := definition.ScalarPlan(); err == nil {
			t.Fatalf("%s/any resolved without an architecture", os)
		}
	}
}
