package sema

import "testing"

func TestAllocationMetadataDefaultsToNoAllocation(t *testing.T) {
	input := `
module main

fn Identity(value: int) int {
    let copy := value
    return copy
}
`

	analyzer, errors := analyzeSourceWithAnalyzerRaw(t, input)
	assertSemaErrors(t, errors, nil)

	functions := analyzer.functions["Identity"]
	if len(functions) != 1 {
		t.Fatalf("wrong function count. got=%d want=1", len(functions))
	}
	if functions[0].AllocationEffect != AllocationEffectNone {
		t.Fatalf("wrong allocation effect. got=%q want=%q", functions[0].AllocationEffect, AllocationEffectNone)
	}
	if analyzer.allocationContext.Available {
		t.Fatalf("allocation context should be unavailable until Arena support is implemented: %+v", analyzer.allocationContext)
	}
}

func TestSymbolStorageOrigins(t *testing.T) {
	input := `
module main

type Peripheral register[8] {
    Enabled: bit,
    _: bit[7],
}

let ordinary := 1

@address(0x40021000)
let mut peripheral: Peripheral
`

	analyzer, errors := analyzeSourceWithAnalyzerRaw(t, input)
	assertSemaErrors(t, errors, nil)

	ordinary := analyzer.symbols["ordinary"]
	if ordinary.Storage != StorageOriginInline {
		t.Fatalf("ordinary local storage origin = %q, want %q", ordinary.Storage, StorageOriginInline)
	}

	peripheral := analyzer.symbols["peripheral"]
	if peripheral.Storage != StorageOriginFixedAddress {
		t.Fatalf("addressed register storage origin = %q, want %q", peripheral.Storage, StorageOriginFixedAddress)
	}
	if !peripheral.Addressed || !peripheral.Volatile {
		t.Fatalf("addressed register metadata missing: %+v", peripheral)
	}
}
