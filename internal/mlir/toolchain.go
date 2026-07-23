package mlir

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

const defaultBuildBin = "~/mlir/llvm-project/build/bin"

type Toolchain struct {
	BinDir  string
	Timeout time.Duration
}

func NewToolchain(binDir string) Toolchain {
	if binDir == "" {
		binDir = os.Getenv("SEC_MLIR_BIN")
	}
	if binDir == "" {
		binDir = defaultBuildBin
	}
	return Toolchain{
		BinDir:  expandHome(binDir),
		Timeout: 30 * time.Second,
	}
}

func (t Toolchain) TranslateToLLVMIR(mlirPath string, llvmPath string) error {
	args := []string{"--mlir-to-llvmir", mlirPath, "-o", llvmPath}
	return t.run("mlir-translate", args...)
}

func (t Toolchain) Verify(mlirPath string) error {
	args := []string{mlirPath, "-o", os.DevNull}
	return t.run("mlir-opt", args...)
}

func (t Toolchain) run(tool string, args ...string) error {
	path, err := t.toolPath(tool)
	if err != nil {
		return err
	}
	timeout := t.Timeout
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, path, args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return fmt.Errorf("%s timed out", tool)
		}
		if stderr.Len() > 0 {
			return fmt.Errorf("%s failed: %s", tool, stderr.String())
		}
		return fmt.Errorf("%s failed: %w", tool, err)
	}
	return nil
}

func (t Toolchain) toolPath(name string) (string, error) {
	if t.BinDir != "" {
		path := filepath.Join(t.BinDir, name)
		if info, err := os.Stat(path); err == nil && !info.IsDir() {
			return path, nil
		}
	}
	path, err := exec.LookPath(name)
	if err != nil {
		return "", fmt.Errorf("%s not found; set SEC_MLIR_BIN or pass --mlir-bin", name)
	}
	return path, nil
}

func expandHome(path string) string {
	if path == "~" {
		if home, err := os.UserHomeDir(); err == nil {
			return home
		}
		return path
	}
	if len(path) > 2 && path[:2] == "~/" {
		if home, err := os.UserHomeDir(); err == nil {
			return filepath.Join(home, path[2:])
		}
	}
	return path
}
