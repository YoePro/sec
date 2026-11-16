package sema

import (
	"fmt"
	"math/big"
	"os"
	"reflect"
	"testing"

	"sec/internal/layout"
	"sec/internal/lexer"
	"sec/internal/parser"
)

// TestResolvedStaticVariantInitializers verifies typed active variants from
// executed Sec, nominal payload-free constants and static dependencies. Exact
// payloads and nested type metadata must be detached from the analyzer.
// Rules: rules/compiler/compile_time_evaluation.md — §32(1–4), §47;
// rules/declarations/static.md — §§15–16.
func TestResolvedStaticVariantInitializers(t *testing.T) {
	data, err := os.ReadFile("../../testdata/sema/static_variants_valid.sec")
	if err != nil {
		t.Fatal(err)
	}
	for _, width := range []uint16{32, 64} {
		for _, depth := range []AnalysisDepth{AnalysisInteractive, AnalysisStandard, AnalysisDeep} {
			t.Run(fmt.Sprintf("%d/%s", width, depth), func(t *testing.T) {
				p := parser.New(lexer.NewWithFile(string(data), "static_variants_valid.sec"))
				program := p.ParseProgram()
				if len(p.Errors()) != 0 {
					t.Fatal(p.Errors())
				}
				a := NewAnalyzerWithScalarPlanAndDepth(layout.ResolvedScalarPlan{PointerWidthBits: width}, depth)
				if errors := a.Analyze(program); len(errors) != 0 {
					t.Fatal(errors)
				}
				want := map[string]string{"Nested": "Ok", "Failure": "Err", "Record": "Record", "NumericValue": "Number", "Text": "Text", "Empty": "Empty", "Absent": "None", "VoidSuccess": "Ok", "Copy": "Number", "Value": "Number"}
				for name, variant := range map[string]string{"InlineSome": "Some", "InlineNone": "None", "InlineOk": "Ok", "InlineErr": "Err", "InlineNumber": "Number", "InlineRecord": "Record", "BytesValue": "Bytes"} {
					want[name] = variant
				}
				for _, binding := range a.ResolvedStaticDeclarations() {
					original := cloneStaticInitializer(binding.Initializer)
					variant := binding.Initializer.Variant
					if variant == nil || variant.Name != want[binding.Name] || canonicalTypeIdentity(variant.Owner) != canonicalTypeIdentity(binding.Type) {
						t.Fatalf("%s: missing canonical static variant: %+v", binding.Name, variant)
					}
					switch binding.Name {
					case "Nested":
						if variant.PayloadType == nil || !isOptionType(*variant.PayloadType) || variant.Payload == nil || variant.Payload.Variant == nil || variant.Payload.Variant.Name != "Some" {
							t.Fatal("nested Result/Option lost", variant)
						}
						point := variant.Payload.Variant
						if point.PayloadType == nil || point.PayloadType.Name != "Point" || point.Payload == nil || len(point.Payload.Components) != 2 || point.Payload.Components[0].Integer.Int64() != 42 || !*point.Payload.Components[1].Bool {
							t.Fatal("nested struct payload lost", point)
						}
						point.Payload.Components[0].Integer.SetInt64(0)
						point.PayloadType.Fields[0].Name = "corrupt"
					case "Failure", "InlineErr":
						if variant.PayloadType == nil || variant.PayloadType.Name != "AppError" || variant.Payload == nil || variant.Payload.Scalar == nil || variant.Payload.Scalar.Nominal.Member != "Bad" {
							t.Fatal("application error identity lost", variant)
						}
						for name, constant := range variant.PayloadType.EnumConsts {
							if constant.Value != nil {
								constant.Value.SetInt64(999)
							}
							delete(variant.PayloadType.EnumConsts, name)
						}
					case "Record", "InlineRecord":
						if variant.PayloadType != nil || len(variant.PayloadFields) != 2 || variant.PayloadFields[0].Name != "Point" || variant.Payload == nil || variant.Payload.Components[1].Integer.Int64() != 7 {
							t.Fatal("named field payload lost", variant)
						}
						variant.PayloadFields[0].Type.Name = "corrupt"
					case "NumericValue", "Copy", "Value", "InlineNumber":
						if variant.PayloadType == nil || variant.PayloadType.Kind != IntType || variant.Payload == nil || variant.Payload.Integer.Int64() != 9 {
							t.Fatal("numeric payload lost", variant)
						}
						variant.Payload.Integer.SetInt64(0)
					case "Text":
						if variant.Payload == nil || variant.Payload.Scalar == nil || variant.Payload.Scalar.String != "Sec" || variant.Payload.ScalarType.Kind != StringType {
							t.Fatal("string payload lost", variant)
						}
					case "BytesValue":
						if variant.PayloadType == nil || variant.PayloadType.Kind != ArrayType || variant.Payload == nil || len(variant.Payload.Components) != 2 || variant.Payload.Components[1].Integer.Int64() != 2 {
							t.Fatal("fixed-array payload lost", variant)
						}
						variant.Payload.Components[1].Integer.SetInt64(0)
					case "InlineSome", "InlineOk":
						if variant.PayloadType == nil || variant.Payload == nil || variant.Payload.Integer == nil {
							t.Fatal("direct tagged payload lost", variant)
						}
					case "Empty", "Absent", "VoidSuccess", "InlineNone":
						if variant.Payload != nil {
							t.Fatal("payload-free value acquired a payload", variant)
						}
					}
					variant.Name = "corrupt"
					variant.Owner.Name = "corrupt"
					again, _ := a.ResolvedStaticOf(binding.Declaration.Name)
					if !reflect.DeepEqual(original, again.Initializer) {
						t.Fatal("mutable static variant snapshot", binding.Name)
					}
					delete(want, binding.Name)
				}
				if len(want) != 0 {
					t.Fatal("missing statics", want)
				}
			})
		}
	}
}

// Invalid source publishes no variant facts, and exact scalar payload snapshots
// cannot share mutable rational state with the established CTE value.
// Rules: rules/compiler/compile_time_evaluation.md — §§32,36,47;
// rules/declarations/static.md — §§15–16.
func TestStaticVariantInitializerFailureAndExactSnapshots(t *testing.T) {
	data, err := os.ReadFile("../../testdata/sema/static_variants_invalid.sec")
	if err != nil {
		t.Fatal(err)
	}
	p := parser.New(lexer.NewWithFile(string(data), "static_variants_invalid.sec"))
	program := p.ParseProgram()
	if len(p.Errors()) != 0 {
		t.Fatal(p.Errors())
	}
	a := NewAnalyzer()
	if errors := a.Analyze(program); len(errors) == 0 {
		t.Fatal("invalid variant source accepted")
	}
	for _, binding := range a.ResolvedStaticDeclarations() {
		if binding.Initializer.Variant != nil {
			t.Fatal("invalid source published a static variant", binding.Name)
		}
	}
	value := DefaultConstant{Kind: DecimalType, Exact: big.NewRat(1, 10)}
	initializer, ok := staticInitializerOf(Type{Kind: DecimalType, Name: "decimal"}, value)
	if !ok || initializer.Scalar == nil || initializer.Scalar.Exact.Cmp(value.Exact) != 0 {
		t.Fatal("exact scalar not preserved")
	}
	copy := cloneStaticInitializer(initializer)
	copy.Scalar.Exact.SetInt64(7)
	if value.Exact.Cmp(big.NewRat(1, 10)) != 0 || initializer.Scalar.Exact.Cmp(value.Exact) != 0 {
		t.Fatal("exact scalar snapshot aliases CTE storage")
	}
}
