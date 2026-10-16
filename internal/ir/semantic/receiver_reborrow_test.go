package semantic

import (
	"strings"
	"testing"
)

const receiverReborrowFixture = `module main

type Inner struct {
    v: int,
}

type Counter struct {
    value: int,
    inner: Inner,
}

fn Read(c: ref Counter) int { return c.value }
fn Write(c: ref mut Counter) void { c.value = 2 }
fn ReadInner(i: ref Inner) int { return i.v }

impl Counter {
    fn Bump() void {
        value = value + 1
    }
    fn Peek() int {
        return value
    }
    fn BumpTwice() int {
        Bump()
        self.Bump()
        return Peek()
    }
    fn Get() int {
        return Read(self) + ReadInner(inner) + ReadInner(self.inner)
    }
    fn Set() void {
        Write(self)
        let whole := ref mut self
        whole.value = 4
    }
}
`

// A method's receiver is a reference parameter, so passing it on or
// borrowing it is a reborrow, never a copy of a ref mut (P15-47,
// package15.md §§ 47-48, 59-60): nested calls on a mutable receiver
// (`Bump()`, `self.Bump()`) reborrow it with the fresh identity Sema resolves
// at the call — mutable or shared as the callee needs; `self` or a receiver
// field passed to a ref/ref mut parameter, and explicit `ref mut self`, derive
// the borrow from the dereferenced receiver. Every derived borrow names the
// receiver's borrow as its source.
//
// Rules:
//   - rules/mlir/packages/sec-mlir-dialect_package15.md — §§ 46-48, 59-60, 78
//   - rules/declarations/impl.md — § 8 "self"
func TestPackage15ReborrowsReceiversForNestedCallsAndArguments(t *testing.T) {
	module, err := analyzedModule(t, receiverReborrowFixture, 15)
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	if err := Verify(module); err != nil {
		t.Fatalf("verify: %v\n%s", err, Format(module))
	}
	for name, want := range map[string]map[OpKind]int{
		"Counter.BumpTwice": {OpReferenceReborrowMutable: 2, OpReferenceReborrowShared: 1},
		"Counter.Get":       {OpReferenceReborrowShared: 3},
		"Counter.Set":       {OpReferenceReborrowMutable: 2},
	} {
		fn := functionNamed(t, module, name)
		receiver := fn.Parameters[0].Reference
		got := map[OpKind]int{}
		borrows := map[uint32]bool{}
		for _, block := range fn.Blocks {
			for _, op := range block.Operations {
				switch op.Kind {
				case OpReferenceReborrowMutable, OpReferenceReborrowShared:
					got[op.Kind]++
					if op.Reference.SourceBorrow != receiver.Borrow || op.Reference.Borrow == receiver.Borrow || borrows[op.Reference.Borrow] {
						t.Errorf("%s: reborrow %+v does not derive a fresh borrow from the receiver %+v", name, op.Reference, receiver)
					}
					borrows[op.Reference.Borrow] = true
				case OpReferenceCopyShared, OpReferenceMove:
					t.Errorf("%s: the mutable receiver was passed with %s", name, op.Kind)
				}
			}
		}
		for kind, count := range want {
			if got[kind] != count {
				t.Errorf("%s: %d %s, want %d\n%s", name, got[kind], kind, count, Format(module))
			}
		}
	}
	if strings.Contains(Format(module), "struct.extract-field") {
		t.Error("a receiver was copied to form a reference")
	}
}
