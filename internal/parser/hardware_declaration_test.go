package parser

import (
	"strings"
	"testing"

	"sec/internal/ast"
	"sec/internal/diagnostics"
	"sec/internal/lexer"
)

// MD-111 § 8: contextual `hardware region|resource|endpoint` declarations
// parse into restricted metadata members, while `hardware` stays an ordinary
// identifier everywhere else.
// Rules: rules/foundations/grammar.md — HardwareDeclaration (Amendment MD-111).
func TestParseHardwareDeclarations(t *testing.T) {
	parser := New(lexer.New(`module main

hardware region PeripheralSpace {
    domain: "cpu-physical",
    kind: HardwareRegionKind.MMIO,
    base: 0x40000000,
    size: 0x10000,
    readable: true,
    writable: false,
}

hardware resource GpioOutputState {
    width: 32
}

hardware endpoint GpioOutput {
    region: drivers.PeripheralSpace,
    read_widths: [32, 16,],
    write_widths: [],
}

fn hardware(hardware: int) int {
    return hardware
}
`))
	program := parser.ParseProgram()
	if errors := parser.Errors(); len(errors) != 0 {
		t.Fatal(errors)
	}
	var declarations []*ast.HardwareDeclaration
	for _, statement := range program.Statements {
		if declaration, ok := statement.(*ast.HardwareDeclaration); ok {
			declarations = append(declarations, declaration)
		}
	}
	if len(declarations) != 3 {
		t.Fatalf("declarations = %d", len(declarations))
	}
	region := declarations[0]
	if region.Category != "region" || region.Name.Value != "PeripheralSpace" || region.Invalid || len(region.Members) != 6 {
		t.Fatalf("region = %s", region)
	}
	want := map[string]string{"domain": `"cpu-physical"`, "kind": "HardwareRegionKind.MMIO", "base": "0x40000000", "size": "0x10000", "readable": "true", "writable": "false"}
	for _, member := range region.Members {
		if member.Value.String() != want[member.Name.Value] {
			t.Errorf("%s = %s", member.Name.Value, member.Value)
		}
	}
	if region.Members[0].Value.Kind != ast.HardwareValueString || region.Members[0].Value.Text != "cpu-physical" || region.Members[1].Value.Kind != ast.HardwareValuePath || region.Members[2].Value.Kind != ast.HardwareValueInteger || region.Members[4].Value.Kind != ast.HardwareValueBool {
		t.Fatalf("region value kinds = %+v", region.Members)
	}
	if declarations[1].Category != "resource" || len(declarations[1].Members) != 1 {
		t.Fatalf("resource = %s", declarations[1])
	}
	endpoint := declarations[2]
	if len(endpoint.Members) != 3 || len(endpoint.Members[0].Value.Path) != 2 || endpoint.Members[1].Value.Kind != ast.HardwareValueList || len(endpoint.Members[1].Value.Elements) != 2 || len(endpoint.Members[2].Value.Elements) != 0 {
		t.Fatalf("endpoint = %s", endpoint)
	}
}

// Malformed metadata reports focused P2036 diagnostics, keeps parsing the
// remaining members and the following declarations, and marks the
// declaration invalid.
func TestHardwareDeclarationRecovery(t *testing.T) {
	parser := New(lexer.New(`module main

hardware region Bad {
    base: -1,
    size: 4 + 4,
    domain "x",
    readable: [true],
    kind: HardwareRegionKind.,
    writable: true
}

hardware endpoint {
}

hardware resource Ok {
    width: 8,
}
`))
	program := parser.ParseProgram()
	var messages []string
	for _, diagnostic := range parser.Diagnostics() {
		if diagnostic.ID != diagnostics.ParserInvalidHardwareDeclaration {
			t.Fatalf("unexpected diagnostic %+v", diagnostic)
		}
		messages = append(messages, diagnostic.Message)
	}
	joined := strings.Join(messages, "\n")
	for _, want := range []string{"not `-`", "separated by commas", "member domain requires `: value`", "lists contain only integer literals", "requires a name after `.`", "hardware endpoint declaration requires a name"} {
		if !strings.Contains(joined, want) {
			t.Errorf("missing %q in\n%s", want, joined)
		}
	}
	var bad, ok *ast.HardwareDeclaration
	for _, statement := range program.Statements {
		if declaration, is := statement.(*ast.HardwareDeclaration); is && declaration.Name != nil {
			switch declaration.Name.Value {
			case "Bad":
				bad = declaration
			case "Ok":
				ok = declaration
			}
		}
	}
	if bad == nil || !bad.Invalid || len(bad.Members) != 2 || bad.Members[0].Name.Value != "size" || bad.Members[1].Name.Value != "writable" {
		t.Fatalf("bad = %+v", bad)
	}
	if ok == nil || ok.Invalid || len(ok.Members) != 1 {
		t.Fatalf("following declaration not recovered: %+v", ok)
	}
}
