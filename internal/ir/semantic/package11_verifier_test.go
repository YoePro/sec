package semantic

import (
	"math/big"
	"strings"
	"testing"
)

func TestPackage11EnumDefinitionVerifierMatrix(t *testing.T) {
	tests := []struct {
		name      string
		mutate    func(*Module)
		wantError string
	}{
		{
			name: "numeric alias accepted",
			mutate: func(module *Module) {
				module.Enums[0].Cases[1].Value.Set(module.Enums[0].Cases[0].Value)
			},
		},
		{
			name: "duplicate case ID",
			mutate: func(module *Module) {
				module.Enums[0].Cases[1].ID = module.Enums[0].Cases[0].ID
			},
			wantError: "invalid enum case",
		},
		{
			name: "duplicate case name",
			mutate: func(module *Module) {
				module.Enums[0].Cases[1].Name = module.Enums[0].Cases[0].Name
			},
			wantError: "invalid enum case",
		},
		{
			name: "signed value too large",
			mutate: func(module *Module) {
				module.Enums[0].Cases[1].Value.SetInt64(128)
			},
			wantError: "not representable",
		},
		{
			name: "signed value too small",
			mutate: func(module *Module) {
				module.Enums[0].Cases[0].Value.SetInt64(-129)
			},
			wantError: "not representable",
		},
		{
			name: "bit width zero",
			mutate: func(module *Module) {
				module.Enums[0].RepresentationKind = EnumRepresentationBitBacked
				module.Enums[0].BitWidth = 0
			},
			wantError: "invalid width",
		},
		{
			name: "unknown representation",
			mutate: func(module *Module) {
				module.Enums[0].RepresentationKind = EnumRepresentationKind("packed")
			},
			wantError: "invalid representation",
		},
		{
			name: "bit width 257",
			mutate: func(module *Module) {
				module.Enums[0].RepresentationKind = EnumRepresentationBitBacked
				module.Enums[0].BitWidth = 257
			},
			wantError: "invalid width",
		},
		{
			name: "bit backing must be unsigned",
			mutate: func(module *Module) {
				module.Enums[0].RepresentationKind = EnumRepresentationBitBacked
				module.Enums[0].BitWidth = 8
			},
			wantError: "requires an unsigned underlying type",
		},
		{
			name: "bit backing width must match",
			mutate: func(module *Module) {
				module.Enums[0].RepresentationKind = EnumRepresentationBitBacked
				module.Enums[0].BitWidth = 7
				underlying := module.Types.Intern(Type{Kind: TypeUint, Name: "bit[8]", BitWidth: 8})
				module.Enums[0].Underlying = underlying
				typ, _ := module.Types.Lookup(module.Enums[0].TypeID)
				typ.Underlying = underlying
				module.Types.types[module.Enums[0].TypeID-1] = typ
			},
			wantError: "requires an unsigned underlying type",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			module := package11EnumModule(8, true)
			test.mutate(module)
			err := Verify(module)
			if test.wantError == "" {
				if err != nil {
					t.Fatal(err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), test.wantError) {
				t.Fatalf("error = %v, want %q", err, test.wantError)
			}
		})
	}
}

func TestPackage11BitBackedBoundaryWidths(t *testing.T) {
	for _, width := range []uint16{1, 256} {
		module := package11BitBackedEnumModule(width)
		module.Enums[0].RepresentationKind = EnumRepresentationBitBacked
		module.Enums[0].BitWidth = width
		module.Enums[0].Cases[0].Value.SetInt64(0)
		module.Enums[0].Cases[1].Value.Sub(new(big.Int).Lsh(big.NewInt(1), uint(width)), big.NewInt(1))
		if err := Verify(module); err != nil {
			t.Fatalf("bit[%d]: %v", width, err)
		}
	}
}

func package11BitBackedEnumModule(width uint16) *Module {
	types := NewTypeTable()
	underlying := types.Intern(Type{Kind: TypeUint, Name: "bit", BitWidth: width})
	enumType := types.Intern(Type{Kind: TypeEnum, Name: "Bits", Module: "main", Identity: "main::Bits", Underlying: underlying})
	return &Module{
		Version:  Version,
		Identity: "main",
		Types:    types,
		Enums: []EnumDefinition{{
			TypeID: enumType, SymbolID: "main::Bits", Name: "Bits", Underlying: underlying,
			RepresentationKind: EnumRepresentationBitBacked,
			BitWidth:           width,
			Cases:              []EnumCase{{ID: 0, Name: "low", Value: big.NewInt(0)}, {ID: 1, Name: "high", Value: big.NewInt(1)}},
		}},
	}
}

func TestPackage11UnionDefinitionVerifierMatrix(t *testing.T) {
	tests := []struct {
		name      string
		mutate    func(*Module)
		wantError string
	}{
		{"requires variant", func(module *Module) { module.Unions[0].Variants = nil }, "invalid union definition"},
		{"contiguous indices", func(module *Module) { module.Unions[0].Variants[1].Index = 3 }, "invalid union variant"},
		{"unique names", func(module *Module) { module.Unions[0].Variants[1].Name = "None" }, "invalid union variant"},
		{"empty shape", func(module *Module) { module.Unions[0].Variants[0].Payload = module.Unions[0].Variants[1].Payload }, "has payload"},
		{"single shape", func(module *Module) { module.Unions[0].Variants[1].Payload = 0 }, "invalid payload"},
		{"field shape", func(module *Module) { module.Unions[0].Variants[2].PayloadFields = nil }, "invalid shape"},
		{"duplicate fields", func(module *Module) { module.Unions[0].Variants[2].PayloadFields[1].Name = "x" }, "invalid field"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			module := package11UnionModule()
			test.mutate(module)
			err := Verify(module)
			if err == nil || !strings.Contains(err.Error(), test.wantError) {
				t.Fatalf("error = %v, want %q", err, test.wantError)
			}
		})
	}
}

func package11EnumModule(width uint16, signed bool) *Module {
	types := NewTypeTable()
	underlying := types.Intern(Type{Kind: TypeInt, Name: "int8", Signed: signed, BitWidth: width})
	enumType := types.Intern(Type{Kind: TypeEnum, Name: "Small", Module: "main", Identity: "main::Small", Underlying: underlying})
	return &Module{
		Version:  Version,
		Identity: "main",
		Types:    types,
		Enums: []EnumDefinition{{
			TypeID: enumType, SymbolID: "main::Small", Name: "Small", Underlying: underlying,
			RepresentationKind: EnumRepresentationInteger,
			Cases:              []EnumCase{{ID: 0, Name: "low", Value: big.NewInt(-128)}, {ID: 1, Name: "high", Value: big.NewInt(127)}},
		}},
	}
}

func package11UnionModule() *Module {
	types := NewTypeTable()
	intType := types.Intern(Type{Kind: TypeInt, Name: "int", Signed: true, TargetSize: true})
	unionType := types.Intern(Type{Kind: TypeUnion, Name: "Value", Module: "main", Identity: "main::Value"})
	return &Module{
		Version:  Version,
		Identity: "main",
		Types:    types,
		Unions: []UnionDefinition{{
			TypeID: unionType, SymbolID: "main::Value", Name: "Value", CopyClassification: "trivial", TriviallyDestructible: true,
			Variants: []UnionVariantDefinition{
				{Index: 0, Name: "None", Kind: UnionVariantEmpty},
				{Index: 1, Name: "One", Kind: UnionVariantSingle, Payload: intType},
				{Index: 2, Name: "Point", Kind: UnionVariantFields, PayloadFields: []UnionPayloadField{{Name: "x", Type: intType}, {Name: "y", Type: intType}}},
			},
		}},
	}
}
