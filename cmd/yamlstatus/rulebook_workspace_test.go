package main

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
)

func newRulebookWorkspaceFixture(t *testing.T) (*rulebookWorkspace, string, string, string) {
	t.Helper()
	root := t.TempDir()
	source, output, outside := filepath.Join(root, "source"), filepath.Join(root, "output"), filepath.Join(root, "outside")
	for _, dir := range []string{source, output, outside} {
		if err := os.Mkdir(dir, 0755); err != nil {
			t.Fatal(err)
		}
	}
	w, err := openRulebookWorkspace(source, output)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := w.Close(); err != nil {
			t.Error(err)
		}
	})
	return w, source, output, outside
}

func TestRulebookWorkspaceRendersAndCreatesWithoutOverwriting(t *testing.T) {
	t.Parallel()
	w, source, output, _ := newRulebookWorkspaceFixture(t)
	text := "# Book\n\n<script>alert(1)</script>\n\n```sec\nfn A() void {}\n```"
	writeFixture(t, source, "memory/book.md", text)
	writeFixture(t, source, "images/λ image.bin", "image bytes")
	page, err := w.RenderPage(rulebookPageOptions{Title: `<img src=x onerror=alert(1)>`}, "memory/book.md")
	if err != nil || len(page) == 0 || bytes.Contains(page, []byte("<script>")) || bytes.Contains(page, []byte("<img src=x")) {
		t.Fatalf("confined rendering/HTML policy failed: %v, %s", err, page)
	}
	if err := w.WriteArtifact("memory/book.html", page); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(filepath.Join(output, "memory/book.html"))
	if err != nil || !bytes.Equal(got, page) {
		t.Fatalf("artifact differs: %v", err)
	}
	if err := w.WriteArtifact("memory/book.html", []byte("overwrite")); err == nil {
		t.Fatal("overwrote an existing artifact")
	}
	got, _ = os.ReadFile(filepath.Join(output, "memory/book.html"))
	if !bytes.Equal(got, page) {
		t.Fatal("existing artifact changed")
	}
	writeFixture(t, output, "keep.txt", "unrelated")
	if err := w.WriteArtifact("keep.txt", []byte("overwrite")); err == nil {
		t.Fatal("overwrote unrelated output")
	}
	got, _ = os.ReadFile(filepath.Join(output, "keep.txt"))
	if string(got) != "unrelated" {
		t.Fatal("unrelated output changed")
	}
	got, _ = os.ReadFile(filepath.Join(source, "memory/book.md"))
	if string(got) != text {
		t.Fatal("source book changed")
	}
	got, err = w.ReadSource("images/λ image.bin")
	if err != nil || string(got) != "image bytes" {
		t.Fatalf("local resource read failed: %s, %v", got, err)
	}
}

func TestRulebookWorkspaceRejectsTraversalAndURLInputs(t *testing.T) {
	t.Parallel()
	w, _, output, outside := newRulebookWorkspaceFixture(t)
	writeFixture(t, outside, "secret.txt", "untouched")
	for _, name := range []string{
		"", ".", "..", "../outside/secret.txt", "a/../../outside/secret.txt",
		"a/../book.md", "/etc/passwd", "a//book.md", "./book.md",
		`..\outside\secret.txt`, `C:\private\book.md`, "file:///private",
		"https://example.com/book.md", "%2e%2e/outside/secret.txt",
		"%252e%252e/outside/secret.txt", "a/%2f../secret.txt",
		"a\x00b", "a\nb", string([]byte{0xff}),
	} {
		t.Run(name, func(t *testing.T) {
			if got, err := w.ReadSource(name); err == nil || got != nil {
				t.Fatalf("unsafe source accepted: %s, %v", got, err)
			}
			if err := w.WriteArtifact(name, []byte("overwrite")); err == nil {
				t.Fatal("unsafe output accepted")
			}
		})
	}
	entries, err := os.ReadDir(output)
	if err != nil || len(entries) != 0 {
		t.Fatalf("invalid writes changed output root: %v, %v", entries, err)
	}
	got, _ := os.ReadFile(filepath.Join(outside, "secret.txt"))
	if string(got) != "untouched" {
		t.Fatal("outside data changed")
	}
}

func TestRulebookWorkspaceSymlinkBoundaries(t *testing.T) {
	t.Parallel()
	w, source, output, outside := newRulebookWorkspaceFixture(t)
	writeFixture(t, source, "book.md", "# Original")
	writeFixture(t, outside, "private.md", "# Private")
	if err := os.Symlink("book.md", filepath.Join(source, "local.md")); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	got, err := w.ReadSource("local.md")
	if err != nil || string(got) != "# Original" {
		t.Fatalf("confined source symlink failed: %s, %v", got, err)
	}
	for name, target := range map[string]string{
		"escape": "../outside", "absolute": outside,
	} {
		if err := os.Symlink(target, filepath.Join(source, name)); err != nil {
			t.Fatal(err)
		}
		if got, err := w.ReadSource(name + "/private.md"); err == nil || got != nil {
			t.Fatalf("read through escaping symlink %s", name)
		}
		if err := os.Symlink(target, filepath.Join(output, name)); err != nil {
			t.Fatal(err)
		}
		if err := w.WriteArtifact(name+"/new.html", []byte("escaped")); err == nil {
			t.Fatalf("write through escaping symlink %s", name)
		}
	}
	if err := os.Symlink("../source/book.md", filepath.Join(output, "source-link.html")); err != nil {
		t.Fatal(err)
	}
	if err := w.WriteArtifact("source-link.html", []byte("overwrite")); err == nil {
		t.Fatal("overwrote source via leaf symlink")
	}
	if err := os.Link(filepath.Join(source, "book.md"), filepath.Join(output, "hard-link.html")); err != nil {
		t.Fatal(err)
	}
	if err := w.WriteArtifact("hard-link.html", []byte("overwrite")); err == nil {
		t.Fatal("overwrote source via hard link")
	}
	got, _ = os.ReadFile(filepath.Join(source, "book.md"))
	if string(got) != "# Original" {
		t.Fatal("source changed through a link")
	}
	if _, err := os.Stat(filepath.Join(outside, "new.html")); !os.IsNotExist(err) {
		t.Fatalf("outside output created: %v", err)
	}
}

func TestRulebookWorkspaceRejectsOverlappingAndInvalidRoots(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	source := filepath.Join(root, "source")
	nested := filepath.Join(source, "nested")
	if err := os.MkdirAll(nested, 0755); err != nil {
		t.Fatal(err)
	}
	for _, pair := range [][2]string{{source, source}, {source, nested}, {nested, source}} {
		if w, err := openRulebookWorkspace(pair[0], pair[1]); err == nil || w != nil || !strings.Contains(err.Error(), "overlap") {
			t.Fatalf("overlapping roots accepted: %v", err)
		}
	}
	alias := filepath.Join(root, "alias")
	if err := os.Symlink(source, alias); err == nil {
		if w, err := openRulebookWorkspace(source, alias); err == nil || w != nil {
			t.Fatal("source/output aliases accepted")
		}
	}
	other := filepath.Join(root, "source-extra")
	if err := os.Mkdir(other, 0755); err != nil {
		t.Fatal(err)
	}
	w, err := openRulebookWorkspace(source, other)
	if err != nil {
		t.Fatalf("disjoint prefix sibling rejected: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	writeFixture(t, root, "not-directory", "file")
	for _, pair := range [][2]string{
		{source, filepath.Join(root, "missing")}, {filepath.Join(root, "missing"), other},
		{filepath.Join(root, "not-directory"), other}, {source, filepath.Join(root, "not-directory")},
	} {
		if w, err := openRulebookWorkspace(pair[0], pair[1]); err == nil || w != nil {
			t.Fatal("missing or non-directory root accepted")
		}
	}
}

func TestRulebookWorkspaceRenderingErrorsReturnNoPage(t *testing.T) {
	t.Parallel()
	w, source, output, _ := newRulebookWorkspaceFixture(t)
	writeFixture(t, source, "unsupported.yaml", "title: Book")
	writeFixture(t, source, "bad.md", string([]byte{0xff}))
	for _, name := range []string{"missing.md", "unsupported.yaml", "bad.md", "../secret.md"} {
		if page, err := w.RenderPage(rulebookPageOptions{Title: "Book"}, name); err == nil || page != nil {
			t.Errorf("invalid source produced page: %s, %v", page, err)
		}
	}
	if err := os.Mkdir(filepath.Join(source, "directory.md"), 0755); err != nil {
		t.Fatal(err)
	}
	if page, err := w.RenderPage(rulebookPageOptions{Title: "Book"}, "directory.md"); err == nil || page != nil {
		t.Fatal("directory treated as book")
	}
	entries, _ := os.ReadDir(output)
	if len(entries) != 0 {
		t.Fatal("rendering failure wrote output")
	}
}

func TestRulebookWorkspaceDoesNotFetchOrExecuteSource(t *testing.T) {
	t.Parallel()
	w, source, _, outside := newRulebookWorkspaceFixture(t)
	var requests atomic.Int64
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		requests.Add(1)
	}))
	defer server.Close()
	marker := filepath.Join(outside, "must-not-exist")
	text := "![remote image](" + server.URL + "/image.png)\n\n```sh\ntouch '" + marker + "'\n```"
	writeFixture(t, source, "book.md", text)
	page, err := w.RenderPage(rulebookPageOptions{Title: "Book"}, "book.md")
	if err != nil || !bytes.Contains(page, []byte(server.URL)) || !bytes.Contains(page, []byte("touch")) {
		t.Fatalf("source example was not preserved: %v", err)
	}
	if requests.Load() != 0 {
		t.Fatal("generation fetched a remote resource")
	}
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Fatalf("generation executed a source example: %v", err)
	}
}
