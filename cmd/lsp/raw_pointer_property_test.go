package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// As* conversion reads are properties in completion, hover and synthetic
// definitions; they do not offer method-call signature help.
// Rules: rules/corrections/applied/md070-as-property-naming-correction-20261106.md §§2,4.
func TestRawPointerReferencePropertyTooling(t *testing.T) {
	data, err := os.ReadFile("../../testdata/tooling/raw_pointer_reference_valid.sec")
	if err != nil {
		t.Fatal(err)
	}
	source := string(data)
	uri := uriFromPath(filepath.Join(t.TempDir(), "main.sec"))
	for _, name := range []string{"AsRef", "AsRefMut"} {
		offset := strings.Index(source, "pointer."+name) + len("pointer.")
		hover, ok := hoverForSource(uri, source, offsetPosition(source, offset))
		if !ok || !strings.Contains(hover.Contents.Value, "unsafe property "+name+":") || strings.Contains(hover.Contents.Value, name+"()") {
			t.Fatalf("hover %s: %+v", name, hover)
		}
		definitions := definitionsForSource(uri, source, offsetPosition(source, offset))
		if len(definitions) != 1 {
			t.Fatalf("missing property definition: %+v", definitions)
		}
		definition, found := compilerKnownDefinitionText(definitions[0].URI)
		if !found || !strings.Contains(definition, "unsafe property "+name+":") || strings.Contains(definition, name+"()") {
			t.Fatal(definition)
		}
		if help, found := signatureHelpForSource(uri, source, offsetPosition(source, offset+len(name))); found {
			t.Fatalf("property offered call help: %+v", help)
		}
		items := completeSource(uri, source, offset)
		completionFound := false
		for _, item := range items {
			if item.Label == name {
				completionFound = true
				if item.Kind != 10 || (item.TextEdit != nil && strings.Contains(item.TextEdit.NewText, "(")) {
					t.Fatalf("method completion: %+v", item)
				}
			}
		}
		if !completionFound {
			t.Fatalf("missing %s", name)
		}
	}
}

// Even legacy parentheses cannot make a property into a callable signature.
// Rules: rules/corrections/applied/md070-as-property-naming-correction-20261106.md §2.
func TestRawPointerReferencePropertyLegacyCallsLSP(t *testing.T) {
	data, err := os.ReadFile("../../testdata/tooling/raw_pointer_property_calls_invalid.sec")
	if err != nil {
		t.Fatal(err)
	}
	source := string(data)
	uri := uriFromPath(filepath.Join(t.TempDir(), "main.sec"))
	for _, name := range []string{"AsRef", "AsRefMut"} {
		offset := strings.Index(source, "pointer."+name+"(") + len("pointer."+name+"(")
		if h, found := signatureHelpForSource(uri, source, offsetPosition(source, offset)); found {
			t.Fatalf("property offered method help: %+v", h)
		}
	}
	items := analyze(uri, source)
	if len(items) != 3 {
		t.Fatal(items)
	}
	for _, d := range items {
		if d.Code != "S1205" || !strings.Contains(d.Message, "property and cannot be called") {
			t.Fatal(d)
		}
	}
}
