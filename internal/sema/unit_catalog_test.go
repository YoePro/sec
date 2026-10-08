package sema

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"sec/internal/ast"
	"sec/internal/lexer"
	"sec/internal/parser"
)

// TestUnitCatalogKinds consumes real catalog declarations and metadata, isolating
// catalog identity from helper-function bodies and their independent dependencies.
// Rules: rules/types/units.md — Kind; implicit and explicit unit conversion.
func TestUnitCatalogKinds(t *testing.T) {
	files, err := filepath.Glob("../../sec/stdlib/unit/*.sec")
	if err != nil {
		t.Fatal(err)
	}
	program := parser.New(lexer.New("module main\n")).ParseProgram()
	for _, path := range files {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		// Project declaration/metadata lines before parsing: legacy helper bodies
		// contain obsolete bracket-dimension syntax unrelated to Kind metadata.
		var metadata strings.Builder
		depth := 0
		for _, line := range strings.Split(string(data), "\n") {
			trimmed := strings.TrimSpace(line)
			if depth == 0 && (strings.HasPrefix(trimmed, "unit ") || strings.HasPrefix(trimmed, "impl ")) || depth == 1 && !strings.HasPrefix(trimmed, "fn ") && !strings.Contains(line, "{") {
				metadata.WriteString(line + "\n")
			}
			depth += strings.Count(line, "{") - strings.Count(line, "}")
		}
		p := parser.New(lexer.NewWithFile(metadata.String(), path))
		parsed := p.ParseProgram()
		if len(p.Errors()) != 0 {
			t.Fatalf("%s: %v", path, p.Errors())
		}
		for _, s := range parsed.Statements {
			switch node := s.(type) {
			case *ast.UnitDeclStatement:
				program.Statements = append(program.Statements, node)
			case *ast.ImplStatement:
				copy := *node
				copy.Members = nil
				for _, member := range node.Members {
					if _, ok := member.(*ast.UnitMetadataDeclaration); ok {
						copy.Members = append(copy.Members, member)
					}
				}
				program.Statements = append(program.Statements, &copy)
			}
		}
	}
	a := NewAnalyzer()
	errors := a.Analyze(program)
	if len(errors) != 0 {
		t.Fatal(errors)
	}
	families := map[string]string{"frequency": "Hz kHz MHz GHz", "rotational_frequency": "rpm", "angular_velocity": "radps rad_s", "angular_acceleration": "radps2", "plane_angle": "rad deg", "solid_angle": "sr", "energy": "J Wh kJ MJ GJ kWh", "torque": "Nm", "power": "W kW MW GW", "apparent_power": "VA", "reactive_power": "var", "radioactivity": "Bq", "absorbed_dose": "Gy", "dose_equivalent": "Sv", "luminous_flux": "lm", "illuminance": "lx phot fc", "luminance": "nit stilb"}
	for kind, names := range families {
		for _, name := range strings.Fields(names) {
			if a.units[name].Kind != kind {
				t.Errorf("%s: %s want %s", name, a.units[name].Kind, kind)
			}
		}
	}
	for _, pair := range [][2]string{{"Hz", "rpm"}, {"J", "Nm"}, {"Gy", "Sv"}, {"W", "VA"}, {"lx", "nit"}, {"rad", "sr"}} {
		for _, source := range []string{
			"fn Bad(value: decimal<" + pair[0] + ">) decimal<" + pair[1] + "> { return value }\n",
			"fn Add(a: decimal<" + pair[0] + ">, b: decimal<" + pair[1] + ">) decimal<" + pair[0] + "> { return a + b }\n",
			"fn Compare(a: decimal<" + pair[0] + ">, b: decimal<" + pair[1] + ">) bool { return a == b }\n",
		} {
			p := parser.New(lexer.New(source))
			consumer := p.ParseProgram()
			if len(p.Errors()) != 0 {
				t.Fatal(p.Errors())
			}
			combined := &ast.Program{Statements: append(append([]ast.Statement{}, program.Statements...), consumer.Statements...)}
			errors := NewAnalyzer().Analyze(combined)
			if len(errors) == 0 {
				t.Fatalf("implicit %s to %s accepted", pair[0], pair[1])
			}
			if !strings.Contains(errors[0].Message, pair[0]) || !strings.Contains(errors[0].Message, pair[1]) {
				t.Fatal(errors)
			}
		}
	}
	for _, source := range []string{
		"fn Same(value: decimal<Hz>) decimal<kHz> { return value }",
		"fn Explicit(value: decimal<J>) decimal<Nm> { return Nm(value) }",
	} {
		consumer := parser.New(lexer.New(source)).ParseProgram()
		combined := &ast.Program{Statements: append(append([]ast.Statement{}, program.Statements...), consumer.Statements...)}
		if errors := NewAnalyzer().Analyze(combined); len(errors) != 0 {
			t.Fatal(errors)
		}
	}
}
