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

func TestCallGraphRecordsOrderedDirectArenaEffects(t *testing.T) {
	analyzer, errors := analyzeSourceWithAnalyzerRaw(t, `
module main

fn Use(buffer: ref mut byte[]) void {
	let mut arena := Arena.FromBuffer(buffer)
	let one := arena.New[int]()
	arena.Reset()
	let many := arena.Alloc[byte](4u)
	arena.Release()
}
`)
	assertSemaErrors(t, errors, nil)

	graph := analyzer.CallGraph()
	useID := callGraphNodeIDByName(t, graph, "Use")
	summary := graph.ArenaSummary(useID)
	want := []ArenaEffectKind{
		ArenaEffectCreateBorrowed,
		ArenaEffectAllocate,
		ArenaEffectReset,
		ArenaEffectAllocate,
		ArenaEffectRelease,
	}
	if len(summary.DirectEffects) != len(want) {
		t.Fatalf("direct Arena effects = %+v, want %v", summary.DirectEffects, want)
	}
	for index, kind := range want {
		if summary.DirectEffects[index].Kind != kind {
			t.Fatalf("Arena effect %d = %q, want %q", index, summary.DirectEffects[index].Kind, kind)
		}
	}
	if !summary.MayAllocate || len(summary.AllocationPath) != 1 || summary.AllocationPath[0] != useID {
		t.Fatalf("Arena summary = %+v, want direct allocation path", summary)
	}
}

func TestCallGraphPropagatesArenaAllocationOnlyAcrossSynchronousCalls(t *testing.T) {
	analyzer, errors := analyzeSourceWithAnalyzerRaw(t, `
module main

fn Allocate() void {
	let arena := Arena.WithCapacity(64u)
}

fn Forward() void {
	Allocate()
}

fn Worker() void {
	Allocate()
}

fn main() void {
	Forward()
	let worker := spawn Worker()
	detach worker
}
`)
	assertSemaErrors(t, errors, nil)

	graph := analyzer.CallGraph()
	allocateID := callGraphNodeIDByName(t, graph, "Allocate")
	forwardID := callGraphNodeIDByName(t, graph, "Forward")
	workerID := callGraphNodeIDByName(t, graph, "Worker")
	mainID := callGraphNodeIDByName(t, graph, "main")

	forward := graph.ArenaSummary(forwardID)
	if !forward.MayAllocate || len(forward.AllocationPath) != 2 || forward.AllocationPath[0] != forwardID || forward.AllocationPath[1] != allocateID {
		t.Fatalf("Forward Arena summary = %+v", forward)
	}
	worker := graph.ArenaSummary(workerID)
	if !worker.MayAllocate || len(worker.AllocationPath) != 2 || worker.AllocationPath[1] != allocateID {
		t.Fatalf("Worker Arena summary = %+v", worker)
	}
	mainSummary := graph.ArenaSummary(mainID)
	if !mainSummary.MayAllocate {
		t.Fatalf("main should inherit allocation through synchronous Forward: %+v", mainSummary)
	}
	if len(mainSummary.AllocationPath) != 3 || mainSummary.AllocationPath[1] != forwardID {
		t.Fatalf("main allocation path = %+v, want main -> Forward -> Allocate", mainSummary.AllocationPath)
	}
}

func TestCallGraphDoesNotPropagateSpawnedArenaAllocationToSpawner(t *testing.T) {
	analyzer, errors := analyzeSourceWithAnalyzerRaw(t, `
module main

fn Worker() void {
	let arena := Arena.Growable(64u)
}

fn main() void {
	let worker := spawn Worker()
	detach worker
}
`)
	assertSemaErrors(t, errors, nil)

	graph := analyzer.CallGraph()
	workerID := callGraphNodeIDByName(t, graph, "Worker")
	mainID := callGraphNodeIDByName(t, graph, "main")
	if !graph.ArenaSummary(workerID).MayAllocate {
		t.Fatal("Worker must retain its direct Arena allocation effect")
	}
	if summary := graph.ArenaSummary(mainID); summary.MayAllocate || len(summary.AllocationPath) != 0 {
		t.Fatalf("spawned body allocation leaked into spawner summary: %+v", summary)
	}
}
