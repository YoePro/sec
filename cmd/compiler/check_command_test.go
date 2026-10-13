package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// Rules: rules/compiler/compiler_pipeline.md — §56(1–3).
func TestCheckCommandWithoutBackendTools(t *testing.T) {
	dir := t.TempDir()
	data, err := os.ReadFile("../../testdata/tooling/emission_boundary.sec")
	if err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(dir, "main.sec")
	if err := os.WriteFile(file, data, 0644); err != nil {
		t.Fatal(err)
	}
	for _, inputs := range [][]string{{file}, {dir}, {filepath.Join(dir, "*.sec")}} {
		for _, target := range []string{"linux-amd64", "linux-armv7"} {
			args := append([]string{"check", "--target", target}, inputs...)
			stdout, stderr, code := runCheckWithoutTools(t, args...)
			if code != 0 || !strings.Contains(stdout, "OK") || !strings.Contains(stderr, "0 errors") {
				t.Fatalf("%v: code %d; %s%s", args, code, stdout, stderr)
			}
		}
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].Name() != "main.sec" {
		t.Fatalf("check produced artifacts: %v", entries)
	}
	after, err := os.ReadFile(file)
	if err != nil || !bytes.Equal(data, after) {
		t.Fatal("check changed source", err)
	}
}

// runCheckWithoutTools invokes the real CLI dispatcher with no executable
// search path, ensuring backend discovery/verification cannot accidentally run.
func runCheckWithoutTools(t *testing.T, args ...string) (string, string, int) {
	t.Helper()
	process := exec.Command(os.Args[0], append([]string{"-test.run=^TestLexerCLIProcess$", "--"}, args...)...)
	process.Env = append(os.Environ(), "SEC_LEXER_CLI_TEST_PROCESS=1", "PATH="+t.TempDir())
	var stdout, stderr bytes.Buffer
	process.Stdout, process.Stderr = &stdout, &stderr
	code := 0
	if err := process.Run(); err != nil {
		if exit, ok := err.(*exec.ExitError); ok {
			code = exit.ExitCode()
		} else {
			t.Fatal(err)
		}
	}
	return stdout.String(), stderr.String(), code
}

// Rules: rules/compiler/compiler.md — §65(2); compiler_pipeline.md — §56(1).
func TestCheckRetainsTargetAndMandatoryDiagnostics(t *testing.T) {
	width := "../../testdata/tooling/check_target_width.sec"
	_, stderr, code := runCheckWithoutTools(t, "check", width, "--target=linux-amd64")
	if code != 0 {
		t.Fatal(stderr)
	}
	_, stderr, code = runCheckWithoutTools(t, "check", "--target", "linux-armv7", width)
	if code != 3 || !strings.Contains(stderr, "4294967296 overflows uint") {
		t.Fatal(code, stderr)
	}
	for _, fixture := range []string{"check_parse_invalid.sec", "ownership_call_mentor_invalid.sec", "borrow_authority_invalid.sec"} {
		path := "../../testdata/tooling/" + fixture
		_, check, checkCode := runCheckWithoutTools(t, "check", path, "--diagnostic-format=json")
		_, baseline, baselineCode := runCLIForDiagnostics(t, "sema", path, "--diagnostic-format=json")
		if checkCode == 0 || checkCode != baselineCode {
			t.Fatalf("%s: check %d, sema %d; %s", fixture, checkCode, baselineCode, check)
		}
		checked, expected := decodeOccurrenceDocument(t, check), decodeOccurrenceDocument(t, baseline)
		if !reflect.DeepEqual(checked, expected) {
			t.Fatalf("check lost shared diagnostics for %s:\n%s\n%s", fixture, check, baseline)
		}
	}
}

func TestCheckCommandUsageErrors(t *testing.T) {
	for _, args := range [][]string{nil, {"--target"}, {"--target", "bogus"}, {"--clang", "clang"}, {"-o", "artifact"}} {
		_, stderr, code := runCheckWithoutTools(t, append([]string{"check"}, args...)...)
		if code != 1 || !strings.Contains(stderr, "usage") {
			t.Fatal(args, code, stderr)
		}
	}
	got, err := parseCheckCommandArgs([]string{"--", "-source.sec"}, hostCompilerTarget())
	if err != nil || !reflect.DeepEqual(got.inputs, []string{"-source.sec"}) {
		t.Fatal(got, err)
	}
}
