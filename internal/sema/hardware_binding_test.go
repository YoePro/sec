package sema

import (
	"strings"
	"testing"

	"sec/internal/ast"
	"sec/internal/diagnostics"
	"sec/internal/layout"
	"sec/internal/lexer"
	"sec/internal/modules"
	"sec/internal/parser"
)

// useHardwareTestTarget prepares a unit-test analyzer for source hardware
// declarations the way the CLI does for a Project source: every source of
// program belongs to one resolver-owned module, and a target without a
// selected plan gets a 64-bit width with the linear cpu-physical domain.
// Programs without hardware declarations are left unchanged.
func useHardwareTestTarget(a *Analyzer, program *ast.Program) {
	if program == nil {
		return
	}
	hasHardware := false
	for _, statement := range program.Statements {
		if _, ok := statement.(*ast.HardwareDeclaration); ok {
			hasHardware = true
			break
		}
	}
	if !hasHardware {
		return
	}
	if a.scalarPlanWidthBits == 0 {
		a.scalarPlanWidthBits = 64
	}
	hasDomain := false
	for _, domain := range a.addressDomains {
		hasDomain = hasDomain || domain.Identity == "cpu-physical"
	}
	if !hasDomain {
		a.addressDomains = append(a.addressDomains, layout.AddressDomain{Identity: "cpu-physical", AddressBits: a.scalarPlanWidthBits, UnitBits: 8, Linear: true})
	}
	module := modules.ModuleIdentity{ImportRootIdentity: "test", CanonicalImportPath: "hardware"}
	context := HardwareModuleContext{Files: map[string]modules.ModuleIdentity{}}
	for _, statement := range program.Statements {
		context.Files[statementToken(statement).File] = module
	}
	a.SetHardwareModuleContext(context)
}

// analyzeWithHardwareTestTarget prepares a with useHardwareTestTarget and
// analyzes program.
func analyzeWithHardwareTestTarget(a *Analyzer, program *ast.Program) []Error {
	useHardwareTestTarget(a, program)
	return a.Analyze(program)
}

func analyzeAddressSource(t *testing.T, source string) []Error {
	t.Helper()
	p := parser.New(lexer.New(source))
	program := p.ParseProgram()
	if len(p.Errors()) > 0 {
		t.Fatal(p.Errors())
	}
	a := NewAnalyzer()
	useHardwareTestTarget(a, program)
	return a.Analyze(program)
}

const addressBindingEndpoints = `
type Control register[32] { Enabled: bit, _: bit[31], }
impl Control { fn Enable() void { self.Enabled = true } }
hardware region Space {
    domain: "cpu-physical",
    kind: HardwareRegionKind.MMIO,
    base: 0x40000000,
    size: 0x10000,
    readable: true,
    writable: true,
}
hardware resource ControlState { width: 32, }
hardware endpoint ControlPort {
    kind: HardwareEndpointKind.MemoryMapped,
    region: Space,
    resource: ControlState,
    offset: 0,
    size: 4,
    alignment: 4,
    read_widths: [32],
    write_widths: [32],
    read_footprint: HardwareFootprintKind.Whole,
    write_footprint: HardwareFootprintKind.Whole,
    read_effect: HardwareReadEffect.NonDestructive,
    write_operation: HardwareWriteOperation.Replace,
    ordering: HardwareOrdering.Volatile,
    completion: HardwareCompletion.Issued,
    atomicity: HardwareAtomicity.Unspecified,
    context: HardwareAccessContext.Current,
    fault: HardwareFaultContract.Platform,
}
`

// TestAddressBindsNamedEndpoint checks the MD-111 § 10(1) binding forms: a
// named source endpoint, with or without the alias marker, binds the
// register; numeric, arithmetic, call and variable forms, non-endpoint
// hardware symbols, unknown names and Project exports are rejected with
// their own identities rather than treated as raw addresses.
// Rules: rules/corrections/MD-111-hardware-facts-approved-decision-20261030.md — §§1(4),10(1),10(7),13(4)–(5).
func TestAddressBindsNamedEndpoint(t *testing.T) {
	for _, binding := range []string{"@address(ControlPort)", "@address(ControlPort, alias)"} {
		source := "module main\n" + addressBindingEndpoints + binding + "\nlet mut control: Control\nfn F() void { control.Enable() }\n"
		if errors := analyzeAddressSource(t, source); len(errors) != 0 {
			t.Fatalf("%s: %v", binding, errors)
		}
	}
	for _, test := range []struct{ name, binding, id, message string }{
		{"numeric", "@address(0x40000000)", diagnostics.HardwareAddressBinding, "not a numeric address"},
		{"arithmetic", "@address(0x40000000 + 4)", diagnostics.HardwareAddressBinding, "not an address expression"},
		{"call", "@address(Base())", diagnostics.HardwareAddressBinding, "not a call"},
		{"region", "@address(Space)", diagnostics.HardwareSymbolResolution, "is region, expected endpoint"},
		{"resource", "@address(ControlState)", diagnostics.HardwareSymbolResolution, "is resource, expected endpoint"},
		{"unknown", "@address(Missing)", diagnostics.HardwareSymbolResolution, "unknown hardware endpoint"},
		{"unknown qualifier", "@address(board.ControlPort)", diagnostics.HardwareSymbolResolution, "imported hardware qualifier"},
		{"project export", "@address(Project.ControlPort)", diagnostics.HardwareContextUnavailable, "Project hardware export"},
	} {
		t.Run(test.name, func(t *testing.T) {
			source := "module main\nfn Base() uint { return 0 }\n" + addressBindingEndpoints + test.binding + "\nlet mut control: Control\n"
			errors := analyzeAddressSource(t, source)
			if len(errors) != 1 || errors[0].ID != test.id || !strings.Contains(errors[0].Message, test.message) {
				t.Fatalf("%s: %v", test.binding, errors)
			}
		})
	}
}
