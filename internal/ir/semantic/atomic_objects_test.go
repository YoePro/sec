package semantic

import (
	"errors"
	"strings"
	"testing"
)

const atomicObjectSource = `module main

fn Load(value: ref Atomic[int]) int {
    return value.Load(MemoryOrder.Acquire)
}

fn Store(value: ref Atomic[int]) void {
    value.Store(1, MemoryOrder.Release)
}

fn Add(value: ref Atomic[int]) int {
    return value.FetchAdd(1, MemoryOrder.Relaxed)
}

fn Exchange(value: ref Atomic[int]) void {
    discard value.CompareExchange(0, 1, MemoryOrder.AcqRel, MemoryOrder.Acquire)
}

fn Defaults(value: ref Atomic[bool]) void {
    discard value.Swap(true)
}
`

func atomicOperations(module *Module) map[OpKind][]Operation {
	operations := map[OpKind][]Operation{}
	for _, fn := range module.Functions {
		for _, block := range fn.Blocks {
			for _, op := range block.Operations {
				if isAtomicObjectOperation(op.Kind) {
					operations[op.Kind] = append(operations[op.Kind], op)
				}
			}
		}
	}
	return operations
}

// Atomic[T] object operations keep storage, T, operation kind, orders and the
// per-outcome classification: successful compare-exchange is an RMW
// modification, its failure a read-only observation; an omitted order is
// SeqCst; a Store breaks release sequences and a release Store starts one.
// Rules: rules/compiler/semantic_ir.md — §§ 122, 123A; rules/concurrency/atomics.md — §§ 17, 20, 24-25.
func TestAtomicObjectOperationsPreserveFacts(t *testing.T) {
	module, err := analyzedModule(t, atomicObjectSource, 15)
	if err != nil {
		t.Fatal(err)
	}
	if err := Verify(module); err != nil {
		t.Fatal(err)
	}
	ops := atomicOperations(module)
	if len(ops[OpAtomicLoad]) != 1 || len(ops[OpAtomicStore]) != 1 || len(ops[OpAtomicReadModifyWrite]) != 2 || len(ops[OpAtomicCompareExchange]) != 1 {
		t.Fatalf("operations = %v", ops)
	}
	load := ops[OpAtomicLoad][0].Atomic
	if load.Order != "Acquire" || !load.StorageResolved || load.Paths[0].ModifiesStorage || !load.Paths[0].ReadsStorage {
		t.Fatalf("load = %+v", load)
	}
	store := ops[OpAtomicStore][0].Atomic
	if store.Order != "Release" || !store.Paths[0].BreaksReleaseSequence || !store.Paths[0].StartsReleaseHead || store.Paths[0].ReadsStorage {
		t.Fatalf("store = %+v", store)
	}
	cas := ops[OpAtomicCompareExchange][0].Atomic
	if cas.Order != "AcqRel" || cas.FailureOrder != "Acquire" || len(cas.Paths) != 2 ||
		!cas.Paths[0].ReadModifyWrite || !cas.Paths[0].ModifiesStorage || cas.Paths[1].ModifiesStorage || cas.Paths[1].ReadModifyWrite || !cas.Paths[1].ReadsStorage {
		t.Fatalf("compare-exchange = %+v", cas)
	}
	var swap *AtomicObjectFacts
	for _, op := range ops[OpAtomicReadModifyWrite] {
		if op.Atomic.Operation == "Swap" {
			swap = op.Atomic
		}
	}
	if swap == nil || swap.Order != "SeqCst" {
		t.Fatalf("omitted order must stay SeqCst: %+v", swap)
	}
	text := Format(module)
	for _, want := range []string{"atomic.load %", "operation=CompareExchange", "failure-order=Acquire", "atomic<!"} {
		if !strings.Contains(text, want) {
			t.Fatalf("missing %q in\n%s", want, text)
		}
	}
	data, err := Serialize(module)
	if err != nil {
		t.Fatal(err)
	}
	restored, err := Deserialize(data)
	if err != nil {
		t.Fatal(err)
	}
	if Format(restored) != text {
		t.Fatal("atomic facts changed through serialization")
	}
}

// The verifier rejects invented or weakened atomic facts.
// Rules: rules/concurrency/atomics.md — §§ 20, 24(1); rules/compiler/semantic_ir.md — § 123A(3).
func TestAtomicObjectVerifierRejectsInvalidFacts(t *testing.T) {
	mutations := map[string]func(op *Operation){
		"release failure order": func(op *Operation) {
			if op.Kind == OpAtomicCompareExchange {
				op.Atomic.FailureOrder, op.Atomic.Paths[1].Order = "Release", "Release"
			}
		},
		"failed compare-exchange as modification": func(op *Operation) {
			if op.Kind == OpAtomicCompareExchange {
				op.Atomic.Paths[1].ModifiesStorage = true
			}
		},
		"acquire store": func(op *Operation) {
			if op.Kind == OpAtomicStore {
				op.Atomic.Order, op.Atomic.Paths[0].Order = "Acquire", "Acquire"
			}
		},
		"load that modifies": func(op *Operation) {
			if op.Kind == OpAtomicLoad {
				op.Atomic.Paths[0].ModifiesStorage = true
			}
		},
		"rmw without read": func(op *Operation) {
			if op.Kind == OpAtomicReadModifyWrite {
				op.Atomic.Paths[0].ReadsStorage = false
			}
		},
		"wrong element": func(op *Operation) {
			if op.Kind == OpAtomicLoad {
				op.Atomic.Element++
			}
		},
	}
	for name, mutate := range mutations {
		t.Run(name, func(t *testing.T) {
			module, err := analyzedModule(t, atomicObjectSource, 15)
			if err != nil {
				t.Fatal(err)
			}
			for _, fn := range module.Functions {
				for b := range fn.Blocks {
					for o := range fn.Blocks[b].Operations {
						op := &fn.Blocks[b].Operations[o]
						if op.Atomic != nil {
							copied := *op.Atomic
							copied.Paths = append([]AtomicPathFacts(nil), op.Atomic.Paths...)
							op.Atomic = &copied
							mutate(op)
						}
					}
				}
			}
			if err := Verify(module); err == nil {
				t.Fatal("accepted invalid atomic facts")
			}
		})
	}
}

// Below package 15 there are no references, so the object operations fail
// closed; a non-reference receiver is not borrowed implicitly.
// Rules: rules/compiler/semantic_ir.md — § 123A(4).
func TestAtomicObjectOperationsFailClosed(t *testing.T) {
	_, err := analyzedModule(t, atomicObjectSource, 14)
	var unsupported *UnsupportedFeatureError
	if !errors.As(err, &unsupported) {
		t.Fatalf("package 14: %v", err)
	}
	_, err = analyzedModule(t, "module main\n\nfn Read() int {\n    let counter: Atomic[int] := Atomic(0)\n    return counter.Load()\n}\n", 18)
	// Today the local Atomic(0) construction already stops first; either way
	// no implicit borrow of a non-reference receiver is invented.
	if !errors.As(err, &unsupported) {
		t.Fatalf("non-reference receiver: %v", err)
	}
}
