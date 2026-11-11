package main

import (
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// A scoped test compilation imports only production sources of direct and
// transitive dependencies. Explicitly selected dependency tests remain eligible.
// Rules: rules/tooling/testing.md §§4.3–4.4,25,28.4.
func TestSecTestDependencyScope(t *testing.T) {
	root := filepath.Join(t.TempDir(), "app")
	fixture := filepath.Join("..", "..", "testdata", "projects", "test_dependency_scope")
	if err := filepath.WalkDir(fixture, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		relative, err := filepath.Rel(fixture, path)
		if err != nil {
			return err
		}
		dest := filepath.Join(root, relative)
		if entry.IsDir() {
			return os.MkdirAll(dest, 0755)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(dest, data, 0644)
	}); err != nil {
		t.Fatal(err)
	}
	// Isolate the compiler root from the repository's library source universe.
	if err := os.MkdirAll(filepath.Join(root, "sec", "platform"), 0755); err != nil {
		t.Fatal(err)
	}
	app := filepath.Join(root, "app_test.sec")
	dependency := filepath.Join(root, "dependency", "dependency_test.sec")
	for _, tc := range []struct {
		name           string
		args           []string
		modules, tests int
	}{
		{"file", []string{app}, 1, 1},
		{"explicit dependency", []string{app, dependency}, 2, 2},
		{"filter", []string{"--run", "selected app$", app}, 1, 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			args := append([]string{"test", "--list", "--json"}, tc.args...)
			stdout, stderr, code := runCLIForDiagnostics(t, args...)
			if code != testExitSuccess {
				t.Fatalf("exit %d:\n%s\n%s", code, stdout, stderr)
			}
			var report testRunReport
			if err := json.Unmarshal([]byte(stdout), &report); err != nil {
				t.Fatal(err, stdout)
			}
			count := 0
			for _, module := range report.Modules {
				if !module.Compiled {
					t.Fatalf("uncompiled module: %+v", module)
				}
				for _, test := range module.Tests {
					if test.File != app && test.File != dependency {
						t.Fatalf("unselected dependency test: %+v", test)
					}
					count++
				}
			}
			if len(report.Modules) != tc.modules || count != tc.tests {
				t.Fatalf("unexpected selection: %+v", report)
			}
		})
	}
	// Selection, rather than import reachability, decides whether invalid tests
	// are parsed/analyzed. Both parser and Sema failures must remain visible.
	for _, path := range []string{"dependency/broken_test.sec", "dependency/leaf/broken_test.sec", "sec/core/broken_test.sec", "sec/stdlib/scopefixture/broken_test.sec", "sec/platform/scopedep/broken_test.sec"} {
		t.Run(path, func(t *testing.T) {
			stdout, stderr, code := runCLIForDiagnostics(t, "test", "--list", "--json", filepath.Join(root, path))
			if code != testExitCompilationFailure {
				t.Fatalf("exit %d:\n%s\n%s", code, stdout, stderr)
			}
		})
	}
	// The same dependency exclusion also protects ordinary production checks.
	if stdout, stderr, code := runCLIForDiagnostics(t, "check", filepath.Join(root, "app.sec")); code != 0 {
		t.Fatalf("production check exit %d:\n%s\n%s", code, stdout, stderr)
	}
	// Test-only source must not make an otherwise absent production import valid.
	if _, stderr, code := runCLIForDiagnostics(t, "check", filepath.Join(root, "missingcase", "implementation.sec")); code != 3 || !strings.Contains(stderr, "S1051") {
		t.Fatalf("test-only import exit %d:\n%s", code, stderr)
	}
}
