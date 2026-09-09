package main

import (
	"bytes"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"testing/iotest"
)

func TestFmtStdinCLI(t *testing.T) {
	input, err := os.ReadFile("../../testdata/formatter/check_noncanonical.sec")
	if err != nil {
		t.Fatal(err)
	}
	canonical, err := os.ReadFile("../../testdata/formatter/check_canonical.sec")
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "unchanged.sec")
	if err := os.WriteFile(path, input, 0o640); err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name, input, want string
		args              []string
		wantError         string
	}{
		{name: "LF", input: string(input), want: string(canonical)},
		{name: "CRLF becomes LF", input: strings.ReplaceAll(string(input), "\n", "\r\n"), want: string(canonical)},
		{name: "mixed endings become LF", input: strings.Replace(string(input), "\n", "\r\n", 1), want: string(canonical)},
		{name: "CR becomes LF", input: strings.ReplaceAll(string(input), "\n", "\r"), want: string(canonical)},
		{name: "empty"},
		{name: "idempotent", input: string(canonical), want: string(canonical)},
		{name: "end of options", input: string(input), want: string(canonical), args: []string{"--"}},
		{name: "check rejected", args: []string{"--check"}, wantError: "--stdin cannot be combined"},
		{name: "file rejected", args: []string{path}, wantError: "--stdin cannot be combined"},
		{name: "fix rejected", args: []string{"--fix"}, wantError: "unknown fmt option"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			args := append([]string{"-test.run=^TestFmtCheckCLIProcess$", "--", "fmt", "--stdin"}, tt.args...)
			cmd := exec.Command(os.Args[0], args...)
			cmd.Env = append(os.Environ(), "SEC_FMT_CHECK_TEST_PROCESS=1")
			cmd.Stdin = strings.NewReader(tt.input)
			var stdout, stderr bytes.Buffer
			cmd.Stdout, cmd.Stderr = &stdout, &stderr
			err := cmd.Run()
			if tt.wantError == "" {
				if err != nil || stderr.Len() != 0 {
					t.Fatalf("run: %v; stderr: %s", err, &stderr)
				}
			} else {
				var exit *exec.ExitError
				if !errors.As(err, &exit) || exit.ExitCode() != 1 || !strings.Contains(stderr.String(), tt.wantError) {
					t.Fatalf("wanted exit 1 and %q, got %v; stderr: %s", tt.wantError, err, &stderr)
				}
			}
			if stdout.String() != tt.want {
				t.Errorf("stdout = %q, want %q", stdout.String(), tt.want)
			}
			got, err := os.ReadFile(path)
			if err != nil || !bytes.Equal(got, input) {
				t.Fatalf("source file changed: %q; error: %v", got, err)
			}
		})
	}
}

func TestFormatStdinIOErrors(t *testing.T) {
	var output bytes.Buffer
	if err := formatStdin(iotest.ErrReader(io.ErrUnexpectedEOF), &output); !errors.Is(err, io.ErrUnexpectedEOF) || !strings.Contains(err.Error(), "read stdin") {
		t.Fatalf("read error not propagated: %v", err)
	}
	if output.Len() != 0 {
		t.Fatal("read failure wrote partial output")
	}
	input, err := os.ReadFile("../../testdata/formatter/check_canonical.sec")
	if err != nil {
		t.Fatal(err)
	}
	for _, failure := range []error{io.ErrClosedPipe, nil} {
		want := failure
		if want == nil {
			want = io.ErrShortWrite
		}
		err := formatStdin(bytes.NewReader(input), fmtStdinFailWriter{failure})
		if !errors.Is(err, want) || !strings.Contains(err.Error(), "write stdout") {
			t.Errorf("write error = %v, want %v", err, want)
		}
	}
}

type fmtStdinFailWriter struct{ err error }

func (w fmtStdinFailWriter) Write(p []byte) (int, error) { return 0, w.err }
