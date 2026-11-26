// Package linking owns native linker-boundary outcomes, not source semantics or
// a second compilation track. Canonical LinkPlan construction has its own owner.
package linking

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"sec/internal/diagnostics"
)

type FailureClass string

const (
	ToolchainInvocationFailure  FailureClass = "ToolchainInvocationFailure"
	LinkResolutionFailure       FailureClass = "LinkResolutionFailure"
	ArtifactVerificationFailure FailureClass = "ArtifactVerificationFailure"
)

// Failure carries producer-classified build failure evidence. No source span or
// semantic proof state is inferred from a linker message.
// Rules: rules/compiler/linking.md §64; rules/tooling/diagnostics.md §§8-10,14.
type Failure struct {
	Class                        FailureClass
	Tool, Target, Output, Detail string
	Cause                        error
}

func (f *Failure) Error() string {
	message := string(f.Class) + ": " + f.Tool
	if f.Detail != "" {
		message += ": " + strings.TrimSpace(f.Detail)
	}
	if f.Cause != nil {
		message += ": " + f.Cause.Error()
	}
	return message
}
func (f *Failure) Unwrap() error { return f.Cause }
func (f *Failure) DiagnosticID() string {
	switch f.Class {
	case ToolchainInvocationFailure:
		return diagnostics.LinkToolchainInvocation
	case LinkResolutionFailure:
		return diagnostics.LinkResolution
	case ArtifactVerificationFailure:
		return diagnostics.LinkArtifactVerification
	}
	return ""
}

type DriverRequest struct {
	Tool, Target, Output string
	Arguments            []string
	Stdout, Stderr       io.Writer
}

// RunDriver classifies by the failing operation, never by stderr keywords.
// Missing/start/crash/cancellation failures are invocation failures; a normally
// exited unsuccessful driver is a resolution failure. A successful driver must
// produce a fresh nonempty regular candidate before atomic publication. This is
// the minimum output check, not the future full LinkPlan artifact verifier.
// Rules: rules/compiler/linking.md §§50-51,64; rules/compiler/compiler_pipeline.md §§32-34.
func RunDriver(ctx context.Context, request DriverRequest) error {
	fail := func(class FailureClass, detail string, cause error) error {
		return &Failure{class, request.Tool, request.Target, request.Output, detail, cause}
	}
	path, err := exec.LookPath(request.Tool)
	if err != nil {
		return fail(ToolchainInvocationFailure, "linker driver not found", err)
	}
	staged, err := os.CreateTemp(filepath.Dir(request.Output), ".sec-link-*"+filepath.Ext(request.Output))
	if err != nil {
		return fail(ToolchainInvocationFailure, "cannot stage linker output", err)
	}
	candidate := staged.Name()
	defer os.Remove(candidate)
	if err := staged.Close(); err != nil {
		return fail(ToolchainInvocationFailure, "cannot close staged output", err)
	}
	if err := os.Remove(candidate); err != nil {
		return fail(ToolchainInvocationFailure, "cannot prepare fresh output", err)
	}
	args := append(append([]string{}, request.Arguments...), "-o", candidate)
	command := exec.CommandContext(ctx, path, args...)
	var stdout, stderr bytes.Buffer
	command.Stdout, command.Stderr = &stdout, &stderr
	err = command.Run()
	output := stdout.String() + stderr.String()
	if err != nil {
		if ctx.Err() != nil {
			return fail(ToolchainInvocationFailure, output, ctx.Err())
		}
		if exit, ok := err.(*exec.ExitError); ok && exit.ProcessState.Exited() {
			return fail(LinkResolutionFailure, output, err)
		}
		return fail(ToolchainInvocationFailure, output, err)
	}
	info, err := os.Lstat(candidate)
	if err != nil {
		return fail(ArtifactVerificationFailure, "linker succeeded without a readable output artifact", err)
	}
	if !info.Mode().IsRegular() || info.Size() == 0 {
		return fail(ArtifactVerificationFailure, "linker output must be a nonempty regular file", nil)
	}
	if err := os.Rename(candidate, request.Output); err != nil {
		return fail(ArtifactVerificationFailure, "cannot publish verified candidate", err)
	}
	if request.Stdout != nil {
		_, _ = request.Stdout.Write(stdout.Bytes())
	}
	if request.Stderr != nil {
		_, _ = request.Stderr.Write(stderr.Bytes())
	}
	return nil
}

// Help names the next action without guessing a source-code correction.
// Rules: rules/compiler/linking.md §64; rules/tooling/diagnostics.md §3(6).
func (f *Failure) Help() string {
	switch f.Class {
	case ToolchainInvocationFailure:
		return "Check the selected linker driver, its executable permissions, toolchain availability and invocation resources."
	case LinkResolutionFailure:
		return "Inspect the linker output and the selected symbols, dependencies, libraries and target options."
	case ArtifactVerificationFailure:
		return "The linker reported success but its candidate could not be accepted. Check the output artifact and destination; the previous output was preserved."
	}
	return fmt.Sprintf("Inspect the %s build failure.", f.Class)
}
