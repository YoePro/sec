package main

import (
	"os"
	"strings"
	"testing"
)

// Open implementation targets stay visible through safe wrappers and deferred
// execution. Unknown provenance is separate from known unsafe dependencies.
// Rules: rules/memory/unsafe.md §§13–14,52(1);
// rules/analysis/call_graph.md §§44–45.
func TestTrustProvenanceUnknownHover(t *testing.T) {
	data, err := os.ReadFile("../../testdata/analysis/trust_graph.sec")
	if err != nil {
		t.Fatal(err)
	}
	source := string(data)
	for _, tc := range []struct {
		name string
		want []string
	}{
		{"Open", []string{"Implementation trust unknown via `Open`", "open callable contract retained"}},
		{"OuterOpen", []string{"Implementation trust unknown via `OuterOpen` -> `Open`", "Caller-facing safety: safe"}},
		{"RecursiveOpen", []string{"Implementation trust unknown via `RecursiveOpen` -> `Open`"}},
		{"DeferredOpen", []string{"Implementation trust unknown via `DeferredOpen`", "`Open`; open callable contract retained"}},
		{"Mixed", []string{"Depends on unsafe code via", "Implementation trust unknown via"}},
		{"foreign_value", []string{"trusted foreign declaration outside Sec source verification"}},
	} {
		offset := strings.Index(source, "fn "+tc.name+"(")
		hover, ok := hoverForSource("file:///tmp/sec-trust-unknown/main.sec", source, offsetPosition(source, offset+3))
		if !ok {
			t.Fatalf("missing hover for %s", tc.name)
		}
		for _, want := range tc.want {
			if !strings.Contains(hover.Contents.Value, want) {
				t.Errorf("%s missing %q: %s", tc.name, want, hover.Contents.Value)
			}
		}
		if strings.Contains(hover.Contents.Value, "no unsafe operation or foreign call") {
			t.Fatalf("unknown implementation described as trust-free: %s", hover.Contents.Value)
		}
		if tc.name == "foreign_value" && strings.Contains(hover.Contents.Value, "Caller-facing safety: safe") {
			t.Fatal("foreign implementation was mislabeled as a verified safe Sec body")
		}
	}
}

// TestTrustProvenanceHover verifies that a safe wrapper keeps the trust
// provenance of the unsafe code and foreign declarations it depends on, that
// an unsafe callable shows unsafe caller-facing safety, and that a callable
// with no trust dependency gets no provenance section.
//
// Rules:
//   - rules/memory/unsafe.md — § 3.4 "Trusted declaration"; § 14 "Trust provenance"; § 52(1) LSP and tooling
func TestTrustProvenanceHover(t *testing.T) {
	source := `module main

extern "C" fn foreign_add(left: int32, right: int32) int32

unsafe fn Leaf(pointer: RawPtr[byte]) byte {
    unsafe {
        return pointer.Read()
    }
}

fn Wrapper(pointer: RawPtr[byte]) byte {
    unsafe {
        return Leaf(pointer)
    }
}

fn Outer(pointer: RawPtr[byte]) byte {
    return Wrapper(pointer)
}

fn Add() int32 {
    unsafe {
        return foreign_add(1, 2)
    }
}

fn Plain(value: int) int {
    return value
}
`
	uri := "file:///tmp/sec-lsp-trust-provenance-hover/main.sec"
	hover := func(declaration string) string {
		t.Helper()
		offset := strings.Index(source, declaration)
		result, ok := hoverForSource(uri, source, offsetPosition(source, offset+len("fn ")))
		if !ok {
			t.Fatalf("no hover for %s", declaration)
		}
		return result.Contents.Value
	}
	checks := []struct {
		declaration string
		want        []string
		reject      []string
	}{
		{"fn Leaf", []string{"**Trust provenance**", "Caller-facing safety: unsafe", "`1` unsafe operations here (`raw-pointer-access`)"}, []string{"Depends on unsafe code via"}},
		{"fn Wrapper", []string{"Caller-facing safety: safe", "`1` unsafe operations here (`unsafe-call`)", "Depends on unsafe code via `Wrapper` -> `Leaf`"}, nil},
		{"fn Outer", []string{"Caller-facing safety: safe", "Depends on unsafe code via `Outer` -> `Wrapper`"}, []string{"unsafe operations here"}},
		{"fn Add", []string{"calls trusted foreign `foreign_add`"}, []string{"Depends on unsafe code via"}},
		{"fn Plain", nil, []string{"Trust provenance"}},
	}
	for _, check := range checks {
		text := hover(check.declaration)
		for _, want := range check.want {
			if !strings.Contains(text, want) {
				t.Errorf("%s hover lacks %q:\n%s", check.declaration, want, text)
			}
		}
		for _, reject := range check.reject {
			if strings.Contains(text, reject) {
				t.Errorf("%s hover has %q:\n%s", check.declaration, reject, text)
			}
		}
	}
}
