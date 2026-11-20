package mlir

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"time"
)

var appliedDataLayoutPattern = regexp.MustCompile(`target datalayout = "([^"]+)"`)

// TargetDataLayout returns the data layout llc applies to a module compiled
// for triple. It is the backend evidence that the maintained target layout
// facts are checked against before object generation (MD-141 § 4(3)-(4)).
//
// llc is today the out-of-process code generator of the native track; when
// Sec is bootstrapped this query moves to the in-process backend.
// Rules: rules/platform/target_profiles.md — § 40(2); rules/memory/layout.md — § 47.2.
func (t Toolchain) TargetDataLayout(triple string) (string, error) {
	path, err := t.toolPath("llc")
	if err != nil {
		return "", err
	}
	directory, err := os.MkdirTemp("", "sec-datalayout-*")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(directory)
	probe := filepath.Join(directory, "probe.ll")
	if err := os.WriteFile(probe, []byte("define void @sec_data_layout_probe() {\n  ret void\n}\n"), 0o600); err != nil {
		return "", err
	}
	timeout := t.Timeout
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, path, "-mtriple="+triple, "-print-after=pre-isel-intrinsic-lowering", "-print-module-scope", probe, "-o", os.DevNull)
	var output bytes.Buffer
	cmd.Stdout, cmd.Stderr = &output, &output
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("llc cannot compile for %s: %s", triple, bytes.TrimSpace(output.Bytes()))
	}
	match := appliedDataLayoutPattern.FindSubmatch(output.Bytes())
	if match == nil {
		return "", fmt.Errorf("llc did not report the data layout it applies for %s", triple)
	}
	return string(match[1]), nil
}
