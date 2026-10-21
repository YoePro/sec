package secmlir

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"sec/internal/testsupport"
)

// sliceVerificationPipeline composes the shared slice guard and every P15
// verifier. Both stages retain semantic types; neither requests layout/ABI.
// Rules: rules/mlir/packages/sec-mlir-dialect_package16.md — §§83–86,138–139.
func sliceVerificationPipeline(resolveScalars bool) []string {
	var args []string
	if resolveScalars {
		args = append(args, "--sec-resolve-scalar-layout")
	}
	return append(args, "--sec-verify-slice-guards", "--sec-verify-places", "--sec-verify-reference-guards", "--sec-verify-borrow-semantics")
}

// Read the canonical lit inputs instead of maintaining another collection of
// malformed contracts. Lit still owns exact diagnostic line/column matching.
// Rules: rules/mlir/packages/sec-mlir-dialect_package16.md — §§138–139.
func sliceVerificationFixture(t *testing.T, file string) string {
	t.Helper()
	input, err := os.ReadFile("../../../mlir/test/" + file)
	if err != nil {
		t.Fatal(err)
	}
	return string(input)
}

// An actual failed verifier invocation must report the owning contract and
// leave neither stdout IR nor the requested artifact, including after P6.
// Rules: rules/compiler/compiler_pipeline.md — §§32–34;
// rules/mlir/sec_mlir_lowering.md — v12.29–31;
// rules/mlir/packages/sec-mlir-dialect_package16.md — §§138–139.
func rejectSliceVerification(t *testing.T, tool, input, want string, resolveScalars bool) {
	t.Helper()
	artifact := filepath.Join(t.TempDir(), "rejected.mlir")
	args := append(sliceVerificationPipeline(resolveScalars), "-o", artifact, "-")
	command := exec.Command(tool, args...)
	command.Stdin = strings.NewReader(input)
	var stdout, stderr bytes.Buffer
	command.Stdout, command.Stderr = &stdout, &stderr
	err := command.Run()
	if err == nil || !strings.Contains(stderr.String(), want) {
		t.Fatalf("rejected contract %q: %v\n%s", want, err, stderr.String())
	}
	if stdout.Len() != 0 {
		t.Fatalf("failed verification emitted stdout IR: %s", stdout.String())
	}
	if _, err := os.Stat(artifact); !os.IsNotExist(err) {
		t.Fatal("failed verification left an artifact", err)
	}
}

// Every local and CFG negative slice suite is exercised through the complete
// verifier composition, before/after scalar resolution. Checking the original
// diagnostic prevents an unrelated earlier failure from passing this matrix.
// Rules: rules/mlir/packages/sec-mlir-dialect_package16.md — §§138–139;
// rules/mlir/sec_mlir_dialect.md — v12.24–25.
func TestPackage16MLIRRejectionMatrix(t *testing.T) {
	tool := testsupport.RequireSecMLIROptPath(t)
	expectation := regexp.MustCompile(`expected-error[^\n]*?\{\{(.*?)\}\}`)
	suites := []struct{ category, file string }{
		{"types", "Dialect/Sec/invalid-schema12-slice-types.mlir"},
		{"ops", "Dialect/Sec/invalid-schema12-slice-ops.mlir"},
		{"authority", "Dialect/Sec/invalid-schema12-slice-authority.mlir"},
		{"aggregate-copy", "Dialect/Sec/invalid-schema12-slice-aggregates.mlir"},
		{"reference-boundaries", "Dialect/Sec/invalid-schema12-slice-reference-boundaries.mlir"},
		{"versioned-types", "Dialect/Sec/invalid-schema12-slice-version.mlir"},
		{"versioned-operations", "Dialect/Sec/invalid-schema12-slice-boundary-version.mlir"},
		{"range-index-reference-guards", "Analysis/invalid-schema12-slice-guards.mlir"},
		{"metadata-copy-move-end-return", "Analysis/invalid-schema12-slice-borrows.mlir"},
	}
	for _, suite := range suites {
		input := sliceVerificationFixture(t, suite.file)
		for index, part := range strings.Split(input, "// -----") {
			matches := expectation.FindAllStringSubmatch(part, -1)
			if len(matches) != 1 {
				t.Fatalf("%s case %d requires one owning diagnostic, got %d", suite.file, index+1, len(matches))
			}
			want := matches[0][1]
			for _, resolve := range []bool{false, true} {
				t.Run(fmt.Sprintf("%s/%02d/resolve=%t", suite.category, index+1, resolve), func(t *testing.T) {
					rejectSliceVerification(t, tool, part, want, resolve)
				})
			}
		}
	}
}

// Slice types (including nested structs/arrays), real guarded/borrowed values
// and P15 schema-11 metadata survive repeated verification and serialization.
// These fixtures are whole-function valid; local-op-only fixtures retain their
// dedicated lit checks rather than bypassing guards to make this test pass.
// Rules: rules/mlir/packages/sec-mlir-dialect_package16.md — §§87–90,138–139.
func TestPackage16MLIRRoundTripMatrix(t *testing.T) {
	tool := testsupport.RequireSecMLIROptPath(t)
	for _, file := range []string{
		"Dialect/Sec/schema12-slice-types.mlir",
		"Dialect/Sec/schema12-slice-reference-boundaries.mlir",
		"Dialect/Sec/schema12-slice-aggregates.mlir",
		"Analysis/schema12-slice-borrows.mlir",
		"Dialect/Sec/schema11-emitter-metadata.mlir",
	} {
		input := sliceVerificationFixture(t, file)
		for _, resolve := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/resolve=%t", file, resolve), func(t *testing.T) {
				args := append(sliceVerificationPipeline(resolve), "-")
				command := exec.Command(tool, args...)
				command.Stdin = strings.NewReader(input)
				first, err := command.Output()
				if err != nil {
					t.Fatalf("positive fixture rejected: %v", err)
				}
				command = exec.Command(tool, args...)
				command.Stdin = bytes.NewReader(first)
				second, err := command.Output()
				if err != nil || !bytes.Equal(first, second) {
					t.Fatalf("verification/round-trip changed canonical output: %v", err)
				}
				version := "12"
				if strings.Contains(file, "schema11-") {
					version = "11"
				}
				if !bytes.Contains(second, []byte("sec.dialect_version = "+version+" : i32")) {
					t.Fatal("round-trip changed the module schema")
				}
			})
		}
	}
}

// Unknown versions and a schema-11 downgrade cannot silently accept even
// type-only P16 inputs. The existing P15 positive fixture above stays schema 11.
// Rules: rules/mlir/packages/sec-mlir-dialect_package16.md — §§87,138–139;
// rules/mlir/sec_mlir_dialect.md — v12.1–4.
func TestPackage16MLIRVersionRejectionMatrix(t *testing.T) {
	tool := testsupport.RequireSecMLIROptPath(t)
	input := sliceVerificationFixture(t, "Dialect/Sec/schema12-slice-types.mlir")
	for _, version := range []int64{0, 11, 13, -1} {
		modified := strings.Replace(input, "\n  sec.dialect_version = 12 : i32,", fmt.Sprintf("\n  sec.dialect_version = %d : i32,", version), 1)
		if modified == input {
			t.Fatal("version mutation did not change the module")
		}
		want := "unsupported Sec dialect schema version"
		if version == 11 {
			want = "slice types require schema 12, module declares schema 11"
		}
		for _, resolve := range []bool{false, true} {
			t.Run(fmt.Sprintf("%d/resolve=%t", version, resolve), func(t *testing.T) {
				rejectSliceVerification(t, tool, modified, want, resolve)
			})
		}
	}
}
