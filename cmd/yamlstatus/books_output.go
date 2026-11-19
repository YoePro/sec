package main

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/gofrs/flock"
)

const bookManifestName = ".sec-rulebooks.json"

type bookManifest struct {
	Version   int               `json:"version"`
	Generator string            `json:"generator"`
	Files     map[string]string `json:"files"`
}

func bookHash(data []byte) string { sum := sha256.Sum256(data); return hex.EncodeToString(sum[:]) }

// bookDestination resolves the existing parent prefix of a possibly new path.
// Publication is restricted to an output tree outside the source checkout.
func bookDestination(sourceDir, outputDir string) (string, error) {
	source, err := filepath.Abs(sourceDir)
	if err != nil {
		return "", err
	}
	source, err = filepath.EvalSymlinks(source)
	if err != nil {
		return "", err
	}
	output, err := filepath.Abs(outputDir)
	if err != nil {
		return "", err
	}
	if info, err := os.Lstat(output); err == nil && info.Mode()&os.ModeSymlink != 0 {
		return "", fmt.Errorf("output root must not be a symlink")
	}
	ancestor := output
	var tail []string
	for {
		_, err := os.Stat(ancestor)
		if err == nil {
			break
		}
		if !os.IsNotExist(err) {
			return "", err
		}
		tail = append(tail, filepath.Base(ancestor))
		next := filepath.Dir(ancestor)
		if next == ancestor {
			return "", err
		}
		ancestor = next
	}
	ancestor, err = filepath.EvalSymlinks(ancestor)
	if err != nil {
		return "", err
	}
	for i := len(tail) - 1; i >= 0; i-- {
		ancestor = filepath.Join(ancestor, tail[i])
	}
	output = ancestor
	contains := func(a, b string) bool {
		rel, err := filepath.Rel(a, b)
		return err == nil && (rel == "." || (rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))))
	}
	if contains(source, output) || contains(output, source) {
		return "", fmt.Errorf("output directory must be outside the source checkout and must not contain it")
	}
	if filepath.Dir(output) == output {
		return "", fmt.Errorf("cannot publish to a filesystem root")
	}
	return output, nil
}

// publishBookArtifacts stages a complete deterministic site, preserving files
// outside the previous hash manifest. It swaps whole directory trees, with
// rollback if installation fails. A per-destination lock rejects overlapping
// cron runs. No source files are modified or hosting services contacted.
func publishBookArtifacts(sourceDir, outputDir string, artifacts map[string][]byte) error {
	output, err := bookDestination(sourceDir, outputDir)
	if err != nil {
		return err
	}
	for name := range artifacts {
		if err := rulebookRelativePath(name); err != nil {
			return err
		}
		if name == bookManifestName {
			return fmt.Errorf("reserved output path %s", name)
		}
	}
	parentPath := filepath.Dir(output)
	base := filepath.Base(output)
	if err := os.MkdirAll(parentPath, 0755); err != nil {
		return err
	}
	lock := flock.New(filepath.Join(parentPath, "."+base+".sec-books.lock"))
	locked, err := lock.TryLock()
	if err != nil {
		return err
	}
	if !locked {
		return fmt.Errorf("another books generation is publishing to this destination")
	}
	defer lock.Close()
	parent, err := os.OpenRoot(parentPath)
	if err != nil {
		return err
	}
	defer parent.Close()
	stageName := ".sec-books-stage-" + rand.Text()
	if err := parent.Mkdir(stageName, 0700); err != nil {
		return err
	}
	defer parent.RemoveAll(stageName)
	stage, err := parent.OpenRoot(stageName)
	if err != nil {
		return err
	}
	defer stage.Close()
	writer := &rulebookWorkspace{output: stage}
	previous := bookManifest{Files: map[string]string{}}
	directories := map[string]os.FileInfo{}
	existing, err := parent.OpenRoot(base)
	hasExisting := err == nil
	outputMode := os.FileMode(0755)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	if hasExisting {
		defer existing.Close()
		info, err := existing.Stat(".")
		if err != nil {
			return err
		}
		outputMode = info.Mode().Perm()
		if info, e := existing.Lstat(bookManifestName); e == nil {
			if !info.Mode().IsRegular() {
				return fmt.Errorf("invalid ownership manifest")
			}
			data, e := existing.ReadFile(bookManifestName)
			if e != nil {
				return e
			}
			if e = json.Unmarshal(data, &previous); e != nil || previous.Version != 1 || previous.Generator != "sec-yamlstatus-books" || previous.Files == nil {
				return fmt.Errorf("invalid ownership manifest")
			}
			for name, hash := range previous.Files {
				if e := rulebookRelativePath(name); e != nil {
					return e
				}
				decoded, e := hex.DecodeString(hash)
				if e != nil || len(decoded) != sha256.Size || name == bookManifestName {
					return fmt.Errorf("invalid ownership manifest entry %q", name)
				}
			}
		} else if !os.IsNotExist(e) {
			return e
		}
		err = fs.WalkDir(existing.FS(), ".", func(name string, entry fs.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if entry.IsDir() {
				if name != "." {
					info, err := entry.Info()
					if err != nil {
						return err
					}
					directories[name] = info
					return stage.MkdirAll(name, 0755)
				}
				return nil
			}
			if entry.Type()&os.ModeSymlink != 0 {
				return fmt.Errorf("output contains a symlink: %s", name)
			}
			info, err := entry.Info()
			if err != nil {
				return err
			}
			if !info.Mode().IsRegular() {
				return fmt.Errorf("output contains a non-regular file: %s", name)
			}
			if name == bookManifestName {
				return nil
			}
			data, err := existing.ReadFile(name)
			if err != nil {
				return err
			}
			if hash, owned := previous.Files[name]; owned {
				if bookHash(data) != hash {
					return fmt.Errorf("generated file was modified outside yamlstatus: %s", name)
				}
				return nil
			}
			if _, collision := artifacts[name]; collision {
				return fmt.Errorf("refusing to overwrite unrelated output file: %s", name)
			}
			if err := writer.WriteArtifact(name, data); err != nil {
				return err
			}
			if err := stage.Chmod(name, info.Mode().Perm()); err != nil {
				return err
			}
			return stage.Chtimes(name, info.ModTime(), info.ModTime())
		})
		if err != nil {
			return err
		}
		if err := existing.Close(); err != nil {
			return err
		}
	}
	manifest := bookManifest{Version: 1, Generator: "sec-yamlstatus-books", Files: map[string]string{}}
	names := make([]string, 0, len(artifacts))
	for name := range artifacts {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		if err := writer.WriteArtifact(name, artifacts[name]); err != nil {
			return err
		}
		manifest.Files[name] = bookHash(artifacts[name])
	}
	data, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	if err := writer.WriteArtifact(bookManifestName, data); err != nil {
		return err
	}
	directoryNames := make([]string, 0, len(directories))
	for name := range directories {
		directoryNames = append(directoryNames, name)
	}
	sort.Slice(directoryNames, func(i, j int) bool { return len(directoryNames[i]) > len(directoryNames[j]) })
	for _, name := range directoryNames {
		info := directories[name]
		if err := stage.Chmod(name, info.Mode().Perm()); err != nil {
			return err
		}
		if err := stage.Chtimes(name, info.ModTime(), info.ModTime()); err != nil {
			return err
		}
	}
	if err := stage.Chmod(".", outputMode); err != nil {
		return err
	}
	if err := stage.Close(); err != nil {
		return err
	}
	return installBookStage(parent, stageName, base, hasExisting, parent.Rename)
}

// The rename argument permits a real-filesystem installation-failure regression.
func installBookStage(parent *os.Root, stageName, base string, hasExisting bool, rename func(string, string) error) error {
	if !hasExisting {
		return rename(stageName, base)
	}
	backup := ".sec-books-backup-" + rand.Text()
	if err := rename(base, backup); err != nil {
		return err
	}
	if err := rename(stageName, base); err != nil {
		if rollback := rename(backup, base); rollback != nil {
			return fmt.Errorf("installation failed: %v; rollback failed: %v (old site retained at %s)", err, rollback, backup)
		}
		return err
	}
	if err := parent.RemoveAll(backup); err != nil {
		return fmt.Errorf("site published, but backup cleanup failed: %w", err)
	}
	return nil
}
