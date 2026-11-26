package linking

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"sec/internal/diagnostics"
)

// TestLinkDriverProcess acts as a real subprocess without a shell or host linker.
// This environment switch is confined to the test executable.
func TestLinkDriverProcess(t *testing.T) {
	args := os.Args
	for len(args) > 0 && args[0] != "--" {
		args = args[1:]
	}
	if len(args) < 2 {
		return
	}
	mode := args[1]
	output := args[len(args)-1]
	switch mode {
	case "success":
		os.WriteFile(output, []byte("linked fixture"), 0700)
		fmt.Fprint(os.Stdout, "driver stdout")
		fmt.Fprint(os.Stderr, "driver warning")
	case "empty":
		os.WriteFile(output, nil, 0700)
	case "directory":
		os.Mkdir(output, 0700)
	case "symlink":
		os.Symlink("outside", output)
	case "failure":
		os.WriteFile(output, []byte("incomplete artifact"), 0700)
		fmt.Fprint(os.Stderr, "undefined native symbol")
		os.Exit(7)
	case "crash":
		process, _ := os.FindProcess(os.Getpid())
		process.Kill()
		os.Exit(99)
	case "missing":
		fmt.Fprint(os.Stderr, "undefined symbol text with successful exit")
	default:
		os.Exit(2)
	}
	os.Exit(0)
}

// Failure categories reflect operation/process status, and rejected candidates
// never overwrite prior output or leak their staging file.
// Rules: rules/compiler/linking.md §§50-51,64.
func TestLinkFailureClassesAndPublication(t *testing.T) {
	for _, mode := range []string{"success", "failure", "crash", "missing", "empty", "directory", "symlink", "unknown-tool", "start", "stage", "cancel", "publish"} {
		t.Run(mode, func(t *testing.T) {
			if runtime.GOOS == "windows" && (mode == "symlink" || mode == "crash") {
				t.Skip("requires Unix symlinks/signals")
			}
			directory := t.TempDir()
			destination := filepath.Join(directory, "program")
			before := []byte("previous artifact")
			if err := os.WriteFile(destination, before, 0600); err != nil {
				t.Fatal(err)
			}
			var stdout, stderr bytes.Buffer
			request := DriverRequest{Tool: os.Args[0], Target: "test-triple", Output: destination, Arguments: []string{"-test.run=^TestLinkDriverProcess$", "--", mode}, Stdout: &stdout, Stderr: &stderr}
			ctx := context.Background()
			want := ArtifactVerificationFailure
			switch mode {
			case "success":
				want = ""
			case "failure":
				want = LinkResolutionFailure
			case "crash":
				want = ToolchainInvocationFailure
			case "unknown-tool":
				request.Tool = filepath.Join(directory, "absent")
				want = ToolchainInvocationFailure
			case "start":
				request.Tool = filepath.Join(directory, "broken")
				os.WriteFile(request.Tool, []byte("unexecutable data"), 0700)
				want = ToolchainInvocationFailure
			case "stage":
				request.Output = filepath.Join(directory, "absent", "program")
				want = ToolchainInvocationFailure
			case "cancel":
				var cancel context.CancelFunc
				ctx, cancel = context.WithCancel(ctx)
				cancel()
				request.Arguments[len(request.Arguments)-1] = "success"
				want = ToolchainInvocationFailure
			case "publish":
				os.Remove(destination)
				os.Mkdir(destination, 0700)
				os.WriteFile(filepath.Join(destination, "sentinel"), before, 0600)
				request.Arguments[len(request.Arguments)-1] = "success"
			}
			err := RunDriver(ctx, request)
			if want == "" {
				if err != nil {
					t.Fatal(err)
				}
				data, _ := os.ReadFile(destination)
				if string(data) != "linked fixture" || stdout.String() != "driver stdout" || stderr.String() != "driver warning" {
					t.Fatal(string(data), stdout.String(), stderr.String())
				}
			} else {
				var failure *Failure
				if !errors.As(err, &failure) || failure.Class != want || failure.Tool != request.Tool || failure.Target != "test-triple" || failure.Output != request.Output || failure.Help() == "" {
					t.Fatal(err)
				}
				definition, ok := diagnostics.Lookup(failure.DiagnosticID())
				if !ok || !definition.Mandatory || definition.Family != "linking" {
					t.Fatal(definition)
				}
				file := destination
				if mode == "publish" {
					file = filepath.Join(destination, "sentinel")
				}
				data, err := os.ReadFile(file)
				if err != nil || !bytes.Equal(data, before) {
					t.Fatal("prior output changed", string(data), err)
				}
			}
			entries, _ := os.ReadDir(directory)
			for _, entry := range entries {
				if strings.HasPrefix(entry.Name(), ".sec-link-") {
					t.Fatal("staging leak", entry.Name())
				}
			}
		})
	}
}
