package sema

import "testing"

// A mutating self call inside a switch case makes the enclosing impl method
// require a writable receiver, exactly as inside an if branch.
//
// Rules:
//   - rules/declarations/impl.md — §8 (implicit self receiver capability)
func TestReceiverMutabilityInferredThroughSwitchCases(t *testing.T) {
	errors := analyzeSourceRaw(t, `
module main

type Cursor struct {
    pos: uint,
}

impl Cursor {
    fn step() uint {
        self.pos += 1
        return self.pos
    }

    fn viaSwitch(ch: rune) uint {
        switch ch {
            case 'a':
                return self.step()
            default:
                let value := self.step()
                return value
        }
    }
}
`)
	assertSemaErrors(t, errors, nil)
}
