package mlir

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"sec/internal/diagnostics"
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

// CompileObject compiles LLVM IR to a target object with the llc of the same
// LLVM build as mlir-translate. Textual LLVM IR is version-specific, so it
// never crosses into a differently versioned tool; the system C driver only
// links the object (position-independent, for default PIE links).
func (t Toolchain) CompileObject(llvmPath string, objectPath string, triple string) error {
	args := []string{"-filetype=obj", "-relocation-model=pic", "-mtriple=" + triple, llvmPath, "-o", objectPath}
	return t.run("llc", args...)
}

// VerifySec parses and verifies Sec-dialect MLIR with the Sec-aware driver.
func (t Toolchain) VerifySec(mlirPath string) error {
	args := append([]string{mlirPath}, secVerificationPasses...)
	args = append(args, "-o", os.DevNull)
	return t.run("sec-mlir-opt", args...)
}

// VerifySecInspection runs the Sec dialect checks plus the schema-11 Place,
// reference-guard and borrow-semantics verifiers that the emit-sec-mlir
// inspection boundary requires before it publishes P15 output (MD-086 § 5).
func (t Toolchain) VerifySecInspection(mlirPath string) error {
	args := append([]string{mlirPath}, secVerificationPasses...)
	args = append(args, secReferenceVerificationPasses...)
	args = append(args, "-o", os.DevNull)
	return t.run("sec-mlir-opt", args...)
}

// secReferenceVerificationPasses check Place and direct-reference facts.
var secReferenceVerificationPasses = []string{
	"--sec-verify-places",
	"--sec-verify-reference-guards",
	"--sec-verify-borrow-semantics",
}

// secVerificationPasses are the Sec dialect proof checks every emission and
// lowering runs before trusting the module.
var secVerificationPasses = []string{
	"--sec-verify-checked-integer-guards",
	"--sec-verify-result-guards",
	"--sec-verify-try-handlers",
	"--sec-verify-union-guards",
	"--sec-verify-match-cfg",
}

// LowerSecToLLVM verifies Sec-dialect MLIR and lowers it through standard
// MLIR to the LLVM dialect: the single compilation track. A non-empty entry
// names the Sec function identity of the command entry and generates the
// hosted startup adapter; an empty entry lowers a module without one.
//
// Rules:
//   - rules/compiler/compiler_pipeline.md — §§ 32–34 staged lowering
//   - rules/compiler/initialization.md — §§ 21, 28 command entry adapter
func (t Toolchain) LowerSecToLLVM(secPath string, llvmDialectPath string, entry string) error {
	args := append([]string{secPath}, secVerificationPasses...)
	args = append(args, "--sec-lower-to-llvm=entry="+entry, "-o", llvmDialectPath)
	err := t.run("sec-mlir-opt", args...)
	if err == nil {
		return nil
	}
	if limits := loweringCapabilityErrors(strings.TrimPrefix(err.Error(), "sec-mlir-opt failed: ")); len(limits) > 0 {
		return errors.Join(limits...)
	}
	return err
}

// LoweringCapabilityError is a construct the Sec MLIR -> LLVM track reported
// it does not lower yet: an implementation capability limit, not a source
// error (rules/compiler/compiler_pipeline.md — §32(2–4)).
type LoweringCapabilityError struct {
	Feature      string
	File         string
	Line, Column int
}

func (e *LoweringCapabilityError) Error() string {
	return fmt.Sprintf("the Sec MLIR -> LLVM track does not lower %s yet at %s:%d:%d", e.Feature, e.File, e.Line, e.Column)
}

func (e *LoweringCapabilityError) ImplementationCapability() diagnostics.ImplementationCapability {
	return diagnostics.ImplementationCapability{Feature: e.Feature, Boundary: "Sec MLIR to LLVM lowering", File: e.File, Line: e.Line, Column: e.Column}
}

// loweringCapabilityPattern matches only the fixed capability diagnostics of
// the track's gates (sec-check-lowering-support, sec-finalize-for-llvm).
var loweringCapabilityPattern = regexp.MustCompile(`(?m)^(.+?):(\d+):(\d+): error: the Sec MLIR -> LLVM track does not lower (.+) yet$`)

func loweringCapabilityErrors(output string) []error {
	limits := []error{}
	for _, match := range loweringCapabilityPattern.FindAllStringSubmatch(output, -1) {
		line, _ := strconv.Atoi(match[2])
		column, _ := strconv.Atoi(match[3])
		limits = append(limits, &LoweringCapabilityError{Feature: match[4], File: match[1], Line: line, Column: column})
	}
	return limits
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
	// sec-mlir-opt lives in the Sec MLIR build while mlir-translate comes
	// from the LLVM/MLIR build, so the default build directory is a fallback.
	if fallback := expandHome(defaultBuildBin); fallback != t.BinDir {
		path := filepath.Join(fallback, name)
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
