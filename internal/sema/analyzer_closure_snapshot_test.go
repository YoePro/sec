package sema

import (
	"math/big"
	"os"
	"reflect"
	"testing"

	"sec/internal/ast"
	"sec/internal/lexer"
	"sec/internal/parser"
)

// TestClosureSnapshotsDetachPlaceTypes exercises both public capture query
// surfaces with resolved reference and aggregate-contained referent facts.
// Rules: rules/analysis/closure_analysis.md — "Capture record", "Dependencies";
// rules/compiler/compiler_analysis.md — §55 "Analysis snapshot consistency".
func TestClosureSnapshotsDetachPlaceTypes(t *testing.T) {
	source, err := os.ReadFile("../../testdata/sema/closure_snapshot_types_valid.sec")
	if err != nil {
		t.Fatal(err)
	}
	p := parser.New(lexer.NewWithFile(string(source), "closure_snapshot_types_valid.sec"))
	parsed := p.Parse()
	if parsed.HasErrors {
		t.Fatalf("parse: %v", p.Errors())
	}
	analyzer := NewAnalyzer()
	if errors := analyzer.Analyze(parsed.Program); len(errors) != 0 {
		t.Fatalf("sema: %v", errors)
	}
	var lambda *ast.LambdaExpression
	for expression := range analyzer.resolvedLambdaCaptures {
		lambda = expression
	}
	if lambda == nil {
		t.Fatal("no closure captures recorded")
	}
	queries := []struct {
		name string
		get  func() []CaptureRecord
	}{
		{"captures", func() []CaptureRecord {
			records, ok := analyzer.ResolvedLambdaCapturesOf(lambda)
			if !ok {
				t.Fatal("missing capture snapshot")
			}
			return records
		}},
		{"creation", func() []CaptureRecord {
			summary, ok := analyzer.ResolvedClosureCreationOf(lambda)
			if !ok {
				t.Fatal("missing creation snapshot")
			}
			return summary.Captures
		}},
	}
	for _, query := range queries {
		t.Run(query.name, func(t *testing.T) {
			snapshot := query.get()
			if len(snapshot) != 2 {
				t.Fatalf("captures: %#v", snapshot)
			}
			for _, record := range snapshot {
				// SourcePlace and CapturedType must retain structural type facts
				// while being independent of each other and every earlier query.
				switch record.Name {
				case "view":
					if record.SourcePlace.Type.Element.Fields[0].Name != "values" {
						t.Fatal("reference source type was corrupted")
					}
					record.SourcePlace.Type.Element.Fields[0].Name = "changed"
					record.SourcePlace.Type.Element.Fields[0].Type.Element.Name = "changed"
					if record.CapturedType.Element.Fields[0].Name != "values" {
						t.Fatal("source and captured types alias")
					}
				case "holder":
					if record.SourcePlace.Type.Fields[0].Name != "view" {
						t.Fatal("carrier source type was corrupted")
					}
					record.SourcePlace.Type.Fields[0].Name = "changed"
					record.SourcePlace.Type.Fields[0].Type.Element.Fields[0].Name = "changed"
				}
				if (record.Name == "view" && len(record.Dependencies.ReferentPlaces) == 0) || len(record.Dependencies.Referents) == 0 {
					t.Fatalf("missing referents for %s", record.Name)
				}
				for _, place := range record.Dependencies.ReferentPlaces {
					if place.Type.Fields[0].Name != "values" {
						t.Fatal("referent type was corrupted")
					}
					place.Type.Fields[0].Name = "changed"
				}
				for _, dependency := range record.Dependencies.Referents {
					for _, place := range dependency.ReferentPlaces {
						if place.Type.Fields[0].Name != "values" {
							t.Fatal("nested referent type was corrupted")
						}
						place.Type.Fields[0].Name = "changed"
					}
				}
			}
			fresh := query.get()
			for _, record := range fresh {
				if record.Name == "view" && record.SourcePlace.Type.Element.Fields[0].Name != "values" {
					t.Fatal("reference source mutation reached analyzer")
				}
				if record.Name == "holder" && record.SourcePlace.Type.Fields[0].Name != "view" {
					t.Fatal("carrier source mutation reached analyzer")
				}
				for _, place := range record.Dependencies.ReferentPlaces {
					if place.Type.Fields[0].Name != "values" {
						t.Fatal("referent mutation reached analyzer")
					}
				}
				for _, dependency := range record.Dependencies.Referents {
					for _, place := range dependency.ReferentPlaces {
						if place.Type.Fields[0].Name != "values" {
							t.Fatal("nested referent mutation reached analyzer")
						}
					}
				}
			}
			creation, _ := analyzer.ResolvedClosureCreationOf(lambda)
			captures, _ := analyzer.ResolvedLambdaCapturesOf(lambda)
			if !reflect.DeepEqual(creation.Captures, captures) {
				t.Fatal("public snapshot surfaces disagree")
			}
		})
	}
}

// TestClosureSnapshotsDetachAlternativeTypes verifies the less common type
// surfaces and nested origin sets on both public closure snapshot APIs.
// Rules: rules/analysis/closure_analysis.md — "SourcePlace", "CapturedType", "Dependencies";
// rules/compiler/compiler_analysis.md — §55 "Analysis snapshot consistency".
func TestClosureSnapshotsDetachAlternativeTypes(t *testing.T) {
	integer := Type{Name: "int", Kind: IntType}
	payload := Type{Name: "Data", Kind: StructType, Named: true,
		Fields: []StructField{{Name: "value", Type: integer, Tags: []StructTag{{Key: "wire", Value: "data"}}}},
	}
	packet := Type{Name: "Packet", Kind: UnionType, Named: true,
		TypeArgs: []Type{NewFixedArrayType(integer, big.NewInt(2))},
		UnionVariants: []UnionVariant{{Name: "Item", Payload: &payload,
			PayloadFields: []StructField{{Name: "value", Type: payload, Tags: []StructTag{{Key: "wire", Value: "item"}}}},
		}},
	}
	index, _ := new(big.Int).SetString("18446744073709551616", 10)
	place := Place{Root: "primary", Type: packet,
		Projections: []PlaceProjection{{Kind: PlaceIndex, ConstantIndex: index}},
		AlternativeOrigins: []Place{{Root: "alternative", Type: packet,
			AlternativeOrigins: []Place{{Root: "nested", Type: packet}},
		}},
	}
	record := CaptureRecord{Name: "value", SourcePlace: place, CapturedType: packet,
		Dependencies: CaptureDependencies{ReferentPlaces: []Place{place},
			Referents:       []CaptureReferentDependency{{CarrierPath: ".view", ReferentPlaces: []Place{place}}},
			CallableTargets: CallableTargetSet{KnownTargets: []CallableBodyID{"body"}, IsClosed: true},
		},
	}
	lambda := &ast.LambdaExpression{}
	analyzer := NewAnalyzer()
	analyzer.resolvedLambdaCaptures = map[*ast.LambdaExpression][]CaptureRecord{lambda: {record}}
	analyzer.resolvedClosureCreations = map[*ast.LambdaExpression]ClosureCreationSummary{lambda: {Captures: []CaptureRecord{record}}}
	getters := []func() []CaptureRecord{
		func() []CaptureRecord { records, _ := analyzer.ResolvedLambdaCapturesOf(lambda); return records },
		func() []CaptureRecord {
			summary, _ := analyzer.ResolvedClosureCreationOf(lambda)
			return summary.Captures
		},
	}
	mutateType := func(typ *Type) {
		typ.TypeArgs[0].Element.Name = "changed"
		typ.UnionVariants[0].Payload.Fields[0].Tags[0].Value = "changed"
		typ.UnionVariants[0].PayloadFields[0].Type.Fields[0].Name = "changed"
		typ.UnionVariants[0].PayloadFields[0].Tags[0].Value = "changed"
	}
	for _, get := range getters {
		snapshot := get()
		before := get()
		mutateType(&snapshot[0].CapturedType)
		places := []*Place{&snapshot[0].SourcePlace, &snapshot[0].Dependencies.ReferentPlaces[0], &snapshot[0].Dependencies.Referents[0].ReferentPlaces[0]}
		for _, target := range places {
			mutateType(&target.Type)
			target.Projections[0].ConstantIndex.SetInt64(0)
			if target.AlternativeOrigins[0].AlternativeOrigins[0].Root != "nested" {
				t.Fatal("nested alternative was dropped")
			}
			mutateType(&target.AlternativeOrigins[0].Type)
			mutateType(&target.AlternativeOrigins[0].AlternativeOrigins[0].Type)
		}
		snapshot[0].Dependencies.CallableTargets.KnownTargets[0] = "changed"
		if !reflect.DeepEqual(before, get()) {
			t.Fatal("snapshot mutation changed an earlier snapshot or analyzer fact")
		}
		if payload.Fields[0].Tags[0].Value != "data" || packet.TypeArgs[0].Element.Name != "int" || index.String() != "18446744073709551616" {
			t.Fatal("snapshot mutation changed compiler-owned type or projection")
		}
	}
	// Named reference recursion terminates at the same scalar surface used
	// for CapturedType while retaining independently owned outer structure.
	node := Type{Name: "Node", Named: true, Kind: StructType}
	node.Fields = []StructField{{Name: "next", Type: Type{Kind: ReferenceType, Element: &node}}}
	analyzer.resolvedLambdaCaptures[lambda] = []CaptureRecord{{SourcePlace: Place{Type: node}, CapturedType: node}}
	recursive, _ := analyzer.ResolvedLambdaCapturesOf(lambda)
	if len(recursive[0].SourcePlace.Type.Fields) != 1 || recursive[0].SourcePlace.Type.Fields[0].Type.Element.Name != "Node" {
		t.Fatal("recursive type surface was lost")
	}
	recursive[0].SourcePlace.Type.Fields[0].Name = "changed"
	if node.Fields[0].Name != "next" {
		t.Fatal("recursive type snapshot aliases its source")
	}
}
