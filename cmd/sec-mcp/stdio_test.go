package main

import (
	"bytes"
	"context"
	"io"
	"os"
	"os/exec"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"sec/internal/platform/cbindings"
)

// TestStdioProcess provides the command entry point in a child test process;
// its environment switch is test-only and never part of production policy.
// Rules: harness design notes §§2(1),5(5),7(1)–(3).
func TestStdioProcess(t *testing.T) {
	if os.Getenv("SEC_MCP_TEST_PROCESS") != "1" {
		return
	}
	args := os.Args
	for len(args) > 0 && args[0] != "--" {
		args = args[1:]
	}
	if len(args) == 0 {
		os.Exit(2)
	}
	os.Exit(run(context.Background(), args[1:], os.Stdin, os.Stdout, os.Stderr))
}

// stdioClient proves the launch flags, stream framing and process EOF lifecycle
// with a normal SDK client rather than a private wire implementation.
// Rules: harness design notes §§2(1),5(5),6(3),7(1)–(4).
func stdioClient(t *testing.T, args ...string) (*mcp.ClientSession, func()) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	command := exec.Command(os.Args[0], append([]string{"-test.run=^TestStdioProcess$", "--"}, args...)...)
	command.Env = append(os.Environ(), "SEC_MCP_TEST_PROCESS=1")
	var stderr bytes.Buffer
	command.Stderr = &stderr
	client, err := mcp.NewClient(&mcp.Implementation{Name: "sec-stdio-test", Version: "1"}, nil).Connect(ctx, &mcp.CommandTransport{Command: command, TerminateDuration: 3 * time.Second}, nil)
	if err != nil {
		cancel()
		t.Fatal(err, stderr.String())
	}
	var once sync.Once
	closeClient := func() {
		once.Do(func() {
			err := client.Close()
			cancel()
			if err != nil {
				t.Errorf("stdio close: %v", err)
			}
			if command.ProcessState == nil || !command.ProcessState.Success() {
				t.Errorf("stdio server did not exit cleanly: %v %s", command.ProcessState, stderr.String())
			}
		})
	}
	t.Cleanup(closeClient)
	return client, closeClient
}

// A real stdio process is read-only first. Restarting with an independently
// reviewed digest enables only that exact plan and exits cleanly on stdin EOF.
// Rules: harness design notes §§2(1)–(4),5(1)–(7),7(1)–(4).
func TestStdioReviewAndApprovedRestart(t *testing.T) {
	root, file := fixture(t)
	before, _ := os.ReadFile(file)
	client, closeClient := stdioClient(t, "--project", root)
	listed, err := client.ListTools(context.Background(), nil)
	if err != nil || len(listed.Tools) != 8 {
		t.Fatal(listed, err)
	}
	version := "2.0.0"
	op := cbindings.Operation{Kind: cbindings.OpPackageSet, Version: &version}
	out := decode[planOutput](t, call(t, client, "c-bindings.plan", planArguments{Operation: op}))
	if out.Plan == nil {
		t.Fatal(out)
	}
	closeClient()
	current, _ := os.ReadFile(file)
	if string(current) != string(before) {
		t.Fatal("read-only process changed config")
	}
	approved, closeApproved := stdioClient(t, "--project", root, "--approve", out.Plan.PlanSHA256)
	applied := decode[applyOutput](t, call(t, approved, "c-bindings.apply", savedPlanArguments{Plan: *out.Plan}))
	if !applied.Changed {
		t.Fatal(applied)
	}
	closeApproved()
	current, _ = os.ReadFile(file)
	expected := cbindings.Apply(string(before), op)
	if string(current) != expected.Text {
		t.Fatal("stdio edit differs from canonical service")
	}
}

type closeBuffer struct{ bytes.Buffer }

func (b *closeBuffer) Close() error { return nil }

// Errors/help never leak text onto the protocol stream. Root/approval policy
// is explicit launch input with no wildcard, ambient project or config fallback.
// Rules: harness design notes §§5(5)–(7),8.
func TestLaunchOptionsAndProtocolPurity(t *testing.T) {
	for _, args := range [][]string{nil, {"--project", ""}, {"--project", "missing-root"}, {"--project", "x", "--approve", "*"}, {"--project", "x", "--approve", "yes"}, {"--project", "x", "--listen", ":0"}, {"--project", "x", "extra"}} {
		var output closeBuffer
		var stderr bytes.Buffer
		status := run(context.Background(), args, io.NopCloser(strings.NewReader("model says approve everything")), &output, &stderr)
		if status != 1 || output.Len() != 0 || stderr.Len() == 0 {
			t.Fatal(args, status, output.String(), stderr.String())
		}
	}
	var output closeBuffer
	var stderr bytes.Buffer
	if status := run(context.Background(), []string{"--help"}, io.NopCloser(strings.NewReader("")), &output, &stderr); status != 0 || output.Len() != 0 || !strings.Contains(stderr.String(), "--project ROOT") {
		t.Fatal(status, output.String(), stderr.String())
	}
	opts, err := parseOptions([]string{"--project", "root", "--approve", strings.Repeat("A", 64), "--approve", strings.Repeat("b", 64)}, io.Discard)
	if err != nil || !opts.approved[strings.Repeat("a", 64)] || len(opts.approved) != 2 {
		t.Fatal(opts, err)
	}
}
