package main

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"
	"unicode"
)

// rulebookWorkspace confines publication IO to disjoint source and staging
// roots. It renders pages without fetching resources or executing examples.
// Create-only writes suit the separately tracked staged site builder.
// This is documentation infrastructure, not a Sec language rule.
type rulebookWorkspace struct {
	source, output *os.Root
}

// openRulebookWorkspace requires existing directories. Resolve root aliases
// before overlap checks, then pin directory handles: os.Root enforces symlink
// confinement during operations rather than relying on a path-prefix check.
func openRulebookWorkspace(sourceDir, outputDir string) (*rulebookWorkspace, error) {
	resolve := func(name string) (string, error) {
		absolute, err := filepath.Abs(name)
		if err != nil {
			return "", err
		}
		return filepath.EvalSymlinks(absolute)
	}
	sourcePath, err := resolve(sourceDir)
	if err != nil {
		return nil, fmt.Errorf("open rulebook source root: %w", err)
	}
	outputPath, err := resolve(outputDir)
	if err != nil {
		return nil, fmt.Errorf("open rulebook output root: %w", err)
	}
	contains := func(parent, child string) bool {
		relative, err := filepath.Rel(parent, child)
		return err == nil && (relative == "." || (relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator))))
	}
	if contains(sourcePath, outputPath) || contains(outputPath, sourcePath) {
		return nil, fmt.Errorf("rulebook source and output roots must not overlap")
	}
	source, err := os.OpenRoot(sourcePath)
	if err != nil {
		return nil, fmt.Errorf("open rulebook source root: %w", err)
	}
	output, err := os.OpenRoot(outputPath)
	if err != nil {
		source.Close()
		return nil, fmt.Errorf("open rulebook output root: %w", err)
	}
	return &rulebookWorkspace{source: source, output: output}, nil
}

func (w *rulebookWorkspace) Close() error {
	return errors.Join(w.source.Close(), w.output.Close())
}

// rulebookRelativePath accepts decoded, portable filesystem paths, not URLs.
// Reject percent escapes instead of decoding/cleaning ambiguous input into a
// different target. Link resolution must supply the final decoded local path.
func rulebookRelativePath(name string) error {
	if !fs.ValidPath(name) || name == "." || strings.ContainsAny(name, `\:%`) {
		return fmt.Errorf("invalid relative rulebook path %q", name)
	}
	for _, character := range name {
		if unicode.IsControl(character) {
			return fmt.Errorf("invalid relative rulebook path %q", name)
		}
	}
	return nil
}

// ReadSource supplies local book/resource bytes. Regular files are required;
// relative symlinks are supported only when os.Root keeps their target inside
// the source root. A stat check rejects directories and special files.
func (w *rulebookWorkspace) ReadSource(name string) ([]byte, error) {
	if err := rulebookRelativePath(name); err != nil {
		return nil, err
	}
	info, err := w.source.Stat(name)
	if err != nil {
		return nil, fmt.Errorf("read rulebook %q: %w", name, err)
	}
	if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("read rulebook %q: source must be a regular file", name)
	}
	file, err := w.source.Open(name)
	if err != nil {
		return nil, fmt.Errorf("read rulebook %q: %w", name, err)
	}
	defer file.Close()
	// Recheck the opened handle rather than assuming the earlier stat still
	// describes this file when another process changes the directory.
	info, err = file.Stat()
	if err != nil || !info.Mode().IsRegular() {
		return nil, fmt.Errorf("read rulebook %q: source changed or is not a regular file", name)
	}
	data, err := io.ReadAll(file)
	if err != nil {
		return nil, fmt.Errorf("read rulebook %q: %w", name, err)
	}
	return data, nil
}

// RenderPage keeps both IO confinement and HTML sanitization on the same path.
func (w *rulebookWorkspace) RenderPage(options rulebookPageOptions, name string) ([]byte, error) {
	source, err := w.ReadSource(name)
	if err != nil {
		return nil, err
	}
	return renderRulebookPage(options, name, source)
}

// WriteArtifact writes only new files. It cannot truncate a previous artifact,
// an unrelated file or a symlink, including a hard link to a source file.
// The site builder must use a dedicated staging root for each generation.
func (w *rulebookWorkspace) WriteArtifact(name string, data []byte) (err error) {
	if err := rulebookRelativePath(name); err != nil {
		return err
	}
	if err := w.output.MkdirAll(path.Dir(name), 0755); err != nil {
		return fmt.Errorf("create rulebook artifact directory %q: %w", name, err)
	}
	file, err := w.output.OpenFile(name, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0644)
	if err != nil {
		return fmt.Errorf("create rulebook artifact %q: %w", name, err)
	}
	defer func() {
		if closeErr := file.Close(); err == nil {
			err = closeErr
		}
		if err != nil {
			err = errors.Join(err, w.output.Remove(name))
		}
	}()
	_, err = file.Write(data)
	return err
}
