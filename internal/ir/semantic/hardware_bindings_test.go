package semantic

import (
	"math/big"
	"os"
	"strings"
	"testing"
)

func hardwareBindingModule(t *testing.T) *Module {
	t.Helper()
	source, err := os.ReadFile("../../../testdata/semantic_ir/hardware_binding_root.sec")
	if err != nil {
		t.Fatal(err)
	}
	module, err := buildPackage15Source(t, string(source), "x86_64-pc-linux-gnu")
	if err != nil {
		t.Fatal(err)
	}
	return module
}

// MD-077 §§ 12(2), 19(5): a hardware register binding becomes one
// register.hw.bind root over HardwareInstance[T] with its resolved endpoint,
// resource, region, domain extent and transfer widths, and no static or Place.
func TestHardwareBindingRoot(t *testing.T) {
	module := hardwareBindingModule(t)
	if err := Verify(module); err != nil {
		t.Fatal(err)
	}
	if len(module.HardwareBindings) != 1 || len(module.Statics) != 0 {
		t.Fatalf("bindings=%#v statics=%#v", module.HardwareBindings, module.Statics)
	}
	binding := module.HardwareBindings[0]
	if binding.Symbol != "main::control" || !binding.Mutable || binding.EndpointKind != "MemoryMapped" || binding.Address.Cmp(big.NewInt(0x40000010)) != 0 || binding.Size.Int64() != 4 {
		t.Fatalf("binding = %#v", binding)
	}
	text := Format(module)
	for _, want := range []string{"hardware-instance<!", `= register.hw.bind !`, `endpoint="test:sample::ControlPort"`, `resource="test:sample::ControlState"`, `region="test:sample::Space"`, `domain="cpu-physical" address=0x40000010 size=4 alignment=4 reads=[32] writes=[32]`} {
		if !strings.Contains(text, want) {
			t.Fatalf("missing %q:\n%s", want, text)
		}
	}
}

// The verifier rejects a binding whose instance type, identities or transfer
// widths no longer match the resolved facts.
func TestHardwareBindingVerifierRejectsForgedRoots(t *testing.T) {
	for name, mutate := range map[string]func(*Module){
		"instance type": func(m *Module) { m.HardwareBindings[0].Instance = m.HardwareBindings[0].Register },
		"no register":   func(m *Module) { m.Registers = nil },
		"no endpoint":   func(m *Module) { m.HardwareBindings[0].Endpoint = "" },
		"endpoint kind": func(m *Module) { m.HardwareBindings[0].EndpointKind = "PortMapped" },
		"width":         func(m *Module) { m.HardwareBindings[0].WriteWidths = []*big.Int{big.NewInt(64)} },
		"identity":      func(m *Module) { m.HardwareBindings[0].ID = 2 },
		"duplicate": func(m *Module) {
			second := m.HardwareBindings[0]
			second.ID = 2
			m.HardwareBindings = append(m.HardwareBindings, second)
		},
	} {
		t.Run(name, func(t *testing.T) {
			module := hardwareBindingModule(t)
			mutate(module)
			if err := Verify(module); err == nil {
				t.Fatal("forged hardware binding verified")
			}
		})
	}
}

// Receiver facts keep the register layout, so an impl method of a register
// with a reserved field builds the same complete register definition.
func TestRegisterReceiverKeepsReservedFields(t *testing.T) {
	module, err := buildPackage15Source(t, `module main
type Control register[32] {
    Enabled: bit,
    _: bit[31],
}
impl Control {
    fn Enable() void { self.Enabled = true }
}
fn main() int {
    return 0
}
`, "")
	if err != nil {
		t.Fatal(err)
	}
	if err := Verify(module); err != nil {
		t.Fatal(err)
	}
	if len(module.Registers) != 1 || len(module.Registers[0].Fields) != 2 || !module.Registers[0].Fields[1].Reserved {
		t.Fatalf("registers = %#v", module.Registers)
	}
	if !strings.Contains(Format(module), "register.value.update") {
		t.Fatal(Format(module))
	}
}

// MD-077 §§ 9, 12: members called on a bound instance run their hardware
// variants on the lent instance authority. Field reads observe, writes
// request (shadow_intent for a shadowed field), Read()/Write()/Write(value)
// address the whole register, IsShadowKnown queries shadow knowledge, and
// a hardware-only member has no software body. No request has a verified
// physical plan yet.
func TestHardwareOperationsLowerThroughHardwareVariants(t *testing.T) {
	source, err := os.ReadFile("../../../testdata/semantic_ir/hardware_operations.sec")
	if err != nil {
		t.Fatal(err)
	}
	module, err := buildPackage15Source(t, string(source), "x86_64-pc-linux-gnu")
	if err != nil {
		t.Fatal(err)
	}
	if err := Verify(module); err != nil {
		t.Fatal(err)
	}
	text := Format(module)
	for _, want := range []string{
		`call.direct "main::Control.Both[hardware]()"(%0[hardware-instance])`,
		`= register.hw.bind #1 : `,
		`call.direct "main::Control.Enable[hardware]()"(%0[hardware-instance])`,
		`register.hw.request_write %0, %1 field=#1 request=write-value shadow=noshadow proof=constant plan=unverified`,
		`register.hw.shadow_intent %0, %1 field=#2 request=write-value shadow=shadow proof=interval plan=unverified`,
		`register.hw.commit %0 whole request=commit plan=unverified`,
		`register.hw.observe %0 field=#0 request=observe shadow=noshadow plan=unverified`,
		`register.hw.shadow_known %0 field=#2 request=shadow-known shadow=shadow plan=unverified`,
		`register.hw.observe %0 whole request=observe plan=unverified`,
		`register.hw.shadow_observe %0, %1 whole request=observe plan=unverified`,
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("missing %q:\n%s", want, text)
		}
	}
	for _, function := range module.Functions {
		for _, hardwareOnly := range []string{"main::Control.Apply()", "main::Control.Snapshot()", "main::Control.Known()"} {
			if string(function.ID) == hardwareOnly {
				t.Fatalf("hardware-only member %s has a software body", hardwareOnly)
			}
		}
	}
}

// The verifier rejects hardware operations that lost their instance,
// request, plan state or field/type agreement.
func TestHardwareOperationVerifierRejectsForgedOperations(t *testing.T) {
	source, err := os.ReadFile("../../../testdata/semantic_ir/hardware_operations.sec")
	if err != nil {
		t.Fatal(err)
	}
	find := func(module *Module, kind OpKind) *Operation {
		for _, function := range module.Functions {
			for _, block := range function.Blocks {
				for index := range block.Operations {
					if block.Operations[index].Kind == kind {
						return &block.Operations[index]
					}
				}
			}
		}
		t.Fatalf("no %s", kind)
		return nil
	}
	for name, mutate := range map[string]func(*Module){
		"verified plan claim": func(m *Module) { find(m, OpHardwareObserve).HardwareRequest.Plan = "verified" },
		"missing request":     func(m *Module) { find(m, OpHardwareRequestWrite).HardwareRequest = nil },
		"reserved field":      func(m *Module) { find(m, OpHardwareObserve).RegisterField = 3 },
		"shadow as request":   func(m *Module) { find(m, OpHardwareShadowIntent).Kind = OpHardwareRequestWrite },
		"write as intent":     func(m *Module) { find(m, OpHardwareRequestWrite).Kind = OpHardwareShadowIntent },
		"unknown binding":     func(m *Module) { find(m, OpHardwareBind).HardwareBinding = 9 },
		"commit of a field":   func(m *Module) { find(m, OpHardwareCommit).HardwareRequest.Whole = false },
	} {
		t.Run(name, func(t *testing.T) {
			module, err := buildPackage15Source(t, string(source), "x86_64-pc-linux-gnu")
			if err != nil {
				t.Fatal(err)
			}
			mutate(module)
			if err := Verify(module); err == nil {
				t.Fatal("forged hardware operation verified")
			}
		})
	}
}
