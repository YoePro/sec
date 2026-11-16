package sema

import "math/big"

// snapshotArenaElementDefault detaches the canonical semantic default, including
// compact array repetitions and nested field/payload values. It does not choose
// physical byte initialization or rerun default resolution in a public query.
// Rules: rules/mlir/packages/sec-mlir-dialect_package19.md — §§31,73,76;
// rules/types/default_values.md — §§18,28, "Compile-time resolution".
func snapshotArenaElementDefault(value DefaultResolution) DefaultResolution {
	value.Value = snapshotArenaDefaultConstant(value.Value)
	if value.Fields != nil {
		value.Fields = append([]DefaultField{}, value.Fields...)
		for i := range value.Fields {
			value.Fields[i].Value = snapshotArenaElementDefault(value.Fields[i].Value)
		}
	}
	if value.Elements != nil {
		value.Elements = append([]DefaultResolution{}, value.Elements...)
		for i := range value.Elements {
			value.Elements[i] = snapshotArenaElementDefault(value.Elements[i])
		}
	}
	if value.ArrayElementDefault != nil {
		element := snapshotArenaElementDefault(*value.ArrayElementDefault)
		value.ArrayElementDefault = &element
	}
	if value.Payload != nil {
		payload := snapshotArenaElementDefault(*value.Payload)
		value.Payload = &payload
	}
	return value
}

// snapshotArenaDefaultConstant retains exact constants and CTE aggregate values
// without sharing their mutable integers, rationals, maps or payloads.
// Rules: rules/mlir/packages/sec-mlir-dialect_package19.md — §§31,76;
// rules/types/default_values.md — §§7,18, "Compile-time resolution".
func snapshotArenaDefaultConstant(value DefaultConstant) DefaultConstant {
	value.Integer = clonePlaceConstantIndex(value.Integer)
	if value.Exact != nil {
		value.Exact = new(big.Rat).Set(value.Exact)
	}
	if value.Aggregate != nil {
		aggregate := *value.Aggregate
		aggregate.Type = semanticSnapshotType(aggregate.Type)
		aggregate.Names = append([]string(nil), aggregate.Names...)
		if aggregate.Elements != nil {
			aggregate.Elements = append([]DefaultConstant{}, aggregate.Elements...)
			for i := range aggregate.Elements {
				aggregate.Elements[i] = snapshotArenaDefaultConstant(aggregate.Elements[i])
			}
		}
		if aggregate.Fields != nil {
			aggregate.Fields = make(map[string]DefaultConstant, len(value.Aggregate.Fields))
			for name, field := range value.Aggregate.Fields {
				aggregate.Fields[name] = snapshotArenaDefaultConstant(field)
			}
		}
		if aggregate.Payload != nil {
			payload := snapshotArenaDefaultConstant(*aggregate.Payload)
			aggregate.Payload = &payload
		}
		value.Aggregate = &aggregate
	}
	return value
}
