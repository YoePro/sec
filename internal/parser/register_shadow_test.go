package parser

import (
	"strings"
	"testing"

	"sec/internal/ast"
	"sec/internal/lexer"
)

// registers.md § 11.5: `shadow` / `noshadow` follow the register header
// modifiers as a register default and follow a field's access modifier as a
// field override; a repeated or contradictory policy is rejected.
func TestParseRegisterShadowModifiers(t *testing.T) {
	input := `
type Configuration register[16] msb-first shadow {
	Mode: bit[3],
	CurrentInput: bit read-only noshadow,
	Events: bit[4] read-clear shadow,
	shadow: bit,
	_: bit[7],
}
`
	p := New(lexer.New(input))
	program := p.ParseProgram()
	checkParserErrors(t, p)
	register := program.Statements[0].(*ast.TypeDeclStatement).RegisterType
	if register.Shadow != "shadow" || register.AllocationOrder != "msb-first" {
		t.Fatalf("register header = shadow %q order %q", register.Shadow, register.AllocationOrder)
	}
	want := []struct {
		name, shadow string
		access       ast.RegisterFieldAccess
	}{{"Mode", "", ast.RegisterReadWrite}, {"CurrentInput", "noshadow", ast.RegisterReadOnly}, {"Events", "shadow", ast.RegisterReadClear}, {"shadow", "", ast.RegisterReadWrite}}
	for index, expected := range want {
		field := register.Fields[index]
		if field.Name.Value != expected.name || field.Shadow != expected.shadow || field.Access != expected.access {
			t.Fatalf("field %d = %s shadow %q access %q, want %+v", index, field.Name.Value, field.Shadow, field.Access, expected)
		}
	}
}

func TestParseRegisterShadowModifierConflicts(t *testing.T) {
	for source, want := range map[string]string{
		"type R register[8] shadow noshadow {\n\tA: bit[8],\n}\n":   `contradictory register shadow modifiers "shadow" and "noshadow"`,
		"type R register[8] shadow shadow {\n\tA: bit[8],\n}\n":     `duplicate register shadow modifier "shadow"`,
		"type R register[8] {\n\tA: bit[8] shadow noshadow,\n}\n":   `contradictory register field A shadow modifiers`,
		"type R register[8] {\n\tA: bit[8] noshadow noshadow,\n}\n": `duplicate register field A shadow modifier "noshadow"`,
		"type R register[8] {\n\tA: bit[8] shadow read-only,\n}\n":  `the access modifier precedes the shadow modifier`,
	} {
		p := New(lexer.New(source))
		p.ParseProgram()
		if errors := strings.Join(p.Errors(), "\n"); !strings.Contains(errors, want) {
			t.Fatalf("%q: errors %q, want %q", source, errors, want)
		}
	}
}
