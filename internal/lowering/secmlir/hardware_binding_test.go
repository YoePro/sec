package secmlir

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"strings"
	"testing"

	"sec/internal/codegen/targetplan"
	"sec/internal/ir/semantic"
	"sec/internal/lexer"
	"sec/internal/modules"
	"sec/internal/parser"
	"sec/internal/sema"
	"sec/internal/testsupport"
)

// MD-077 § 19(5), dialect v11.41.5: a register.hw.bind root is emitted as a
// module-level sec.hw.binding record with every resolved identity and the exact extent,
// and verifies natively at schemas 11/12.
func TestEmitHardwareBindingRoot(t *testing.T) {
	module := buildHardwareFixture(t, "hardware_binding_root.sec")
	tool := testsupport.RequireSecMLIROptPath(t)
	for _, schema := range []uint32{11, 12} {
		output, err := EmitWithOptions(module, testPlan(64), EmitOptions{DialectVersion: schema})
		if err != nil {
			t.Fatal(schema, err)
		}
		text := string(output)
		for _, want := range []string{`"sec.hw.binding"() <{address = "1073741840", alias = false, alignment = "4", domain = "cpu-physical", endpoint_id = "test:binding::ControlPort", endpoint_kind = "MemoryMapped", hardware_binding_id = 1 : i32, hardware_resource_id = "test:binding::ControlState", instance_type = !sec.hw_instance<"main::Control">, is_mutable = true, read_widths = [32], region_id = "test:binding::Space", size = "4", sym_name = "main::control", write_widths = [32]}>`, `sec.registers = [`} {
			if !strings.Contains(text, want) {
				t.Fatalf("schema %d missing %q:\n%s", schema, want, text)
			}
		}
		command := exec.Command(tool, "-")
		command.Stdin = bytes.NewReader(output)
		canonical, err := command.CombinedOutput()
		if err != nil || !strings.Contains(string(canonical), "sec.hw.binding") {
			t.Fatalf("native rejected the binding root: %v\n%s\n%s", err, canonical, output)
		}
	}
}

func buildHardwareFixture(t *testing.T, name string) *semantic.Module {
	t.Helper()
	source, err := os.ReadFile("../../../testdata/semantic_ir/" + name)
	if err != nil {
		t.Fatal(err)
	}
	p := parser.New(lexer.NewWithFile(string(source), "binding.sec"))
	parsed := p.Parse()
	if parsed.HasErrors {
		t.Fatal(p.Errors())
	}
	plan, err := targetplan.Plan("x86_64-pc-linux-gnu")
	if err != nil {
		t.Fatal(err)
	}
	analyzer := sema.NewAnalyzerWithScalarPlan(plan)
	analyzer.SetHardwareModuleContext(sema.HardwareModuleContext{Files: map[string]modules.ModuleIdentity{"binding.sec": {ImportRootIdentity: "test", CanonicalImportPath: "binding"}}})
	if errs := analyzer.Analyze(parsed.Program); len(errs) != 0 {
		t.Fatal(errs)
	}
	module, err := semantic.Build(parsed.Program, analyzer, semantic.BuildOptions{RequestedModule: "main", SourceFiles: []string{"binding.sec"}, MaxPackage: 15})
	if err != nil {
		t.Fatal(err)
	}
	return module
}

// MD-077 § 11(4): register.hw.* operations without a verified physical plan
// publish no Sec MLIR.
func TestEmitRejectsUnverifiedHardwareOperations(t *testing.T) {
	module := buildHardwareFixture(t, "hardware_operations.sec")
	output, err := Emit(module, testPlan(64))
	var unsupported *UnsupportedLoweringError
	if len(output) != 0 || !errors.As(err, &unsupported) || !strings.Contains(unsupported.Feature, "needs a verified MD-111 operation plan") {
		t.Fatalf("output=%q err=%v", output, err)
	}
}
