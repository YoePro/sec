package sema

import (
	"strings"
	"testing"
)

// Checked access keeps its access-specific error family (collections.md
// § 16): under try, an index into a fixed array, list, owning T[], slice or
// a map key fails with IndexError (a missing map key included, § 14.4), and
// a slice range fails with RangeError. Neither collapses into
// CollectionError.
func TestCheckedAccessErrorFamilies(t *testing.T) {
	valid := analyzeSourceRaw(t, `module main

fn Fixed(a: int[4], i: uint) Result[int, IndexError] {
    let v := try a[i]
    return Ok(v)
}

fn List(l: list[int], i: uint) Result[int, IndexError] {
    let v := try l[i]
    return Ok(v)
}

fn Owning(d: int[], i: uint) Result[int, IndexError] {
    let v := try d[i]
    return Ok(v)
}

fn Slice(s: ref int[], i: uint) Result[int, IndexError] {
    let v := try s[i]
    return Ok(v)
}

fn Map(m: map[string, int]) Result[int, IndexError] {
    let v := try m["a"]
    return Ok(v)
}

fn Range(d: ref int[], a: uint, b: uint) Result[uint, RangeError] {
    let s := try ref d[a..<b]
    return Ok(s.Len)
}

fn RangeHandled(d: ref int[], a: uint, b: uint) uint {
    let s := try ref d[a..<b] {
        Err(_) => {
            return 0
        }
    }
    return s.Len
}
`)
	if len(valid) != 0 {
		t.Fatalf("unexpected errors: %v", valid)
	}
	cases := map[string]string{
		"index into CollectionError": `fn F(d: int[], i: uint) Result[int, CollectionError] {
    let v := try d[i]
    return Ok(v)
}`,
		"map key into CollectionError": `fn F(m: map[string, int]) Result[int, CollectionError] {
    let v := try m["a"]
    return Ok(v)
}`,
		"range into IndexError": `fn F(d: ref int[], a: uint, b: uint) Result[uint, IndexError] {
    let s := try ref d[a..<b]
    return Ok(s.Len)
}`,
	}
	for name, body := range cases {
		t.Run(name, func(t *testing.T) {
			errors := analyzeSourceRaw(t, "module main\n\n"+body+"\n")
			for _, err := range errors {
				if strings.Contains(err.Message, "bodyless try propagates") {
					return
				}
			}
			t.Fatalf("expected an error-channel mismatch; got %v", errors)
		})
	}
}
