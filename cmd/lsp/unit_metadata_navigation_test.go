package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Explicit metadata uses navigate to their accepted key, including migration
// spelling; implicit/defaulted fields use the unit declaration. These are
// Sema definitions, not an LSP search by member name.
// Rules: rules/types/units.md — "LSP requirements"; MD-062 §12(9);
// rules/tooling/lsp.md — "Navigation and references".
func TestUnitMetadataDefinitions(t *testing.T) {
	bytes, err := os.ReadFile("../../testdata/tooling/unit_metadata_navigation.sec")
	if err != nil {
		t.Fatal(err)
	}
	source := string(bytes)
	uri := uriFromPath(filepath.Join(t.TempDir(), "source.sec"))
	for _, tc := range []struct {
		expression, declaration string
		skip                    int
	}{
		{"<plain>.Name", "unit plain", len("unit ")}, {"<plain>.Category", "unit plain", len("unit ")},
		{"<flow>.LongName", "long_name:", 0}, {"<flow>.Symbol", "Symbol:", 0},
		{"<flow>.BaseUnit", "BaseUnit:", 0}, {"<flow>.Status", "Status:", 0},
		{"<flow>.Dimension", "Dimension:", 0}, {"<flow>.Kind", "Kind:", 0},
		{"<flow>.Scale", "Scale:", 0}, {"<flow>.System", "System:", 0},
		{"<flow>.Origin", "Origin:", 0}, {"<flow>.Transform", "Transform:", 0},
		{"<point>.Offset", "Offset:", 0}, {"<logarithm>.LogBase", "LogBase:", 0},
		{"<logarithm>.LogFactor", "LogFactor:", 0}, {"<logarithm>.Reference", "Reference:", 0},
		{"<plain>.Kind", "unit plain", len("unit ")}, {"<plain>.Scale", "unit plain", len("unit ")},
		{"<plain>.Dimension", "unit plain", len("unit ")},
	} {
		t.Run(tc.expression, func(t *testing.T) {
			dot := strings.Index(tc.expression, ">.") + 2
			cursor := strings.Index(source, tc.expression) + dot
			got := definitionsForSource(uri, source, offsetPosition(source, cursor))
			want := offsetPosition(source, strings.Index(source, tc.declaration)+tc.skip)
			if len(got) != 1 || got[0].URI != uri || got[0].Range.Start != want {
				t.Fatalf("definitions=%+v, want %s at %+v", got, uri, want)
			}
		})
	}
	// Type navigation follows the concrete custom enum, not its metadata key.
	cursor := strings.Index(source, "<flow>.Kind") + len("<flow>.")
	got := typeDefinitionsForSource(uri, source, offsetPosition(source, cursor))
	want := offsetPosition(source, strings.Index(source, "enum Domain")+len("enum "))
	if len(got) != 1 || got[0].URI != uri || got[0].Range.Start != want {
		t.Fatalf("custom enum type definition=%+v, want %+v", got, want)
	}
	// The two source reads are references to the same metadata declaration.
	refs := referencesForSource(uri, source, offsetPosition(source, strings.Index(source, "Kind:")), false, sourceOverlay{Sources: map[string]string{normalizedSourcePath(pathFromURI(uri)): source}})
	if len(refs) != 2 {
		t.Fatalf("Kind references=%+v, want two reads", refs)
	}
}

// The opaque dimension's properties navigate through the existing synthetic
// compiler-known registry. Metadata and dimension properties are not methods
// and must not display callable signature help.
// Rules: rules/library/core-library.md §27.5; MD-062 §12(9);
// rules/tooling/lsp.md — "Signature help", "Compiler-known definitions".
func TestUnitMetadataPropertyNavigationAndSignatures(t *testing.T) {
	bytes, err := os.ReadFile("../../testdata/tooling/unit_metadata_navigation.sec")
	if err != nil {
		t.Fatal(err)
	}
	source := string(bytes)
	uri := uriFromPath(filepath.Join(t.TempDir(), "source.sec"))
	for _, name := range []string{"AxisCount", "IsDimensionless"} {
		cursor := strings.Index(source, "dimension."+name) + len("dimension.")
		got := definitionsForSource(uri, source, offsetPosition(source, cursor))
		if len(got) != 1 || !strings.HasPrefix(got[0].URI, "sec-compiler-known:/CKM-UNITDIMENSION-") {
			t.Fatalf("%s definitions=%+v", name, got)
		}
		if text, ok := compilerKnownDefinitionText(got[0].URI); !ok || !strings.Contains(text, name) {
			t.Fatalf("missing synthetic property %s: %s", name, text)
		}
		if help, ok := signatureHelpForSource(uri, source, offsetPosition(source, cursor+len(name))); ok {
			t.Fatalf("property signature fabricated: %+v", help)
		}
	}
	for _, expression := range []string{"<flow>.Kind", "<flow>.Scale", "<flow>.Dimension"} {
		cursor := strings.Index(source, expression) + len(expression)
		if help, ok := signatureHelpForSource(uri, source, offsetPosition(source, cursor)); ok {
			t.Fatalf("metadata signature fabricated: %+v", help)
		}
	}
}

// Imported aliases retain declaration/type provenance from the live module
// snapshot. Re-analysis must not retain old positions after that snapshot moves.
// Rules: rules/tooling/lsp.md — "Snapshots", "Navigation and references";
// rules/types/units.md — "Unit metadata value types" (11); MD-062 §12(11).
func TestUnitMetadataNavigationImportedSnapshot(t *testing.T) {
	catalogBytes, err := os.ReadFile("../../testdata/tooling/unit_metadata_navigation.sec")
	if err != nil {
		t.Fatal(err)
	}
	useBytes, err := os.ReadFile("../../testdata/lsp/unit_metadata/imported_reads.sec")
	if err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	for _, dir := range []string{".sec", "catalog"} {
		if err := os.Mkdir(filepath.Join(root, dir), 0755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(root, ".sec", "sec.toml"), []byte("[project]\nname = \"metadata-navigation\"\n"), 0644); err != nil {
		t.Fatal(err)
	}
	catalogFile := filepath.Join(root, "catalog", "catalog.sec")
	catalog := strings.Replace(string(catalogBytes), "module sample", "module catalog", 1)
	if err := os.WriteFile(catalogFile, []byte(catalog), 0644); err != nil {
		t.Fatal(err)
	}
	uri := uriFromPath(filepath.Join(root, "main.sec"))
	source := string(useBytes)
	for _, snapshot := range []string{catalog, strings.Replace(catalog, "    long_name:", "    // current unsaved snapshot\n\n    long_name:", 1)} {
		overlay := sourceOverlay{Sources: map[string]string{catalogFile: snapshot}}
		for _, tc := range []struct {
			expression, declaration string
			skip                    int
		}{
			{"<q.flow>.Name", "unit flow", len("unit ")},
			{"<q.flow>.Kind", "Kind:", 0}, {"<q.flow>.LongName", "long_name:", 0}, {"<q.plain>.Scale", "unit plain", len("unit ")},
		} {
			offset := strings.Index(source, tc.expression) + strings.Index(tc.expression, ">.") + 2
			got := definitionsForSource(uri, source, offsetPosition(source, offset), overlay)
			want := offsetPosition(snapshot, strings.Index(snapshot, tc.declaration)+tc.skip)
			if len(got) != 1 || got[0].URI != uriFromPath(catalogFile) || got[0].Range.Start != want {
				t.Fatalf("%s definition=%+v, want %s at %+v", tc.expression, got, uriFromPath(catalogFile), want)
			}
		}
		nameOffset := strings.Index(source, "<q.flow>.Name") + len("<q.flow>.")
		if hover, ok := hoverForSource(uri, source, offsetPosition(source, nameOffset), overlay); !ok || !strings.Contains(hover.Contents.Value, "`\"flow\"`") {
			t.Fatalf("imported Name lost its short declaration name: %+v", hover)
		}
		completionSource := strings.Replace(source, "<q.flow>.Kind", "<q.flow>.", 1)
		cursor := strings.Index(completionSource, "<q.flow>.\n") + len("<q.flow>.")
		items := completeSource(uri, completionSource, cursor, overlay)
		found := false
		for _, item := range items {
			if item.Label == "Kind" && strings.Contains(item.Detail, "catalog.Domain") {
				found = true
			}
		}
		if !found {
			t.Fatalf("imported Kind completion lost concrete enum: %+v", items)
		}
		offset := strings.Index(source, "<q.flow>.Kind") + len("<q.flow>.")
		got := typeDefinitionsForSource(uri, source, offsetPosition(source, offset), overlay)
		want := offsetPosition(snapshot, strings.Index(snapshot, "enum Domain")+len("enum "))
		if len(got) != 1 || got[0].URI != uriFromPath(catalogFile) || got[0].Range.Start != want {
			t.Fatalf("imported concrete enum type=%+v, want %+v", got, want)
		}
	}
}

// Fixed metadata names cannot be renamed. Renaming the owning unit must
// change only unit-name uses, preserving implicit/defaulted metadata names.
// Rules: rules/tooling/lsp.md — "Rename"; MD-062 §§2(1),6(3),12(9).
func TestUnitMetadataRenameBoundaries(t *testing.T) {
	bytes, err := os.ReadFile("../../testdata/tooling/unit_metadata_navigation.sec")
	if err != nil {
		t.Fatal(err)
	}
	source := string(bytes)
	uri := uriFromPath(filepath.Join(t.TempDir(), "source.sec"))
	overlay := sourceOverlay{Sources: map[string]string{normalizedSourcePath(pathFromURI(uri)): source}}
	for _, marker := range []string{"Kind:", "long_name:", "<flow>.Kind", "<plain>.Name", "<plain>.Scale"} {
		cursor := strings.Index(source, marker)
		if strings.HasPrefix(marker, "<") {
			cursor += strings.Index(marker, ">.") + 2
		}
		pos := offsetPosition(source, cursor)
		if _, err := prepareRenameForSource(uri, source, pos, overlay); err == nil || !strings.Contains(err.Error(), "metadata") {
			t.Fatalf("prepare rename %s: %v", marker, err)
		}
		if _, err := renameForSource(uri, source, pos, "Changed", overlay); err == nil {
			t.Fatalf("renamed fixed metadata %s", marker)
		}
	}
	pos := offsetPosition(source, strings.Index(source, "unit plain")+len("unit "))
	edit, err := renameForSource(uri, source, pos, "plainRenamed", overlay)
	if err != nil {
		t.Fatal(err)
	}
	got := applyTextEdits(source, edit.Changes[uri])
	for _, want := range []string{"unit plainRenamed", "<plainRenamed>.Name", "<plainRenamed>.Kind", "<plainRenamed>.Scale", "<plainRenamed>.Dimension"} {
		if !strings.Contains(got, want) {
			t.Fatalf("unit rename lost %q:\n%s", want, got)
		}
	}
}

// Erroneous property-call syntax never acquires method signature help during
// recovery. Unit metadata access does not introduce callable properties.
// Rules: rules/types/units.md — "Unit metadata value types" (1), (7);
// rules/tooling/lsp.md — "Signature help"; MD-062 §12(9).
func TestUnitMetadataInvalidCallsHaveNoSignature(t *testing.T) {
	bytes, err := os.ReadFile("../../testdata/lsp/unit_metadata/callable_invalid.sec")
	if err != nil {
		t.Fatal(err)
	}
	source := string(bytes)
	uri := uriFromPath(filepath.Join(t.TempDir(), "source.sec"))
	if diagnostics := analyze(uri, source); len(diagnostics) == 0 {
		t.Fatal("property-call negative fixture must be rejected")
	}
	for _, call := range []string{"<u>.Kind(", "<u>.Scale(", "dimension.AxisCount(", "dimension.IsDimensionless("} {
		cursor := strings.Index(source, call) + len(call)
		if help, ok := signatureHelpForSource(uri, source, offsetPosition(source, cursor)); ok {
			t.Fatalf("invalid property call %s has help: %+v", call, help)
		}
	}
}
