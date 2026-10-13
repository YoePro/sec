package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Rules: rules/projects/projects.md — §§28.1(3),28.2(2),48(1–2).
func TestBuildAndInitRejectInvalidCounterStorage(t *testing.T) {
	fixture, err := os.ReadFile("../../testdata/projects/model/sec.toml")
	if err != nil {
		t.Fatal(err)
	}
	source, err := os.ReadFile("../../testdata/projects/init/main.sec")
	if err != nil {
		t.Fatal(err)
	}
	for _, declaration := range []string{
		"[target.library.build_counter]\nenabled=true\n",
		"[target.sec.build_counter]\nenabled=true\nscope='project'\n",
	} {
		t.Run(strings.Split(declaration, "\n")[0], func(t *testing.T) {
			root := t.TempDir()
			if err := os.MkdirAll(filepath.Join(root, ".sec"), 0755); err != nil {
				t.Fatal(err)
			}
			directory := filepath.Join(root, "cmd", "sec")
			if err := os.MkdirAll(directory, 0755); err != nil {
				t.Fatal(err)
			}
			input := filepath.Join(directory, "main.sec")
			if err := os.WriteFile(input, source, 0644); err != nil {
				t.Fatal(err)
			}
			path := filepath.Join(root, ".sec", "sec.toml")
			text := string(fixture) + "\n" + declaration
			if err := os.WriteFile(path, []byte(text), 0644); err != nil {
				t.Fatal(err)
			}
			output := filepath.Join(root, "output")
			// Counter validation happens before any backend/tool invocation.
			_, stderr, code := runCLIForDiagnostics(t, "build", input, "-o", output, "--clang", "missing-clang")
			if code != 1 || !strings.Contains(stderr, "build_counter") || !strings.Contains(stderr, path+":") || strings.Contains(stderr, "clang not found") {
				t.Fatal(code, stderr)
			}
			if _, err := os.Stat(output); !os.IsNotExist(err) {
				t.Fatal("invalid counter produced output", err)
			}
			// Existing-project init and Target additions use the same validator.
			for _, args := range [][]string{{root, "--name", "Existing"}, {"library", "additional"}} {
				output, err := runInitCLIInDirectory(t, root, args...)
				if err == nil || !strings.Contains(output, "build_counter") {
					t.Fatal(args, err, output)
				}
			}
			data, err := os.ReadFile(path)
			if err != nil || string(data) != text {
				t.Fatal("rejected commands changed the manifest", err)
			}
			if _, err := os.Stat(filepath.Join(root, "additional")); !os.IsNotExist(err) {
				t.Fatal("rejected init created files", err)
			}
		})
	}
}

// Rules: rules/projects/projects.md — §§28,48(1–2).
func TestBuildCounterValidationRecognizesOnlyDeclarations(t *testing.T) {
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, ".sec"), 0755); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile("../../testdata/projects/model/sec.toml")
	if err != nil {
		t.Fatal(err)
	}
	text := string(data) + "\n[parameters]\ntext='''\n[target.library.build_counter]\nenabled=true\n'''\n"
	path := filepath.Join(root, ".sec", "sec.toml")
	for _, appendix := range []string{"", "\n[target.sec.build_counter]\nenabled=true\n", "\n[target.library.build_counter]\nenabled=false\n"} {
		if err := os.WriteFile(path, []byte(text+appendix), 0644); err != nil {
			t.Fatal(err)
		}
		if err := validateBuildCounterManifest(filepath.Join(root, "cmd", "sec", "main.sec")); err != nil {
			t.Fatal(err)
		}
	}
}
