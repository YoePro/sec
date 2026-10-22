package semantic

import (
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

const remainingPayloadFixture = `module main

enum Problem error {
    Bad,
}

type Point struct {
    x: int,
}

type Figure union {
    Dot(Point)
    Empty
}

type Outline union {
    Rectangle { width: int, height: int }
    None
}

type Holder struct {
    figure: Figure,
}

fn ResultOk(value: Result[Point, Problem]) int {
    match value {
        Ok(ref point) => {
            return point.x
        }
        Err(ref problem) => {
            return 0
        }
    }
}

fn StructLike(shape: Outline) int {
    match shape {
        Rectangle(ref rect) => {
            return rect.width
        }
        None => {
            return 0
        }
    }
}

fn Element(figures: Figure[3], at: int) int {
    match figures[at] {
        Dot(ref point) => {
            return point.x
        }
        Empty => {
            return 0
        }
    }
}

fn Through(holder: ref Holder) int {
    match holder.figure {
        Dot(ref point) => {
            return point.x
        }
        Empty => {
            return 0
        }
    }
}
`

// The remaining P15-45 shapes lower without copying the subject or payload:
// Result Ok/Err payloads (guarded by result.is-err, Ok on its false edge),
// struct-like payloads as their Package 13 synthetic payload struct, and
// subjects reached through an array element or a reference holder, whose
// Place is formed once and both read for the match and projected for the
// borrow, so the index is evaluated a single time.
//
// Rules:
//   - rules/mlir/packages/sec-mlir-dialect_package15.md — §§ 19, 44, 71, 99, 100, 121
func TestPackage15LowersRemainingPayloadBorrowShapes(t *testing.T) {
	module, err := analyzedModule(t, remainingPayloadFixture, 15)
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	if err := Verify(module); err != nil {
		t.Fatalf("verify: %v\n%s", err, Format(module))
	}
	for name, want := range map[string]int{"ResultOk": 2, "StructLike": 1, "Element": 1, "Through": 1} {
		fn := functionNamed(t, module, name)
		payloads, bounds := 0, 0
		for _, block := range fn.Blocks {
			for _, op := range block.Operations {
				switch op.Kind {
				case OpPlaceUnionPayload:
					payloads++
				case OpUnionUnwrapPayload, OpUnionUnwrapField, OpResultUnwrapOk, OpResultUnwrapErr, OpArrayExtract:
					t.Errorf("%s copied the subject or payload with %s", name, op.Kind)
				case OpPlaceArrayIndexInBounds:
					bounds++
				}
			}
		}
		if payloads != want {
			t.Errorf("%s: %d payload Places, want %d\n%s", name, payloads, want, Format(module))
		}
		if name == "Element" && bounds != 1 {
			t.Errorf("Element: the index must be checked exactly once, got %d checks", bounds)
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

const fieldSubjectPayloadFixture = `module main

type Point struct {
    x: int,
}

type Figure union {
    Dot(Point)
    Empty
}

type Holder struct {
    figure: Figure,
}

type Outer struct {
    holder: Holder,
}

fn Read(outer: Outer) int {
    match outer.holder.figure {
        Dot(ref point) => {
            return point.x
        }
        Empty => {
            return 0
        }
    }
}

fn Write() int {
    let mut holder := Holder { figure: Figure.Dot(Point { x: 1 }) }
    match holder.figure {
        Dot(ref mut point) => {
            point.x = 5
        }
        Empty => {
        }
    }
    return 0
}
`

// A match subject reached through stored fields (`match holder.figure`)
// borrows its payload from the root Place projected through the same fields
// the subject value was extracted from: place.field steps, then
// place.union-payload guarded by the arm's variant test. Nothing is copied
// to form the reference, and ref mut writes through place.write (P15-45).
//
// Rules:
//   - rules/mlir/packages/sec-mlir-dialect_package15.md — §§ 39-44, 99, 100
func TestPackage15LowersFieldSubjectPayloadBorrows(t *testing.T) {
	module, err := analyzedModule(t, fieldSubjectPayloadFixture, 15)
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	if err := Verify(module); err != nil {
		t.Fatalf("verify: %v\n%s", err, Format(module))
	}
	read := functionNamed(t, module, "Read")
	var kinds []OpKind
	for _, block := range read.Blocks {
		for _, op := range block.Operations {
			switch op.Kind {
			case OpPlaceValueRoot, OpPlaceField, OpPlaceUnionPayload, OpUnionUnwrapPayload:
				kinds = append(kinds, op.Kind)
			}
		}
	}
	want := []OpKind{OpPlaceValueRoot, OpPlaceField, OpPlaceField, OpPlaceUnionPayload, OpPlaceField}
	if len(kinds) != len(want) {
		t.Fatalf("Read Place ops = %v, want %v\n%s", kinds, want, Format(module))
	}
	for index := range want {
		if kinds[index] != want[index] {
			t.Fatalf("Read Place ops = %v, want %v", kinds, want)
		}
	}
	if text := Format(module); !strings.Contains(text, "place.storage-root") || !strings.Contains(text, "place.write") {
		t.Errorf("Write must borrow from address-taken storage and write through the payload:\n%s", text)
	}
}

// The verifier rejects a payload Place whose field path differs from the
// field path of the union value its guard tested, even when both fields have
// the same union type.
func TestPackage15VerifierRejectsPayloadPlaceOfOtherField(t *testing.T) {
	module, err := analyzedModule(t, `module main

type Figure union {
    Dot(int)
    Empty
}

type Pair struct {
    left: Figure,
    right: Figure,
}

fn Read(pair: Pair) int {
    match pair.left {
        Dot(ref value) => {
            return 1
        }
        Empty => {
            return 0
        }
    }
}
`, 15)
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	if err := Verify(module); err != nil {
		t.Fatalf("verify: %v", err)
	}
	read := functionNamed(t, module, "Read")
	mutated := false
	for blockIndex := range read.Blocks {
		for opIndex := range read.Blocks[blockIndex].Operations {
			op := &read.Blocks[blockIndex].Operations[opIndex]
			if op.Kind == OpPlaceField && !mutated {
				op.StructField = 1
				mutated = true
			}
		}
	}
	if !mutated {
		t.Fatal("no field projection to mutate")
	}
	if err := Verify(module); err == nil || !strings.Contains(err.Error(), "not a payload of the union its guard tested") {
		t.Fatalf("verifier accepted a payload Place of another field: %v", err)
	}
}

// Inside a method, `match self.figure` and the implicit `match figure` reach
// the union through the receiver reference: the payload Place is projected
// from ref.deref-place of the receiver through the same stored field the
// subject was read from, and the verifier relates the payload Place to that
// read (P15-45).
func TestPackage15LowersReceiverFieldPayloadBorrows(t *testing.T) {
	module, err := analyzedModule(t, `module main

type Point struct {
    x: int,
}

type Figure union {
    Dot(Point)
    Empty
}

type Holder struct {
    figure: Figure,
}

impl Holder {
    fn Explicit() int {
        match self.figure {
            Dot(ref point) => {
                return point.x
            }
            Empty => {
                return 0
            }
        }
    }

    fn Implicit() int {
        match figure {
            Dot(ref point) => {
                return point.x
            }
            Empty => {
                return 0
            }
        }
    }
}
`, 15)
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	if err := Verify(module); err != nil {
		t.Fatalf("verify: %v\n%s", err, Format(module))
	}
	for _, name := range []string{"Explicit", "Implicit"} {
		var fn *Function
		for _, candidate := range module.Functions {
			if strings.HasSuffix(candidate.Name, name) {
				fn = candidate
			}
		}
		if fn == nil {
			t.Fatalf("missing method %s", name)
		}
		payloads, derefs := 0, 0
		for _, block := range fn.Blocks {
			for _, op := range block.Operations {
				switch op.Kind {
				case OpPlaceUnionPayload:
					payloads++
				case OpReferenceDerefPlace:
					derefs++
				case OpUnionUnwrapPayload:
					t.Errorf("%s: payload borrow copied the payload", name)
				}
			}
		}
		if payloads != 1 || derefs < 2 {
			t.Errorf("%s: payload Places %d, receiver derefs %d\n%s", name, payloads, derefs, Format(module))
		}
	}
}

// A Result payload Place is proven only on the matching edge of result.is-err:
// Err on the true edge, Ok on the false edge. Claiming the Err payload on the
// Ok path is rejected.
func TestPackage15VerifierChecksResultPayloadEdges(t *testing.T) {
	module, err := analyzedModule(t, remainingPayloadFixture, 15)
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	fn := functionNamed(t, module, "ResultOk")
	mutated := false
	for blockIndex := range fn.Blocks {
		for opIndex := range fn.Blocks[blockIndex].Operations {
			op := &fn.Blocks[blockIndex].Operations[opIndex]
			if op.Kind == OpPlaceUnionPayload && op.UnionVariant == 0 && !mutated {
				op.UnionVariant = 1
				mutated = true
			}
		}
	}
	if !mutated {
		t.Fatal("no Ok payload Place to mutate")
	}
	if err := Verify(module); err == nil || !strings.Contains(err.Error(), "union-payload") {
		t.Fatalf("verifier accepted an Err payload on the Ok edge: %v", err)
	}
}

const payloadBorrowEndFixture = `module main

enum Problem error {
    Bad,
}

type Point struct {
    x: int,
}

type Figure union {
    Dot(Point)
    Empty
}

fn Guarded(figure: Figure) int {
    match figure {
        Dot(ref point) where point.x > 0 => {
            return point.x
        }
        _ => {
            return 0
        }
    }
}

fn Falls(figure: Figure) int {
    let mut total := 0
    match figure {
        Dot(ref point) => {
            total = point.x
        }
        Empty => {
        }
    }
    return total
}

fn Writes() int {
    let mut figure := Figure.Dot(Point { x: 1 })
    match figure {
        Dot(ref mut point) => {
            point.x = 2
        }
        Empty => {
        }
    }
    return 0
}

fn Failure(value: Result[Point, Problem]) int {
    match value {
        Ok(_) => {
            return 1
        }
        Err(ref problem) => {
            return 0
        }
    }
}

fn ResultWrites(input: Result[Point, Problem]) int {
    let mut value := input
    match value {
        Ok(ref mut point) => {
            point.x = 3
        }
        Err(_) => {
        }
    }
    return 0
}

fn Breaks(figure: Figure, count: int) int {
    let mut index := 0
    while index < count {
        match figure {
            Dot(ref point) => {
                if point.x > index {
                    break
                }
            }
            Empty => {
            }
        }
        index = index + 1
    }
    return index
}
`

// Every exit of an arm that borrows its payload carries ref.end-borrow for
// that borrow, with the borrow's own BorrowID and LifetimeID: returns, the
// fall-through to the match merge, a break out of the enclosing loop, and the
// false edge of a guard that reads the borrowed payload (on its own edge
// block, since the next pattern test is shared). The verifier accepts the
// loop re-entering the borrow (P15-63). Since MD-065 a borrow whose last
// required use comes earlier (here: never used in Failure) ends there instead
// of at the arm exit, with no second marker on that path.
//
// Rules:
//   - rules/mlir/packages/sec-mlir-dialect_package15.md — §§ 49, 50, 82
func TestPackage15PayloadBorrowsEndOnEveryArmExit(t *testing.T) {
	module, err := analyzedModule(t, payloadBorrowEndFixture, 15)
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	if err := Verify(module); err != nil {
		t.Fatalf("verify: %v\n%s", err, Format(module))
	}
	text := Format(module)
	for name, wantEnds := range map[string]int{"Guarded": 2, "Falls": 1, "Writes": 1, "Failure": 1, "ResultWrites": 1, "Breaks": 2} {
		fn := functionNamed(t, module, name)
		borrows := map[ValueID]*ReferenceMetadata{}
		for _, block := range fn.Blocks {
			for _, op := range block.Operations {
				if op.Kind == OpReferenceBorrowShared || op.Kind == OpReferenceBorrowMutable {
					borrows[op.Results[0].ID] = op.Reference
				}
			}
		}
		ends := 0
		for _, block := range fn.Blocks {
			for index, op := range block.Operations {
				switch op.Kind {
				case OpReferenceEndBorrow:
					ends++
					borrow := borrows[op.Operands[0]]
					if borrow == nil || op.Reference == nil || op.Reference.Borrow != borrow.Borrow || op.Reference.Lifetime != borrow.Lifetime {
						t.Errorf("%s: end-borrow does not name its borrow: %+v", name, op)
					}
					// MD-065: a marker sits at an arm exit, or earlier at Sema's
					// last required use; then nothing after it uses the borrow.
					if next := block.Operations[index+1]; !next.IsTerminator() {
						for _, later := range block.Operations[index+1:] {
							for _, operand := range later.Operands {
								if operand == op.Operands[0] {
									t.Errorf("%s: end-borrow precedes a use of its borrow (%s)", name, later.Kind)
								}
							}
						}
					}
				}
			}
		}
		if len(borrows) != 1 || ends != wantEnds {
			t.Errorf("%s: %d borrows and %d end markers, want 1 and %d\n%s", name, len(borrows), ends, wantEnds, text)
		}
	}
	guarded := functionNamed(t, module, "Guarded")
	blocks := map[BlockID]*Block{}
	for _, block := range guarded.Blocks {
		blocks[block.ID] = block
	}
	for _, block := range guarded.Blocks {
		terminator := block.Operations[len(block.Operations)-1]
		if terminator.MatchStage != "guard" {
			continue
		}
		edge := blocks[terminator.Successors[1].Block]
		if edge == nil || len(edge.Operations) != 2 || edge.Operations[0].Kind != OpReferenceEndBorrow || edge.Operations[1].Kind != OpBranch {
			t.Errorf("the guard's false edge does not end the payload borrow:\n%s", text)
		}
	}
}

// The verifier requires ref.end-borrow to name the BorrowID and LifetimeID of
// the reference it ends and rejects a use of that reference after the marker.
func TestPackage15VerifierChecksPayloadBorrowEnds(t *testing.T) {
	for name, mutate := range map[string]func(block *Block, index int){
		"foreign borrow id": func(block *Block, index int) { block.Operations[index].Reference.Borrow++ },
		"missing facts":     func(block *Block, index int) { block.Operations[index].Reference = nil },
		"use after end": func(block *Block, index int) {
			for at := index - 1; at >= 0; at-- {
				if uses := block.Operations[at]; uses.Kind == OpReferenceDerefPlace {
					end := block.Operations[index]
					copy(block.Operations[at+1:index+1], block.Operations[at:index])
					block.Operations[at] = end
					return
				}
			}
		},
	} {
		t.Run(name, func(t *testing.T) {
			module, err := analyzedModule(t, payloadBorrowEndFixture, 15)
			if err != nil {
				t.Fatalf("build: %v", err)
			}
			fn := functionNamed(t, module, "Falls")
			mutated := false
			for _, block := range fn.Blocks {
				for index, op := range block.Operations {
					if op.Kind == OpReferenceEndBorrow && !mutated {
						mutate(block, index)
						mutated = true
					}
				}
			}
			if !mutated {
				t.Fatal("fixture has no end-borrow")
			}
			if err := Verify(module); err == nil {
				t.Fatalf("verifier accepted %s:\n%s", name, Format(module))
			}
		})
	}
}
