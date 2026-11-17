package secmlir

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"testing"

	"sec/internal/ir/semantic"
	"sec/internal/layout"
	"sec/internal/lexer"
	"sec/internal/parser"
	"sec/internal/sema"
	"sec/internal/testsupport"
)

// P17-49: foreign resource ownership is managed by ordinary Sec wrappers. The
// foreign call receives only copyable values, never the owner; the wrapper
// consumes its owning parameter and implements the foreign transfer protocol
// (here: a rejected resource is handed back as Some(owner)). The caller's
// consuming argument transfers the owner, which is then unavailable, and every
// path destroys it exactly once (schema-13 ownership and cleanup verifiers).
// Rules: rules/platform/ffi.md — §§ 35–38; rules/memory/ownership.md — § 30;
// rules/memory/destruction.md — §§ 9.2, 22;
// rules/corrections/applied/P17-49-ffi-ownership-wrapper-implementation-20261109-20261117.md — § 2.
func TestSchema13FFIWrapperOwnership(t *testing.T) {
	tool := testsupport.RequireSecMLIROptPath(t)
	source, err := os.ReadFile("../../../testdata/semantic_ir/ffi_wrapper_ownership.sec")
	if err != nil {
		t.Fatal(err)
	}
	for _, width := range []uint16{32, 64} {
		t.Run(fmt.Sprint(width), func(t *testing.T) {
			parsed := parser.New(lexer.NewWithFile(string(source), "ffi.sec")).Parse()
			if parsed.HasErrors {
				t.Fatal(parsed.Diagnostics)
			}
			analyzer := sema.NewAnalyzerWithScalarPlan(layout.ResolvedScalarPlan{PointerWidthBits: width})
			if errors := analyzer.Analyze(parsed.Program); len(errors) != 0 {
				t.Fatal(errors)
			}
			module, err := semantic.Build(parsed.Program, analyzer, semantic.BuildOptions{RequestedModule: "main", SourceFiles: []string{"ffi.sec"}, MaxPackage: 17})
			if err != nil {
				t.Fatal(err)
			}
			if err := semantic.Verify(module); err != nil {
				t.Fatal(err)
			}
			text := semantic.Format(module)
			body := func(name string) string {
				start := strings.Index(text, `func "main::`+name+`(`)
				if start < 0 {
					t.Fatalf("missing function %s", name)
				}
				end := strings.Index(text[start:], "\n  }\n")
				return text[start : start+end]
			}
			give := body("Give")
			for _, want := range []string{"cleanup.track-owned %0", `call.foreign "main::foreign_adopt(int32)"(%1[copy-trivial])`, "cleanup.cancel action=#1 reason=moved"} {
				if !strings.Contains(give, want) {
					t.Errorf("Give lacks %q:\n%s", want, give)
				}
			}
			for _, line := range strings.Split(give, "\n") {
				if strings.Contains(line, "call.foreign") && strings.Contains(line, "[move]") {
					t.Errorf("the foreign call consumes an owner: %s", line)
				}
			}
			use := body("Use")
			for _, want := range []string{"reason=transferred", `call.direct "main::Give(main::Handle)"`, "[move]"} {
				if !strings.Contains(use, want) {
					t.Errorf("Use lacks %q:\n%s", want, use)
				}
			}
			output, err := EmitWithOptions(module, testPlan(width), EmitOptions{DialectVersion: 13})
			if err != nil {
				t.Fatal(err)
			}
			verify := exec.Command(tool, "--sec-verify-places", "--sec-verify-cleanups", "--sec-verify-destruction-plans", "--sec-verify-ownership", "-", "-o", os.DevNull)
			verify.Stdin = bytes.NewReader(output)
			if result, err := verify.CombinedOutput(); err != nil {
				t.Fatalf("%v\n%s", err, result)
			}
		})
	}

	// The transferred owner is unavailable to the caller afterwards.
	reuse := strings.Replace(string(source), "        Some(back) => return 1", "        Some(back) => return int(handle.token)", 1)
	parsed := parser.New(lexer.NewWithFile(reuse, "ffi_reuse.sec")).Parse()
	errors := sema.NewAnalyzerWithScalarPlan(layout.ResolvedScalarPlan{PointerWidthBits: 64}).Analyze(parsed.Program)
	found := false
	for _, err := range errors {
		found = found || strings.Contains(err.Message, "value handle was consumed by call") && strings.Contains(err.Message, "no longer available")
	}
	if !found {
		t.Fatalf("use of the transferred owner was accepted: %v", errors)
	}
}
