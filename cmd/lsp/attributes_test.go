package main

import (
	"strings"
	"testing"

	"sec/internal/diagnostics"
)

func completionLabels(items []completionItem) string {
	labels := []string{}
	for _, item := range items {
		labels = append(labels, item.Label)
	}
	return strings.Join(labels, ",")
}

// After `@` completion offers only the compiler-known attributes valid on the
// declaration that follows; inside @target it offers unused selector names
// and the known os/arch values from the canonical target registry.
//
// Rules:
//   - rules/foundations/attributes.md — "LSP completion"
func TestAttributeCompletion(t *testing.T) {
	complete := func(source string) string {
		offset := strings.Index(source, "|")
		text := source[:offset] + source[offset+1:]
		items, ok := attributeCompletionItems(text, offset)
		if !ok {
			t.Fatalf("%q is not an attribute completion site", source)
		}
		return completionLabels(items)
	}
	for source, want := range map[string]string{
		"module main\n\n@|\nfn Work() void {}\n":                                 "target,when,interrupt,isr,interruptSafe,noAlloc,noPanic,noBlock",
		"module main\n\n@|\ntype ID int\n":                                       "target,when,noCopy",
		"module main\n\n@|\nlet mut Device: uint32\n":                            "target,when,address",
		"module main\n\n@no|\n// comment\n@noPanic\nextern \"C\" fn f() int32\n": "noAlloc,noPanic,noBlock",
		"module main\n\n@|\nextern \"C\" fn f() int32\n":                         "target,when,noAlloc,noPanic,noBlock,link_name",
		"module main\n\nimpl Reader {\n    @i|\n    fn Read() void {}\n}\n":      "isr,interrupt,interruptSafe",
		"module main\n\n@target(|\nfn F() void {}\n":                             "os,arch,cpu,device,board",
		"module main\n\n@target(os: \"linux\", |\nfn F() void {}\n":              "arch,cpu,device,board",
		"module main\n\n@target(os: \"li|\nfn F() void {}\n":                     "linux",
		"module main\n\n@interrupt(|\nfn F() void {}\n":                          "vector",
	} {
		got := complete(source)
		if source == "module main\n\nimpl Reader {\n    @i|\n    fn Read() void {}\n}\n" {
			// completionLabelMatches is a prefix or fuzzy match; compare as sets.
			for _, name := range strings.Split(want, ",") {
				if !strings.Contains(","+got+",", ","+name+",") {
					t.Errorf("%q completions = %s, want to include %s", source, got, name)
				}
			}
			continue
		}
		if got != want {
			t.Errorf("%q completions = %s, want %s", source, got, want)
		}
	}
	if items, ok := attributeCompletionItems("fn F() void {\n    let x := Call(|", 0); ok && len(items) != 0 {
		t.Errorf("ordinary call position offered attribute completions")
	}
	if _, ok := attributeCompletionItems("let x := Call(", len("let x := Call(")); ok {
		t.Errorf("an ordinary call argument list is not an attribute completion site")
	}
}

// Hover on a written attribute shows its meaning, allowed targets, status,
// the declaration's effective guarantees with their implication sources, and
// the compiler's cause path.
//
// Rules:
//   - rules/foundations/attributes.md — "LSP behavior", "Attribute implications"
func TestAttributeHover(t *testing.T) {
	text := `module main

type Message struct {
	value: int,
}

@noBlock
fn Wait(rx: Receiver[Message]) void {
	discard rx.Receive()
}

@interrupt(vector: 3)
fn Handler() void {
}

@noPanic
extern "C" fn native() int32
`
	hover := func(line, character int) string {
		result, ok := hoverForSource("file:///attributes.sec", text, position{Line: line, Character: character})
		if !ok {
			t.Fatalf("no hover at %d:%d", line, character)
		}
		return result.Contents.Value
	}
	for _, check := range []struct {
		line, character int
		want            []string
	}{
		{6, 2, []string{"**@noBlock**", "Allowed on: a function or method", "Status: verified by the compiler", "Effective guarantees: `noBlock` (written)", "May block: `yes` via `Wait` (Receiver.Receive at 9:13)"}},
		{11, 2, []string{"**@interrupt**", "Arguments: named: vector (required: vector)", "Status: not implemented yet", "`isr` (implied by @interrupt)", "`noBlock` (implied by @interrupt via @isr)", "May panic: `no`", "May block: `no`"}},
		{15, 2, []string{"**@noPanic**", "Status: trusted foreign contract"}},
	} {
		contents := hover(check.line, check.character)
		for _, want := range check.want {
			if !strings.Contains(contents, want) {
				t.Errorf("hover at %d:%d is missing %q:\n%s", check.line, check.character, want, contents)
			}
		}
	}
}

// Attribute diagnostics offer edits that remove a duplicate, keep the first of
// two conflicting values, remove an attribute from a declaration it cannot
// modify, or remove arguments from an argument-free attribute.
//
// Rules:
//   - rules/foundations/attributes.md — "Duplicate attributes", "Conflicting attributes", "LSP behavior"
//   - rules/tooling/lsp.md — "Code actions"
func TestAttributeCodeActions(t *testing.T) {
	for _, check := range []struct {
		name, source, code, title, result string
	}{
		{"duplicate", "module main\n\n@noPanic\n@noPanic\nfn F() void {}\n", diagnostics.AttributeDuplicate, "Remove duplicate @noPanic", "module main\n\n@noPanic\nfn F() void {}\n"},
		{"conflict", "module main\n\n@address(0x40000000)\n@address(0x50000000)\nlet mut D: uint32\n", diagnostics.AttributeConflict, "Keep the first @address and remove this one", "module main\n\n@address(0x40000000)\nlet mut D: uint32\n"},
		{"wrong target", "module main\n\n@noCopy\nfn F() void {}\n", diagnostics.AttributeNotAllowedOnTarget, "Remove @noCopy", "module main\n\nfn F() void {}\n"},
		{"arguments", "module main\n\n@noPanic(fast)\nfn F() void {}\n", diagnostics.AttributeInvalidArgument, "Remove the arguments of @noPanic", "module main\n\n@noPanic\nfn F() void {}\n"},
	} {
		t.Run(check.name, func(t *testing.T) {
			reported := []diagnostic{}
			for _, candidate := range analyze("file:///actions.sec", check.source) {
				if candidate.Code == check.code {
					reported = append(reported, candidate)
				}
			}
			if len(reported) == 0 {
				t.Fatalf("no %s diagnostic in %+v", check.code, analyze("file:///actions.sec", check.source))
			}
			actions := attributeCodeActions("file:///actions.sec", check.source, reported)
			if len(actions) != 1 || actions[0].Title != check.title {
				t.Fatalf("actions = %+v, want %q", actions, check.title)
			}
			edits := actions[0].Edit.Changes["file:///actions.sec"]
			if got := applyTextEdits(check.source, edits); got != check.result {
				t.Fatalf("result =\n%q\nwant\n%q", got, check.result)
			}
		})
	}
}

// The attribute completion site is reached through the ordinary completion
// entry point.
func TestAttributeCompletionThroughCompleteSource(t *testing.T) {
	text := "module main\n\n@no\ntype ID int\n"
	offset := strings.Index(text, "@no") + len("@no")
	if got := completionLabels(completeSource("file:///complete.sec", text, offset)); got != "noCopy" {
		t.Fatalf("completions = %s, want noCopy", got)
	}
}
