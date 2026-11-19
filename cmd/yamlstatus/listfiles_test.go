package main

import (
	"bytes"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The report counts canonical checklist entries rather than integration status,
// includes empty/metadata and library files, and leaves source files unchanged.
func TestListFilesCLI(t *testing.T) {
	dir := t.TempDir()
	fixtures := map[string]string{
		"index.yaml":       "fragments:\n  - path: stdlib.yaml\n    implementation_scope: library\n",
		"a.yaml":           "integrations:\n  - id: a\n    status: planned\n    implemented: [a, {id: b, description: done}]\n    remaining: [c]\n    partial: [d]\n",
		"z.yaml":           "id: z\nimplemented: [a]\n---\n- id: zz\n  partial: [b]\n",
		"nested/extra.YML": "- id: nested\n  remaining: [a, b, c]\n",
		"empty.yaml":       "",
		"stdlib.yaml":      "integrations:\n  - id: library\n    status: implemented\n    remaining: [a, b]\n    partial: [c]\n",
		"README.md":        "not YAML",
	}
	for name, contents := range fixtures {
		writeFixture(t, dir, name, contents)
	}
	var out, errOut bytes.Buffer
	if err := runCLI([]string{"--listfiles", "-dir", dir}, &out, &errOut); err != nil {
		t.Fatal(err, errOut.String())
	}
	var lines []string
	for _, line := range strings.Split(strings.TrimSpace(out.String()), "\n") {
		lines = append(lines, strings.Join(strings.Fields(line), " "))
	}
	want := []string{
		"File Implemented Remaining Partial",
		"---- ----------- ----------- ----------",
		"a.yaml 2 (50.0%) 1 (25.0%) 1 (25.0%)",
		"empty.yaml 0 (0.0%) 0 (0.0%) 0 (0.0%)",
		"index.yaml 0 (0.0%) 0 (0.0%) 0 (0.0%)",
		"nested/extra.YML 0 (0.0%) 3 (100.0%) 0 (0.0%)",
		"stdlib.yaml [library] 0 (0.0%) 2 (66.7%) 1 (33.3%)",
		"z.yaml 1 (50.0%) 0 (0.0%) 1 (50.0%)",
		"",
		"[library] Files are excluded from compiler implementation totals.",
	}
	if strings.Join(lines, "\n") != strings.Join(want, "\n") {
		t.Fatalf("unexpected table:\n%s", out.String())
	}
	var again bytes.Buffer
	if err := runCLI([]string{"-listfiles", dir}, &again, &errOut); err != nil || again.String() != out.String() {
		t.Fatalf("directory form or output ordering differs: %v\n%s", err, again.String())
	}
	for name, contents := range fixtures {
		data, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil || string(data) != contents {
			t.Fatalf("input %s changed: %v", name, err)
		}
	}
	stats, err := collectStats(dir)
	if err != nil || stats.TotalImplemented != 3 || stats.TotalRemaining != 4 || stats.TotalPartial != 2 || stats.LibraryObjects != 1 {
		t.Fatalf("compiler scope changed: %+v %v", stats, err)
	}
}

func TestListFilesEmptyAndInvalidInput(t *testing.T) {
	dir := t.TempDir()
	var out bytes.Buffer
	if err := runCLI([]string{"--listfiles", dir}, &out, io.Discard); err != nil || out.String() != "No YAML files found.\n" {
		t.Fatalf("empty report: %v %q", err, out.String())
	}
	writeFixture(t, dir, "a.yaml", "id: good\nremaining: [x]\n")
	writeFixture(t, dir, "z.yaml", "integrations: [")
	out.Reset()
	if err := runCLI([]string{"--listfiles", dir}, &out, io.Discard); err == nil || !strings.Contains(err.Error(), "z.yaml") || out.Len() != 0 {
		t.Fatalf("malformed input produced a partial report: %v %s", err, out.String())
	}
	out.Reset()
	if err := runCLI([]string{"--listfiles", filepath.Join(dir, "missing")}, &out, io.Discard); err == nil || out.Len() != 0 {
		t.Fatalf("missing input accepted: %v %s", err, out.String())
	}
}

func TestListFilesRejectsConflictingModes(t *testing.T) {
	for _, options := range [][]string{
		{"-file", "core.yaml"}, {"-id", "x"}, {"-random"}, {"-point", "1"},
		{"-json"}, {"-html"}, {"-verbose"}, {"-itemonly"}, {"-paths"}, {"-go"},
		{"-exclude", "bootstrap"}, {"-grep", ""}, {"-uses", ""},
		{"-snapshot", "unused.json"}, {"-diff", "HEAD"},
	} {
		t.Run(options[0], func(t *testing.T) {
			var out bytes.Buffer
			args := append([]string{"--listfiles"}, options...)
			if err := runCLI(args, &out, io.Discard); err == nil || !strings.Contains(err.Error(), "cannot be combined") || out.Len() != 0 {
				t.Fatalf("conflicting options accepted: %v %s", err, out.String())
			}
		})
	}
}

type listFilesFailingWriter struct{ err error }

func (w listFilesFailingWriter) Write([]byte) (int, error) { return 0, w.err }

func TestListFilesOutputFailure(t *testing.T) {
	want := errors.New("output unavailable")
	err := printFileChecklists(listFilesFailingWriter{want}, []fileChecklistCounts{{File: "a.yaml", Remaining: 1}})
	if !errors.Is(err, want) {
		t.Fatalf("output failure lost: %v", err)
	}
}
