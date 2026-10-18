package sema

// cloneSliceCreationPlan detaches every mutable endpoint/type/origin fact.
// Rules: rules/mlir/packages/sec-mlir-dialect_package16.md — §§ 25, 37.
func cloneSliceCreationPlan(plan ResolvedSliceCreationPlan) ResolvedSliceCreationPlan {
	plan.ElementType = semanticSnapshotType(plan.ElementType)
	plan.SourceSliceType = semanticSnapshotType(plan.SourceSliceType)
	plan.SourcePlace = cloneSlicePlanPlace(plan.SourcePlace)
	plan.SourceLength = clonePlaceConstantIndex(plan.SourceLength)
	plan.Start.Type, plan.End.Type = semanticSnapshotType(plan.Start.Type), semanticSnapshotType(plan.End.Type)
	plan.Start.Constant, plan.End.Constant = clonePlaceConstantIndex(plan.Start.Constant), clonePlaceConstantIndex(plan.End.Constant)
	if plan.StaticRange != nil {
		plan.StaticRange = &StaticSliceRange{clonePlaceConstantIndex(plan.StaticRange.Start), clonePlaceConstantIndex(plan.StaticRange.EndExclusive)}
	}
	plan.ReferenceFacts = cloneSlicePlanReference(plan.ReferenceFacts)
	plan.ErrorType = cloneSlicePlanErrorType(plan.ErrorType)
	return plan
}

// cloneSlicePlanPlace detaches the typed origin tree, including index types;
// callers cannot change a plan's nested types through a Place returned earlier.
// Rules: rules/mlir/packages/sec-mlir-dialect_package16.md — § 37.
func cloneSlicePlanPlace(place Place) Place {
	place.Projections = clonePlaceProjections(place.Projections)
	place.AlternativeOrigins = append([]Place(nil), place.AlternativeOrigins...)
	place.Type = semanticSnapshotType(place.Type)
	for index := range place.Projections {
		place.Projections[index].IndexType = semanticSnapshotType(place.Projections[index].IndexType)
	}
	for index := range place.AlternativeOrigins {
		place.AlternativeOrigins[index] = cloneSlicePlanPlace(place.AlternativeOrigins[index])
	}
	return place
}

// cloneSlicePlanReference preserves the entire finite origin tree before
// detaching it, rather than passing through P15's flat-origin clone.
// Rules: rules/mlir/packages/sec-mlir-dialect_package16.md — §§25,32,37,51.
func cloneSlicePlanReference(plan ResolvedReferencePlan) ResolvedReferencePlan {
	plan.Place = cloneSlicePlanPlace(plan.Place)
	plan.Facts.Referent = semanticSnapshotType(plan.Facts.Referent)
	plan.Facts.StorageIdentities = append([]StorageIdentityID(nil), plan.Facts.StorageIdentities...)
	origins := make([]Place, len(plan.Facts.OriginPlaces))
	for index, origin := range plan.Facts.OriginPlaces {
		origins[index] = cloneSlicePlanPlace(origin)
	}
	plan.Facts.OriginPlaces = origins
	return plan
}

// cloneSlicePlanErrorType preserves canonical enum identity and detaches its
// exact variant values for public slice plan queries.
// Rules: rules/mlir/packages/sec-mlir-dialect_package16.md — §§37,51,53.
func cloneSlicePlanErrorType(typ Type) Type {
	errorType := semanticSnapshotType(typ)
	errorType.EnumConsts = make(map[string]EnumValue, len(typ.EnumConsts))
	for name, value := range typ.EnumConsts {
		value.Value = clonePlaceConstantIndex(value.Value)
		if value.StringValue != nil {
			text := *value.StringValue
			value.StringValue = &text
		}
		errorType.EnumConsts[name] = value
	}
	return errorType
}
