package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func bookCheckoutFixture(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	writeFixture(t, root, "rules/README.md", "# Books\n\n| Directory | Scope |\n| --- | --- |\n| `types/` | Type rules. |\n| `memory/` | Ownership rules. |\n")
	writeFixture(t, root, "language-rulebook-status.md", "# Inventory\n\n| Book | Status | Notes |\n| --- | --- | --- |\n| `types/one.md` | **Written** | canonical |\n| `memory/two.md` | **Living** | canonical |\n| `types/one.md` | **Written** | repeated reference |\n| `types/future.md` | **Planned** | absent |\n| `types/covered.md` | **Covered** | no duplicate |\n| `types/pending.md` | **Written — repository sync pending** | absent |\n| `types/notes.txt` | **Living** | text |\n")
	writeFixture(t, root, "rules/types/one.md", "# One\n\n**Document revision:** 2.3\n\nSee [Two](../memory/two.md#δ-section), `two.md`, and memory/two.md.\n\n[planned](future.md)\n\n## Åäö λ\n\n## Åäö λ\n\n<a id=\"explicit\"></a>\n\n```sec\n// two.md must remain code\n```\n")
	writeFixture(t, root, "rules/memory/two.md", "# Two\n\n## Δ section\n\n[back](../types/one.md#åäö-λ-1)\n\n[explicit](../types/one.md#explicit)\n\n[web](https://example.com/book.md)\n")
	writeFixture(t, root, "rules/types/notes.txt", "Text notes\n==========\n\n  %value = 3 < 4\n")
	writeFixture(t, root, "rules/types/unlisted.md", "# Not selected")
	writeFixture(t, root, "rules/corrections/pending.md", "# Not public")
	return root
}

func TestBooksCLIEndToEnd(t *testing.T) {
	t.Parallel()
	source := bookCheckoutFixture(t)
	output := filepath.Join(t.TempDir(), "nested", "site")
	text, err := os.ReadFile(filepath.Join(source, "rules/memory/two.md"))
	if err != nil {
		t.Fatal(err)
	}
	writeFixture(t, source, "rules/memory/two.md", string(text)+"\n<a href=\"../types/one.html?mode=all#explicit\">HTML link</a>\n")
	var out, diagnostics bytes.Buffer
	if err := runCLI([]string{"books", "--indir", source, "--outdir", output}, &out, &diagnostics); err != nil {
		t.Fatal(err, diagnostics.String())
	}
	if !strings.Contains(out.String(), "Published 3 rulebooks") {
		t.Fatal(out.String())
	}
	for _, name := range []string{"index.html", "types/index.html", "memory/index.html", "types/one.html", "memory/two.html", "types/notes.html", bookManifestName} {
		if _, err := os.Stat(filepath.Join(output, name)); err != nil {
			t.Fatal(name, err)
		}
	}
	page, err := os.ReadFile(filepath.Join(output, "types/one.html"))
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`href="../memory/two.html#%CE%B4-section"`, `href="../memory/two.html"`, `id="åäö-λ"`, `id="åäö-λ-1"`, `id="explicit"`, `id="one"`, "Rulebook revision", "2.3", "Documentation status", "Written", "// two.md must remain code"} {
		if !bytes.Contains(page, []byte(want)) {
			t.Errorf("missing %q", want)
		}
	}
	if bytes.Count(page, []byte("<h1")) != 1 {
		t.Fatal("book title duplicated")
	}
	two, _ := os.ReadFile(filepath.Join(output, "memory/two.html"))
	if !bytes.Contains(two, []byte(`href="../types/one.html?mode=all#explicit"`)) {
		t.Fatal("HTML book link/query was not preserved")
	}
	notes, _ := os.ReadFile(filepath.Join(output, "types/notes.html"))
	if !bytes.Contains(notes, []byte("<h1>Text notes</h1>")) {
		t.Fatal("plain-text title not extracted")
	}
	if bytes.Contains(page, []byte(`href="future.md"`)) {
		t.Fatal("unavailable book emitted as a broken link")
	}
	index, _ := os.ReadFile(filepath.Join(output, "index.html"))
	if !bytes.Contains(index, []byte("future.md")) || !bytes.Contains(index, []byte("Planned")) {
		t.Fatal("unavailable inventory not explained")
	}
	for _, name := range []string{"types/unlisted.html", "corrections/pending.html", "types/future.html"} {
		if _, err := os.Stat(filepath.Join(output, name)); !os.IsNotExist(err) {
			t.Fatal("unselected page published", name)
		}
	}
	manifest, _ := os.ReadFile(filepath.Join(output, bookManifestName))
	writeFixture(t, output, "keep.txt", "unrelated")
	if err := runCLI([]string{"books", "--indir", source, "--outdir", output}, &out, &diagnostics); err != nil {
		t.Fatal(err, diagnostics.String())
	}
	again, _ := os.ReadFile(filepath.Join(output, bookManifestName))
	if !bytes.Equal(manifest, again) {
		t.Fatal("manifest is not reproducible")
	}
	again, _ = os.ReadFile(filepath.Join(output, "types/one.html"))
	if !bytes.Equal(page, again) {
		t.Fatal("page is not reproducible")
	}
	keep, _ := os.ReadFile(filepath.Join(output, "keep.txt"))
	if string(keep) != "unrelated" {
		t.Fatal("unrelated file lost")
	}
}

func TestBooksDeploymentURLsAndReadOnlyCheck(t *testing.T) {
	t.Parallel()
	source := bookCheckoutFixture(t)
	for _, base := range []string{"", "/books/", "https://example.com/reference/"} {
		var out, diag bytes.Buffer
		output := filepath.Join(t.TempDir(), "absent")
		args := []string{"books", "--indir", source, "--outdir", output, "--check"}
		if base != "" {
			args = append(args, "--base-url", base)
		}
		if err := runCLI(args, &out, &diag); err != nil {
			t.Fatal(base, err, diag.String())
		}
		if _, err := os.Stat(output); !os.IsNotExist(err) {
			t.Fatal("check created output")
		}
		if !strings.Contains(out.String(), "Validated 3 rulebooks") {
			t.Fatal(out.String())
		}
	}
}

func TestBooksValidationFailurePreservesDestination(t *testing.T) {
	t.Parallel()
	source := bookCheckoutFixture(t)
	output := t.TempDir()
	writeFixture(t, output, "keep.txt", "untouched")
	writeFixture(t, source, "rules/types/one.md", "# One\n\n[bad](../memory/two.md#missing)\n\n[absent](unknown.md)\n")
	var out, diag bytes.Buffer
	err := runCLI([]string{"books", "--indir", source, "--outdir", output}, &out, &diag)
	if err == nil || out.Len() != 0 || !strings.Contains(diag.String(), "missing anchor") || !strings.Contains(diag.String(), "unresolved local book") {
		t.Fatalf("missing actionable errors: %v, %s", err, diag.String())
	}
	entries, _ := os.ReadDir(output)
	if len(entries) != 1 || entries[0].Name() != "keep.txt" {
		t.Fatal("validation failure changed destination")
	}
}

func TestBooksResourcesAndHeadingCollisions(t *testing.T) {
	t.Parallel()
	source := bookCheckoutFixture(t)
	writeFixture(t, source, "rules/types/image.png", "image bytes")
	writeFixture(t, source, "rules/types/one.md", "# One\n\n![diagram](image.png)\n\n<a id=\"section\"></a>\n\n## Section\n")
	root, err := os.OpenRoot(source)
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	catalog, err := buildBookSite(&rulebookWorkspace{source: root}, "", time.Time{})
	if err != nil {
		t.Fatal(err)
	}
	// Replace the incoming link to a now-absent heading before checking this fixture.
	writeFixture(t, source, "rules/memory/two.md", "# Two\n")
	catalog, err = buildBookSite(&rulebookWorkspace{source: root}, "", time.Time{})
	if err != nil {
		t.Fatal(err)
	}
	validateBookArtifacts(catalog, "")
	if len(catalog.Diagnostics) > 0 {
		t.Fatal(catalog.Diagnostics)
	}
	if string(catalog.Artifacts["assets/types/image.png"]) != "image bytes" {
		t.Fatal("resource not copied")
	}
	page := catalog.Artifacts["types/one.html"]
	if !bytes.Contains(page, []byte(`src="../assets/types/image.png"`)) || !bytes.Contains(page, []byte(`id="section-1"`)) {
		t.Fatal("resource/explicit-anchor collision handling failed")
	}
	writeFixture(t, source, "rules/types/one.md", "# One\n\n<a id=\"twice\"></a><a id=\"twice\"></a>\n")
	if _, err := buildBookSite(&rulebookWorkspace{source: root}, "", time.Time{}); err == nil {
		t.Fatal("duplicate explicit anchors accepted")
	}
}

func TestBooksCLIRejectsInvalidRequests(t *testing.T) {
	t.Parallel()
	for _, args := range [][]string{{"books"}, {"books", "--indir", "", "--check"}, {"books", "unexpected"}, {"books", "--check", "--base-url", "javascript:evil"}, {"books", "--check", "--base-url", "relative/"}, {"books", "--check", "--base-url", "https://example.com/?q=1"}, {"books", "--file", "types.yaml"}} {
		var out, diag bytes.Buffer
		if err := runCLI(args, &out, &diag); err == nil {
			t.Fatal("invalid request accepted", args)
		}
	}
	var out, diag bytes.Buffer
	if err := runCLI([]string{"books", "--help"}, &out, &diag); err != nil || !strings.Contains(diag.String(), "--outdir") {
		t.Fatal("help unavailable", err)
	}
}

func TestBooksRealCorpus(t *testing.T) {
	if testing.Short() {
		t.Skip("repository corpus")
	}
	var out, diag bytes.Buffer
	if err := runCLI([]string{"books", "--indir", "../..", "--check"}, &out, &diag); err != nil {
		t.Fatal(err, diag.String())
	}
	if !strings.Contains(out.String(), "Validated") {
		t.Fatal(out.String())
	}
}

func TestBooksInventoryConsistencyFailures(t *testing.T) {
	t.Parallel()
	for _, kind := range []string{"missing-source", "unknown-category", "output-collision"} {
		t.Run(kind, func(t *testing.T) {
			source := bookCheckoutFixture(t)
			switch kind {
			case "missing-source":
				if err := os.Remove(filepath.Join(source, "rules/types/one.md")); err != nil {
					t.Fatal(err)
				}
			case "unknown-category":
				writeFixture(t, source, "rules/README.md", "| Directory | Scope |\n| - | - |\n| `memory/` | Memory |\n")
			case "output-collision":
				inventory, err := os.ReadFile(filepath.Join(source, "language-rulebook-status.md"))
				if err != nil {
					t.Fatal(err)
				}
				writeFixture(t, source, "language-rulebook-status.md", string(inventory)+"| `types/notes.md` | **Written** | collision |\n")
				writeFixture(t, source, "rules/types/notes.md", "# Notes\n")
			}
			var out, diag bytes.Buffer
			if err := runCLI([]string{"books", "--indir", source, "--check"}, &out, &diag); err == nil || out.Len() != 0 {
				t.Fatal("inconsistent inventory accepted", kind)
			}
		})
	}
}
