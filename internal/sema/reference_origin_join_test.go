package sema

import (
	"strings"
	"testing"
)

const arenaHolderPrelude = `    let mut mem: Arena := Arena {}
    let mut other: byte[4] := [1, 2, 3, 4]
    let mut holder: ref mut byte[] := ref mut other[..]
`

// A reference holder rebound to Arena storage on one path keeps that Arena
// dependency after the join, so a Reset before a later use is still a stale
// use whatever branch, switch case or loop iteration rebound it (arena.md
// §§36, 44; correction25/28 corrected branch/switch/loop flow state).
func TestArenaHolderOriginSurvivesControlFlowJoins(t *testing.T) {
	for name, body := range map[string]string{
		"if": `    if flag {
        holder = try mem.Alloc[byte](4u)
    }
    mem.Reset()
    discard holder`,
		"if-else": `    if flag {
        discard flag
    } else {
        holder = try mem.Alloc[byte](4u)
    }
    mem.Reset()
    discard holder`,
		"switch": `    switch count {
        case 1:
            holder = try mem.Alloc[byte](4u)
        default:
            discard count
    }
    mem.Reset()
    discard holder`,
		"while": `    let mut index := 0
    while index < count {
        holder = try mem.Alloc[byte](4u)
        index += 1
    }
    mem.Reset()
    discard holder`,
		"for": `    for value in values {
        holder = try mem.Alloc[byte](4u)
        discard value
    }
    mem.Reset()
    discard holder`,
		"break": `    let mut index := 0
    while index < count {
        if flag {
            holder = try mem.Alloc[byte](4u)
            break
        }
        index += 1
    }
    mem.Reset()
    discard holder`,
		"backedge": `    let mut index := 0
    while index < count {
        discard holder
        holder = try mem.Alloc[byte](4u)
        mem.Reset()
        index += 1
    }`,
	} {
		source := "module main\n\nfn Use(flag: bool, count: int, values: int[]) Result[void, AllocationError] {\n" + arenaHolderPrelude + body + "\n    return Ok()\n}\n"
		errors := analyzeSourceRaw(t, source)
		if len(errors) != 1 || !strings.Contains(errors[0].Message, "cannot use holder after arena mem was reset") {
			t.Errorf("%s: errors = %v", name, errors)
		}
	}
}

// Joins keep the pre-branch origin when no continuing path rebinds the
// holder, and a Reset followed by a fresh allocation stays valid.
func TestArenaHolderOriginJoinKeepsValidPrograms(t *testing.T) {
	for name, body := range map[string]string{
		"terminating branch": `    if flag {
        holder = try mem.Alloc[byte](4u)
        return Ok()
    }
    mem.Reset()
    discard holder`,
		"reset then allocate": `    let mut index := 0
    while index < count {
        mem.Reset()
        holder = try mem.Alloc[byte](4u)
        discard holder
        index += 1
    }`,
		"use before reset": `    if flag {
        holder = try mem.Alloc[byte](4u)
    }
    discard holder
    mem.Reset()`,
	} {
		source := "module main\n\nfn Use(flag: bool, count: int) Result[void, AllocationError] {\n" + arenaHolderPrelude + body + "\n    return Ok()\n}\n"
		if errors := analyzeSourceRaw(t, source); len(errors) != 0 {
			t.Errorf("%s: errors = %v", name, errors)
		}
	}
}
