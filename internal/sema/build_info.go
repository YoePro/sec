package sema

import (
	"math/big"
	"strconv"

	"sec/internal/ast"
	"sec/internal/diagnostics"
)

// BuildInfoSnapshot is the resolved build metadata of one CompilationPlan
// that BuildInfo.Current() exposes (MD-143). Optional facts are nil when
// absent; no field is ever filled from the compiler host.
// Rules: rules/library/core-library.md — § 28; rules/compiler/compiler_known_members.md — § 113B.
type BuildInfoSnapshot struct {
	ProjectUUID, ProjectName, TargetName, VariantName          string
	VersionMajor, VersionMinor, VersionRevision, VersionBuild  uint64
	VersionUserDefined                                         *string
	VersionRendered                                            string
	OperatingSystem, Architecture, BuildProfile, TargetProfile string
	SourceRevision                                             *string
	DirtySourceStatus                                          *bool
}

// SetBuildInfo freezes the plan's build metadata before analysis. Without a
// snapshot every BuildInfo.Current() reports S1258.
// Rules: rules/compiler/compiler_known_members.md — § 113B(1)-(3).
func (a *Analyzer) SetBuildInfo(snapshot BuildInfoSnapshot) {
	copied := cloneBuildInfoSnapshot(snapshot)
	a.recordConfiguration(func(b *Analyzer) { b.SetBuildInfo(copied) })
	a.buildInfo = &copied
}

// BuildInfo returns the configured snapshot, if any.
// Rules: rules/compiler/compiler_known_members.md — § 113B(1).
func (a *Analyzer) BuildInfo() (BuildInfoSnapshot, bool) {
	if a.buildInfo == nil {
		return BuildInfoSnapshot{}, false
	}
	return cloneBuildInfoSnapshot(*a.buildInfo), true
}

// installCompilerKnownBuildInfo gives core.BuildInfo its canonical fields.
// Rules: rules/library/core-library.md — § 28(1)-(3).
func installCompilerKnownBuildInfo(types map[string]Type) {
	option := func(value Type) Type {
		typ := types["Option"]
		typ.TypeArgs = []Type{value}
		if len(typ.GenericParameters) == 1 {
			typ = substituteGenericType(typ, map[string]Type{typ.GenericParameters[0]: value})
			typ.GenericParameters = nil
		}
		return typ
	}
	text, number := types["string"], types["uint64"]
	info := types["BuildInfo"]
	info.Fields = []StructField{
		{Name: "ProjectUUID", Type: text}, {Name: "ProjectName", Type: text}, {Name: "TargetName", Type: text}, {Name: "VariantName", Type: text},
		{Name: "VersionMajor", Type: number}, {Name: "VersionMinor", Type: number}, {Name: "VersionRevision", Type: number}, {Name: "VersionBuild", Type: number},
		{Name: "VersionUserDefined", Type: option(text)}, {Name: "VersionRendered", Type: text},
		{Name: "OperatingSystem", Type: text}, {Name: "Architecture", Type: text}, {Name: "BuildProfile", Type: text}, {Name: "TargetProfile", Type: text},
		{Name: "SourceRevision", Type: option(text)}, {Name: "DirtySourceStatus", Type: option(types["bool"])},
	}
	types["BuildInfo"] = info
}

// inferBuildInfoCurrent types BuildInfo.Current(): no arguments, a BuildInfo
// result, and S1258 when the plan has no resolved Target and Variant.
// Rules: rules/compiler/compiler_known_members.md — § 113B(2)-(3).
func (a *Analyzer) inferBuildInfoCurrent(expr *ast.CallExpression, member CompilerKnownMember) (Type, expressionValue, bool) {
	if !a.checkCompilerKnownCallArity(expr, "BuildInfo.Current", 0, 0) {
		return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}, true
	}
	if a.buildInfo == nil {
		a.addErrorAtTokenWithMetadata(expr.Token, diagnostics.BuildInfoUnavailable,
			"Analyze the source through a Project Target and Variant (sec build <Target>, or a manifest with exactly one Variant for the target).",
			"BuildInfo.Current() needs a CompilationPlan with a resolved Target and Variant")
		return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}, true
	}
	if a.buildInfoCalls == nil {
		a.buildInfoCalls = map[*ast.CallExpression]bool{}
	}
	a.buildInfoCalls[expr] = true
	return a.refinedCompilerKnownType(member.Result), expressionValue{Display: expr.String()}, true
}

// BuildInfoCurrentOf reports whether call is an accepted BuildInfo.Current()
// and returns the snapshot Semantic IR materializes for it.
// Rules: rules/compiler/compiler_known_members.md — § 113B(2).
func (a *Analyzer) BuildInfoCurrentOf(call *ast.CallExpression) (BuildInfoSnapshot, bool) {
	if a.buildInfo == nil || !a.buildInfoCalls[call] {
		return BuildInfoSnapshot{}, false
	}
	return cloneBuildInfoSnapshot(*a.buildInfo), true
}

// IsCoreBuildInfoType reports the compiler-known core.BuildInfo struct.
// Rules: rules/library/core-library.md — § 28.
func IsCoreBuildInfoType(t Type) bool {
	return t.Kind == StructType && t.Name == "BuildInfo" && t.Module == "core" && t.Named
}

// isBuildInfoCurrentCall recognizes `BuildInfo.Current()` naming the core type.
// Rules: compiler_known_members.md § 113B.
func (a *Analyzer) isBuildInfoCurrentCall(expr ast.Expression) bool {
	call, ok := expr.(*ast.CallExpression)
	if !ok || len(call.Arguments) != 0 {
		return false
	}
	member, ok := call.Callee.(*ast.MemberExpression)
	if !ok || member.Property == nil || member.Property.Value != "Current" {
		return false
	}
	owner, ok := member.Object.(*ast.Identifier)
	if !ok || owner.Value != "BuildInfo" {
		return false
	}
	if _, shadowed := a.symbols.lookup(owner.Value); shadowed {
		return false
	}
	typ, exists := a.types["BuildInfo"]
	return exists && typ.Module == "core"
}

// buildInfoAggregate is the snapshot as a semantic CTE struct value: string
// and uint64 fields are scalar constants, Option fields Some of a constant
// or None, in declaration order.
// Rules: rules/compiler/compiler_known_members.md — § 113B(2); rules/library/core-library.md — § 28;
// rules/compiler/compile_time_evaluation.md — §§ 11, 20.
func (a *Analyzer) buildInfoAggregate() DefaultConstant {
	info := a.buildInfo
	typ := a.types["BuildInfo"]
	text := func(value string) DefaultConstant {
		return DefaultConstant{Kind: StringType, String: value, Lexeme: strconv.Quote(value)}
	}
	number := func(value uint64) DefaultConstant {
		integer := new(big.Int).SetUint64(value)
		return DefaultConstant{Kind: UintType, Integer: integer, Lexeme: integer.String()}
	}
	aggregate := &CompileTimeAggregate{Type: typ, Fields: map[string]DefaultConstant{}}
	for _, field := range typ.Fields {
		var value DefaultConstant
		switch field.Name {
		case "ProjectUUID":
			value = text(info.ProjectUUID)
		case "ProjectName":
			value = text(info.ProjectName)
		case "TargetName":
			value = text(info.TargetName)
		case "VariantName":
			value = text(info.VariantName)
		case "VersionRendered":
			value = text(info.VersionRendered)
		case "OperatingSystem":
			value = text(info.OperatingSystem)
		case "Architecture":
			value = text(info.Architecture)
		case "BuildProfile":
			value = text(info.BuildProfile)
		case "TargetProfile":
			value = text(info.TargetProfile)
		case "VersionMajor":
			value = number(info.VersionMajor)
		case "VersionMinor":
			value = number(info.VersionMinor)
		case "VersionRevision":
			value = number(info.VersionRevision)
		case "VersionBuild":
			value = number(info.VersionBuild)
		case "VersionUserDefined", "SourceRevision":
			present := info.VersionUserDefined
			if field.Name == "SourceRevision" {
				present = info.SourceRevision
			}
			value = variantValue(field.Type, "None", nil)
			if present != nil {
				payload := text(*present)
				value = variantValue(field.Type, "Some", &payload)
			}
		case "DirtySourceStatus":
			value = variantValue(field.Type, "None", nil)
			if info.DirtySourceStatus != nil {
				payload := DefaultConstant{Kind: BoolType, Bool: *info.DirtySourceStatus, Lexeme: strconv.FormatBool(*info.DirtySourceStatus)}
				value = variantValue(field.Type, "Some", &payload)
			}
		}
		aggregate.Names = append(aggregate.Names, field.Name)
		aggregate.Fields[field.Name] = value
	}
	return DefaultConstant{Kind: StructType, Aggregate: aggregate}
}

// buildInfoCurrentConstant evaluates a whole BuildInfo.Current() in the
// semantic CTE executor; without a snapshot the call is not constant (Sema
// has already reported S1258).
// Rules: rules/compiler/compiler_known_members.md — § 113B(2)-(3).
func (a *Analyzer) buildInfoCurrentConstant(expr ast.Expression) (DefaultConstant, compileTimeOutcome, bool) {
	call, ok := expr.(*ast.CallExpression)
	if !ok || !a.buildInfoCalls[call] && !a.isBuildInfoCurrentCall(call) {
		return DefaultConstant{}, 0, false
	}
	if a.buildInfo == nil {
		return DefaultConstant{}, compileTimeNotConstant, true
	}
	return a.buildInfoAggregate(), compileTimeEvaluated, true
}

// buildInfoFieldConstant evaluates `BuildInfo.Current().Field` in semantic
// CTE to the snapshot field: a scalar constant, or an Option variant value.
// Rules: rules/compiler/compiler_known_members.md — § 113B(2); rules/compiler/compile_time_evaluation.md — § 2(3).
func (a *Analyzer) buildInfoFieldConstant(expr ast.Expression) (DefaultConstant, compileTimeOutcome, bool) {
	member, ok := expr.(*ast.MemberExpression)
	if !ok || member.Property == nil {
		return DefaultConstant{}, 0, false
	}
	whole, outcome, handled := a.buildInfoCurrentConstant(member.Object)
	if !handled || outcome != compileTimeEvaluated {
		return whole, outcome, handled
	}
	value, exists := whole.Aggregate.Fields[member.Property.Value]
	if !exists {
		return DefaultConstant{}, compileTimeNotConstant, true
	}
	return value, compileTimeEvaluated, true
}
