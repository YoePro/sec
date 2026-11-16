package main

import (
	"os"
	"strings"
	"testing"
)

// TestMethodGenericHover presents independent method parameter declarations and
// exact call bindings from Sema, without mixing sibling scopes or owner types.
// Rules: rules/tooling/lsp.md — §12(1),(3);
// rules/declarations/generics.md — §§9,18–20,30.
func TestMethodGenericHover(t *testing.T) {
	data, err := os.ReadFile("../../testdata/tooling/generics/method_hover.sec")
	if err != nil {
		t.Fatal(err)
	}
	source := string(data)
	for _, test := range []struct {
		marker string
		offset int
		wants  []string
		absent string
	}{
		{"Choose[U]", len("Choose["), []string{"generic U", "Constraints: _none_"}, "Impl target parameter"},
		{"Check[U: Named]", len("Check["), []string{"generic U: Named", "fn Name() string"}, "Impl target parameter"},
		{"impl Box[T]", len("impl Box["), []string{"generic T", "Impl target parameter of `Box`"}, "Method type arguments"},
		{"box.Choose[bool]", len("box."), []string{"fn Box.Choose(value: bool) bool", "Owner type arguments: `T = int`", "Method type arguments: `U = bool`"}, "[U]"},
		{"box.Choose(1)", len("box."), []string{"fn Box.Choose(value: int) int", "Owner type arguments: `T = bool`", "Method type arguments: `U = int`"}, "U = bool"},
		{"Box[int].Make[bool]", len("Box[int]."), []string{"static fn Box.Make(value: bool) bool", "Owner type arguments: `T = int`", "Method type arguments: `U = bool`"}, "[U]"},
		{"box.Check(label)", len("box."), []string{"Method type arguments: `U = Label`", "value: ref Label"}, "U = bool"},
	} {
		offset := strings.Index(source, test.marker)
		if offset < 0 {
			t.Fatal(test.marker)
		}
		hover, ok := hoverForSource("", source, offsetPosition(source, offset+test.offset))
		if !ok {
			t.Fatal("missing hover", test.marker)
		}
		for _, want := range test.wants {
			if !strings.Contains(hover.Contents.Value, want) {
				t.Errorf("%s lacks %q: %s", test.marker, want, hover.Contents.Value)
			}
		}
		if strings.Contains(hover.Contents.Value, test.absent) {
			t.Errorf("%s includes %q: %s", test.marker, test.absent, hover.Contents.Value)
		}
	}
}
