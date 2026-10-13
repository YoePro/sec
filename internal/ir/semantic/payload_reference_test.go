package semantic

import (
	"errors"
	"strings"
	"testing"
)

const payloadReferenceFixture = `module main

type Point struct {
    x: int,
    y: int,
}

type Figure union {
    Dot(Point)
    Empty
}

fn Read(figure: Figure) int {
    match figure {
        Dot(ref point) => {
            return point.x
        }
        Empty => {
            return 0
        }
    }
}

fn Move() int {
    let mut figure := Figure.Dot(Point { x: 1, y: 2 })
    match figure {
        Dot(ref mut point) => {
            point.y = 7
        }
        Empty => {
        }
    }
    return 0
}

fn First(value: Option[Point]) int {
    match value {
        Some(ref point) => {
            return point.y
        }
        None => {
            return 0
        }
    }
}

fn Loop(figure: Figure, count: int) int {
    let mut total := 0
    let mut index := 0
    while index < count {
        match figure {
            Dot(ref point) => {
                total = total + point.x
            }
            Empty => {
            }
        }
        index = index + 1
    }
    return total
}
`

// A ref or ref mut union/Option payload binding lowers from Sema's resolved
// reference plan for the binding: the match subject's own Place root, one
// place.union-payload naming the arm's union.is-variant predicate as its
// active-variant proof, and a borrow carrying the plan's facts. The union and
// the payload are never unwrapped or copied to form the reference, a mutable
// payload borrow needs read/write storage and writes through place.write, and
// the proof stays valid when the match sits in a loop (P15-45).
//
// Rules:
//   - rules/mlir/packages/sec-mlir-dialect_package15.md — §§ 19, 44, 71, 99, 100, 121
//   - rules/memory/borrowing.md — § 12(5)
func TestPackage15LowersUnionPayloadReferenceBindings(t *testing.T) {
	module, err := analyzedModule(t, payloadReferenceFixture, 15)
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	if err := Verify(module); err != nil {
		t.Fatalf("verify: %v\n%s", err, Format(module))
	}
	text := Format(module)
	for _, want := range []string{"place.union-payload", "variant-proven", "ref.borrow-shared", "ref.borrow-mut", "place.write", "address-taken"} {
		if !strings.Contains(text, want) {
			t.Errorf("module lacks %q:\n%s", want, text)
		}
	}
	for _, name := range []string{"Read", "Move", "First", "Loop"} {
		fn := functionNamed(t, module, name)
		payloads := 0
		for _, block := range fn.Blocks {
			for _, op := range block.Operations {
				switch op.Kind {
				case OpUnionUnwrapPayload, OpUnionUnwrapField:
					t.Errorf("%s: payload borrow unwrapped (copied) the payload:\n%s", name, text)
				case OpPlaceUnionPayload:
					payloads++
					if op.VariantGuard == 0 || op.Place == nil || !op.Place.VariantProven {
						t.Errorf("%s: payload Place without its variant guard: %+v", name, op)
					}
				}
			}
		}
		if payloads != 1 {
			t.Errorf("%s: want one payload Place, got %d", name, payloads)
		}
	}
}

// Borrowed payloads outside the supported shape stay package-15 unsupported
// with a precise reason instead of being copied: Result payloads (Result is
// not a Semantic IR union) and subjects that are not a whole local binding.
func TestPackage15PayloadReferenceBoundaries(t *testing.T) {
	for name, test := range map[string]struct{ source, want string }{
		"result payload": {`module main

enum Problem error {
    Bad,
}

fn Use(value: Result[int, Problem]) int {
    match value {
        Ok(ref number) => {
            return 1
        }
        Err(problem) => {
            return 0
        }
    }
}
`, "borrowed result-ok payload"},
		"field subject": {`module main

type Figure union {
    Dot(int)
    Empty
}

type Holder struct {
    figure: Figure,
}

fn Use(holder: Holder) int {
    match holder.figure {
        Dot(ref value) => {
            return 1
        }
        Empty => {
            return 0
        }
    }
}
`, "not a whole local binding"},
	} {
		module, err := analyzedModule(t, test.source, 15)
		var unsupported *UnsupportedFeatureError
		if module != nil || !errors.As(err, &unsupported) || unsupported.Package != 15 || !strings.Contains(unsupported.Feature, test.want) {
			t.Errorf("%s: module=%v err=%v, want package-15 unsupported %q", name, module != nil, err, test.want)
		}
	}
}

// The verifier checks the active-variant proof of place.union-payload rather
// than trusting the metadata flag: the guard must be a union.is-variant
// predicate for the same variant over the Place root's own union value.
func TestPackage15VerifierRejectsUnprovenPayloadPlaces(t *testing.T) {
	for name, mutate := range map[string]func(op *Operation, fn *Function){
		"missing guard": func(op *Operation, fn *Function) { op.VariantGuard = 0 },
		"other variant": func(op *Operation, fn *Function) { op.UnionVariant++ },
		"foreign guard": func(op *Operation, fn *Function) {
			for _, block := range fn.Blocks {
				for _, other := range block.Operations {
					if other.Kind == OpUnionIsVariant && other.Results[0].ID != op.VariantGuard {
						op.VariantGuard = other.Results[0].ID
						return
					}
				}
			}
		},
	} {
		module, err := analyzedModule(t, payloadReferenceFixture, 15)
		if err != nil {
			t.Fatalf("build: %v", err)
		}
		fn := functionNamed(t, module, "Read")
		mutated := false
		for blockIndex := range fn.Blocks {
			for opIndex := range fn.Blocks[blockIndex].Operations {
				op := &fn.Blocks[blockIndex].Operations[opIndex]
				if op.Kind == OpPlaceUnionPayload {
					mutate(op, fn)
					mutated = true
				}
			}
		}
		if !mutated {
			t.Fatalf("%s: no payload Place to mutate", name)
		}
		if err := Verify(module); err == nil || !strings.Contains(err.Error(), "union-payload") {
			t.Errorf("%s: verifier accepted an unproven payload Place: %v", name, err)
		}
	}
}
