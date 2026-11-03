package main

import (
	"os"
	"strings"
	"testing"
)

// The normal definition endpoint projects Sema's validated impl-block links,
// including same-module siblings and unsaved source positions. Header type
// identifiers retain their existing type-definition behavior.
// Rules: rules/declarations/impl.md — §§4–5, §22;
// rules/tooling/lsp.md — "One semantic source of truth", "Navigation".
func TestImplExtensionDefinitionNavigation(t *testing.T) {
	dir := t.TempDir()
	sources, paths := map[string]string{}, map[string]string{}
	for _, name := range []string{"primary", "extension_a", "extension_b"} {
		data, err := os.ReadFile("../../testdata/tooling/impl_navigation/" + name + ".sec")
		if err != nil {
			t.Fatal(err)
		}
		sources[name] = string(data)
		paths[name] = writeWorkspaceSource(t, dir, name+".sec", sources[name])
	}
	// Only the overlay contains the new positions; disk keeps the old source.
	sources["extension_b"] = "// unsaved changes\n\n" + sources["extension_b"]
	overlay := sourceOverlay{Sources: map[string]string{paths["extension_b"]: sources["extension_b"]}}
	for _, test := range []struct {
		file, header string
		targets      []string
		targetHeader string
	}{
		{"primary", "impl Widget", []string{"extension_a", "extension_b"}, "impl extends Widget"},
		{"extension_a", "impl extends Widget", []string{"primary"}, "impl Widget"},
		{"extension_b", "impl extends Widget", []string{"primary"}, "impl Widget"},
		{"primary", "impl Box[T]", []string{"extension_a"}, "impl extends Box[T]"},
		{"extension_a", "impl extends Box[T]", []string{"primary"}, "impl Box[T]"},
	} {
		t.Run(test.file+"/"+test.header, func(t *testing.T) {
			source := sources[test.file]
			uri := uriFromPath(paths[test.file])
			at := offsetPosition(source, strings.Index(source, test.header)+1)
			got := definitionsForSource(uri, source, at, overlay)
			if len(got) != len(test.targets) {
				t.Fatalf("locations=%v, want %v", got, test.targets)
			}
			for i, name := range test.targets {
				start := offsetPosition(sources[name], strings.Index(sources[name], test.targetHeader))
				end := start
				end.Character += 4
				if got[i].URI != uriFromPath(paths[name]) || got[i].Range.Start != start || got[i].Range.End != end {
					t.Fatalf("location=%v, want %s at %v..%v", got[i], name, start, end)
				}
			}
		})
	}
	for _, test := range []struct{ file, header, name string }{
		{"primary", "impl Widget", "Widget"}, {"extension_a", "impl extends Widget", "Widget"},
		{"primary", "impl Box[T]", "Box"}, {"extension_a", "impl extends Box[T]", "Box"},
	} {
		source := sources[test.file]
		offset := strings.Index(source, test.header) + strings.Index(test.header, test.name)
		got := definitionsForSource(uriFromPath(paths[test.file]), source, offsetPosition(source, offset), overlay)
		want := offsetPosition(sources["primary"], strings.Index(sources["primary"], "type "+test.name)+len("type "))
		if len(got) != 1 || got[0].URI != uriFromPath(paths["primary"]) || got[0].Range.Start != want {
			t.Fatalf("header target lost type navigation: %v, want %v", got, want)
		}
	}
}

// Invalid extensions must not guess a primary from matching source spelling.
// Rules: rules/declarations/impl.md — §§5.1,27;
// rules/tooling/lsp.md — "Unknown facts", "Navigation".
func TestImplExtensionDefinitionNavigationRejectsInvalidHeader(t *testing.T) {
	dir := t.TempDir()
	var source, path string
	for _, file := range []struct{ fixture, name string }{
		{"tooling/impl_navigation/primary.sec", "primary.sec"},
		{"sema/impl_navigation_invalid/same_module_invalid.sec", "invalid.sec"},
	} {
		data, err := os.ReadFile("../../testdata/" + file.fixture)
		if err != nil {
			t.Fatal(err)
		}
		written := writeWorkspaceSource(t, dir, file.name, string(data))
		if file.name == "invalid.sec" {
			source, path = string(data), written
		}
	}
	for _, header := range []string{"impl extends Orphan", "impl extends Box[int]"} {
		got := definitionsForSource(uriFromPath(path), source, offsetPosition(source, strings.Index(source, header)+1))
		if len(got) != 0 {
			t.Fatalf("invalid header %s navigates to %v", header, got)
		}
	}
}
