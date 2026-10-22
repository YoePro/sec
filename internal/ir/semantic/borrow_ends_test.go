package semantic

import (
	"strings"
	"testing"
)

// MD-065 (borrowing.md § 8.1(9)–(14)): Semantic IR places ref.end-borrow at
// Sema's last required use. A let holder's borrow ends right after its last
// use in its own statement list, before the statement that would otherwise
// conflict; a payload borrow that is last used mid-arm ends there and its
// arm exit carries no second marker on that path; an exit reached before the
// end keeps its own marker. Every marker names its borrow and nothing uses
// the borrow after it.
func TestBorrowEndsFollowLastRequiredUse(t *testing.T) {
	source := `
module main

type Point struct {
    x: int,
}

fn Holder() int {
    let mut x := 1
    let r := ref x
    let a := *r
    x = 2
    return x + a
}

fn Payload(value: Option[Point], early: bool) int {
    match value {
        Some(ref point) => {
            if early {
                return 1
            }
            let copy := point.x
            let doubled := copy * 2
            return doubled
        }
        None => {
            return 0
        }
    }
}
`
	module, err := buildPackage15Source(t, source, "")
	if err != nil {
		t.Fatal(err)
	}
	if err := Verify(module); err != nil {
		t.Fatalf("verify: %v\n%s", err, Format(module))
	}
	text := Format(module)
	holder := text[strings.Index(text, `func "main::Holder()"`):]
	holder = holder[:strings.Index(holder, "\n  }")]
	end := strings.Index(holder, "ref.end-borrow")
	read := strings.Index(holder, "place.read")
	store := strings.Index(holder, "storage.store")
	if end < 0 || !(read < end && end < store) {
		t.Fatalf("Holder must end r after its last use and before x = 2:\n%s", holder)
	}

	payload := functionNamed(t, module, "Payload")
	borrowed := ValueID(0)
	for _, block := range payload.Blocks {
		for _, op := range block.Operations {
			if op.Kind == OpReferenceBorrowShared {
				borrowed = op.Results[0].ID
			}
		}
	}
	ends := 0
	for _, block := range payload.Blocks {
		for index, op := range block.Operations {
			if op.Kind != OpReferenceEndBorrow || op.Operands[0] != borrowed {
				continue
			}
			ends++
			for _, later := range block.Operations[index+1:] {
				for _, operand := range later.Operands {
					if operand == borrowed {
						t.Fatalf("Payload uses its borrow after ending it:\n%s", Format(module))
					}
				}
			}
		}
	}
	// One mid-arm end after `let copy := point.x` and one exit marker on the
	// early return taken before it; the final return gets no second marker.
	if ends != 2 {
		t.Fatalf("Payload has %d end markers for its payload borrow, want 2:\n%s", ends, Format(module))
	}
}
