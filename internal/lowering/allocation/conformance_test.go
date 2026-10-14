package allocation_test

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"sec/internal/codegen/llvm"
	"sec/internal/codegen/mlir"
	"sec/internal/codegen/targetplan"
	"sec/internal/ir/semantic"
	"sec/internal/lexer"
	"sec/internal/lowering/secmlir"
	"sec/internal/parser"
	"sec/internal/sema"
	"sec/internal/testsupport"
)

// TestAllocationFreeLowering audits all emitted calls against the canonical
// source-function inventory after Sema proves every body allocation-free.
// Thus an unexpected allocation helper fails even if its name is unfamiliar;
// LLVM stack slots remain ordinary automatic storage, not dynamic allocation.
// Rules: rules/memory/allocation.md — §§2(4)-(5),3(5)-(6),24,30(6).
func TestAllocationFreeLowering(t *testing.T) {
	for _, triple := range []string{"x86_64-pc-linux-gnu", "armv7-unknown-linux-gnueabihf"} {
		for _, file := range []string{"../../../testdata/sema/allocation_free_lowering.sec", "../../../testdata/defaults/backends/scalars.sec"} {
			t.Run(triple+"/"+filepath.Base(file), func(t *testing.T) {
				data, err := os.ReadFile(file)
				if err != nil {
					t.Fatal(err)
				}
				p := parser.New(lexer.NewWithFile(string(data), file))
				program := p.ParseProgram()
				if len(p.Errors()) != 0 {
					t.Fatal(p.Errors())
				}
				plan, err := targetplan.Plan(triple)
				if err != nil {
					t.Fatal(err)
				}
				a := sema.NewAnalyzerWithScalarPlan(plan)
				if errors := a.Analyze(program); len(errors) != 0 {
					t.Fatal(errors)
				}
				allowed := map[string]bool{}
				graph := a.CallGraph()
				facts := a.AllocationFacts()
				if len(facts) == 0 {
					t.Fatal("no allocation proofs")
				}
				for _, fact := range facts {
					if fact.Knowledge != sema.AllocationFree || fact.HasUnknown {
						t.Fatal("fixture has no allocation-free proof", fact)
					}
					node, _ := graph.Node(fact.Callable)
					allowed[node.Name] = true
				}
				ll, err := llvm.GenerateAnalyzed(program, a, triple)
				if err != nil {
					t.Fatal(err)
				}
				auditCalls(t, ll, allowed, regexp.MustCompile(`(?m)^\s*(?:%[^=\s]+\s*=\s*)?call\s+[^\n@]*@([A-Za-z_][A-Za-z_0-9.]*)\(`))
				verifyLLVM(t, ll, triple)
				legacy, err := mlir.GenerateAnalyzed(program, a, triple)
				if err != nil && filepath.Base(file) == "scalars.sec" && legacy == "" && strings.Contains(err.Error(), "return representation for char") {
					// The maintained legacy MLIR path must reject unsupported
					// character transport before producing any runtime substitute.
				} else if err != nil {
					t.Fatal(err)
				} else {
					auditCalls(t, legacy, allowed, regexp.MustCompile(`(?m)^\s*(?:%[^=\s]+\s*=\s*)?llvm\.call\s+@([A-Za-z_][A-Za-z_0-9.]*)\(`))
				}
				module, err := semantic.Build(program, a, semantic.BuildOptions{RequestedModule: "main"})
				if err != nil {
					t.Fatal(err)
				}
				text, err := secmlir.Emit(module, plan)
				if err != nil {
					t.Fatal(err)
				}
				physical := map[string]bool{}
				for index := range module.Functions {
					physical["sec_fn_"+strconv.Itoa(index)] = true
				}
				auditCalls(t, string(text), physical, regexp.MustCompile(`callee\s*=\s*@([A-Za-z_][A-Za-z_0-9.]*)`))
				t.Run("physical-sec-mlir", func(t *testing.T) {
					tool := testsupport.RequireSecMLIROptPath(t)
					path := filepath.Join(t.TempDir(), "free.mlir")
					if err := os.WriteFile(path, text, 0600); err != nil {
						t.Fatal(err)
					}
					output, err := exec.Command(tool, path, "--sec-lower-trivial-core", "--sec-lower-integer-core", "--sec-lower-scalar-core", "--verify-each").CombinedOutput()
					if err != nil {
						t.Fatalf("%v\n%s", err, output)
					}
					auditCalls(t, string(output), physical, regexp.MustCompile(`(?m)^\s*(?:%[^=\s]+\s*=\s*)?(?:llvm\.)?call\s+@([A-Za-z_][A-Za-z_0-9.]*)\(`))
				})
			})
		}
	}
}

// auditCalls accepts only proven source callees. Declaration/operation checks
// also cover empty-call artifacts; no allocator-name heuristic is used.
// Rules: rules/memory/allocation.md — §§3(5)-(6),30(6).
func auditCalls(t *testing.T, text string, allowed map[string]bool, pattern *regexp.Regexp) {
	t.Helper()
	if err := allocationAudit(text, allowed, pattern); err != nil {
		t.Fatalf("%v\n%s", err, text)
	}
}

// allocationAudit rejects calls without a source-proven target and dynamic
// allocation operations. Unrecognized indirect/quoted calls fail closed.
// Rules: rules/memory/allocation.md — §§3(5)-(6),24(6),30(6).
func allocationAudit(text string, allowed map[string]bool, pattern *regexp.Regexp) error {
	calls := pattern.FindAllStringSubmatch(text, -1)
	operations := regexp.MustCompile(`(?m)^\s*(?:%[^=\s]+\s*=\s*)?(?:(?:tail |musttail )?(?:call|invoke|callbr)\b|(?:llvm|func)\.call\b|"sec\.call\.[^"]+")`).FindAllString(text, -1)
	if len(calls) != len(operations) {
		return fmt.Errorf("call without a resolved source-proof target")
	}
	for _, call := range calls {
		if !allowed[call[1]] {
			return fmt.Errorf("hidden runtime/helper call %s", call[1])
		}
	}
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "declare ") || strings.Contains(line, "call void asm") || strings.Contains(line, `"sec.alloc`) || regexp.MustCompile(`\bmemref\.alloc\(`).MatchString(line) {
			return fmt.Errorf("unexpected runtime operation %s", line)
		}
	}
	return nil

}

// TestAllocationAuditRejectsHiddenCalls prevents a future helper from evading
// the conformance check through unfamiliar names, quoting or indirect calls.
// A similarly named ordinary source function remains legal by proof identity.
// Rules: rules/memory/allocation.md — §§3(5)-(6),24(6),30(6).
func TestAllocationAuditRejectsHiddenCalls(t *testing.T) {
	pattern := regexp.MustCompile(`(?m)^\s*(?:%[^=\s]+\s*=\s*)?call\s+[^\n@]*@([A-Za-z_][A-Za-z_0-9.]*)\(`)
	allowed := map[string]bool{"Copy": true, "malloc": true}
	for _, output := range []string{
		"  call void @new_backend_storage_helper()",
		"  call void @\"new-backend-storage-helper\"()",
		"  call void %function()",
		"declare ptr @unexpected_allocator(i64)",
		`  %0 = "sec.alloc.dynamic"() : () -> !llvm.ptr`,
	} {
		if err := allocationAudit(output, allowed, pattern); err == nil {
			t.Fatal("hidden operation escaped audit", output)
		}
	}
	if err := allocationAudit("  %0 = call i32 @malloc()\n  %1 = call i32 @Copy()", allowed, pattern); err != nil {
		t.Fatal("source-proven names treated as allocator heuristics", err)
	}
}

// TestAllocationFreeBorrowBoundary verifies allocation-free borrow/discard
// facts independently from backend support. Unsupported representations must
// produce no artifact instead of repairing the borrow with heap storage.
// Rules: rules/memory/allocation.md — §§2(5),4,30(6).
func TestAllocationFreeBorrowBoundary(t *testing.T) {
	file := "../../../testdata/sema/allocation_free_borrow_lowering.sec"
	data, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	p := parser.New(lexer.NewWithFile(string(data), file))
	program := p.ParseProgram()
	if len(p.Errors()) != 0 {
		t.Fatal(p.Errors())
	}
	plan, err := targetplan.Plan("x86_64-pc-linux-gnu")
	if err != nil {
		t.Fatal(err)
	}
	a := sema.NewAnalyzerWithScalarPlan(plan)
	if errors := a.Analyze(program); len(errors) != 0 {
		t.Fatal(errors)
	}
	allowed := map[string]bool{"Borrow": true, "main": true}
	for _, fact := range a.AllocationFacts() {
		if fact.Knowledge != sema.AllocationFree || fact.HasUnknown {
			t.Fatal(fact)
		}
	}
	ll, err := llvm.GenerateAnalyzed(program, a, plan.LLVMTriple)
	if err == nil {
		auditCalls(t, ll, allowed, regexp.MustCompile(`(?m)^\s*(?:%[^=\s]+\s*=\s*)?call\s+[^\n@]*@([A-Za-z_][A-Za-z_0-9.]*)\(`))
		verifyLLVM(t, ll, plan.LLVMTriple)
	} else if ll != "" || !strings.Contains(strings.ToLower(err.Error()), "ref") {
		t.Fatal(ll, err)
	}
	legacy, err := mlir.GenerateAnalyzed(program, a, plan.LLVMTriple)
	if err == nil {
		auditCalls(t, legacy, allowed, regexp.MustCompile(`(?m)^\s*(?:%[^=\s]+\s*=\s*)?llvm\.call\s+@([A-Za-z_][A-Za-z_0-9.]*)\(`))
	} else if legacy != "" || !strings.Contains(strings.ToLower(err.Error()), "ref") {
		t.Fatal(legacy, err)
	}
	module, err := semantic.Build(program, a, semantic.BuildOptions{RequestedModule: "main"})
	if err == nil {
		_, err = secmlir.Emit(module, plan)
	}
	if err == nil || !strings.Contains(strings.ToLower(err.Error()), "ref") {
		t.Fatal("missing explicit reference capability boundary", err)
	}
}

// verifyLLVM requires target-aware LLVM verification rather than accepting
// textual output that might not represent a valid allocation-free artifact.
// Rules: rules/memory/allocation.md — §30(6); rules/compiler/compiler_testing.md — backend verification.
func verifyLLVM(t *testing.T, text, triple string) {
	t.Helper()
	clang, err := exec.LookPath("clang")
	if err != nil {
		t.Skip("clang is unavailable")
	}
	path := filepath.Join(t.TempDir(), "free.ll")
	if err := os.WriteFile(path, []byte(text), 0600); err != nil {
		t.Fatal(err)
	}
	if out, err := exec.Command(clang, "-target", triple, "-x", "ir", "-c", path, "-o", filepath.Join(t.TempDir(), "free.o")).CombinedOutput(); err != nil {
		t.Fatalf("%v\n%s", err, out)
	}
}
