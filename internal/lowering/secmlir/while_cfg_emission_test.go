package secmlir

import (
	"bytes"
	"os"
	"os/exec"
	"sec/internal/ir/semantic"
	"sec/internal/layout"
	"sec/internal/lexer"
	"sec/internal/parser"
	"sec/internal/sema"
	"sec/internal/testsupport"
	"strconv"
	"strings"
	"testing"
)

// Verified loop records are discharged into explicit standard cf branches;
// their enter/condition/back/continue/break edges survive in the single track.
// Mutating a loop record cannot bypass verification before its metadata erasure.
// Rules: rules/compiler/semantic_ir.md §§66–67;
// rules/control-flow/flowcontrol_while.md §§4,13–15,29;
// rules/compiler/compiler_pipeline.md §§32–34.
func TestWhileCFGEmission(t *testing.T) {
	tool := testsupport.RequireSecMLIROptPath(t)
	source, err := os.ReadFile("../../../testdata/semantic_ir/while_cfg_emission.sec")
	if err != nil {
		t.Fatal(err)
	}
	for _, bits := range []uint16{32, 64} {
		t.Run(strconv.Itoa(int(bits)), func(t *testing.T) {
			p := parser.New(lexer.NewWithFile(string(source), "while.sec"))
			program := p.ParseProgram()
			if len(p.Errors()) != 0 {
				t.Fatal(p.Errors())
			}
			a := sema.NewAnalyzerWithScalarPlan(layout.ResolvedScalarPlan{PointerWidthBits: bits})
			if errors := a.Analyze(program); len(errors) != 0 {
				t.Fatal(errors)
			}
			module, err := semantic.Build(program, a, semantic.BuildOptions{RequestedModule: "main", SourceFiles: []string{"while.sec"}, MaxPackage: 17})
			if err != nil {
				t.Fatal(err)
			}
			records := 0
			edges := map[semantic.LoopEdge]bool{}
			for _, fn := range module.Functions {
				records += len(fn.Loops)
				for _, block := range fn.Blocks {
					for _, op := range block.Operations {
						if op.LoopID != 0 {
							edges[op.LoopEdge] = true
						}
					}
				}
			}
			if records != 3 {
				t.Fatalf("got %d loop records", records)
			}
			for _, edge := range []semantic.LoopEdge{semantic.LoopEdgeEnter, semantic.LoopEdgeCondition, semantic.LoopEdgeBack, semantic.LoopEdgeContinue, semantic.LoopEdgeBreak} {
				if !edges[edge] {
					t.Fatalf("missing loop edge %s", edge)
				}
			}
			output, err := EmitWithOptions(module, testPlan(bits), EmitOptions{DialectVersion: 13})
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(string(output), "cf.br") || !strings.Contains(string(output), "cf.cond_br") {
				t.Fatal("loop CFG not emitted")
			}
			cmd := exec.Command(tool, "-", "-o", os.DevNull)
			cmd.Stdin = bytes.NewReader(output)
			if out, err := cmd.CombinedOutput(); err != nil {
				t.Fatalf("%v: %s", err, out)
			}
			for _, fn := range module.Functions {
				if len(fn.Loops) > 0 {
					fn.Loops[0].ConditionBlock = 999999
					if emitted, err := EmitWithOptions(module, testPlan(bits), EmitOptions{DialectVersion: 13}); err == nil || len(emitted) != 0 {
						t.Fatalf("forged loop evidence accepted: %v", err)
					}
					break
				}
			}
		})
	}
}
