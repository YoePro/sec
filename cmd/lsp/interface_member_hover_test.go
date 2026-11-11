package main

import (
	"fmt"
	"path/filepath"
	"strings"
	"testing"
)

// Hover at an interface requirement shows its declaring interface, receiver
// capability and implementing types; hover at an implementing method shows
// the conformed interface, inherited-requirement provenance and both
// receiver capabilities.
// Rules: rules/declarations/interfaces.md — §§ 3, 5–7; rules/tooling/lsp.md — "Hover".
func TestInterfaceMemberHover(t *testing.T) {
	text := `module main

interface Counter {
    fn Value() int
    mut fn Increment() void
}

interface Resettable implements Counter {
    mut fn Reset() void
}

type Tally struct {
    count: int,
}

impl Tally implements Resettable {
    fn Value() int {
        return self.count
    }
    fn Increment() void {
        self.count = self.count + 1
    }
    fn Reset() void {
        self.count = 0
    }
}

fn Main() int {
    let mut tally := Tally { count: 0 }
    tally.Increment()
    return tally.Value()
}
`
	uri := uriFromPath(filepath.Join(t.TempDir(), "main.sec"))
	cases := []struct {
		marker string
		skip   int
		want   []string
	}{
		{"mut fn Increment", 7, []string{"**Interface requirement**", "requirement of `Counter`", "mutable/exclusive receiver access (`mut fn`)", "implemented by: `Tally` via `Resettable` (20:"}},
		{"fn Value() int {", 3, []string{"**Interface implementation**", "implements `Resettable.Value` for `Tally`; inherited from `Counter` (requirement at 4:", "requirement shared receiver access (`fn`)", "does not mutate its receiver"}},
		{"fn Increment() void {", 3, []string{"inherited from `Counter`", "this method mutates its receiver"}},
		{"fn Reset() void {", 3, []string{"implements `Resettable.Reset` for `Tally` (requirement at 9:"}},
	}
	for _, test := range cases {
		idx := strings.Index(text, test.marker)
		if idx < 0 {
			t.Fatalf("marker %q missing", test.marker)
		}
		hover, ok := hoverForSource(uri, text, offsetPosition(text, idx+test.skip))
		if !ok {
			t.Fatalf("%q: no hover", test.marker)
		}
		for _, want := range test.want {
			if !strings.Contains(hover.Contents.Value, want) {
				t.Errorf("%q: hover lacks %q", test.marker, want)
			}
		}
	}
}

// Completion on an interface view keeps the receiver capability in the label,
// names the declaring interface of an inherited requirement and hides `mut fn`
// requirements from a shared view.
// Rules: rules/declarations/interfaces.md — §§ 3, 5, 7; rules/tooling/lsp.md — "Completion".
func TestInterfaceCompletionShowsCapabilityAndProvenance(t *testing.T) {
	base := `module main

interface Counter {
    fn Value() int
    mut fn Increment() void
}

interface Resettable implements Counter {
    mut fn Reset() void
}
`
	for _, test := range []struct {
		parameter string
		want      []string
		reject    string
	}{
		{"ref mut Resettable", []string{"Value", "Increment", "Reset"}, ""},
		{"ref Resettable", []string{"Value"}, "Increment"},
	} {
		source := base + "\nfn Use(view: " + test.parameter + ") void {\n    discard view.\n}\n"
		offset := strings.LastIndex(source, "view.") + len("view.")
		items := completeSource("file:///interface-provenance/main.sec", source, offset)
		assertCompletionLabels(t, items, test.want)
		for _, item := range items {
			if test.reject != "" && item.Label == test.reject {
				t.Errorf("%s: shared view offers mut fn %s", test.parameter, item.Label)
			}
			documentation := fmt.Sprint(item.Documentation)
			if item.Label == "Increment" && (!strings.Contains(documentation, "mut fn Increment") || !strings.Contains(documentation, "inherited from Counter")) {
				t.Errorf("Increment documentation = %q", documentation)
			}
			if item.Label == "Reset" && strings.Contains(documentation, "inherited") {
				t.Errorf("own requirement marked inherited: %q", documentation)
			}
		}
	}
}
