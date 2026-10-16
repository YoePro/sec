package semantic

import (
	"strings"
	"testing"
)

const staticStorageFixture = `module main

let limit: int := 7
let mut counter: int := -2
let mut ready: bool

fn Next() int {
    static let mut calls: int := 0
    calls = calls + 1
    return calls
}

fn Bump() int {
    counter = counter + limit
    ready = true
    return counter
}
`

// Static-duration bindings lower to module statics with compile-time
// initializers (semantic_ir.md § 28; static.md §§ 2-4): module-level lets and
// a function-local `static let mut`, which keeps one storage location across
// calls instead of being re-initialized as automatic local storage — the
// pre-existing miscompilation this fixes at every package level. Reads and
// writes are static.load / static.store; no startup function exists.
//
// Rules:
//   - rules/compiler/semantic_ir.md — § 28
//   - rules/declarations/static.md — §§ 2, 3, 4
func TestStaticStorageLowersToModuleStatics(t *testing.T) {
	for _, pkg := range []uint8{14, 15} {
		module, err := analyzedModule(t, staticStorageFixture, pkg)
		if err != nil {
			t.Fatalf("package %d: build: %v", pkg, err)
		}
		if err := Verify(module); err != nil {
			t.Fatalf("package %d: verify: %v", pkg, err)
		}
		text := Format(module)
		for _, want := range []string{
			`@1 "main::limit": !1 mutable=false init=7`,
			`@2 "main::counter": !1 mutable=true init=-2`,
			`"main::ready": !2 mutable=true init=false`,
			`"main::Next::calls": !1 mutable=true function="Next" init=0`,
			"static.store @", "static.load @",
		} {
			if !strings.Contains(text, want) {
				t.Errorf("package %d: module lacks %q:\n%s", pkg, want, text)
			}
		}
		next := functionNamed(t, module, "Next")
		if len(next.Storages) != 0 {
			t.Errorf("package %d: the function-local static became automatic storage: %+v", pkg, next.Storages)
		}
		for _, block := range next.Blocks {
			for _, op := range block.Operations {
				if op.Kind == OpStorageDeclare || op.Kind == OpStorageInit {
					t.Errorf("package %d: Next re-initializes its static with %s on every call", pkg, op.Kind)
				}
			}
		}
	}
}

// Statics are Place roots at package 15 (P15-42a): `ref` / `ref mut` of a
// static forms place.static-root with the static's authority and marks it
// address-taken, the borrow carries Sema's static-storage facts, and a
// function may return a reference to module static storage, recorded as a
// static origin in its summary and verified by tracing.
//
// Rules:
//   - rules/mlir/packages/sec-mlir-dialect_package15.md — §§ 10, 61, 62, 64, 115
func TestPackage15LowersStaticPlaceRoots(t *testing.T) {
	module, err := analyzedModule(t, `module main

let shared: int := 7
let mut counter: int := 0
let mut untouched: int := 1

fn Borrow() int {
    let r := ref shared
    let w := ref mut counter
    return 0
}

fn Fixed() ref int {
    return ref shared
}

fn Use() int {
    let fixed := Fixed()
    return untouched
}
`, 15)
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	if err := Verify(module); err != nil {
		t.Fatalf("verify: %v\n%s", err, Format(module))
	}
	text := Format(module)
	for _, want := range []string{"place.static-root @1", "place.static-root @2", "kind=static", "proof=static-storage", "provenance=static", "returns-from=[static@1]"} {
		if !strings.Contains(text, want) {
			t.Errorf("module lacks %q:\n%s", want, text)
		}
	}
	for _, static := range module.Statics {
		if static.AddressTaken != (static.Name != "main::untouched") {
			t.Errorf("%s address-taken=%t", static.Name, static.AddressTaken)
		}
	}
	use := functionNamed(t, module, "Use")
	for _, block := range use.Blocks {
		for _, op := range block.Operations {
			if op.Kind == OpDirectCall && (op.Reference == nil || op.Reference.Provenance != "static") {
				t.Errorf("Fixed() result lacks instantiated static facts: %+v", op.Reference)
			}
		}
	}
}

// The verifier checks static declarations and operations: initializer and
// type agreement, stores only to mutable statics, typed loads, and static
// Place roots that do not exceed the static's mutability and that address an
// address-taken static.
func TestVerifierChecksStaticStorage(t *testing.T) {
	for name, mutate := range map[string]func(module *Module){
		"store to immutable": func(module *Module) { module.Statics[1].Mutable = false },
		"bool initializer on int": func(module *Module) {
			value := true
			module.Statics[0].Initializer = StaticInitializer{Bool: &value}
		},
		"missing initializer": func(module *Module) { module.Statics[0].Initializer = StaticInitializer{} },
		"unknown static": func(module *Module) {
			for _, fn := range module.Functions {
				for _, block := range fn.Blocks {
					for index := range block.Operations {
						if block.Operations[index].Kind == OpStaticLoad {
							block.Operations[index].Static = 99
							return
						}
					}
				}
			}
		},
		"root without address-taken": func(module *Module) { module.Statics[1].AddressTaken = false },
		"root authority exceeds mutability": func(module *Module) {
			module.Statics[1].Mutable = false
			for _, fn := range module.Functions {
				for _, block := range fn.Blocks {
					for index := range block.Operations {
						if block.Operations[index].Kind == OpStaticStore {
							block.Operations[index].Kind = OpStaticLoad
							block.Operations[index].Results = []Value{{ID: 999, Type: module.Statics[1].Type}}
							block.Operations[index].Operands = nil
						}
					}
				}
			}
		},
	} {
		t.Run(name, func(t *testing.T) {
			module, err := analyzedModule(t, `module main

let limit: int := 7
let mut counter: int := 0

fn Use() int {
    counter = limit
    let w := ref mut counter
    return limit
}
`, 15)
			if err != nil {
				t.Fatalf("build: %v", err)
			}
			if err := Verify(module); err != nil {
				t.Fatalf("unmutated module does not verify: %v", err)
			}
			mutate(module)
			if err := Verify(module); err == nil {
				t.Fatalf("verifier accepted %s:\n%s", name, Format(module))
			}
		})
	}
}
