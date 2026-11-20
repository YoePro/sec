package project

import (
	"strings"
	"testing"
)

// MD-139: profile and allocation selections apply Variant, Target and
// Target-specific Variant scopes in order, each key independently.
// Rules: rules/projects/projects.md — §§ 19(11)-(15), 25(2).
func TestResolvePlanSelectionPrecedence(t *testing.T) {
	text := `
[variant.board]
os = "baremetal"
profile = "baremetal"

[variant.board.allocation]
mode = "embedded-arena"
provider = "board-primary-arena"

[target.firmware]
kind = "firmware"

[target.firmware.allocation]
mode = "noalloc"

[target.firmware.variant.board.allocation]
mode = "embedded-arena"
`
	selection, errs := ResolvePlanSelection(text, "firmware", "board")
	if len(errs) != 0 {
		t.Fatal(errs)
	}
	if selection.Profile != "baremetal" || selection.ProfileScope != "variant.board" ||
		selection.AllocationMode != AllocationModeEmbeddedArena || selection.ModeScope != "target.firmware.variant.board.allocation" ||
		selection.Provider != "board-primary-arena" || selection.ProviderScope != "variant.board.allocation" {
		t.Fatalf("selection = %+v", selection)
	}
	if empty, errs := ResolvePlanSelection("[project]\nname = \"x\"\n", "app", "linux"); len(errs) != 0 || empty.Profile != "" || empty.AllocationMode != "" {
		t.Fatalf("absent selection = %+v %v", empty, errs)
	}
}

// MD-139 § 2(5), § 3(6), § 3(8): configuration errors are reported, not
// repaired.
func TestResolvePlanSelectionErrors(t *testing.T) {
	for name, check := range map[string]struct{ text, want string }{
		"build profile name":   {"[variant.v]\nprofile = \"release\"\n", "not a target profile"},
		"unknown key":          {"[variant.v.allocation]\nmode = \"noalloc\"\nheap = 4096\n", "unknown key \"heap\""},
		"invalid mode":         {"[variant.v.allocation]\nmode = \"malloc\"\n", "not hosted, embedded-arena or noalloc"},
		"empty provider":       {"[variant.v.allocation]\nprovider = \"\"\n", "non-empty provider"},
		"noalloc provider":     {"[variant.v.allocation]\nmode = \"noalloc\"\nprovider = \"heap\"\n", "together with mode \"noalloc\""},
		"merged contradiction": {"[variant.v.allocation]\nprovider = \"heap\"\n[target.t.allocation]\nmode = \"noalloc\"\n", "contradicts mode \"noalloc\""},
	} {
		_, errs := ResolvePlanSelection(check.text, "t", "v")
		joined := ""
		for _, err := range errs {
			joined += err.Error() + ";"
		}
		if !strings.Contains(joined, check.want) {
			t.Errorf("%s: errors %q, want %q", name, joined, check.want)
		}
	}
}
