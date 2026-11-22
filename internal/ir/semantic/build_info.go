package semantic

import (
	"math/big"

	"sec/internal/ast"
	"sec/internal/sema"
)

// buildBuildInfoCurrent materializes BuildInfo.Current() as a statically
// constant struct.construct of the plan's snapshot: strings and uint64
// fields are constants, Option fields an empty None or a Some of a constant.
// No runtime environment is inspected and nothing is allocated.
// Rules: rules/compiler/compiler_known_members.md — § 113B(2);
// rules/library/core-library.md — § 28; rules/compiler/semantic_ir.md — §§ 23, 35.
func (fb *functionBuilder) buildBuildInfoCurrent(call *ast.CallExpression, snapshot sema.BuildInfoSnapshot) (builtValue, error) {
	typ, typed := fb.owner.analyzer.ResolvedTypeOf(call)
	if !typed || !sema.IsCoreBuildInfoType(typ) {
		return builtValue{}, fb.unsupported("BuildInfo.Current() without its core.BuildInfo type", call.Token)
	}
	typeID, err := fb.owner.internType(typ)
	if err != nil {
		return builtValue{}, err
	}
	loc := location(call.Token)
	strings := map[string]string{
		"ProjectUUID": snapshot.ProjectUUID, "ProjectName": snapshot.ProjectName, "TargetName": snapshot.TargetName,
		"VariantName": snapshot.VariantName, "VersionRendered": snapshot.VersionRendered,
		"OperatingSystem": snapshot.OperatingSystem, "Architecture": snapshot.Architecture,
		"BuildProfile": snapshot.BuildProfile, "TargetProfile": snapshot.TargetProfile,
	}
	numbers := map[string]uint64{
		"VersionMajor": snapshot.VersionMajor, "VersionMinor": snapshot.VersionMinor,
		"VersionRevision": snapshot.VersionRevision, "VersionBuild": snapshot.VersionBuild,
	}
	options := map[string]any{"VersionUserDefined": snapshot.VersionUserDefined, "SourceRevision": snapshot.SourceRevision, "DirtySourceStatus": snapshot.DirtySourceStatus}
	op := Operation{Kind: OpStructConstruct, Location: loc}
	for _, field := range typ.Fields {
		fieldType, err := fb.owner.internType(field.Type)
		if err != nil {
			return builtValue{}, err
		}
		var value builtValue
		if text, ok := strings[field.Name]; ok {
			value = fb.result(Operation{Kind: OpConstString, String: text, Location: loc}, fieldType)
		} else if number, ok := numbers[field.Name]; ok {
			value = fb.result(Operation{Kind: OpConstInt, Integer: new(big.Int).SetUint64(number), Location: loc}, fieldType)
		} else if present, ok := options[field.Name]; ok {
			value, err = fb.buildBuildInfoOption(present, fieldType, call)
			if err != nil {
				return builtValue{}, err
			}
		} else {
			return builtValue{}, fb.unsupported("BuildInfo field "+field.Name, call.Token)
		}
		op.Operands = append(op.Operands, value.id)
		op.StructOrigins = append(op.StructOrigins, StructOriginExplicit)
		op.StructActions = append(op.StructActions, StructActionCopyTrivial)
	}
	return fb.result(op, typeID), nil
}

// buildBuildInfoOption constructs None, or Some of the present constant.
// Rules: rules/compiler/compiler_known_members.md — § 113B(2); rules/compiler/semantic_ir.md — § 23.
func (fb *functionBuilder) buildBuildInfoOption(present any, optionType TypeID, call *ast.CallExpression) (builtValue, error) {
	definition, ok := fb.owner.unionDefinition(optionType)
	if !ok {
		return builtValue{}, fb.unsupported("BuildInfo Option field without its union definition", call.Token)
	}
	want, payload := "None", Operation{}
	switch value := present.(type) {
	case *string:
		if value != nil {
			want, payload = "Some", Operation{Kind: OpConstString, String: *value}
		}
	case *bool:
		if value != nil {
			flag := *value
			want, payload = "Some", Operation{Kind: OpConstBool, Bool: &flag}
		}
	}
	for _, variant := range definition.Variants {
		if variant.Name != want {
			continue
		}
		op := Operation{Kind: OpUnionConstruct, UnionVariant: variant.Index, Location: location(call.Token)}
		if want == "Some" {
			if variant.Kind != UnionVariantSingle {
				break
			}
			payload.Location = op.Location
			inner := fb.result(payload, variant.Payload)
			op.Operands = []ValueID{inner.id}
			op.PayloadActions = []UnionPayloadAction{UnionPayloadCopyTrivial}
		}
		return fb.result(op, optionType), nil
	}
	return builtValue{}, fb.unsupported("BuildInfo Option field variant "+want, call.Token)
}
