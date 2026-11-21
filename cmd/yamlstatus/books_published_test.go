package main

import (
	"bytes"
	"os"
	"strings"
	"testing"
	"time"
)

// The title page footer shows the publication time; other pages, and a build
// without a time (books --check), show none, so output stays deterministic for
// a given --published-at. Every page's logo links to https://sec-lang.com/.
func TestBooksPublicationTimeAndLogo(t *testing.T) {
	source := bookCheckoutFixture(t)
	root, err := os.OpenRoot(source)
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	published := time.Date(2026, 11, 21, 3, 15, 0, 0, time.FixedZone("CET", 3600))
	catalog, err := buildBookSite(&rulebookWorkspace{source: root}, "", published)
	if err != nil {
		t.Fatal(err)
	}
	index := catalog.Artifacts["index.html"]
	if !bytes.Contains(index, []byte(`Published <time datetime="2026-11-21T02:15:00Z">2026-11-21 02:15 UTC</time>`)) {
		t.Fatalf("title page footer lacks the UTC publication time:\n%s", index)
	}
	for name, page := range catalog.Artifacts {
		if !strings.HasSuffix(name, ".html") {
			continue
		}
		if name != "index.html" && bytes.Contains(page, []byte("Published <time")) {
			t.Fatalf("%s shows the publication time", name)
		}
		if !bytes.Contains(page, []byte(`<a class="site-logo" href="https://sec-lang.com/"`)) {
			t.Fatalf("%s logo does not link to https://sec-lang.com/", name)
		}
	}
	again, err := buildBookSite(&rulebookWorkspace{source: root}, "", published)
	if err != nil || !bytes.Equal(again.Artifacts["index.html"], index) {
		t.Fatal("the same publication time does not give the same title page")
	}
	unpublished, err := buildBookSite(&rulebookWorkspace{source: root}, "", time.Time{})
	if err != nil || bytes.Contains(unpublished.Artifacts["index.html"], []byte("Published")) {
		t.Fatal("a build without a publication time shows one")
	}
}

// --published-at must be RFC 3339.
func TestBooksRejectsInvalidPublishedAt(t *testing.T) {
	source := bookCheckoutFixture(t)
	var out, errOut bytes.Buffer
	if err := runBooks([]string{"--indir", source, "--check", "--published-at", "yesterday"}, &out, &errOut); err == nil || !strings.Contains(err.Error(), "--published-at") {
		t.Fatalf("err = %v", err)
	}
}
