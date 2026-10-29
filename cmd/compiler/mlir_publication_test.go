package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	mlirtoolchain "sec/internal/mlir"
	"sec/internal/testsupport"
)

// TestSecMLIRPublicationWaitsForVerification injects a rejecting verifier at
// the external boundary. Neither inspection nor retained build output may
// create/overwrite the requested artifact before that boundary succeeds.
// Rules: rules/compiler/compiler_pipeline.md — §88(1–3), §§59–60.
func TestSecMLIRPublicationWaitsForVerification(t *testing.T) {
	if _, err := exec.LookPath("sh"); err != nil {
		t.Skip("shell unavailable for rejecting verifier fixture")
	}
	for _, command := range []string{"emit-sec-mlir", "build"} {
		for _, existing := range []bool{false, true} {
			name := "new"
			if existing {
				name = "existing"
			}
			t.Run(command+"/"+name, func(t *testing.T) {
				root := t.TempDir()
				program := filepath.Join(root, "program")
				artifact := program + ".mlir"
				const previous = "previous verified artifact\n"
				if existing {
					if err := os.WriteFile(artifact, []byte(previous), 0600); err != nil {
						t.Fatal(err)
					}
				}
				// These environment values are confined to this subprocess test;
				// the generated script contains no embedded machine paths.
				t.Setenv("SEC_TEST_MLIR_ARTIFACT", artifact)
				tool := `#!/bin/sh
if [ "$1" = "$SEC_TEST_MLIR_ARTIFACT" ]; then
    echo 'verifier was given the published artifact instead of a temporary input' >&2
    exit 9
fi
echo 'test verifier rejected module' >&2
exit 1
`
				if err := os.WriteFile(filepath.Join(root, "sec-mlir-opt"), []byte(tool), 0700); err != nil {
					t.Fatal(err)
				}
				args := []string{command, "../../testdata/build/native/add.sec", "--mlir-bin", root, "-o", artifact}
				if command == "build" {
					args = []string{command, "../../testdata/build/native/add.sec", "--mlir-bin", root, "--keep-mlir", "-o", program}
				}
				stdout, stderr, code := runCLIForDiagnostics(t, args...)
				if code != 4 || stdout != "" || !strings.Contains(stderr, "test verifier rejected module") {
					t.Fatalf("verification failure not preserved: code=%d stdout=%q stderr=%s", code, stdout, stderr)
				}
				data, err := os.ReadFile(artifact)
				if existing {
					if err != nil || string(data) != previous {
						t.Fatal("rejection overwrote an existing artifact", err)
					}
				} else if !os.IsNotExist(err) {
					t.Fatal("rejection published an artifact", err)
				}
			})
		}
	}
}

// TestRetainedSecMLIRIsVerified builds with real tools and rechecks the retained
// module, proving successful builds still publish useful high-level output.
// Rules: rules/compiler/compiler_pipeline.md — §88, §§32–34.
func TestRetainedSecMLIRIsVerified(t *testing.T) {
	bin := filepath.Dir(testsupport.RequireSecMLIROptPath(t))
	if _, err := exec.LookPath("clang"); err != nil {
		t.Skip("clang unavailable")
	}
	program := filepath.Join(t.TempDir(), "program")
	stdout, stderr, code := runCLIForDiagnostics(t, "build", "../../testdata/build/native/add.sec", "--keep-mlir", "--mlir-bin", bin, "-o", program)
	if code != 0 || stdout != "" {
		t.Fatalf("build: %d %s %s", code, stdout, stderr)
	}
	if err := mlirtoolchain.NewToolchain(bin).VerifySec(program + ".mlir"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(program); err != nil {
		t.Fatal("successful build lost its program", err)
	}
}
