package main

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gofrs/flock"
)

func TestBooksOutputStaleCleanupAndUnrelatedFiles(t *testing.T) {
	t.Parallel()
	source := t.TempDir()
	output := filepath.Join(t.TempDir(), "site")
	first := map[string][]byte{"index.html": []byte("index"), "old/book.html": []byte("old")}
	if err := publishBookArtifacts(source, output, first); err != nil {
		t.Fatal(err)
	}
	writeFixture(t, output, "private.txt", "unrelated")
	writeFixture(t, output, "private-directory/keep.txt", "private directory")
	if err := os.Chmod(filepath.Join(output, "private-directory"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(filepath.Join(output, "private.txt"), 0600); err != nil {
		t.Fatal(err)
	}
	before, _ := os.Stat(filepath.Join(output, "private.txt"))
	second := map[string][]byte{"index.html": []byte("updated"), "new/book.html": []byte("new")}
	if err := publishBookArtifacts(source, output, second); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(output, "old/book.html")); !os.IsNotExist(err) {
		t.Fatal("stale generated file remains")
	}
	for name, data := range second {
		got, err := os.ReadFile(filepath.Join(output, name))
		if err != nil || !bytes.Equal(got, data) {
			t.Fatal("artifact differs", name, err)
		}
	}
	got, _ := os.ReadFile(filepath.Join(output, "private.txt"))
	after, _ := os.Stat(filepath.Join(output, "private.txt"))
	if string(got) != "unrelated" || after.Mode().Perm() != 0600 || !after.ModTime().Equal(before.ModTime()) {
		t.Fatal("unrelated data, permissions or timestamp changed")
	}
	dirInfo, err := os.Stat(filepath.Join(output, "private-directory"))
	if err != nil || dirInfo.Mode().Perm() != 0700 {
		t.Fatal("unrelated directory permissions changed", err)
	}
}

func TestBooksOutputFailuresPreserveOldSite(t *testing.T) {
	t.Parallel()
	for _, failure := range []string{"modified", "unrelated", "bad-manifest", "bad-artifact", "symlink"} {
		t.Run(failure, func(t *testing.T) {
			source := t.TempDir()
			output := filepath.Join(t.TempDir(), "site")
			if err := publishBookArtifacts(source, output, map[string][]byte{"index.html": []byte("old")}); err != nil {
				t.Fatal(err)
			}
			artifacts := map[string][]byte{"index.html": []byte("new")}
			switch failure {
			case "modified":
				writeFixture(t, output, "index.html", "manual modification")
			case "unrelated":
				writeFixture(t, output, "keep.txt", "private")
				artifacts["keep.txt"] = []byte("overwrite")
			case "bad-manifest":
				writeFixture(t, output, bookManifestName, "invalid json")
			case "bad-artifact":
				artifacts["../escape.html"] = []byte("escape")
			case "symlink":
				if err := os.Symlink(source, filepath.Join(output, "escape")); err != nil {
					t.Skip(err)
				}
			}
			before, _ := os.ReadFile(filepath.Join(output, "index.html"))
			manifest, _ := os.ReadFile(filepath.Join(output, bookManifestName))
			if err := publishBookArtifacts(source, output, artifacts); err == nil {
				t.Fatal("unsafe/failed build accepted")
			}
			after, _ := os.ReadFile(filepath.Join(output, "index.html"))
			afterManifest, _ := os.ReadFile(filepath.Join(output, bookManifestName))
			if !bytes.Equal(before, after) || !bytes.Equal(manifest, afterManifest) {
				t.Fatal("failure modified the installed site")
			}
			entries, _ := os.ReadDir(filepath.Dir(output))
			for _, entry := range entries {
				if strings.HasPrefix(entry.Name(), ".sec-books-stage-") {
					t.Fatal("failed staging directory leaked")
				}
			}
		})
	}
}

func TestBooksOutputLockAndRootProtection(t *testing.T) {
	t.Parallel()
	source := t.TempDir()
	parent := t.TempDir()
	output := filepath.Join(parent, "site")
	lock := flock.New(filepath.Join(parent, ".site.sec-books.lock"))
	if err := lock.Lock(); err != nil {
		t.Fatal(err)
	}
	defer lock.Close()
	if err := publishBookArtifacts(source, output, map[string][]byte{"index.html": []byte("x")}); err == nil || !strings.Contains(err.Error(), "another books generation") {
		t.Fatal("concurrent publisher accepted", err)
	}
	if _, err := os.Stat(output); !os.IsNotExist(err) {
		t.Fatal("locked destination created")
	}
	for _, destination := range []string{source, filepath.Join(source, "out"), filepath.Dir(source)} {
		if err := publishBookArtifacts(source, destination, map[string][]byte{"index.html": []byte("x")}); err == nil {
			t.Fatal("overlapping destination accepted")
		}
	}
}

func TestBooksOutputInstallationRollback(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeFixture(t, root, "site/index.html", "old complete site")
	writeFixture(t, root, "stage/index.html", "new complete site")
	parent, err := os.OpenRoot(root)
	if err != nil {
		t.Fatal(err)
	}
	defer parent.Close()
	calls := 0
	err = installBookStage(parent, "stage", "site", true, func(from, to string) error {
		calls++
		if calls == 2 {
			return errors.New("injected installation failure")
		}
		return parent.Rename(from, to)
	})
	if err == nil || calls != 3 {
		t.Fatal("installation failure did not trigger rollback", err, calls)
	}
	got, _ := os.ReadFile(filepath.Join(root, "site/index.html"))
	if string(got) != "old complete site" {
		t.Fatal("old site not restored")
	}
}
