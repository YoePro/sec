package main

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"sec/internal/testsupport"
)

// MD-143 § 113B(2): runtime reads of BuildInfo.Current() fields run
// natively. The materialized constant struct folds to its field operands
// before the LLVM track's entry gate, so no struct or string representation
// is selected; the values are the plan's (stored build 10 of the Target).
// Rules: rules/compiler/compiler_known_members.md — § 113B(2)-(4).
func TestBuildInfoFieldsRunNatively(t *testing.T) {
	testsupport.RequireSecMLIROptPath(t)
	if _, err := exec.LookPath("clang"); err != nil {
		t.Skip("clang unavailable")
	}
	manifest := strings.NewReplacer("os='linux'", "os='"+runtime.GOOS+"'", "arch='arm64'", "arch='"+runtime.GOARCH+"'").Replace(buildInfoManifest)
	source := writeBuildInfoProject(t, manifest)
	program := "module main\n\nfn main() int {\n    let info := BuildInfo.Current()\n    if info.VersionBuild == 10 && BuildInfo.Current().VersionMajor == 1 && info.VersionRevision == 3 {\n        return 42\n    }\n    return 1\n}\n"
	if err := os.WriteFile(source, []byte(program), 0644); err != nil {
		t.Fatal(err)
	}
	before, _ := os.ReadFile(filepath.Join(filepath.Dir(filepath.Dir(filepath.Dir(source))), ".sec", "sec.toml"))
	var exit *exec.ExitError
	if err := runNativeProgram(t, source); !errors.As(err, &exit) || exit.ExitCode() != 42 {
		t.Fatalf("status = %v, want 42", err)
	}
	after, _ := os.ReadFile(filepath.Join(filepath.Dir(filepath.Dir(filepath.Dir(source))), ".sec", "sec.toml"))
	if string(before) != string(after) {
		t.Fatal("building with BuildInfo changed the manifest")
	}
}
