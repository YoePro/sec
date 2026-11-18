package secmlir

import (
	"bytes"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"testing"

	"sec/internal/ir/semantic"
	"sec/internal/layout"
	"sec/internal/lexer"
	"sec/internal/parser"
	"sec/internal/sema"
	"sec/internal/testsupport"
)

// package18Module builds a P18 fixture under the hosted allocation profile and
// returns its Semantic IR text and verified schema-14 module.
func package18Module(t *testing.T, path string) (string, []byte) {
	t.Helper()
	source, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	parsed := parser.New(lexer.NewWithFile(string(source), "matrix.sec")).Parse()
	if parsed.HasErrors {
		t.Fatal(parsed.Diagnostics)
	}
	analyzer := sema.NewAnalyzerWithScalarPlan(layout.ResolvedScalarPlan{TargetOS: "linux", TargetArch: "amd64", Profile: "hosted", PointerWidthBits: 64})
	analyzer.SetAllocationProfile("hosted")
	if errors := analyzer.Analyze(parsed.Program); len(errors) != 0 {
		t.Fatal(errors)
	}
	module, err := semantic.Build(parsed.Program, analyzer, semantic.BuildOptions{RequestedModule: "main", SourceFiles: []string{"matrix.sec"}, MaxPackage: 18})
	if err != nil {
		t.Fatal(err)
	}
	output, err := EmitWithOptions(module, testPlan(64), EmitOptions{DialectVersion: 14})
	if err != nil {
		t.Fatal(err)
	}
	tool := testsupport.RequireSecMLIROptPath(t)
	verify := exec.Command(tool, "--sec-verify-dynamic-arrays", "--sec-verify-storage-transitions", "--sec-verify-places",
		"--sec-verify-slice-guards", "--sec-verify-reference-guards", "--sec-verify-borrow-semantics", "--sec-verify-cleanups",
		"--sec-verify-destruction-plans", "--sec-verify-ownership", "-", "-o", os.DevNull)
	verify.Stdin = bytes.NewReader(output)
	if result, err := verify.CombinedOutput(); err != nil {
		t.Fatalf("%v\n%s", err, result)
	}
	return semantic.Format(module), output
}

// functionText returns the printed body of one Semantic IR function.
func functionText(t *testing.T, text, id string) string {
	t.Helper()
	start := strings.Index(text, `func "`+id+`"`)
	if start < 0 {
		t.Fatalf("missing function %s", id)
	}
	end := strings.Index(text[start+1:], "\n  func \"")
	if end < 0 {
		return text[start:]
	}
	return text[start : start+1+end]
}

type package18Row struct {
	name  string
	check func() bool
}

func runPackage18Rows(t *testing.T, rows []package18Row) {
	t.Helper()
	for _, row := range rows {
		if !row.check() {
			t.Errorf("row %q does not hold", row.name)
		}
	}
}

// P18-36/37/38/39/40/41 (§§ 121–126): the dynamic-array invariant, indexing,
// slice, storage-transition, destruction and growth matrices, checked on the
// Semantic IR of package18_matrix.sec and dynamic_array_ownership.sec, whose
// schema-14 modules pass every verifier (the protocol and lifecycle
// rejections are the invalid-schema14-* lit tests; the Sema rows are
// TestPackage18BorrowAndElementMatrix). Named boundaries: no source producer
// allocates with capacity (P18-18, MD-072) or constructs partially (P18-22);
// growth never selects replace-domain or proves in-place growth, so every
// growth advances the epoch conservatively; no allocator-backed context is
// selected (MD-034); and no non-relocatable element type exists (§ 69).
//
// Rules:
//   - rules/mlir/packages/sec-mlir-dialect_package18.md — §§ 121–126
func TestPackage18Matrices(t *testing.T) {
	text, _ := package18Module(t, "../../../testdata/semantic_ir/package18_matrix.sec")
	ownership, _ := package18Module(t, "../../../testdata/semantic_ir/dynamic_array_ownership.sec")
	matrix := functionText(t, text, "main::Matrix(uint)")
	checked := functionText(t, text, "main::Checked(int)")
	handles := functionText(t, text, "main::Handles()")
	two := functionText(t, text, "main::Two()")
	has := func(body string, parts ...string) func() bool {
		return func() bool {
			for _, part := range parts {
				if !strings.Contains(body, part) {
					return false
				}
			}
			return true
		}
	}
	lacks := func(body string, part string) func() bool {
		return func() bool { return !strings.Contains(body, part) }
	}
	t.Run("invariants §121", func(t *testing.T) {
		runPackage18Rows(t, []package18Row{
			{"length zero/capacity zero/no backing", has(matrix, "dynamic-array.empty", "dynamic-array.has-backing")},
			{"length zero/capacity nonzero is a named boundary (no capacity producer)", lacks(text, "dynamic-array.allocate")},
			{"length <= capacity: append only after ensured capacity", has(matrix, "dynamic-array.has-capacity", "dynamic-array.append-prepared")},
			{"initialized prefix and uninitialized capacity: every element Place is length-guarded", func() bool {
				return strings.Count(matrix, "dynamic-array.element-place") == strings.Count(matrix, "dynamic-array.index-in-bounds") &&
					strings.Count(matrix, "dynamic-array.element-place") == strings.Count(matrix, "proof=guarded guard=%")
			}},
			{"append initializes the evaluated element before the length grows", has(matrix, "dynamic-array.append-prepared %3, %4")},
			{"construction failure is a named boundary: owners come only from dynamic-array.empty", func() bool {
				return strings.Count(text, "dynamic-array.empty") > 0 && !strings.Contains(text, "dynamic-array.allocate")
			}},
		})
	})
	t.Run("indexing §122", func(t *testing.T) {
		runPackage18Rows(t, []package18Row{
			{"copyable element read", has(matrix, "place.read %")},
			{"element replacement", has(matrix, "place.write %")},
			{"shared element ref", has(matrix, "index=1 {root=#1 storage=#1 backing=#1}")},
			{"runtime signed index", has(matrix, "dynamic-array.index-in-bounds %35, %36 signed=true")},
			{"runtime unsigned index", has(matrix, "signed=false")},
			{"IndexError fallible path on an empty owner", func() bool {
				return strings.Contains(checked, "dynamic-array.empty") && strings.Contains(checked, "dynamic-array.index-in-bounds") && !strings.Contains(checked, "fail.bounds")
			}},
			{"ordinary panic.bounds path", has(matrix, `fail.bounds operation="dynamic-array-index"`)},
		})
	})
	t.Run("slices §123", func(t *testing.T) {
		runPackage18Rows(t, []package18Row{
			{"full shared slice from the runtime length", has(matrix, "proof=source-open-bound", "slice.borrow-shared")},
			{"full mutable slice", has(matrix, "slice.borrow-mut")},
			{"subslice", has(matrix, "slice.range-check")},
			{"slice shares the owner's backing identity", has(matrix, "origins=[#1]")},
		})
	})
	t.Run("storage transitions §124", func(t *testing.T) {
		runPackage18Rows(t, []package18Row{
			{"first allocation establishes the domain", has(matrix, "storage.establish-domain")},
			{"in-place mutation is no transition", func() bool {
				for _, block := range regexp.MustCompile(`\n\s*\^\d+[:(]`).Split(matrix, -1) {
					if strings.Contains(block, "place.write %") {
						return !strings.Contains(block, "storage.") && !strings.Contains(block, "grow-backing")
					}
				}
				return false
			}},
			{"reallocation preserving logical identity advances the epoch", has(matrix, "storage.advance-epoch")},
			{"replace-domain is a named boundary (never selected)", lacks(text, "replace-domain")},
			{"destruction ends the domain, reclaim distinct from end", has(text, "backing=[end-domain,reclaim-per-allocation-plan]")},
			{"distinct owners have distinct domains", func() bool {
				domains := map[string]bool{}
				for _, match := range regexp.MustCompile(`domain=#\d+`).FindAllString(two, -1) {
					domains[match] = true
				}
				return len(domains) == 2
			}},
		})
	})
	t.Run("destruction §125", func(t *testing.T) {
		runPackage18Rows(t, []package18Row{
			{"empty owner is tracked for cleanup", has(handles, "cleanup.track-owned", "destroy:array:dynamic:main:struct:Handle")},
			{"trivial elements: no element destruction step", has(text, `"destroy:array:dynamic::int:int" !8 dynamic-array element="destroy::int:int" backing=[end-domain`)},
			{"non-trivial elements destroyed in reverse runtime order", has(text, "backing=[destroy-elements-reverse,end-domain,reclaim-per-allocation-plan]")},
			{"Arena-backed owner never reclaims individually", lacks(matrix, "storage.reclaim")},
			{"moved owner source not destroyed", has(ownership, "cleanup.cancel")},
			{"replacement destroys the old owner after the RHS", has(ownership, "own.replace-place")},
		})
	})
	t.Run("growth §126", func(t *testing.T) {
		runPackage18Rows(t, []package18Row{
			{"capacity already sufficient: tested before growth", has(matrix, "dynamic-array.has-capacity %3 count=1")},
			{"relocating growth records its transition", has(matrix, "transition=advance-epoch")},
			{"allocation failure maps to CollectionError without a panic", lacks(matrix, "fail.allocation")},
			{"growth never source-moves elements", lacks(matrix, "own.move")},
		})
	})
}

// P18-33 (§§ 116–118): P18 selects no physical representation. The schema-14
// module names no heap, garbage collector, registry, LLVM struct, memref or
// allocator call, and the Sec MLIR -> LLVM track rejects a dynamic array as an
// unsupported lowering instead of choosing a descriptor or allocator. The
// no-allocation profile rows are TestPackage18AllocationPlanMatrix.
//
// Rules:
//   - rules/mlir/packages/sec-mlir-dialect_package18.md — §§ 116–118
func TestPackage18NoPhysicalLowering(t *testing.T) {
	_, output := package18Module(t, "../../../testdata/semantic_ir/package18_matrix.sec")
	for _, physical := range []string{"llvm.", "memref", "malloc", "\"free\"", "realloc", "mmap", "gc_", "refcount"} {
		if bytes.Contains(output, []byte(physical)) {
			t.Errorf("schema-14 module chooses a physical representation: %q", physical)
		}
	}
	tool := testsupport.RequireSecMLIROptPath(t)
	lower := exec.Command(tool, "--sec-lower-to-llvm", "-", "-o", os.DevNull)
	lower.Stdin = bytes.NewReader(output)
	result, err := lower.CombinedOutput()
	if err == nil || !bytes.Contains(result, []byte("[unsupported-lowering]")) || !bytes.Contains(result, []byte("sec.dynamic_array")) {
		t.Fatalf("the LLVM track did not reject the dynamic array as an unsupported lowering: %v\n%s", err, result)
	}
}
