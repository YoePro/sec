package semantic

import (
	"strings"
	"testing"
)

const returnedReferenceFixture = `module main

type Cell struct {
    v: int,
}

type Pair struct {
    left: Cell,
    right: Cell,
    cells: Cell[3],
}

impl Pair {
    fn First() ref Cell {
        return ref self.left
    }
}

fn Whole(pair: ref Pair) ref Pair {
    return pair
}

fn Left(pair: ref Pair) ref Cell {
    return ref pair.left
}

fn RightMut(pair: ref mut Pair) ref mut Cell {
    return ref mut pair.right
}

fn Pick(a: ref Cell, b: ref Cell, first: bool) ref Cell {
    if first {
        return a
    }
    return b
}

fn Holder(pair: ref Pair) ref Cell {
    let picked := ref pair.left
    return picked
}

fn Element(pair: ref Pair, at: int) ref Cell {
    return ref pair.cells[at]
}

fn Constant(pair: ref Pair) ref Cell {
    return ref pair.cells[1]
}

fn Again(pair: ref Pair) ref Cell {
    return Left(pair)
}

fn Use() int {
    let mut pair := Pair { left: Cell { v: 1 }, right: Cell { v: 2 }, cells: [Cell { v: 3 }, Cell { v: 4 }, Cell { v: 5 }] }
    let left := Left(ref pair)
    let x := left.v
    let right := RightMut(ref mut pair)
    right.v = 5
    return x
}
`

// Returned references (P15-47, P15-64): a function returning a direct
// reference carries its returned-origin summary as positional metadata
// (parameter or receiver, then P13 field identities and element steps), a
// returned reference is the parameter's own reference, a sub-Place reborrow
// of it, a local holder of such a reborrow, or a call result traced through
// the callee's summary; and a call site's result carries caller-side facts
// instantiated from the arguments. No aggregate is copied to form a result.
//
// Rules:
//   - rules/mlir/packages/sec-mlir-dialect_package15.md — §§ 46-48, 59-62, 78
//   - rules/compiler/semantic_ir.md — §§ 45(12), 73(4), 96(5)
func TestPackage15LowersReturnedReferences(t *testing.T) {
	module, err := analyzedModule(t, returnedReferenceFixture, 15)
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	if err := Verify(module); err != nil {
		t.Fatalf("verify: %v\n%s", err, Format(module))
	}
	text := Format(module)
	for name, want := range map[string]string{
		"Pair.First": "returns-from=[receiver.field#0]",
		"Whole":      "returns-from=[param#0]",
		"Left":       "returns-from=[param#0.field#0]",
		"RightMut":   "returns-from=[param#0.field#1]",
		"Pick":       "returns-from=[param#0, param#1]",
		"Holder":     "returns-from=[param#0.field#0]",
		"Element":    "returns-from=[param#0.field#2[*]]",
		"Constant":   "returns-from=[param#0.field#2[1]]",
		"Again":      "returns-from=[param#0.field#0]",
	} {
		fn := functionNamed(t, module, name)
		var header string
		for _, line := range strings.Split(text, "\n") {
			if strings.Contains(line, "func \""+string(fn.ID)+"\"") {
				header = line
			}
		}
		if !strings.Contains(header, want) {
			t.Errorf("%s: header lacks %q: %s", name, want, header)
		}
		for _, block := range fn.Blocks {
			for _, op := range block.Operations {
				if op.Kind == OpStructExtractField || op.Kind == OpArrayExtract || op.Kind == OpStorageLoad {
					t.Errorf("%s copied an aggregate to form its result: %s", name, op.Kind)
				}
			}
		}
	}
	use := functionNamed(t, module, "Use")
	calls := 0
	for _, block := range use.Blocks {
		for _, op := range block.Operations {
			if op.Kind != OpDirectCall {
				continue
			}
			calls++
			if op.Reference == nil || op.Reference.Provenance != "local" || len(op.Reference.OriginStorages) != 1 {
				t.Errorf("call %s result lacks instantiated caller-side facts: %+v", op.Callee, op.Reference)
			}
		}
	}
	if calls != 2 {
		t.Errorf("Use: want two calls returning references, got %d", calls)
	}
	again, err := analyzedModule(t, returnedReferenceFixture, 15)
	if err != nil || Format(again) != text {
		t.Errorf("returned-origin summaries and instantiated call facts are not deterministic (%v)", err)
	}
}

// The verifier requires the origin summary and checks every returned
// reference against it: a missing summary, a summary that omits the actual
// path, and a returned borrow of local storage are rejected.
func TestPackage15VerifierChecksReturnedReferenceOrigins(t *testing.T) {
	for name, mutate := range map[string]func(fn *Function){
		"missing summary": func(fn *Function) { fn.ReturnOrigins = nil },
		"wrong path": func(fn *Function) {
			fn.ReturnOrigins = []ReturnOrigin{{Kind: ReturnOriginParameter, Parameter: 0, Path: []ReturnOriginStep{{Kind: ReturnOriginField, Field: 1}}}}
		},
		"wrong parameter": func(fn *Function) {
			fn.ReturnOrigins = []ReturnOrigin{{Kind: ReturnOriginParameter, Parameter: 1, Path: fn.ReturnOrigins[0].Path}}
		},
	} {
		t.Run(name, func(t *testing.T) {
			module, err := analyzedModule(t, returnedReferenceFixture, 15)
			if err != nil {
				t.Fatalf("build: %v", err)
			}
			mutate(functionNamed(t, module, "Left"))
			if err := Verify(module); err == nil {
				t.Fatalf("verifier accepted %s", name)
			}
		})
	}
	t.Run("local origin", func(t *testing.T) {
		module, err := analyzedModule(t, returnedReferenceFixture, 15)
		if err != nil {
			t.Fatalf("build: %v", err)
		}
		// Return Use's local borrow of pair from Left instead.
		left := functionNamed(t, module, "Left")
		for _, block := range left.Blocks {
			for index, op := range block.Operations {
				if op.Kind == OpReferenceDerefPlace {
					block.Operations[index].Kind = OpPlaceValueRoot
					block.Operations[index].Reference = nil
					block.Operations[index].Place = &PlaceMetadata{Root: 1, RootKind: PlaceRootLocal, Storage: 1, AddressSpace: AddressSpaceOrdinary}
				}
			}
		}
		if err := Verify(module); err == nil {
			t.Fatal("verifier accepted a returned reference that does not derive from a parameter")
		}
	})
}
