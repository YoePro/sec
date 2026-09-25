package main

import (
	"flag"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestFmtCheckCLI(t *testing.T) {
	canonical, err := os.ReadFile("../../testdata/formatter/check_canonical.sec")
	if err != nil {
		t.Fatal(err)
	}
	noncanonical, err := os.ReadFile("../../testdata/formatter/check_noncanonical.sec")
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	clean := filepath.Join(dir, "clean.sec")
	dirty := filepath.Join(dir, "dirty.sec")
	dirtyCRLF := filepath.Join(dir, "dirty crlf.sec")
	inputs := map[string]string{
		clean: string(canonical), dirty: string(noncanonical),
		dirtyCRLF: strings.ReplaceAll(string(noncanonical), "\n", "\r\n"),
	}
	before := map[string]os.FileInfo{}
	for path, input := range inputs {
		if err := os.WriteFile(path, []byte(input), 0o440); err != nil {
			t.Fatal(err)
		}
		before[path], err = os.Stat(path)
		if err != nil {
			t.Fatal(err)
		}
	}
	tests := []struct {
		name string
		args []string
		code int
		want []string
	}{
		{"canonical", []string{"--check", clean}, 0, nil},
		{"all changed files", []string{"--check", dirty, clean, dirtyCRLF}, 1, []string{dirty + ": format.noncanonical-source:", dirtyCRLF + ": format.noncanonical-source:"}},
		{"option after path", []string{dirty, "--check"}, 1, []string{dirty + ": format.noncanonical-source:"}},
		{"end of options", []string{"--check", "--", clean}, 0, nil},
		{"no files", []string{"--check"}, 1, []string{"expected at least one source file"}},
		{"unknown option", []string{dirty, "--fix"}, 1, []string{"unknown fmt option: --fix"}},
		{"missing file", []string{"--check", filepath.Join(dir, "missing.sec")}, 1, []string{"missing.sec"}},
		{"directory unsupported", []string{"--check", dir}, 1, []string{"not a regular source file"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			args := append([]string{"-test.run=^TestFmtCheckCLIProcess$", "--", "fmt"}, tt.args...)
			cmd := exec.Command(os.Args[0], args...)
			cmd.Env = append(os.Environ(), "SEC_FMT_CHECK_TEST_PROCESS=1")
			output, err := cmd.CombinedOutput()
			code := 0
			if err != nil {
				if exit, ok := err.(*exec.ExitError); ok {
					code = exit.ExitCode()
				} else {
					t.Fatal(err)
				}
			}
			if code != tt.code {
				t.Fatalf("exit code = %d, want %d; output: %s", code, tt.code, output)
			}
			if tt.code == 0 && len(output) != 0 {
				t.Fatalf("canonical check should be silent: %s", output)
			}
			for _, want := range tt.want {
				if !strings.Contains(string(output), want) {
					t.Errorf("output %q does not contain %q", output, want)
				}
			}
			if strings.Contains(string(output), "format.noncanonical-source") {
				if strings.Contains(string(output), clean) || strings.Contains(string(output), "fmt error:") {
					t.Errorf("formatting report includes a canonical file or an error label: %s", output)
				}
			}
			for path, input := range inputs {
				got, err := os.ReadFile(path)
				if err != nil {
					t.Fatal(err)
				}
				info, err := os.Stat(path)
				if err != nil {
					t.Fatal(err)
				}
				if string(got) != input || !os.SameFile(before[path], info) || !info.ModTime().Equal(before[path].ModTime()) || info.Mode() != before[path].Mode() {
					t.Errorf("check modified %s or its metadata", path)
				}
			}
		})
	}
}

func TestFmtCheckCLIProcess(t *testing.T) {
	if os.Getenv("SEC_FMT_CHECK_TEST_PROCESS") != "1" {
		return
	}
	os.Args = append(os.Args[:1], os.Args[3:]...)
	flag.CommandLine = flag.NewFlagSet("sec", flag.ExitOnError)
	main()
	os.Exit(0)
}

func TestFmtCheckAfterFormatting(t *testing.T) {
	input, err := os.ReadFile("../../testdata/formatter/check_noncanonical.sec")
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "source.sec")
	if err := os.WriteFile(path, input, 0o640); err != nil {
		t.Fatal(err)
	}
	if err := runFmtCommand([]string{"--check", path}); err == nil {
		t.Fatal("expected noncanonical source to fail check")
	}
	if err := runFmtCommand([]string{path}); err != nil {
		t.Fatal(err)
	}
	if err := runFmtCommand([]string{"--check", path}); err != nil {
		t.Fatalf("formatted source failed check: %v", err)
	}
}

// A byte-preserved malformed file is not falsely reported as canonically
// formatted merely because safe formatting declines to change it.
//
// Rules: rules/tooling/formatter.md — §25(9).
func TestFmtCheckRejectsPreservedMalformedSource(t *testing.T) {
	input := "fn Broken() void {\r\n\tlet values := [1, 2\r\n"
	path := filepath.Join(t.TempDir(), "broken.sec")
	if err := os.WriteFile(path, []byte(input), 0o640); err != nil {
		t.Fatal(err)
	}
	err := runFmtCommand([]string{"--check", path})
	if err == nil || !strings.Contains(err.Error(), path+": format.malformed-source:") {
		t.Fatalf("malformed check error = %v", err)
	}
	got, readErr := os.ReadFile(path)
	if readErr != nil {
		t.Fatal(readErr)
	}
	if string(got) != input {
		t.Fatalf("check changed malformed source: %q", got)
	}
}
