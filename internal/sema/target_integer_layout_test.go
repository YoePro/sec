package sema

import (
	"math/big"
	"os"
	"testing"

	"sec/internal/layout"
	"sec/internal/lexer"
	"sec/internal/parser"
	"sec/internal/platform/target"
)

// Sema bounds and the shared physical layout resolver agree for signed/unsigned
// target integers, nominal carriers and aggregates under every concrete target.
// A shared layout cache must retain each target's width and ABI independently.
// Rules: rules/types/types.md — "int and uint"; rules/memory/layout.md —
// §§18(1)–(3),26,47; rules/platform/target_profiles.md — §40.
func TestTargetIntegerLayoutConsistency(t *testing.T) {
	data, err := os.ReadFile("../../testdata/sema/target_integer_layout.sec")
	if err != nil {
		t.Fatal(err)
	}
	cache := layout.NewLayoutCache()
	checked := 0
	for _, definition := range target.Definitions() {
		if definition.LLVMTriple == "" {
			continue
		}
		checked++
		t.Run(definition.OS+"/"+definition.Arch, func(t *testing.T) {
			plan, err := definition.ScalarPlan()
			if err != nil {
				t.Fatal(err)
			}
			p := parser.New(lexer.NewWithFile(string(data), "target_integer_layout.sec"))
			program := p.ParseProgram()
			if len(p.Errors()) != 0 {
				t.Fatal(p.Errors())
			}
			a := NewAnalyzerWithScalarPlan(plan)
			if errors := a.Analyze(program); len(errors) != 0 {
				t.Fatal(errors)
			}
			resolver, err := layout.NewResolverWithCache(plan, plan.Alignments, cache)
			if err != nil {
				t.Fatal(err)
			}
			for _, name := range []string{"int", "uint", "Signed", "Unsigned", "int32", "uint64"} {
				typ := a.types[name]
				carrier := typ
				for carrier.NamedBase != nil {
					carrier = *carrier.NamedBase
				}
				bits := plan.PointerWidthBits
				if name == "int32" {
					bits = 32
				}
				if name == "uint64" {
					bits = 64
				}
				max := new(big.Int).Lsh(big.NewInt(1), uint(bits))
				min := new(big.Int)
				if carrier.Kind == IntType {
					max.Rsh(max, 1)
					min.Neg(max)
				}
				max.Sub(max, big.NewInt(1))
				if carrier.MinInteger == nil || carrier.MaxInteger == nil || carrier.MinInteger.Cmp(min) != 0 || carrier.MaxInteger.Cmp(max) != 0 {
					t.Fatalf("%s Sema bounds %v..%v, want %v..%v", name, carrier.MinInteger, carrier.MaxInteger, min, max)
				}
				resolved := resolver.Resolve(LayoutRequirementOf(typ))
				if !resolved.Complete() || resolved.SizeBytes.Int64() != int64(bits/8) || resolved.AlignBytes.Uint64() != plan.Alignments[layout.StorageRepresentation{Class: layout.IntegerStorage, WidthBits: bits}] {
					t.Fatalf("%s layout %+v", name, resolved)
				}
			}
			pair := resolver.Resolve(LayoutRequirementOf(a.types["NativePair"]))
			if !pair.Complete() || pair.SizeBytes.Int64() != int64(plan.PointerWidthBits/4) || pair.Fields[1].Offset.Int64() != int64(plan.PointerWidthBits/8) {
				t.Fatalf("native aggregate %+v", pair)
			}
		})
	}
	if checked == 0 {
		t.Fatal("no concrete targets checked")
	}
}
