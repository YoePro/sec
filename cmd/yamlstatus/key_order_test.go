package main

import (
	"bytes"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestCanonicalIntegrationKeyOrder(t *testing.T) {
	// Deliberately scrambled fields, with extension fields interspersed. Nested
	// checklist IDs are data, not integration boundaries.
	entry := `extension_z: null
verification:
  - result: passed
    command: test
remaining:
  - description: keep this field order
    id: nested
  - second
summary: >-
  Two words.
document_revision: 2.0
integrated: 2026-11-18
extension_a: true
status: partial
id: core.one
`
	wantKeys := []string{"id", "status", "integrated", "document_revision", "summary", "remaining", "verification", "extension_z", "extension_a"}
	for _, shape := range []string{"standalone", "sequence", "wrapped"} {
		t.Run(shape, func(t *testing.T) {
			source := entry
			if shape != "standalone" {
				source = "- " + strings.ReplaceAll(strings.TrimSuffix(entry, "\n"), "\n", "\n  ") + "\n"
			}
			if shape == "wrapped" {
				source = "area: core\nschema_version: 1\nintegrations:\n  " + strings.ReplaceAll(strings.TrimSuffix(source, "\n"), "\n", "\n  ") + "\n"
			}
			doc, err := parseDocument("test.yaml", []byte(source))
			if err != nil {
				t.Fatal(err)
			}
			// Capture the original tree, including its field order and annotations.
			doc.integrations()[0].HeadComment = "source comment"
			before, err := yaml.Marshal(doc.Root)
			if err != nil {
				t.Fatal(err)
			}
			rendered, err := render(doc.Root)
			if err != nil {
				t.Fatal(err)
			}
			if err := verifyRendered(doc.Root, rendered); err != nil {
				t.Fatal(err)
			}
			after, err := yaml.Marshal(doc.Root)
			if err != nil || !bytes.Equal(before, after) {
				t.Fatalf("source tree mutated: %v", err)
			}
			canonical, err := parseDocument("canonical.yaml", rendered)
			if err != nil {
				t.Fatal(err)
			}
			integration := canonical.integrations()[0]
			if got := keyNames(integration); !reflect.DeepEqual(got, wantKeys) {
				t.Fatalf("keys %v, want %v", got, wantKeys)
			}
			remaining := mappingValue(integration, "remaining")
			if got := keyNames(remaining.Content[0]); !reflect.DeepEqual(got, []string{"description", "id"}) {
				t.Fatalf("nested item reordered: %v", got)
			}
			verification := mappingValue(integration, "verification").Content[0]
			if got := keyNames(verification); !reflect.DeepEqual(got, []string{"result", "command"}) {
				t.Fatalf("verification reordered: %v", got)
			}
			if shape == "wrapped" && !strings.HasPrefix(string(rendered), "area: core\nschema_version: 1\n") {
				t.Fatal("fragment header reordered")
			}
			second, err := render(canonical.Root)
			if err != nil || !bytes.Equal(second, rendered) {
				t.Fatalf("not idempotent: %v", err)
			}
		})
	}
	registry := []byte("status_values:\n  partial: some\n  implemented: done\nfragments:\n  - status: metadata\n    id: not-an-integration\nschema_version: 1\n")
	doc, err := parseDocument("index.yaml", registry)
	if err != nil {
		t.Fatal(err)
	}
	got, err := render(doc.Root)
	if err != nil || !bytes.Equal(got, registry) {
		t.Fatalf("registry order changed: %s (%v)", got, err)
	}
}

func keyNames(node *yaml.Node) []string {
	var keys []string
	for i := 0; i < len(node.Content); i += 2 {
		keys = append(keys, node.Content[i].Value)
	}
	return keys
}

func TestKeyOrderFmtPreviewAndSubsequentEdit(t *testing.T) {
	source := "schema_version: 1\narea: core\nintegrations:\n  - remaining:\n      - first\n    status: partial\n    id: core.one\n\n  - status: planned\n    id: core.two\n"
	dir := governanceFixture(t, map[string]string{"core.yaml": source})
	path := filepath.Join(dir, "core.yaml")
	if _, err := runCommand(t, "fmt", "-check", path); err == nil {
		t.Fatal("accepted unordered fields")
	}
	if out, err := runCommand(t, "fmt", "-diff", path); err != nil || !strings.Contains(out, "+  - id: core.one") {
		t.Fatalf("preview %s: %v", out, err)
	}
	if readFile(t, path) != source {
		t.Fatal("preview wrote the source")
	}
	if out, err := runCommand(t, "fmt", "-write", path); err != nil {
		t.Fatalf("fmt: %s: %v", out, err)
	}
	if out, err := runCommand(t, "set", "-dir", dir, "-write", "core.one", "summary", "New summary"); err != nil {
		t.Fatalf("edit: %s: %v", out, err)
	}
	if out, err := runCommand(t, "fmt", "-check", path); err != nil {
		t.Fatalf("check: %s: %v", out, err)
	}
	doc, err := loadDocument(path)
	if err != nil {
		t.Fatal(err)
	}
	entries := doc.integrations()
	if len(entries) != 2 || integrationID(entries[0]) != "core.one" || integrationID(entries[1]) != "core.two" {
		t.Fatal("integration list order changed")
	}
	if got := keyNames(entries[0]); !reflect.DeepEqual(got, []string{"id", "status", "summary", "remaining"}) {
		t.Fatalf("new field not in canonical position: %v", got)
	}
}
