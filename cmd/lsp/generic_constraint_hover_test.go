package main

import (
	"strings"
	"testing"
)

const orderedConstraintHoverSource = `module main

interface Named {
    fn Name() string
}

interface Labelled {
    fn Name() string
    mut fn Relabel(text: string) void
}

interface Source[E] {
    fn Next() E
}

type Pair[K: Labelled & Named, V] struct {
    key: K,
    value: V,
}

impl Pair[K, V] {
    fn Describe(other: K, extra: V) string {
        discard extra
        return other.Name()
    }
}

fn Pick[T: Labelled & Named & Source[int], U](first: ref mut T, second: U) string {
    first.Relabel("x")
    discard second
    return first.Name()
}
`

func hoverAtNeedle(t *testing.T, needle string, delta int) string {
	t.Helper()
	offset := strings.Index(orderedConstraintHoverSource, needle)
	if offset < 0 {
		t.Fatalf("missing needle %q", needle)
	}
	result, ok := hoverForSource("", orderedConstraintHoverSource, offsetPosition(orderedConstraintHoverSource, offset+delta))
	if !ok {
		t.Fatalf("missing hover at %q", needle)
	}
	return result.Contents.Value
}

// Hover presents ordered multiple-constraint sets from Sema's resolved
// template and generic-parameter facts: declaration headers keep parameter
// and conjunct order, parameter hover lists every required constraint and the
// composed guaranteed instance methods, impl target parameters present the
// target declaration's constraints, and unconstrained parameters claim no
// members.
//
// Rules:
//   - rules/declarations/generics.md — §8 "Generic impl blocks"
//   - rules/declarations/generics.md — §11 "Constraints", §12 "Multiple constraints"
//   - rules/declarations/generics.md — §15 "Operations available on generic parameters"
//   - rules/tooling/lsp.md — "Hover"
func TestHoverPresentsOrderedMultipleGenericConstraints(t *testing.T) {
	tests := []struct {
		name    string
		needle  string
		delta   int
		want    []string
		without []string
	}{
		{
			name: "function header", needle: "Pick[", want: []string{
				"fn Pick[T: Labelled & Named & Source[int], U](first: ref mut T, second: U) string",
			},
		},
		{
			name: "type header", needle: "Pair[", want: []string{"type Pair[K: Labelled & Named, V]"},
		},
		{
			name: "parameter declaration", needle: "T: Labelled", want: []string{
				"generic T: Labelled & Named & Source[int]",
				"Constraints (all required, in source order): `Labelled` & `Named` & `Source[int]`",
				"fn Name() string\nmut fn Relabel(text: string) void\nfn Next() int",
			},
		},
		{
			name: "parameter use", needle: "ref mut T", delta: len("ref mut "), want: []string{
				"generic T: Labelled & Named & Source[int]",
			},
		},
		{
			name: "type parameter use", needle: "key: K", delta: len("key: "), want: []string{
				"generic K: Labelled & Named",
				"fn Name() string\nmut fn Relabel(text: string) void\n```",
			},
		},
		{
			name: "impl target parameter", needle: "Pair[K, V] {", delta: len("Pair["), want: []string{
				"generic K: Labelled & Named",
				"Impl target parameter of `Pair`; its constraints come from that declaration.",
				"fn Name() string\nmut fn Relabel(text: string) void\n```",
			},
		},
		{
			name: "impl method parameter use", needle: "other: K", delta: len("other: "), want: []string{
				"generic K: Labelled & Named",
				"Impl target parameter of `Pair`",
			},
		},
		{
			name: "unconstrained impl parameter", needle: "extra: V", delta: len("extra: "), want: []string{
				"generic V\n",
				"Impl target parameter of `Pair`",
				"Constraints: _none_; no interface members are guaranteed.",
			},
		},
		{
			name: "unconstrained", needle: "U](", want: []string{
				"generic U\n",
				"Constraints: _none_; no interface members are guaranteed.",
			},
			without: []string{"Guaranteed instance methods"},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			contents := hoverAtNeedle(t, test.needle, test.delta)
			for _, want := range test.want {
				if !strings.Contains(contents, want) {
					t.Errorf("hover does not contain %q:\n%s", want, contents)
				}
			}
			for _, unwanted := range test.without {
				if strings.Contains(contents, unwanted) {
					t.Errorf("hover contains %q:\n%s", unwanted, contents)
				}
			}
			if strings.Count(contents, "fn Name() string") > 1 {
				t.Errorf("equivalent requirement shown more than once:\n%s", contents)
			}
		})
	}
}
