package semantic

import (
	"fmt"
	"math/big"
	"sort"
	"strings"

	"sec/internal/ast"
	"sec/internal/lexer"
	"sec/internal/sema"
)

type BuildOptions struct {
	RequestedModule string
	SourceFiles     []string
	MaxPackage      uint8
}

func Build(program *ast.Program, analyzer *sema.Analyzer, options BuildOptions) (*Module, error) {
	if program == nil || analyzer == nil {
		return nil, fmt.Errorf("program and completed analyzer are required")
	}
	if options.MaxPackage == 0 {
		options.MaxPackage = 13
	}
	identity := options.RequestedModule
	if identity == "" {
		identity = requestedModule(program, options.SourceFiles)
	}
	module := &Module{Version: Version, Identity: identity, Types: NewTypeTable(), SourceFiles: uniqueSorted(options.SourceFiles)}
	b := &builder{module: module, analyzer: analyzer, maxPackage: options.MaxPackage, definedEnums: map[TypeID]bool{}, definedUnions: map[TypeID]bool{}, definedStructs: map[TypeID]bool{}}
	currentModule := ""
	for _, statement := range program.Statements {
		if declaration, ok := statement.(*ast.ModuleStatement); ok {
			currentModule = declaration.Path
			continue
		}
		fn, ok := statement.(*ast.FunctionDeclaration)
		if !ok || currentModule != identity {
			continue
		}
		if err := b.buildFunction(fn); err != nil {
			return nil, err
		}
	}
	return module, nil
}

type builder struct {
	module         *Module
	analyzer       *sema.Analyzer
	maxPackage     uint8
	definedEnums   map[TypeID]bool
	definedUnions  map[TypeID]bool
	definedStructs map[TypeID]bool
}
type functionBuilder struct {
	owner       *builder
	fn          *Function
	current     *Block
	nextValue   ValueID
	nextBlock   BlockID
	nextStorage StorageID
	nextMatch   MatchID
	bindings    map[sema.BindingID]binding
}
type binding struct {
	value   ValueID
	storage StorageID
	typ     TypeID
	mutable bool
}

func (b *builder) buildFunction(decl *ast.FunctionDeclaration) error {
	resolved, ok := b.analyzer.ResolvedFunctionForDeclaration(decl)
	if !ok {
		return fmt.Errorf("missing resolved function %s", decl.Name.Value)
	}
	returnType, err := b.internType(resolved.ReturnType)
	if err != nil {
		return err
	}
	fn := &Function{ID: functionID(resolved, b.module.Types), Name: resolved.Name, LinkName: resolved.LinkName, ReturnType: returnType, Unsafe: decl.Unsafe, Extern: resolved.Extern, ABI: resolved.ABI, Location: location(decl.Token)}
	b.module.Functions = append(b.module.Functions, fn)
	fb := &functionBuilder{owner: b, fn: fn, bindings: map[sema.BindingID]binding{}, nextStorage: 1, nextMatch: 1}
	for i, parameter := range resolved.Parameters {
		typeID, err := b.internType(parameter.Type)
		if err != nil {
			return err
		}
		ownership, err := ownershipForParameter(parameter, b.maxPackage)
		if err != nil {
			return err
		}
		value := fb.newValue(typeID, ownership, location(parameter.Token))
		fn.Parameters = append(fn.Parameters, Parameter{Name: parameter.Name, Value: value, Location: location(parameter.Token)})
		if i < len(decl.Parameters) && decl.Parameters[i].Name != nil {
			if fact, ok := b.analyzer.ResolvedBindingOf(decl.Parameters[i].Name); ok {
				fb.bindings[fact.ID] = binding{value: value.ID, typ: typeID}
			}
		}
	}
	if resolved.Extern {
		return nil
	}
	entry := fb.newBlock()
	fn.Entry = entry.ID
	fb.current = entry
	if decl.Body == nil {
		return fmt.Errorf("non-extern function %s has no body", fn.ID)
	}
	if err := fb.buildStatements(decl.Body.Statements); err != nil {
		return err
	}
	if fb.current != nil && resolved.ReturnType.Kind == sema.VoidType {
		fb.emit(Operation{Kind: OpReturn, Location: location(decl.Token)})
		fb.current = nil
	}
	// Package 14 sections 88-89 and 103 permit fixed-array type identity but
	// defer owned parameter cleanup for arrays whose elements are not trivially
	// destructible. Reject after the body so a more specific unsupported array
	// operation encountered there remains the primary boundary diagnostic.
	if b.maxPackage >= 14 && !resolved.Extern {
		for _, parameter := range resolved.Parameters {
			if _, fixed := sema.FixedArrayLength(parameter.Type); fixed && !sema.TriviallyDestructible(parameter.Type) {
				return fb.unsupported("non-trivial fixed-array parameter destruction", decl.Token)
			}
		}
	}
	return nil
}

func (b *builder) internType(t sema.Type) (TypeID, error) {
	if t.Kind == sema.ArrayType {
		if b.maxPackage < 14 {
			return 0, &UnsupportedFeatureError{Feature: "array type " + t.Name, Package: b.maxPackage}
		}
		return b.internArrayType(t)
	}
	if t.Kind == sema.ResultType && len(t.TypeArgs) == 2 {
		success, err := b.internType(t.TypeArgs[0])
		if err != nil {
			return 0, err
		}
		failure, err := b.internType(t.TypeArgs[1])
		if err != nil {
			return 0, err
		}
		return b.module.Types.Intern(Type{Kind: TypeResult, Name: "Result", Success: success, Error: failure}), nil
	}
	if t.Kind == sema.EnumType && b.maxPackage < 11 && t.Name == "ArithmeticError" && t.Intrinsic {
		return b.module.Types.Intern(Type{Kind: TypeCoreError, Name: "ArithmeticError", Identity: "core::ArithmeticError"}), nil
	}
	if t.Kind == sema.EnumType {
		if b.maxPackage < 11 {
			return 0, &UnsupportedFeatureError{Feature: "type " + t.Name, Package: b.maxPackage}
		}
		return b.internEnumType(t)
	}
	if t.Kind == sema.UnionType {
		if b.maxPackage < 11 {
			return 0, &UnsupportedFeatureError{Feature: "union type " + t.Name, Package: b.maxPackage}
		}
		return b.internUnionType(t)
	}
	if t.Kind == sema.StructType && !t.Intrinsic {
		if b.maxPackage < 13 {
			return 0, &UnsupportedFeatureError{Feature: "struct type " + t.Name, Package: b.maxPackage}
		}
		return b.internStructType(t)
	}
	kind, signed, width, target, ok := builtinType(t)
	if !ok {
		return 0, &UnsupportedFeatureError{Feature: "type " + t.Name, Package: b.maxPackage}
	}
	base := Type{Kind: kind, Name: t.Name, Signed: signed, BitWidth: width, TargetSize: target}
	if t.Named || t.Declared {
		if len(t.Contracts) > 0 || t.Unit != "" || !t.Dimension.IsZero() {
			return 0, &UnsupportedFeatureError{Feature: "named type contracts or units", Package: b.maxPackage}
		}
		baseID := b.module.Types.Intern(base)
		module := t.Module
		if module == "" {
			module = b.module.Identity
		}
		return b.module.Types.Intern(Type{Kind: TypeNamed, Name: t.Name, Module: module, Identity: module + "::" + t.Name, Base: baseID}), nil
	}
	return b.module.Types.Intern(base), nil
}

// internArrayType implements SEC-MLIR Package 14 sections 9-13 and 27-28 for
// the Semantic IR type layer only. Array construction, indexing and storage
// operations remain separate Package 14 work items.
func (b *builder) internArrayType(t sema.Type) (TypeID, error) {
	if t.Element == nil {
		return 0, &UnsupportedFeatureError{Feature: "array type with missing element", Package: b.maxPackage}
	}
	length, fixed := sema.FixedArrayLength(t)
	if !fixed {
		return 0, &UnsupportedFeatureError{Feature: "dynamic array type " + t.Name, Package: b.maxPackage}
	}
	element, err := b.internType(*t.Element)
	if err != nil {
		return 0, err
	}
	return b.module.Types.Intern(Type{
		Kind:    TypeArray,
		Name:    t.Name,
		Element: element,
		Length:  length.String(),
	}), nil
}

// internStructType implements rules/mlir/semantic-ir/
// sec_semantic_ir_struct_v1.md sections 2-6 while retaining the later
// P14-P18-compatible concrete field types and P17 ownership metadata.
func (b *builder) internStructType(t sema.Type) (TypeID, error) {
	if len(t.GenericParameters) != 0 {
		return 0, &UnsupportedFeatureError{Feature: "unresolved generic struct " + t.Name, Package: b.maxPackage}
	}
	// Sema names a struct-like union binding as Union.Variant. Sections 18 and
	// 66 of rules/mlir/packages/sec-mlir-dialect_package13.md require that view
	// to reuse the union-index-derived synthetic identity from internUnionType.
	for _, definition := range b.module.Structs {
		if definition.SyntheticOrigin != StructSyntheticUnionPayload || definition.Name != t.Name || len(definition.Fields) != len(t.Fields) {
			continue
		}
		matches := true
		for index, field := range t.Fields {
			if definition.Fields[index].Name != field.Name {
				matches = false
				break
			}
		}
		if matches {
			return definition.TypeID, nil
		}
	}
	module, identity := b.semanticIdentity(t)
	args := make([]TypeID, 0, len(t.TypeArgs))
	for _, arg := range t.TypeArgs {
		id, err := b.internType(arg)
		if err != nil {
			return 0, err
		}
		args = append(args, id)
	}
	typeID := b.module.Types.Intern(Type{Kind: TypeStruct, Name: t.Name, Module: module, Identity: identity, TypeArgs: args})
	if b.definedStructs[typeID] {
		return typeID, nil
	}
	b.definedStructs[typeID] = true
	definition := StructDefinition{TypeID: typeID, SymbolID: SymbolID(identity), Name: t.Name, TypeArguments: args,
		CopyClassification: string(sema.CopyClassificationOf(t)), TriviallyDestructible: sema.TriviallyDestructible(t), Defaultable: sema.IsDefaultable(t)}
	if token, ok := b.analyzer.ResolvedTypeDeclarationLocation(t); ok {
		definition.Location = location(token)
	}
	for index, field := range t.Fields {
		fieldType, err := b.internType(field.Type)
		if err != nil {
			return 0, err
		}
		resolved := StructFieldDefinition{ID: StructFieldID(index), Name: field.Name, Type: fieldType, Location: location(field.Token)}
		for _, tag := range field.Tags {
			resolved.Tags = append(resolved.Tags, StructTag{Key: tag.Key, Value: tag.Value})
		}
		definition.Fields = append(definition.Fields, resolved)
	}
	b.module.Structs = append(b.module.Structs, definition)
	return typeID, nil
}

func (b *builder) internEnumType(t sema.Type) (TypeID, error) {
	module, identity := b.semanticIdentity(t)
	underlyingSema, ok := b.lookupEnumUnderlying(t)
	if !ok {
		return 0, &UnsupportedFeatureError{Feature: "enum underlying type " + t.Underlying, Package: b.maxPackage}
	}
	underlying, err := b.internType(underlyingSema)
	if err != nil {
		return 0, err
	}
	typeID := b.module.Types.Intern(Type{Kind: TypeEnum, Name: t.Name, Module: module, Identity: identity, Underlying: underlying, BitWidth: uint16(t.BitWidth)})
	if b.definedEnums[typeID] {
		return typeID, nil
	}
	b.definedEnums[typeID] = true
	representation := EnumRepresentationInteger
	if t.BitWidth > 0 {
		representation = EnumRepresentationBitBacked
	}
	definition := EnumDefinition{TypeID: typeID, SymbolID: SymbolID(identity), Name: t.Name, Underlying: underlying, RepresentationKind: representation, BitWidth: uint16(t.BitWidth)}
	if token, ok := b.analyzer.ResolvedTypeDeclarationLocation(t); ok {
		definition.Location = location(token)
	}
	for ordinal, name := range t.EnumValues {
		value, ok := t.EnumConsts[name]
		if !ok || value.Value == nil {
			return 0, fmt.Errorf("enum %s case %s has no resolved value", identity, name)
		}
		definition.Cases = append(definition.Cases, EnumCase{ID: EnumCaseID(ordinal), Name: name, Value: new(big.Int).Set(value.Value), Location: location(value.Token)})
	}
	b.module.Enums = append(b.module.Enums, definition)
	return typeID, nil
}

func (b *builder) lookupEnumUnderlying(t sema.Type) (sema.Type, bool) {
	if t.BitWidth > 0 {
		return sema.Type{Name: t.Underlying, Kind: sema.UintType, BitWidth: t.BitWidth}, true
	}
	typ, ok := b.analyzer.Types()[t.Underlying]
	if ok {
		return typ, true
	}
	return sema.Type{}, false
}

func (b *builder) internUnionType(t sema.Type) (TypeID, error) {
	if len(t.GenericParameters) != 0 {
		return 0, &UnsupportedFeatureError{Feature: "unresolved generic union " + t.Name, Package: b.maxPackage}
	}
	module, identity := b.semanticIdentity(t)
	typeArguments := make([]TypeID, 0, len(t.TypeArgs))
	for _, argument := range t.TypeArgs {
		id, err := b.internType(argument)
		if err != nil {
			return 0, err
		}
		typeArguments = append(typeArguments, id)
	}
	typeID := b.module.Types.Intern(Type{Kind: TypeUnion, Name: t.Name, Module: module, Identity: identity, TypeArgs: typeArguments})
	if b.definedUnions[typeID] {
		return typeID, nil
	}
	b.definedUnions[typeID] = true
	definition := UnionDefinition{
		TypeID: typeID, SymbolID: SymbolID(identity), Name: t.Name,
		TypeArguments: typeArguments, CopyClassification: string(sema.CopyClassificationOf(t)),
		TriviallyDestructible: sema.TriviallyDestructible(t),
	}
	if token, ok := b.analyzer.ResolvedTypeDeclarationLocation(t); ok {
		definition.Location = location(token)
	}
	for index, variant := range t.UnionVariants {
		resolved := UnionVariantDefinition{Index: UnionVariantIndex(index), Name: variant.Name, Location: location(variant.Token)}
		switch {
		case variant.Payload != nil:
			resolved.Kind = UnionVariantSingle
			payload, err := b.internType(*variant.Payload)
			if err != nil {
				return 0, err
			}
			resolved.Payload = payload
		case len(variant.PayloadFields) != 0:
			resolved.Kind = UnionVariantFields
			syntheticIdentity := fmt.Sprintf("%s#%d$payload", identity, index)
			syntheticType := b.module.Types.Intern(Type{
				Kind: TypeStruct, Name: t.Name + "." + variant.Name,
				Module: module, Identity: syntheticIdentity,
			})
			resolved.SyntheticPayloadStruct = syntheticType
			syntheticSemaType := sema.Type{Kind: sema.StructType, Fields: variant.PayloadFields}
			syntheticDefinition := StructDefinition{
				TypeID: syntheticType, SymbolID: SymbolID(syntheticIdentity),
				Name:                  t.Name + "." + variant.Name,
				CopyClassification:    string(sema.CopyClassificationOf(syntheticSemaType)),
				TriviallyDestructible: sema.TriviallyDestructible(syntheticSemaType),
				Defaultable:           sema.IsDefaultable(syntheticSemaType),
				SyntheticOrigin:       StructSyntheticUnionPayload, Location: location(variant.Token),
			}
			for fieldIndex, field := range variant.PayloadFields {
				fieldType, err := b.internType(field.Type)
				if err != nil {
					return 0, err
				}
				resolved.PayloadFields = append(resolved.PayloadFields, UnionPayloadField{Name: field.Name, Type: fieldType, Location: location(field.Token)})
				syntheticField := StructFieldDefinition{ID: StructFieldID(fieldIndex), Name: field.Name, Type: fieldType, Location: location(field.Token)}
				for _, tag := range field.Tags {
					syntheticField.Tags = append(syntheticField.Tags, StructTag{Key: tag.Key, Value: tag.Value})
				}
				syntheticDefinition.Fields = append(syntheticDefinition.Fields, syntheticField)
			}
			if !b.definedStructs[syntheticType] {
				b.definedStructs[syntheticType] = true
				b.module.Structs = append(b.module.Structs, syntheticDefinition)
			}
		default:
			resolved.Kind = UnionVariantEmpty
		}
		definition.Variants = append(definition.Variants, resolved)
	}
	b.module.Unions = append(b.module.Unions, definition)
	return typeID, nil
}

func (b *builder) semanticIdentity(t sema.Type) (string, string) {
	module := t.Module
	if module == "" {
		if t.Intrinsic {
			module = "core"
		} else {
			module = b.module.Identity
		}
	}
	identity := module + "::" + t.Name
	if len(t.TypeArgs) != 0 {
		parts := make([]string, len(t.TypeArgs))
		for index, argument := range t.TypeArgs {
			parts[index] = canonicalSemaType(argument)
		}
		identity += "<" + strings.Join(parts, ",") + ">"
	}
	return module, identity
}

func builtinType(t sema.Type) (TypeKind, bool, uint16, bool, bool) {
	name := t.Name
	switch t.Kind {
	case sema.VoidType:
		return TypeVoid, false, 0, false, true
	case sema.NeverType:
		return TypeNever, false, 0, false, true
	case sema.BoolType:
		return TypeBool, false, 1, false, true
	case sema.CharType:
		return TypeChar, false, 8, false, true
	case sema.RuneType:
		return TypeRune, false, 32, false, true
	case sema.StringType:
		return TypeString, false, 0, false, true
	case sema.DecimalType:
		if name == "decimal128" {
			return TypeDecimal128, true, 128, false, true
		}
		return TypeDecimal, true, 0, false, true
	case sema.FloatType:
		w := uint16(0)
		if name == "float32" {
			w = 32
		}
		if name == "float64" {
			w = 64
		}
		return TypeFloat, true, w, name == "float", true
	case sema.IntType:
		if name == "byte" {
			return TypeByte, false, 8, false, true
		}
		w, target := numericWidth(name), name == "int"
		return TypeInt, true, w, target, true
	case sema.UintType:
		if name == "byte" {
			return TypeByte, false, 8, false, true
		}
		if t.BitWidth > 0 {
			return TypeUint, false, uint16(t.BitWidth), false, true
		}
		w, target := numericWidth(name), name == "uint"
		return TypeUint, false, w, target, true
	default:
		return "", false, 0, false, false
	}
}
func numericWidth(name string) uint16 {
	switch name {
	case "int8", "uint8":
		return 8
	case "int16", "uint16":
		return 16
	case "int32", "uint32":
		return 32
	case "int64", "uint64":
		return 64
	case "int128", "uint128":
		return 128
	case "int256", "uint256":
		return 256
	}
	return 0
}

func (fb *functionBuilder) buildStatements(statements []ast.Statement) error {
	for _, statement := range statements {
		if fb.current == nil {
			return &UnsupportedFeatureError{Feature: "unreachable statement after terminator", Package: fb.owner.maxPackage, Location: statementLocation(statement)}
		}
		switch stmt := statement.(type) {
		case *ast.LetStatement:
			if err := fb.buildLet(stmt); err != nil {
				return err
			}
		case *ast.AssignmentStatement:
			if err := fb.buildAssignment(stmt); err != nil {
				return err
			}
		case *ast.ReturnStatement:
			if err := fb.buildReturn(stmt); err != nil {
				return err
			}
		case *ast.IfStatement:
			if fb.owner.maxPackage < 3 {
				return fb.unsupported("if control flow", stmt.Token)
			}
			if err := fb.buildIf(stmt); err != nil {
				return err
			}
		case *ast.MatchStatement:
			if fb.owner.maxPackage < 12 {
				return fb.unsupported("match statement", stmt.Token)
			}
			if err := fb.buildMatchStatement(stmt); err != nil {
				return err
			}
		case *ast.ExpressionStatement:
			call, ok := stmt.Expression.(*ast.CallExpression)
			if !ok {
				return fb.unsupported("expression statement", stmt.Token)
			}
			resolved, ok := fb.owner.analyzer.ResolvedCallTarget(call)
			if !ok {
				return fb.unsupported("unresolved function call", call.Token)
			}
			if resolved.Function.ReturnType.Kind != sema.VoidType {
				return fb.unsupported("standalone non-void call", call.Token)
			}
			if _, err := fb.buildCall(call); err != nil {
				return err
			}
		case *ast.UnsafeStatement:
			if stmt.Body == nil {
				return fb.unsupported("empty unsafe block", stmt.Token)
			}
			if err := fb.buildStatements(stmt.Body.Statements); err != nil {
				return err
			}
		default:
			return fb.unsupported(fmt.Sprintf("%T", statement), statementToken(statement))
		}
	}
	return nil
}

func (fb *functionBuilder) buildLet(stmt *ast.LetStatement) error {
	if stmt.Value == nil {
		return fb.unsupported("mutable local declaration without initializer", stmt.Token)
	}
	// Package 14 sections 44 and 103 defer element move-out until the ownership
	// packages can represent partial array availability and cleanup explicitly.
	if stmt.Ownership == ast.OwnershipMove {
		if index, ok := stmt.Value.(*ast.IndexExpression); ok {
			if plan, resolved := fb.owner.analyzer.ResolvedArrayIndexPlanOf(index); resolved {
				if _, fixed := sema.FixedArrayLength(plan.ArrayType); fixed {
					return fb.unsupported("fixed-array element move-out", index.Token)
				}
			}
		}
	}
	fact, ok := fb.owner.analyzer.ResolvedBindingOf(stmt.Name)
	if !ok {
		return fmt.Errorf("missing resolved binding for %s", stmt.Name.Value)
	}
	var value builtValue
	var err error
	if stmt.SynthesizedDefault && ((fb.owner.maxPackage >= 13 && fact.Type.Kind == sema.StructType) ||
		(fb.owner.maxPackage >= 14 && fact.Type.Kind == sema.ArrayType)) {
		// Package 13 section 26 and Package 14 sections 24-26 require the new IR
		// to consume canonical compact DefaultResolution facts instead of
		// treating Sema's bounded compatibility AST nodes as semantic authority.
		value, err = fb.buildResolvedDefault(fact.Type, sema.DefaultValueOf(fact.Type), location(stmt.Token))
	} else {
		value, err = fb.buildExpr(stmt.Value, 0)
	}
	if err != nil {
		return err
	}
	if stmt.Mutable {
		if fb.owner.maxPackage < 3 {
			return fb.unsupported("mutable local storage", stmt.Token)
		}
		if !fb.storageAllowed(value.typ) {
			return fb.unsupported("non-trivial mutable local storage", stmt.Token)
		}
		id := fb.nextStorage
		fb.nextStorage++
		storage := Storage{ID: id, Name: stmt.Name.Value, Type: value.typ, Mutable: true, Class: StorageLocalAutomatic, Location: location(stmt.Token)}
		fb.fn.Storages = append(fb.fn.Storages, storage)
		fb.bindings[fact.ID] = binding{storage: id, typ: value.typ, mutable: true}
		fb.emit(Operation{Kind: OpStorageDeclare, Storage: id, Location: location(stmt.Token)})
		fb.emit(Operation{Kind: OpStorageInit, Storage: id, Operands: []ValueID{value.id}, Location: location(stmt.Token)})
		return nil
	}
	fb.bindings[fact.ID] = binding{value: value.id, typ: value.typ}
	return nil
}

func (fb *functionBuilder) buildAssignment(stmt *ast.AssignmentStatement) error {
	if fb.owner.maxPackage < 3 {
		return fb.unsupported("assignment", stmt.Token)
	}
	if stmt.Operator != "=" {
		return fb.unsupported("compound assignment", stmt.Token)
	}
	if index, ok := stmt.Target.(*ast.IndexExpression); ok && fb.owner.maxPackage >= 14 {
		if _, simple := index.Left.(*ast.Identifier); !simple {
			return fb.buildNestedAggregateAssignment(stmt)
		}
		return fb.buildArrayIndexAssignment(stmt, index)
	}
	if member, ok := stmt.Target.(*ast.MemberExpression); ok && fb.owner.maxPackage >= 13 {
		if fb.owner.maxPackage >= 14 && aggregatePathContainsIndex(member) {
			return fb.buildNestedAggregateAssignment(stmt)
		}
		return fb.buildStructFieldAssignment(stmt, member)
	}
	id, ok := stmt.Target.(*ast.Identifier)
	if !ok {
		return fb.unsupported("non-local assignment", stmt.Token)
	}
	fact, ok := fb.owner.analyzer.ResolvedBindingOf(id)
	if !ok {
		return fmt.Errorf("missing resolved assignment binding")
	}
	bind, ok := fb.bindings[fact.ID]
	if !ok || bind.storage == 0 {
		return fmt.Errorf("assignment target has no semantic storage")
	}
	value, err := fb.buildExpr(stmt.Value, bind.typ)
	if err != nil {
		return err
	}
	fb.emit(Operation{Kind: OpStorageStore, Storage: bind.storage, Operands: []ValueID{value.id}, Location: location(stmt.Token)})
	return nil
}

type aggregateProjectionKind uint8

const (
	aggregateProjectionField aggregateProjectionKind = iota + 1
	aggregateProjectionIndex
)

type aggregateAssignmentProjection struct {
	kind       aggregateProjectionKind
	field      sema.ResolvedStructMemberPlan
	index      sema.ResolvedArrayIndexPlan
	indexExpr  ast.Expression
	parent     builtValue
	indexValue builtValue
	checkKind  ArrayIndexCheckKind
	proofKind  ArrayIndexProofKind
	guard      ValueID
}

func aggregatePathContainsIndex(expr ast.Expression) bool {
	switch expression := expr.(type) {
	case *ast.IndexExpression:
		return true
	case *ast.MemberExpression:
		return aggregatePathContainsIndex(expression.Object)
	default:
		return false
	}
}

// buildNestedAggregateAssignment implements the leaf-to-root rebuild required
// by rules/mlir/packages/sec-mlir-dialect_package14.md section 47. It supports
// stored struct fields and fixed-array indexes rooted in one trivial mutable
// local. Each index is evaluated and guarded once before the RHS; reverse
// replacement commits the exact root type through one storage.store.
func (fb *functionBuilder) buildNestedAggregateAssignment(stmt *ast.AssignmentStatement) error {
	root, projections, err := fb.resolveAggregateAssignmentPath(stmt.Target)
	if err != nil {
		return err
	}
	if root == nil || len(projections) < 2 {
		return fb.unsupported("non-nested aggregate assignment", statementToken(stmt))
	}
	fact, ok := fb.owner.analyzer.ResolvedBindingOf(root)
	if !ok {
		return fmt.Errorf("missing resolved aggregate root binding")
	}
	bind, ok := fb.bindings[fact.ID]
	if !ok || bind.storage == 0 || !bind.mutable || !fb.storageAllowed(bind.typ) {
		return fb.unsupported("nested assignment without trivial mutable root storage", root.Token)
	}

	current := fb.result(Operation{Kind: OpStorageLoad, Storage: bind.storage, Location: location(stmt.Token)}, bind.typ)
	for index := range projections {
		projection := &projections[index]
		projection.parent = current
		last := index == len(projections)-1
		switch projection.kind {
		case aggregateProjectionField:
			if projection.field.Kind != sema.MemberStoredField || projection.field.Action != sema.StructFieldCopyTrivial {
				return fb.unsupported("non-trivial nested struct field", expressionToken(stmt.Target))
			}
			if !last {
				fieldType, internErr := fb.owner.internType(projection.field.MemberType)
				if internErr != nil {
					return internErr
				}
				current = fb.result(Operation{
					Kind: OpStructExtractField, Operands: []ValueID{current.id},
					StructField:   StructFieldID(projection.field.FieldID),
					StructActions: []StructFieldAction{StructActionCopyTrivial}, Location: location(stmt.Token),
				}, fieldType)
			}
		case aggregateProjectionIndex:
			indexType, internErr := fb.owner.internType(projection.index.IndexType)
			if internErr != nil {
				return internErr
			}
			projection.indexValue, internErr = fb.buildExpr(projection.indexExpr, indexType)
			if internErr != nil {
				return internErr
			}
			if projection.index.CheckKind == sema.ArrayIndexRuntimeCheck {
				if projection.index.FailureMode != sema.ArrayIndexFailureOrdinary {
					return fb.unsupported("non-ordinary nested array index", expressionToken(projection.indexExpr))
				}
				boolType, boolErr := fb.owner.internType(sema.Type{Name: "bool", Kind: sema.BoolType})
				if boolErr != nil {
					return boolErr
				}
				predicate := fb.result(Operation{
					Kind: OpArrayIndexInBounds, Operands: []ValueID{current.id, projection.indexValue.id},
					ArrayIndexSigned: projection.index.IndexSigned, Location: location(stmt.Token),
				}, boolType)
				success := fb.newBlock()
				failure := fb.newBlock()
				fb.emit(Operation{
					Kind: OpCondBranch, Operands: []ValueID{predicate.id},
					Successors: []BranchTarget{{Block: success.ID}, {Block: failure.ID}}, Location: location(stmt.Token),
				})
				fb.current = failure
				fb.emit(Operation{Kind: OpBoundsFailure, ArrayOperation: "fixed-array-index", Location: location(stmt.Token)})
				fb.current = success
				projection.checkKind = ArrayIndexRuntimeCheck
				projection.proofKind = ArrayIndexProofGuarded
				projection.guard = predicate.id
			} else {
				proof, proofOK := semanticArrayIndexProof(projection.index.ProofKind)
				if projection.index.CheckKind != sema.ArrayIndexProvenSafe || projection.index.FailureMode != sema.ArrayIndexFailureNone || !proofOK {
					return fmt.Errorf("invalid proven-safe nested array index plan")
				}
				projection.checkKind = ArrayIndexProvenSafe
				projection.proofKind = proof
			}
			if !last {
				if projection.index.Action != sema.ArrayTransferCopyTrivial {
					return fb.unsupported("non-trivial nested array traversal", expressionToken(projection.indexExpr))
				}
				elementType, elementErr := fb.owner.internType(projection.index.ElementType)
				if elementErr != nil {
					return elementErr
				}
				current = fb.result(Operation{
					Kind: OpArrayExtract, Operands: []ValueID{current.id, projection.indexValue.id},
					ArrayCheckKind: projection.checkKind, ArrayProofKind: projection.proofKind, ArrayGuard: projection.guard,
					ArrayActions: []ArrayTransferAction{ArrayActionCopyTrivial}, Location: location(stmt.Token),
				}, elementType)
			}
		default:
			return fmt.Errorf("unknown aggregate assignment projection")
		}
	}

	leaf := projections[len(projections)-1]
	var leafType TypeID
	if leaf.kind == aggregateProjectionField {
		leafType, err = fb.owner.internType(leaf.field.MemberType)
	} else {
		leafType, err = fb.owner.internType(leaf.index.ElementType)
	}
	if err != nil {
		return err
	}
	replacement, err := fb.buildExpr(stmt.Value, leafType)
	if err != nil {
		return err
	}
	for index := len(projections) - 1; index >= 0; index-- {
		projection := projections[index]
		switch projection.kind {
		case aggregateProjectionField:
			replacement = fb.result(Operation{
				Kind: OpStructReplaceField, Operands: []ValueID{projection.parent.id, replacement.id},
				StructField: StructFieldID(projection.field.FieldID), Location: location(stmt.Token),
			}, projection.parent.typ)
		case aggregateProjectionIndex:
			replacement = fb.result(Operation{
				Kind: OpArrayReplace, Operands: []ValueID{projection.parent.id, projection.indexValue.id, replacement.id},
				ArrayCheckKind: projection.checkKind, ArrayProofKind: projection.proofKind,
				ArrayGuard: projection.guard, Location: location(stmt.Token),
			}, projection.parent.typ)
		}
	}
	fb.emit(Operation{Kind: OpStorageStore, Storage: bind.storage, Operands: []ValueID{replacement.id}, Location: location(stmt.Token)})
	return nil
}

func (fb *functionBuilder) resolveAggregateAssignmentPath(target ast.Expression) (*ast.Identifier, []aggregateAssignmentProjection, error) {
	projections := []aggregateAssignmentProjection{}
	var walk func(ast.Expression) (*ast.Identifier, error)
	walk = func(expression ast.Expression) (*ast.Identifier, error) {
		switch current := expression.(type) {
		case *ast.Identifier:
			return current, nil
		case *ast.MemberExpression:
			root, err := walk(current.Object)
			if err != nil {
				return nil, err
			}
			plan, ok := fb.owner.analyzer.ResolvedStructMemberOf(current)
			if !ok {
				return nil, fb.unsupported("unresolved nested struct member", current.Token)
			}
			projections = append(projections, aggregateAssignmentProjection{kind: aggregateProjectionField, field: plan})
			return root, nil
		case *ast.IndexExpression:
			root, err := walk(current.Left)
			if err != nil {
				return nil, err
			}
			plan, ok := fb.owner.analyzer.ResolvedArrayIndexPlanOf(current)
			if !ok {
				return nil, fb.unsupported("unresolved nested array index", current.Token)
			}
			projections = append(projections, aggregateAssignmentProjection{kind: aggregateProjectionIndex, index: plan, indexExpr: current.Index})
			return root, nil
		default:
			return nil, fb.unsupported("non-local nested aggregate assignment", expressionToken(expression))
		}
	}
	root, err := walk(target)
	return root, projections, err
}

// buildArrayIndexAssignment implements the simple-root transactional update in
// rules/mlir/packages/sec-mlir-dialect_package14.md sections 43 and 46. Root
// and index are resolved once, runtime bounds failure precedes RHS evaluation,
// and successful replacement commits through exactly one storage.store.
// Nested aggregate paths remain the separate P14-52 boundary.
func (fb *functionBuilder) buildArrayIndexAssignment(stmt *ast.AssignmentStatement, target *ast.IndexExpression) error {
	root, ok := target.Left.(*ast.Identifier)
	if !ok {
		return fb.unsupported("nested fixed-array index assignment", target.Token)
	}
	plan, ok := fb.owner.analyzer.ResolvedArrayIndexPlanOf(target)
	if !ok || plan.UseKind != sema.ArrayIndexWrite || plan.Action != sema.ArrayTransferConstructDirect {
		return fb.unsupported("unresolved or non-trivial fixed-array index assignment", target.Token)
	}
	fact, ok := fb.owner.analyzer.ResolvedBindingOf(root)
	if !ok {
		return fmt.Errorf("missing resolved array assignment root binding")
	}
	bind, ok := fb.bindings[fact.ID]
	if !ok || bind.storage == 0 || !bind.mutable || !fb.storageAllowed(bind.typ) {
		return fb.unsupported("fixed-array assignment without trivial mutable root storage", root.Token)
	}
	arrayType, err := fb.owner.internType(plan.ArrayType)
	if err != nil {
		return err
	}
	if bind.typ != arrayType {
		return fmt.Errorf("fixed-array assignment root type mismatch")
	}
	indexType, err := fb.owner.internType(plan.IndexType)
	if err != nil {
		return err
	}
	elementType, err := fb.owner.internType(plan.ElementType)
	if err != nil {
		return err
	}
	index, err := fb.buildExpr(target.Index, indexType)
	if err != nil {
		return err
	}
	loc := location(target.Token)

	var current builtValue
	checkKind := ArrayIndexProvenSafe
	proofKind, proofOK := semanticArrayIndexProof(plan.ProofKind)
	guard := ValueID(0)
	if plan.CheckKind == sema.ArrayIndexRuntimeCheck {
		if plan.FailureMode != sema.ArrayIndexFailureOrdinary {
			return fb.unsupported("non-ordinary fixed-array assignment failure mode", target.Token)
		}
		current = fb.result(Operation{Kind: OpStorageLoad, Storage: bind.storage, Location: loc}, bind.typ)
		boolType, boolErr := fb.owner.internType(sema.Type{Name: "bool", Kind: sema.BoolType})
		if boolErr != nil {
			return boolErr
		}
		predicate := fb.result(Operation{
			Kind: OpArrayIndexInBounds, Operands: []ValueID{current.id, index.id},
			ArrayIndexSigned: plan.IndexSigned, Location: loc,
		}, boolType)
		success := fb.newBlock()
		failure := fb.newBlock()
		fb.emit(Operation{
			Kind: OpCondBranch, Operands: []ValueID{predicate.id},
			Successors: []BranchTarget{{Block: success.ID}, {Block: failure.ID}}, Location: loc,
		})
		fb.current = failure
		fb.emit(Operation{Kind: OpBoundsFailure, ArrayOperation: "fixed-array-index", Location: loc})
		fb.current = success
		checkKind = ArrayIndexRuntimeCheck
		proofKind = ArrayIndexProofGuarded
		guard = predicate.id
	} else {
		if plan.CheckKind != sema.ArrayIndexProvenSafe || plan.FailureMode != sema.ArrayIndexFailureNone || !proofOK {
			return fmt.Errorf("invalid proven-safe fixed-array assignment plan")
		}
	}

	replacement, err := fb.buildExpr(stmt.Value, elementType)
	if err != nil {
		return err
	}
	if plan.CheckKind == sema.ArrayIndexProvenSafe {
		// With no runtime predicate, defer the whole-array load until the RHS is
		// complete as required by Package 14 section 46.
		current = fb.result(Operation{Kind: OpStorageLoad, Storage: bind.storage, Location: loc}, bind.typ)
	}
	updated := fb.result(Operation{
		Kind: OpArrayReplace, Operands: []ValueID{current.id, index.id, replacement.id},
		ArrayCheckKind: checkKind, ArrayProofKind: proofKind, ArrayGuard: guard, Location: loc,
	}, bind.typ)
	fb.emit(Operation{Kind: OpStorageStore, Storage: bind.storage, Operands: []ValueID{updated.id}, Location: location(stmt.Token)})
	return nil
}

// buildStructFieldAssignment implements the leaf-to-root aggregate rebuild in
// rules/mlir/lowering-versions/sec_mlir_lowering_v9.md sections 10-11. The RHS
// is evaluated first and the root storage is written exactly once.
func (fb *functionBuilder) buildStructFieldAssignment(stmt *ast.AssignmentStatement, target *ast.MemberExpression) error {
	var reversed []sema.ResolvedStructMemberPlan
	var root *ast.Identifier
	for expression := ast.Expression(target); ; {
		switch current := expression.(type) {
		case *ast.MemberExpression:
			plan, ok := fb.owner.analyzer.ResolvedStructMemberOf(current)
			if !ok || plan.Kind != sema.MemberStoredField || plan.Action != sema.StructFieldCopyTrivial {
				return fb.unsupported("non-trivial or unresolved struct field assignment", current.Token)
			}
			reversed = append(reversed, plan)
			expression = current.Object
		case *ast.Identifier:
			root = current
		default:
			return fb.unsupported("non-local struct field assignment", target.Token)
		}
		if root != nil {
			break
		}
	}
	plans := make([]sema.ResolvedStructMemberPlan, len(reversed))
	for index := range reversed {
		plans[len(reversed)-1-index] = reversed[index]
	}
	if len(plans) == 0 {
		return fb.unsupported("empty struct field assignment", target.Token)
	}
	fact, ok := fb.owner.analyzer.ResolvedBindingOf(root)
	if !ok {
		return fmt.Errorf("missing resolved struct root binding")
	}
	bind, ok := fb.bindings[fact.ID]
	if !ok || bind.storage == 0 || !bind.mutable || !fb.storageAllowed(bind.typ) {
		return fb.unsupported("struct field assignment without trivial mutable root storage", root.Token)
	}
	leafType, err := fb.owner.internType(plans[len(plans)-1].MemberType)
	if err != nil {
		return err
	}
	replacement, err := fb.buildExpr(stmt.Value, leafType)
	if err != nil {
		return err
	}
	rootValue := fb.result(Operation{Kind: OpStorageLoad, Storage: bind.storage, Location: location(stmt.Token)}, bind.typ)
	parents := make([]builtValue, len(plans))
	current := rootValue
	for index := 0; index < len(plans)-1; index++ {
		parents[index] = current
		fieldType, internErr := fb.owner.internType(plans[index].MemberType)
		if internErr != nil {
			return internErr
		}
		current = fb.result(Operation{Kind: OpStructExtractField, Operands: []ValueID{current.id}, StructField: StructFieldID(plans[index].FieldID), StructActions: []StructFieldAction{StructActionCopyTrivial}, Location: location(stmt.Token)}, fieldType)
	}
	parents[len(plans)-1] = current
	for index := len(plans) - 1; index >= 0; index-- {
		replacement = fb.result(Operation{Kind: OpStructReplaceField, Operands: []ValueID{parents[index].id, replacement.id}, StructField: StructFieldID(plans[index].FieldID), Location: location(stmt.Token)}, parents[index].typ)
	}
	fb.emit(Operation{Kind: OpStorageStore, Storage: bind.storage, Operands: []ValueID{replacement.id}, Location: location(stmt.Token)})
	return nil
}

func (fb *functionBuilder) buildReturn(stmt *ast.ReturnStatement) error {
	op := Operation{Kind: OpReturn, Location: location(stmt.Token)}
	if stmt.Value != nil {
		value, err := fb.buildExpr(stmt.Value, fb.fn.ReturnType)
		if err != nil {
			return err
		}
		op.Operands = []ValueID{value.id}
	}
	fb.emit(op)
	fb.current = nil
	return nil
}

func (fb *functionBuilder) buildIf(stmt *ast.IfStatement) error {
	condition, err := fb.buildExpr(stmt.Condition, 0)
	if err != nil {
		return err
	}
	conditionType, _ := fb.owner.module.Types.Lookup(condition.typ)
	if conditionType.Kind != TypeBool {
		return fb.unsupported("non-boolean if condition", stmt.Token)
	}
	thenBlock := fb.newBlock()
	var elseBlock, merge *Block
	if stmt.Alternative != nil {
		elseBlock = fb.newBlock()
	} else {
		merge = fb.newBlock()
		elseBlock = merge
	}
	fb.emit(Operation{Kind: OpCondBranch, Operands: []ValueID{condition.id}, Successors: []BranchTarget{{Block: thenBlock.ID}, {Block: elseBlock.ID}}, Location: location(stmt.Token)})
	fb.current = thenBlock
	if err := fb.buildStatements(stmt.Consequence.Statements); err != nil {
		return err
	}
	thenEnd := fb.current
	if stmt.Alternative != nil {
		fb.current = elseBlock
		if err := fb.buildStatements(stmt.Alternative.Statements); err != nil {
			return err
		}
	}
	elseEnd := fb.current
	if merge == nil && (thenEnd != nil || elseEnd != nil) {
		merge = fb.newBlock()
	}
	if thenEnd != nil {
		fb.current = thenEnd
		fb.emit(Operation{Kind: OpBranch, Successors: []BranchTarget{{Block: merge.ID}}, Location: location(stmt.Token)})
	}
	if elseEnd != nil && elseEnd != merge {
		fb.current = elseEnd
		fb.emit(Operation{Kind: OpBranch, Successors: []BranchTarget{{Block: merge.ID}}, Location: location(stmt.Token)})
	}
	fb.current = merge
	return nil
}

type builtValue struct {
	id  ValueID
	typ TypeID
}

// buildExpr lowers expressions supported by the selected Semantic IR package
// and stops deferred ownership/view operations at their explicit package gate.
//
// Rules:
//   - rules/compiler/semantic_ir.txt — "Unsupported lowerings"
//   - rules/mlir/packages/sec-mlir-dialect_package14.md — sections 44, 89, 103
func (fb *functionBuilder) buildExpr(expr ast.Expression, expected TypeID) (builtValue, error) {
	if okExpr, ok := expr.(*ast.OkExpression); ok {
		return fb.buildResultConstructor(okExpr.Value, expected, true, location(okExpr.Token))
	}
	if errExpr, ok := expr.(*ast.ErrExpression); ok {
		return fb.buildResultConstructor(errExpr.Value, expected, false, location(errExpr.Token))
	}
	if arrayLiteral, ok := expr.(*ast.ArrayLiteral); ok && fb.owner.maxPackage >= 14 {
		plan, resolved := fb.owner.analyzer.ResolvedArrayLiteralPlanOf(arrayLiteral)
		if !resolved || plan.Length == nil {
			return builtValue{}, fb.unsupported("unresolved array literal", arrayLiteral.Token)
		}
		resultType := expected
		if resultType == 0 {
			var err error
			resultType, err = fb.owner.internType(sema.NewFixedArrayType(plan.ElementType, plan.Length))
			if err != nil {
				return builtValue{}, err
			}
		}
		return fb.buildArrayLiteral(arrayLiteral, resultType)
	}
	if reference, ok := expr.(*ast.RefExpression); ok && fb.owner.maxPackage >= 14 {
		if index, indexed := reference.Value.(*ast.IndexExpression); indexed {
			if plan, resolved := fb.owner.analyzer.ResolvedArrayIndexPlanOf(index); resolved {
				if _, fixed := sema.FixedArrayLength(plan.ArrayType); fixed {
					kind := "shared"
					if reference.Mutable {
						kind = "mutable"
					}
					return builtValue{}, fb.unsupported(kind+" fixed-array element borrow", reference.Token)
				}
			}
		}
		if _, sliced := reference.Value.(*ast.SliceExpression); sliced {
			return builtValue{}, fb.unsupported("array-to-slice creation", reference.Token)
		}
	}
	if slice, ok := expr.(*ast.SliceExpression); ok && fb.owner.maxPackage >= 14 {
		return builtValue{}, fb.unsupported("slice expression", slice.Token)
	}
	resolved, ok := fb.owner.analyzer.ResolvedTypeOf(expr)
	if !ok {
		// Generic union constructors may be resolved from the expected type or
		// their payload arguments without acquiring a standalone expression-type
		// fact.  The construction fact is nevertheless complete and immutable, so
		// use its concrete substituted union type directly.
		if fb.owner.maxPackage >= 11 {
			if plan, resolved := fb.owner.analyzer.ResolvedUnionConstructionOf(expr); resolved {
				typeID, err := fb.owner.internType(plan.UnionType)
				if err != nil {
					return builtValue{}, err
				}
				if expected != 0 {
					typeID = expected
				}
				return fb.buildUnionConstruction(expr, plan, typeID)
			}
		}
		return builtValue{}, fmt.Errorf("missing resolved type for %T at %s", expr, formatLocation(location(expressionToken(expr))))
	}
	typeID, err := fb.owner.internType(resolved)
	if err != nil {
		return builtValue{}, err
	}
	if expected != 0 {
		typeID = expected
	}
	loc := locationFromExpression(expr)
	switch e := expr.(type) {
	case *ast.IntegerLiteral:
		resolvedType, _ := fb.owner.module.Types.Lookup(typeID)
		if resolvedType.Kind == TypeNamed {
			resolvedType, _ = fb.owner.module.Types.Lookup(resolvedType.Base)
		}
		if resolvedType.Kind == TypeDecimal || resolvedType.Kind == TypeDecimal128 {
			decimal, err := parseDecimal(e.Token.Lexeme)
			if err != nil {
				return builtValue{}, err
			}
			return fb.result(Operation{Kind: OpConstDecimal, Decimal: &decimal, Location: loc}, typeID), nil
		}
		if resolvedType.Kind == TypeFloat {
			return fb.result(Operation{Kind: OpConstFloat, FloatLexeme: e.Token.Lexeme, Location: loc}, typeID), nil
		}
		value, ok := ast.ParseIntegerLiteralLexeme(e.Token.Lexeme)
		if !ok {
			return builtValue{}, fmt.Errorf("invalid integer literal %q", e.Token.Lexeme)
		}
		return fb.result(Operation{Kind: OpConstInt, Integer: value, Location: loc}, typeID), nil
	case *ast.BooleanLiteral:
		v := e.Value
		return fb.result(Operation{Kind: OpConstBool, Bool: &v, Location: loc}, typeID), nil
	case *ast.StringLiteral:
		return fb.result(Operation{Kind: OpConstString, String: e.Value, Location: loc}, typeID), nil
	case *ast.FloatLiteral:
		t, _ := fb.owner.module.Types.Lookup(typeID)
		if t.Kind == TypeFloat {
			return fb.result(Operation{Kind: OpConstFloat, FloatLexeme: e.Token.Lexeme, Location: loc}, typeID), nil
		}
		decimal, err := parseDecimal(e.Token.Lexeme)
		if err != nil {
			return builtValue{}, err
		}
		return fb.result(Operation{Kind: OpConstDecimal, Decimal: &decimal, Location: loc}, typeID), nil
	case *ast.Identifier:
		fact, ok := fb.owner.analyzer.ResolvedBindingOf(e)
		if !ok {
			if fb.owner.maxPackage >= 11 {
				if plan, resolved := fb.owner.analyzer.ResolvedUnionConstructionOf(e); resolved {
					return fb.buildUnionConstruction(e, plan, typeID)
				}
			}
			return builtValue{}, fmt.Errorf("missing resolved binding for %s", e.Value)
		}
		bind, ok := fb.bindings[fact.ID]
		if !ok {
			return builtValue{}, fmt.Errorf("binding %d has no IR value", fact.ID)
		}
		if _, fixed := sema.FixedArrayLength(fact.Type); fixed && sema.CopyClassificationOf(fact.Type) != sema.CopyTrivial {
			return builtValue{}, fb.unsupported("non-trivial fixed-array value transfer", e.Token)
		}
		if bind.storage != 0 {
			return fb.result(Operation{Kind: OpStorageLoad, Storage: bind.storage, Location: loc}, bind.typ), nil
		}
		return builtValue{id: bind.value, typ: bind.typ}, nil
	case *ast.MemberExpression:
		if fb.owner.maxPackage >= 14 && e.Property != nil {
			member, resolved := fb.owner.analyzer.CompilerKnownMemberAt(e.Property.Token.File, e.Property.Token.Line, e.Property.Token.Column)
			if resolved && member.ID == "CKM-LEN-ARRAY" {
				return fb.buildFixedArrayLength(e, typeID)
			}
		}
		if fb.owner.maxPackage >= 13 {
			if plan, resolved := fb.owner.analyzer.ResolvedStructMemberOf(e); resolved && plan.Kind == sema.MemberStoredField {
				if plan.Action != sema.StructFieldCopyTrivial {
					return builtValue{}, fb.unsupported("non-trivial struct field read", e.Token)
				}
				source, err := fb.buildExpr(e.Object, 0)
				if err != nil {
					return builtValue{}, err
				}
				return fb.result(Operation{Kind: OpStructExtractField, Operands: []ValueID{source.id}, StructField: StructFieldID(plan.FieldID), StructActions: []StructFieldAction{StructActionCopyTrivial}, Location: loc}, typeID), nil
			}
		}
		if fb.owner.maxPackage >= 11 {
			if enumCase, resolved := fb.owner.analyzer.ResolvedEnumCaseOf(e); resolved {
				return fb.result(Operation{Kind: OpEnumConstant, EnumCase: EnumCaseID(enumCase.Ordinal), Location: loc}, typeID), nil
			}
			if plan, resolved := fb.owner.analyzer.ResolvedUnionConstructionOf(e); resolved {
				return fb.buildUnionConstruction(e, plan, typeID)
			}
		}
		return builtValue{}, fb.unsupported("member expression", e.Token)
	case *ast.CallExpression:
		if fb.owner.maxPackage >= 11 {
			if conversion, resolved := fb.owner.analyzer.ResolvedEnumConversionOf(e); resolved {
				return fb.buildEnumConversion(e, conversion, typeID)
			}
			if plan, resolved := fb.owner.analyzer.ResolvedUnionConstructionOf(e); resolved {
				return fb.buildUnionConstruction(e, plan, typeID)
			}
		}
		if fb.owner.maxPackage < 3 {
			return builtValue{}, fb.unsupported("function call", e.Token)
		}
		return fb.buildCall(e)
	case *ast.PrefixExpression, *ast.InfixExpression:
		if fb.owner.maxPackage < 7 {
			return builtValue{}, fb.unsupported("integer operator", expressionToken(expr))
		}
		return fb.buildResolvedOperator(expr)
	case *ast.StructLiteral:
		if fb.owner.maxPackage >= 13 {
			if plan, resolved := fb.owner.analyzer.ResolvedStructLiteralPlanOf(e); resolved {
				return fb.buildStructLiteral(e, plan, typeID)
			}
		}
		if fb.owner.maxPackage >= 11 {
			if plan, resolved := fb.owner.analyzer.ResolvedUnionConstructionOf(e); resolved {
				return fb.buildUnionConstruction(e, plan, typeID)
			}
		}
		return builtValue{}, fb.unsupported("struct literal", e.Token)
	case *ast.ArrayLiteral:
		if fb.owner.maxPackage < 14 {
			return builtValue{}, fb.unsupported("array literal", e.Token)
		}
		return fb.buildArrayLiteral(e, typeID)
	case *ast.IndexExpression:
		if fb.owner.maxPackage < 14 {
			return builtValue{}, fb.unsupported("fixed-array index", e.Token)
		}
		return fb.buildArrayIndexRead(e, typeID)
	case *ast.TryExpression:
		if fb.owner.maxPackage < 9 {
			return builtValue{}, fb.unsupported("try expression", e.Token)
		}
		return fb.buildTryExpression(e)
	case *ast.MatchExpression:
		if fb.owner.maxPackage < 12 {
			return builtValue{}, fb.unsupported("match expression", e.Token)
		}
		return fb.buildMatchExpression(e, typeID)
	default:
		return builtValue{}, fb.unsupported(fmt.Sprintf("expression %T", expr), expressionToken(expr))
	}
}

// buildFixedArrayLength connects the compiler-known fixed-array Len property to
// the compact, foldable Semantic IR array.len operation without reconstructing
// member semantics from its source spelling.
//
// Rules:
//   - rules/mlir/packages/sec-mlir-dialect_package14.md sections 29 and 65
//   - rules/mlir/semantic-ir/sec_semantic_ir_fixed_array_v1.md — "Array length"
func (fb *functionBuilder) buildFixedArrayLength(expr *ast.MemberExpression, resultType TypeID) (builtValue, error) {
	receiverType, ok := fb.owner.analyzer.ResolvedTypeOf(expr.Object)
	if !ok {
		return builtValue{}, fmt.Errorf("fixed-array Len receiver has no resolved type")
	}
	length, fixed := sema.FixedArrayLength(receiverType)
	if !fixed {
		return builtValue{}, fb.unsupported("non-fixed array Len", expr.Token)
	}
	arrayType, err := fb.owner.internType(receiverType)
	if err != nil {
		return builtValue{}, err
	}
	receiver, err := fb.buildExpr(expr.Object, arrayType)
	if err != nil {
		return builtValue{}, err
	}
	if receiver.typ != arrayType {
		return builtValue{}, fmt.Errorf("fixed-array Len receiver type mismatch")
	}
	return fb.result(Operation{
		Kind: OpArrayLength, Operands: []ValueID{receiver.id},
		ArrayLength: length.String(), Location: location(expr.Property.Token),
	}, resultType), nil
}

// buildArrayLiteral consumes the compact compiler-owned plan required by
// rules/mlir/packages/sec-mlir-dialect_package14.md sections 14-23 and 62-63.
// It emits one segment per source entry and never expands a spread by N.
func (fb *functionBuilder) buildArrayLiteral(expr *ast.ArrayLiteral, resultType TypeID) (builtValue, error) {
	plan, ok := fb.owner.analyzer.ResolvedArrayLiteralPlanOf(expr)
	if !ok || plan.Length == nil {
		return builtValue{}, fb.unsupported("unresolved array literal", expr.Token)
	}
	if sema.CopyClassificationOf(plan.ElementType) != sema.CopyTrivial || !sema.TriviallyDestructible(plan.ElementType) {
		return builtValue{}, fb.unsupported("non-trivial array literal element", expr.Token)
	}
	resolvedResult, resultOK := fb.owner.module.Types.Lookup(resultType)
	if !resultOK || resolvedResult.Kind != TypeArray || resolvedResult.Length != plan.Length.String() {
		return builtValue{}, fmt.Errorf("array literal result type disagrees with resolved plan")
	}
	elementType := resolvedResult.Element
	op := Operation{
		Kind: OpArrayConstruct, ArrayElementType: elementType,
		ArrayLength: plan.Length.String(), Location: location(expr.Token),
	}
	for _, entry := range plan.Entries {
		if entry.SourceIndex < 0 || entry.SourceIndex >= len(expr.Elements) || entry.Length == nil {
			return builtValue{}, fmt.Errorf("array literal plan has invalid source entry")
		}
		source := expr.Elements[entry.SourceIndex]
		expected := elementType
		kind := ArraySegmentElement
		action := ArrayActionConstructDirect
		if entry.Kind == sema.ArrayLiteralSpread {
			spread, spreadOK := source.(*ast.SpreadExpression)
			resolvedAction, supported := semanticArraySpreadAction(entry.Action)
			if !spreadOK || !supported {
				return builtValue{}, fb.unsupported("non-trivial array spread action "+string(entry.Action), expressionToken(source))
			}
			source = spread.Value
			var internErr error
			expected, internErr = fb.owner.internType(entry.Type)
			if internErr != nil {
				return builtValue{}, internErr
			}
			kind = ArraySegmentSpread
			action = resolvedAction
		} else if entry.Kind != sema.ArrayLiteralElement || entry.Action != sema.ArrayTransferConstructDirect {
			return builtValue{}, fb.unsupported("non-trivial array element action "+string(entry.Action), expressionToken(source))
		}
		value, buildErr := fb.buildExpr(source, expected)
		if buildErr != nil {
			return builtValue{}, buildErr
		}
		op.Operands = append(op.Operands, value.id)
		op.ArraySegmentKinds = append(op.ArraySegmentKinds, kind)
		op.ArraySegmentLengths = append(op.ArraySegmentLengths, entry.Length.String())
		op.ArrayActions = append(op.ArrayActions, action)
	}
	return fb.result(op, resultType), nil
}

// semanticArraySpreadAction admits only the transfer represented by Package 14
// and keeps semantic copy, move and borrow actions visible for later packages.
//
// Rules:
//   - rules/mlir/packages/sec-mlir-dialect_package14.md — sections 21, 89, 103
//   - rules/mlir/semantic-ir/sec_semantic_ir_fixed_array_v1.md — "Array construction"
func semanticArraySpreadAction(action sema.ResolvedArrayTransferAction) (ArrayTransferAction, bool) {
	if action == sema.ArrayTransferCopyTrivial {
		return ArrayActionCopyTrivial, true
	}
	return "", false
}

// buildArrayIndexRead implements
// rules/mlir/packages/sec-mlir-dialect_package14.md sections 42, 48, 71, and
// 73. The array expression is evaluated first and the index second, each
// exactly once. Runtime checks branch through one canonical predicate;
// proven-safe reads carry Sema's explicit proof and emit no failure path.
func (fb *functionBuilder) buildArrayIndexRead(expr *ast.IndexExpression, resultType TypeID) (builtValue, error) {
	plan, ok := fb.owner.analyzer.ResolvedArrayIndexPlanOf(expr)
	if !ok || plan.UseKind != sema.ArrayIndexRead {
		return builtValue{}, fb.unsupported("unresolved or non-read fixed-array index", expr.Token)
	}
	if plan.Action != sema.ArrayTransferCopyTrivial {
		return builtValue{}, fb.unsupported("non-trivial fixed-array element read", expr.Token)
	}
	arrayType, err := fb.owner.internType(plan.ArrayType)
	if err != nil {
		return builtValue{}, err
	}
	indexType, err := fb.owner.internType(plan.IndexType)
	if err != nil {
		return builtValue{}, err
	}
	array, err := fb.buildExpr(expr.Left, arrayType)
	if err != nil {
		return builtValue{}, err
	}
	index, err := fb.buildExpr(expr.Index, indexType)
	if err != nil {
		return builtValue{}, err
	}
	if array.typ != arrayType || index.typ != indexType {
		return builtValue{}, fmt.Errorf("fixed-array index operand type mismatch")
	}

	op := Operation{
		Kind: OpArrayExtract, Operands: []ValueID{array.id, index.id},
		ArrayActions: []ArrayTransferAction{ArrayActionCopyTrivial}, Location: location(expr.Token),
	}
	if plan.CheckKind == sema.ArrayIndexProvenSafe {
		proof, proofOK := semanticArrayIndexProof(plan.ProofKind)
		if !proofOK || plan.FailureMode != sema.ArrayIndexFailureNone {
			return builtValue{}, fmt.Errorf("invalid proven-safe fixed-array index plan")
		}
		op.ArrayCheckKind = ArrayIndexProvenSafe
		op.ArrayProofKind = proof
		return fb.result(op, resultType), nil
	}
	if plan.CheckKind != sema.ArrayIndexRuntimeCheck || plan.FailureMode != sema.ArrayIndexFailureOrdinary {
		return builtValue{}, fb.unsupported("non-ordinary fixed-array index failure mode", expr.Token)
	}
	boolType, err := fb.owner.internType(sema.Type{Name: "bool", Kind: sema.BoolType})
	if err != nil {
		return builtValue{}, err
	}
	predicate := fb.result(Operation{
		Kind: OpArrayIndexInBounds, Operands: []ValueID{array.id, index.id},
		ArrayIndexSigned: plan.IndexSigned, Location: location(expr.Token),
	}, boolType)
	success := fb.newBlock()
	failure := fb.newBlock()
	fb.emit(Operation{
		Kind: OpCondBranch, Operands: []ValueID{predicate.id},
		Successors: []BranchTarget{{Block: success.ID}, {Block: failure.ID}}, Location: location(expr.Token),
	})
	fb.current = failure
	fb.emit(Operation{Kind: OpBoundsFailure, ArrayOperation: "fixed-array-index", Location: location(expr.Token)})
	fb.current = success
	op.ArrayCheckKind = ArrayIndexRuntimeCheck
	op.ArrayProofKind = ArrayIndexProofGuarded
	op.ArrayGuard = predicate.id
	return fb.result(op, resultType), nil
}

func semanticArrayIndexProof(proof sema.ArrayIndexProofKind) (ArrayIndexProofKind, bool) {
	switch proof {
	case sema.ArrayIndexProofConstant:
		return ArrayIndexProofConstant, true
	case sema.ArrayIndexProofRange:
		return ArrayIndexProofRange, true
	case sema.ArrayIndexProofBranch:
		return ArrayIndexProofBranch, true
	case sema.ArrayIndexProofContract:
		return ArrayIndexProofContract, true
	case sema.ArrayIndexProofOther:
		return ArrayIndexProofAnalysis, true
	default:
		return "", false
	}
}

func (fb *functionBuilder) buildStructLiteral(expr *ast.StructLiteral, plan sema.ResolvedStructLiteralPlan, resultType TypeID) (builtValue, error) {
	definition, ok := fb.owner.structDefinition(resultType)
	if !ok || len(definition.Fields) != len(plan.FinalFields) || !plan.FullyInitialized {
		return builtValue{}, fb.unsupported("struct construction without complete definition/plan", expr.Token)
	}
	explicit := map[int]builtValue{}
	spread := map[int][]builtValue{}
	for _, entry := range plan.Entries {
		switch entry.Kind {
		case sema.StructEntryExplicit:
			fieldType := definition.Fields[entry.FieldID].Type
			value, err := fb.buildExpr(entry.Expression, fieldType)
			if err != nil {
				return builtValue{}, err
			}
			explicit[entry.SourceIndex] = value
		case sema.StructEntrySpread:
			source, err := fb.buildExpr(entry.Expression, resultType)
			if err != nil {
				return builtValue{}, err
			}
			op := Operation{Kind: OpStructSpreadFields, Operands: []ValueID{source.id}, Location: locationFromExpression(entry.Expression)}
			values := make([]builtValue, len(definition.Fields))
			for index, field := range definition.Fields {
				if sema.CopyClassificationOf(plan.FinalFields[index].FieldType) != sema.CopyTrivial {
					return builtValue{}, fb.unsupported("non-trivial struct spread", expressionToken(entry.Expression))
				}
				value := fb.newValue(field.Type, OwnershipImmediate, op.Location)
				op.Results = append(op.Results, value)
				op.StructActions = append(op.StructActions, StructActionCopyTrivial)
				values[index] = builtValue{id: value.ID, typ: value.Type}
			}
			fb.emit(op)
			spread[entry.SourceIndex] = values
		}
	}
	op := Operation{Kind: OpStructConstruct, Location: location(expr.Token)}
	for index, field := range plan.FinalFields {
		var value builtValue
		var err error
		switch field.SourceKind {
		case sema.StructFieldSourceExplicit:
			value = explicit[field.SourceEntryIndex]
			op.StructOrigins = append(op.StructOrigins, StructOriginExplicit)
		case sema.StructFieldSourceSpread:
			value = spread[field.SourceEntryIndex][field.SpreadFieldID]
			op.StructOrigins = append(op.StructOrigins, StructOriginSpread)
		case sema.StructFieldSourceDefault:
			value, err = fb.buildResolvedDefault(field.FieldType, field.Default, location(expr.Token))
			if err != nil {
				return builtValue{}, err
			}
			op.StructOrigins = append(op.StructOrigins, StructOriginDefault)
		default:
			return builtValue{}, fmt.Errorf("struct field %d has no resolved source", index)
		}
		action, ok := semanticStructAction(field.Action)
		if !ok || action != StructActionConstructDirect && action != StructActionCopyTrivial {
			return builtValue{}, fb.unsupported("non-trivial struct field action "+string(field.Action), expr.Token)
		}
		op.Operands = append(op.Operands, value.id)
		op.StructActions = append(op.StructActions, action)
	}
	return fb.result(op, resultType), nil
}

func semanticStructAction(action sema.ResolvedStructFieldAction) (StructFieldAction, bool) {
	switch action {
	case sema.StructFieldConstructDirect:
		return StructActionConstructDirect, true
	case sema.StructFieldCopyTrivial:
		return StructActionCopyTrivial, true
	case sema.StructFieldCopySemanticInfallible:
		return StructActionCopySemanticInfallible, true
	case sema.StructFieldMove:
		return StructActionMove, true
	default:
		return "", false
	}
}

func (fb *functionBuilder) buildResolvedDefault(typ sema.Type, resolution sema.DefaultResolution, loc Location) (builtValue, error) {
	typeID, err := fb.owner.internType(typ)
	if err != nil {
		return builtValue{}, err
	}
	switch resolution.Kind {
	case sema.PrimitiveDefault, sema.NamedDefault, sema.RangeDefault, sema.MembershipDefault, sema.ExplicitTypeDefault:
		switch typ.Kind {
		case sema.BoolType:
			value := resolution.Value.Bool
			return fb.result(Operation{Kind: OpConstBool, Bool: &value, Location: loc}, typeID), nil
		case sema.StringType:
			return fb.result(Operation{Kind: OpConstString, String: resolution.Value.String, Location: loc}, typeID), nil
		case sema.IntType, sema.UintType, sema.CharType, sema.RuneType:
			if resolution.Value.Integer == nil {
				return builtValue{}, fb.unsupported("non-integer scalar default", lexer.Token{})
			}
			return fb.result(Operation{Kind: OpConstInt, Integer: new(big.Int).Set(resolution.Value.Integer), Location: loc}, typeID), nil
		case sema.FloatType:
			return fb.result(Operation{Kind: OpConstFloat, FloatLexeme: resolution.Value.Lexeme, Location: loc}, typeID), nil
		case sema.DecimalType:
			decimal, parseErr := parseDecimal(resolution.Value.Lexeme)
			if parseErr != nil {
				return builtValue{}, parseErr
			}
			return fb.result(Operation{Kind: OpConstDecimal, Decimal: &decimal, Location: loc}, typeID), nil
		}
	case sema.StructDefault:
		definition, ok := fb.owner.structDefinition(typeID)
		if !ok {
			break
		}
		op := Operation{Kind: OpStructConstruct, Location: loc}
		resolvedByName := map[string]sema.DefaultResolution{}
		for _, field := range resolution.Fields {
			resolvedByName[field.Name] = field.Value
		}
		for index, field := range typ.Fields {
			value, valueErr := fb.buildResolvedDefault(field.Type, resolvedByName[field.Name], loc)
			if valueErr != nil {
				return builtValue{}, valueErr
			}
			op.Operands = append(op.Operands, value.id)
			op.StructOrigins = append(op.StructOrigins, StructOriginDefault)
			op.StructActions = append(op.StructActions, StructActionConstructDirect)
			_ = definition.Fields[index]
		}
		return fb.result(op, typeID), nil
	case sema.ArrayDefault:
		// SEC-MLIR Package 14 sections 24-27: array defaults remain one compact
		// semantic operation. Zero length never queries or constructs an element;
		// positive lengths are restricted to the infallible trivial P14 subset.
		length, fixed := sema.FixedArrayLength(typ)
		if typ.Kind != sema.ArrayType || typ.Element == nil || !fixed {
			return builtValue{}, fb.unsupported("dynamic or malformed array default", lexer.Token{})
		}
		if resolution.ArrayLengthDecimal != length.String() {
			return builtValue{}, fmt.Errorf("array default length fact mismatch for %s", typ.Name)
		}
		if length.Sign() != 0 {
			if resolution.ArrayElementDefault == nil || sema.CopyClassificationOf(*typ.Element) != sema.CopyTrivial || !sema.TriviallyDestructible(*typ.Element) {
				return builtValue{}, fb.unsupported("non-trivial fixed-array default for "+typ.Name, lexer.Token{})
			}
		}
		elementType, elementErr := fb.owner.internType(*typ.Element)
		if elementErr != nil {
			return builtValue{}, elementErr
		}
		return fb.result(Operation{
			Kind: OpArrayDefault, ArrayElementType: elementType,
			ArrayLength: length.String(), Location: loc,
		}, typeID), nil
	}
	return builtValue{}, fb.unsupported("resolved default "+string(resolution.Kind)+" for "+typ.Name, lexer.Token{})
}

func (fb *functionBuilder) buildEnumConversion(call *ast.CallExpression, conversion sema.ResolvedEnumConversion, resultType TypeID) (builtValue, error) {
	if len(call.Arguments) != 1 {
		return builtValue{}, fb.unsupported("enum conversion arity", call.Token)
	}
	operandType, err := fb.owner.internType(conversion.IntegerType)
	if conversion.Kind == sema.ResolvedEnumToInteger {
		operandType, err = fb.owner.internType(conversion.EnumType)
	}
	if err != nil {
		return builtValue{}, err
	}
	operand, err := fb.buildExpr(call.Arguments[0], operandType)
	if err != nil {
		return builtValue{}, err
	}
	kind := OpEnumFromInteger
	if conversion.Kind == sema.ResolvedEnumToInteger {
		kind = OpEnumToInteger
	}
	return fb.result(Operation{Kind: kind, Operands: []ValueID{operand.id}, Location: location(call.Token)}, resultType), nil
}

func (fb *functionBuilder) buildUnionConstruction(expr ast.Expression, plan sema.ResolvedUnionConstruction, resultType TypeID) (builtValue, error) {
	definition, ok := fb.owner.unionDefinition(resultType)
	if !ok || int(plan.VariantIndex) >= len(definition.Variants) {
		return builtValue{}, fb.unsupported("union construction without definition", expressionToken(expr))
	}
	variant := definition.Variants[plan.VariantIndex]
	op := Operation{Kind: OpUnionConstruct, UnionVariant: UnionVariantIndex(plan.VariantIndex), Location: locationFromExpression(expr)}
	switch plan.Kind {
	case sema.ResolvedUnionVariantEmpty:
		if variant.Kind != UnionVariantEmpty {
			return builtValue{}, fmt.Errorf("resolved empty union variant disagrees with definition")
		}
	case sema.ResolvedUnionVariantSingle:
		call, ok := expr.(*ast.CallExpression)
		if !ok || len(call.Arguments) != 1 || variant.Kind != UnionVariantSingle {
			return builtValue{}, fb.unsupported("single-payload union construction", expressionToken(expr))
		}
		semaVariant := plan.UnionType.UnionVariants[plan.VariantIndex]
		if semaVariant.Payload == nil || sema.CopyClassificationOf(*semaVariant.Payload) != sema.CopyTrivial {
			return builtValue{}, fb.unsupported("non-trivial union payload transfer", expressionToken(call.Arguments[0]))
		}
		payload, err := fb.buildExpr(call.Arguments[0], variant.Payload)
		if err != nil {
			return builtValue{}, err
		}
		op.Operands = []ValueID{payload.id}
		op.PayloadActions = []UnionPayloadAction{UnionPayloadCopyTrivial}
	case sema.ResolvedUnionVariantFields:
		literal, ok := expr.(*ast.StructLiteral)
		if !ok || variant.Kind != UnionVariantFields {
			return builtValue{}, fb.unsupported("field-payload union construction", expressionToken(expr))
		}
		values := map[string]builtValue{}
		fieldTypes := map[string]TypeID{}
		for _, field := range variant.PayloadFields {
			fieldTypes[field.Name] = field.Type
		}
		semaVariant := plan.UnionType.UnionVariants[plan.VariantIndex]
		semaFields := map[string]sema.Type{}
		for _, field := range semaVariant.PayloadFields {
			semaFields[field.Name] = field.Type
		}
		// Evaluate in source order before canonical declaration-order assembly.
		for _, sourceField := range literal.Fields {
			if sourceField == nil || sourceField.Name == nil || sourceField.Spread {
				continue
			}
			name := sourceField.Name.Value
			if sema.CopyClassificationOf(semaFields[name]) != sema.CopyTrivial {
				return builtValue{}, fb.unsupported("non-trivial union field transfer", sourceField.Token)
			}
			value, err := fb.buildExpr(sourceField.Value, fieldTypes[name])
			if err != nil {
				return builtValue{}, err
			}
			values[name] = value
		}
		for _, field := range variant.PayloadFields {
			value, exists := values[field.Name]
			if !exists {
				return builtValue{}, fmt.Errorf("union field %s was not evaluated", field.Name)
			}
			op.Operands = append(op.Operands, value.id)
			op.UnionFields = append(op.UnionFields, field.Name)
			op.PayloadActions = append(op.PayloadActions, UnionPayloadCopyTrivial)
		}
	default:
		return builtValue{}, fb.unsupported("union variant kind", expressionToken(expr))
	}
	return fb.result(op, resultType), nil
}

func (b *builder) unionDefinition(typeID TypeID) (UnionDefinition, bool) {
	for _, definition := range b.module.Unions {
		if definition.TypeID == typeID {
			return definition, true
		}
	}
	return UnionDefinition{}, false
}

func (b *builder) structDefinition(typeID TypeID) (StructDefinition, bool) {
	for _, definition := range b.module.Structs {
		if definition.TypeID == typeID {
			return definition, true
		}
	}
	return StructDefinition{}, false
}

func (b *builder) enumDefinition(typeID TypeID) (EnumDefinition, bool) {
	for _, definition := range b.module.Enums {
		if definition.TypeID == typeID {
			return definition, true
		}
	}
	return EnumDefinition{}, false
}

func enumCaseIDByName(definition EnumDefinition, name string) (EnumCaseID, bool) {
	for _, enumCase := range definition.Cases {
		if enumCase.Name == name {
			return enumCase.ID, true
		}
	}
	return 0, false
}

func (fb *functionBuilder) buildMatchExpression(expr *ast.MatchExpression, resultType TypeID) (builtValue, error) {
	return fb.buildResolvedMatch(expr, resultType, true)
}

func (fb *functionBuilder) buildMatchStatement(stmt *ast.MatchStatement) error {
	if stmt == nil || stmt.Match == nil {
		return fb.unsupported("empty match statement", stmt.Token)
	}
	_, err := fb.buildResolvedMatch(stmt.Match, 0, false)
	return err
}

func (fb *functionBuilder) buildResolvedMatch(expr *ast.MatchExpression, resultType TypeID, valueContext bool) (builtValue, error) {
	plan, ok := fb.owner.analyzer.ResolvedMatchPlanOf(expr)
	if !ok || !plan.Exhaustive || plan.ValueContext != valueContext {
		return builtValue{}, fb.unsupported("unresolved or non-exhaustive match", expr.Token)
	}
	switch plan.SubjectKind {
	case sema.MatchSubjectEnum, sema.MatchSubjectUnion, sema.MatchSubjectResult, sema.MatchSubjectOption:
	default:
		return builtValue{}, fb.unsupported("match subject kind "+string(plan.SubjectKind), expr.Token)
	}
	subject, err := fb.buildExpr(expr.Subject, 0)
	if err != nil {
		return builtValue{}, err
	}
	if subject.typ == 0 || valueContext && resultType == 0 {
		return builtValue{}, fb.unsupported("untyped match", expr.Token)
	}
	boolType, err := fb.owner.internType(sema.Type{Name: "bool", Kind: sema.BoolType})
	if err != nil {
		return builtValue{}, err
	}
	var merge *Block
	var parameter Value
	recordResultType := resultType
	if valueContext {
		merge = fb.newBlock()
		parameter = fb.newValue(resultType, OwnershipImmediate, location(expr.Token))
		merge.Parameters = []Value{parameter}
	} else {
		recordResultType, err = fb.owner.internType(sema.Type{Name: "void", Kind: sema.VoidType})
		if err != nil {
			return builtValue{}, err
		}
		if matchPlanContinues(plan) {
			merge = fb.newBlock()
		}
	}
	matchID := fb.nextMatch
	fb.nextMatch++
	record := MatchRecord{
		ID: matchID, Subject: subject.id, SubjectType: subject.typ, ResultType: recordResultType,
		ValueContext: valueContext, Exhaustive: true, MergeBlock: blockIDOrZero(merge, merge != nil), Location: location(expr.Token),
	}

	for planIndex, armPlan := range plan.Arms {
		if armPlan.SourceIndex < 0 || armPlan.SourceIndex >= len(expr.Arms) {
			return builtValue{}, fmt.Errorf("resolved match arm source index is invalid")
		}
		arm := expr.Arms[armPlan.SourceIndex]
		patternBlock := fb.current
		if patternBlock == nil {
			return builtValue{}, fmt.Errorf("match pattern chain terminated before arm %d", armPlan.SourceIndex)
		}
		bodyBlock := fb.newBlock()
		matchedBlock := bodyBlock
		if armPlan.Guarded {
			matchedBlock = fb.newBlock()
		}
		needsNext := armPlan.PatternKind != sema.MatchPatternCatchAll || armPlan.Guarded
		var nextBlock *Block
		if needsNext {
			nextBlock = fb.newBlock()
		}
		if err := fb.emitMatchPattern(subject, plan, armPlan, matchedBlock, nextBlock, boolType, matchID, arm.Token); err != nil {
			return builtValue{}, err
		}

		fb.current = matchedBlock
		payload, hasPayload, err := fb.projectMatchPayload(subject, armPlan, matchID, arm.Token)
		if err != nil {
			return builtValue{}, err
		}
		restore, err := fb.bindMatchArm(arm, armPlan, payload, hasPayload)
		if err != nil {
			return builtValue{}, err
		}
		if armPlan.Guarded {
			guard, err := fb.buildExpr(arm.Guard, boolType)
			if err != nil {
				restore()
				return builtValue{}, err
			}
			fb.emit(Operation{Kind: OpCondBranch, Operands: []ValueID{guard.id}, Successors: []BranchTarget{{Block: bodyBlock.ID}, {Block: nextBlock.ID}}, MatchID: matchID, MatchArmIndex: armPlan.SourceIndex, MatchStage: "guard", MatchPatternKind: string(armPlan.PatternKind), Location: location(arm.Token)})
		}
		fb.current = bodyBlock
		newBodyBlocksFrom := len(fb.fn.Blocks)
		if valueContext {
			err = fb.buildMatchExpressionArm(arm, armPlan, merge, resultType, matchID)
		} else {
			err = fb.buildMatchStatementArm(arm, armPlan, merge, matchID)
		}
		if err != nil {
			restore()
			return builtValue{}, err
		}
		fb.markMatchArmTerminators(append([]*Block{bodyBlock}, fb.fn.Blocks[newBodyBlocksFrom:]...), armPlan, matchID)
		restore()
		record.Arms = append(record.Arms, MatchArmRecord{
			SourceIndex: armPlan.SourceIndex, PatternKind: string(armPlan.PatternKind), PatternBlock: patternBlock.ID,
			GuardBlock: blockIDOrZero(matchedBlock, armPlan.Guarded), BodyBlock: bodyBlock.ID,
			VariantIndex: UnionVariantIndex(armPlan.UnionVariantIndex), EnumValue: cloneBigInt(armPlan.EnumNumericValue),
			Guarded: armPlan.Guarded, Flow: string(armPlan.Flow), Location: location(arm.Token),
		})
		if nextBlock == nil {
			fb.current = nil
			break
		}
		fb.current = nextBlock
		if planIndex == len(plan.Arms)-1 {
			fb.emit(Operation{Kind: OpUnreachable, Synthesized: true, Reason: "exhaustive-match-fallthrough", MatchID: matchID, MatchArmIndex: armPlan.SourceIndex, MatchStage: "residual", MatchPatternKind: string(armPlan.PatternKind), Location: location(expr.Token)})
			fb.current = nil
		}
	}
	fb.fn.Matches = append(fb.fn.Matches, record)
	fb.current = merge
	if !valueContext {
		return builtValue{}, nil
	}
	return builtValue{id: parameter.ID, typ: resultType}, nil
}

func (fb *functionBuilder) markMatchArmTerminators(blocks []*Block, arm sema.ResolvedMatchArm, matchID MatchID) {
	for _, block := range blocks {
		if block == nil || len(block.Operations) == 0 {
			continue
		}
		terminator := &block.Operations[len(block.Operations)-1]
		if terminator.MatchID != 0 || (terminator.Kind != OpReturn && terminator.Kind != OpUnreachable && terminator.Kind != OpArithmeticFailure) {
			continue
		}
		terminator.MatchID = matchID
		terminator.MatchArmIndex = arm.SourceIndex
		terminator.MatchStage = "body-exit"
		terminator.MatchPatternKind = string(arm.PatternKind)
	}
}

func matchPlanContinues(plan sema.ResolvedMatchPlan) bool {
	for _, arm := range plan.Arms {
		if arm.Flow == sema.MatchArmContinues || arm.Flow == sema.MatchArmProducesValue {
			return true
		}
	}
	return false
}

func (fb *functionBuilder) emitMatchPattern(subject builtValue, plan sema.ResolvedMatchPlan, arm sema.ResolvedMatchArm, matchedBlock, nextBlock *Block, boolType TypeID, matchID MatchID, token lexer.Token) error {
	meta := Operation{
		MatchID: matchID, MatchArmIndex: arm.SourceIndex, MatchStage: "pattern",
		MatchPatternKind: string(arm.PatternKind), Location: location(token),
	}
	if arm.PatternKind == sema.MatchPatternCatchAll {
		meta.Kind = OpBranch
		meta.Successors = []BranchTarget{{Block: matchedBlock.ID}}
		fb.emit(meta)
		return nil
	}
	if nextBlock == nil {
		return fmt.Errorf("non-catch-all match arm %d has no false successor", arm.SourceIndex)
	}

	var condition builtValue
	switch plan.SubjectKind {
	case sema.MatchSubjectEnum:
		definition, ok := fb.owner.enumDefinition(subject.typ)
		if !ok || arm.EnumNumericValue == nil {
			return fb.unsupported("unresolved enum match pattern", token)
		}
		caseID, ok := enumCaseIDByName(definition, arm.EnumCaseName)
		if !ok {
			return fb.unsupported("unresolved enum match case", token)
		}
		constant := fb.result(Operation{
			Kind: OpEnumConstant, EnumCase: caseID, MatchID: matchID,
			MatchArmIndex: arm.SourceIndex, MatchStage: "pattern",
			MatchPatternKind: string(arm.PatternKind), Location: location(token),
		}, subject.typ)
		condition = fb.result(Operation{
			Kind: OpEnumCompare, Operands: []ValueID{subject.id, constant.id},
			IntegerCompare: IntegerCompareEQ, MatchID: matchID,
			MatchArmIndex: arm.SourceIndex, MatchStage: "pattern",
			MatchPatternKind: string(arm.PatternKind), Location: location(token),
		}, boolType)
	case sema.MatchSubjectUnion, sema.MatchSubjectOption:
		definition, ok := fb.owner.unionDefinition(subject.typ)
		if !ok || int(arm.UnionVariantIndex) >= len(definition.Variants) {
			return fb.unsupported("unresolved union match variant", token)
		}
		condition = fb.result(Operation{
			Kind: OpUnionIsVariant, Operands: []ValueID{subject.id},
			UnionVariant: UnionVariantIndex(arm.UnionVariantIndex), MatchID: matchID,
			MatchArmIndex: arm.SourceIndex, MatchStage: "pattern",
			MatchPatternKind: string(arm.PatternKind), Location: location(token),
		}, boolType)
	case sema.MatchSubjectResult:
		condition = fb.result(Operation{
			Kind: OpResultIsErr, Operands: []ValueID{subject.id}, MatchID: matchID,
			MatchArmIndex: arm.SourceIndex, MatchStage: "pattern",
			MatchPatternKind: string(arm.PatternKind), Location: location(token),
		}, boolType)
	default:
		return fb.unsupported("match subject kind "+string(plan.SubjectKind), token)
	}

	meta.Kind = OpCondBranch
	meta.Operands = []ValueID{condition.id}
	if plan.SubjectKind == sema.MatchSubjectResult && arm.PatternKind == sema.MatchPatternResultOk {
		// result.is-err is false for an Ok pattern.
		meta.Successors = []BranchTarget{{Block: nextBlock.ID}, {Block: matchedBlock.ID}}
	} else {
		meta.Successors = []BranchTarget{{Block: matchedBlock.ID}, {Block: nextBlock.ID}}
	}
	fb.emit(meta)
	return nil
}

func (fb *functionBuilder) projectMatchPayload(subject builtValue, arm sema.ResolvedMatchArm, matchID MatchID, token lexer.Token) (builtValue, bool, error) {
	switch arm.BindingAction {
	case sema.MatchBindingNone, sema.MatchBindingDiscard:
		return builtValue{}, false, nil
	case sema.MatchBindingCopyTrivial:
		// Package 12 deliberately supports only payload copies that need no
		// ownership commit or cleanup when a later guard rejects the arm.
	default:
		return builtValue{}, false, fb.unsupported("ownership-sensitive match payload binding "+string(arm.BindingAction), token)
	}

	meta := Operation{
		Operands: []ValueID{subject.id}, MatchID: matchID, MatchArmIndex: arm.SourceIndex,
		MatchStage: "pattern", MatchPatternKind: string(arm.PatternKind), Location: location(token),
	}
	switch arm.PatternKind {
	case sema.MatchPatternUnionVariant, sema.MatchPatternOptionSome:
		definition, ok := fb.owner.unionDefinition(subject.typ)
		if !ok || int(arm.UnionVariantIndex) >= len(definition.Variants) {
			return builtValue{}, false, fb.unsupported("unresolved union match payload", token)
		}
		variant := definition.Variants[arm.UnionVariantIndex]
		if variant.Kind == UnionVariantFields && variant.SyntheticPayloadStruct != 0 {
			// Package 13 sections 66-68 materialize a whole struct-like payload
			// only on the proven variant path, using guarded P11 projections.
			construct := Operation{Kind: OpStructConstruct, MatchID: matchID, MatchArmIndex: arm.SourceIndex, MatchStage: "pattern", MatchPatternKind: string(arm.PatternKind), Location: location(token)}
			for _, field := range variant.PayloadFields {
				projection := meta
				projection.Kind = OpUnionUnwrapField
				projection.UnionVariant = UnionVariantIndex(arm.UnionVariantIndex)
				projection.UnionField = field.Name
				projection.PayloadActions = []UnionPayloadAction{UnionPayloadCopyTrivial}
				value := fb.result(projection, field.Type)
				construct.Operands = append(construct.Operands, value.id)
				construct.StructOrigins = append(construct.StructOrigins, StructOriginExplicit)
				construct.StructActions = append(construct.StructActions, StructActionCopyTrivial)
			}
			return fb.result(construct, variant.SyntheticPayloadStruct), true, nil
		}
		bindingType, err := fb.owner.internType(arm.BindingType)
		if err != nil {
			return builtValue{}, false, err
		}
		if variant.Kind != UnionVariantSingle || variant.Payload != bindingType {
			return builtValue{}, false, fb.unsupported("mismatched union match payload", token)
		}
		meta.Kind = OpUnionUnwrapPayload
		meta.UnionVariant = UnionVariantIndex(arm.UnionVariantIndex)
		meta.PayloadActions = []UnionPayloadAction{UnionPayloadCopyTrivial}
	case sema.MatchPatternResultOk, sema.MatchPatternResultErr:
		bindingType, err := fb.owner.internType(arm.BindingType)
		if err != nil {
			return builtValue{}, false, err
		}
		result, ok := fb.owner.module.Types.Lookup(subject.typ)
		if !ok || result.Kind != TypeResult {
			return builtValue{}, false, fb.unsupported("Result match payload without Result subject", token)
		}
		expected := result.Success
		meta.Kind = OpResultUnwrapOk
		if arm.PatternKind == sema.MatchPatternResultErr {
			expected = result.Error
			meta.Kind = OpResultUnwrapErr
		}
		if expected != bindingType {
			return builtValue{}, false, fb.unsupported("mismatched Result match payload", token)
		}
	default:
		return builtValue{}, false, fb.unsupported("payload binding for "+string(arm.PatternKind), token)
	}
	bindingType, err := fb.owner.internType(arm.BindingType)
	if err != nil {
		return builtValue{}, false, err
	}
	return fb.result(meta, bindingType), true, nil
}

func matchArmBindingIdentifier(pattern ast.Expression) *ast.Identifier {
	unwrap := func(expression ast.Expression) ast.Expression {
		if reference, ok := expression.(*ast.RefExpression); ok {
			return reference.Value
		}
		return expression
	}
	switch pattern := pattern.(type) {
	case *ast.CallExpression:
		if len(pattern.Arguments) == 1 {
			identifier, _ := unwrap(pattern.Arguments[0]).(*ast.Identifier)
			return identifier
		}
	case *ast.OkExpression:
		identifier, _ := unwrap(pattern.Value).(*ast.Identifier)
		return identifier
	case *ast.ErrExpression:
		identifier, _ := unwrap(pattern.Value).(*ast.Identifier)
		return identifier
	case *ast.Identifier:
		return pattern
	}
	return nil
}

func (fb *functionBuilder) bindMatchArm(arm *ast.MatchArm, plan sema.ResolvedMatchArm, value builtValue, hasValue bool) (func(), error) {
	if plan.BindingName == "" || plan.BindingName == "_" {
		return func() {}, nil
	}
	if !hasValue {
		return nil, fmt.Errorf("match binding %s has no projected payload", plan.BindingName)
	}
	identifier := matchArmBindingIdentifier(arm.Pattern.Expression())
	if identifier == nil || identifier.Value == "_" {
		return nil, fmt.Errorf("match binding %s has no source identifier", plan.BindingName)
	}
	fact, ok := fb.owner.analyzer.ResolvedBindingOf(identifier)
	if !ok {
		return nil, fmt.Errorf("match binding %s has no resolved identity", identifier.Value)
	}
	previous, existed := fb.bindings[fact.ID]
	fb.bindings[fact.ID] = binding{value: value.id, typ: value.typ}
	return func() {
		if existed {
			fb.bindings[fact.ID] = previous
		} else {
			delete(fb.bindings, fact.ID)
		}
	}, nil
}

func (fb *functionBuilder) buildMatchExpressionArm(arm *ast.MatchArm, plan sema.ResolvedMatchArm, merge *Block, resultType TypeID, matchID MatchID) error {
	if arm.ReturnBody != nil {
		return fb.buildReturn(arm.ReturnBody)
	}
	if arm.BlockBody != nil || arm.Body == nil || plan.Flow != sema.MatchArmProducesValue {
		return fb.unsupported("non-expression match arm", arm.Token)
	}
	value, err := fb.buildExpr(arm.Body, resultType)
	if err != nil {
		return err
	}
	fb.emit(Operation{Kind: OpBranch, Successors: []BranchTarget{{Block: merge.ID, Arguments: []ValueID{value.id}}}, MatchID: matchID, MatchArmIndex: plan.SourceIndex, MatchStage: "body-exit", MatchPatternKind: string(plan.PatternKind), Location: location(arm.Token)})
	fb.current = nil
	return nil
}

func (fb *functionBuilder) buildMatchStatementArm(arm *ast.MatchArm, plan sema.ResolvedMatchArm, continuation *Block, matchID MatchID) error {
	if plan.Flow == sema.MatchArmLoopControl {
		return fb.unsupported("match arm loop control before loop Semantic IR", arm.Token)
	}
	if arm.ReturnBody != nil {
		return fb.buildReturn(arm.ReturnBody)
	}
	if arm.BlockBody != nil {
		if err := fb.buildStatements(arm.BlockBody.Statements); err != nil {
			return err
		}
	} else if arm.Body != nil {
		armType, ok := fb.owner.analyzer.ResolvedTypeOf(arm.Body)
		if !ok {
			return fb.unsupported("untyped match statement arm", arm.Token)
		}
		armTypeID, err := fb.owner.internType(armType)
		if err != nil {
			return err
		}
		if _, err := fb.buildExpr(arm.Body, armTypeID); err != nil {
			return err
		}
		if armType.Kind != sema.VoidType && (sema.CopyClassificationOf(armType) != sema.CopyTrivial || !sema.TriviallyDestructible(armType)) {
			return fb.unsupported("ignored non-trivial match statement arm value", arm.Token)
		}
	}
	if fb.current == nil {
		return nil
	}
	if continuation == nil {
		return fmt.Errorf("continuing match arm %d has no continuation", plan.SourceIndex)
	}
	fb.emit(Operation{
		Kind: OpBranch, Successors: []BranchTarget{{Block: continuation.ID}}, MatchID: matchID,
		MatchArmIndex: plan.SourceIndex, MatchStage: "body-exit",
		MatchPatternKind: string(plan.PatternKind), Location: location(arm.Token),
	})
	fb.current = nil
	return nil
}

func blockIDOrZero(block *Block, present bool) BlockID {
	if !present || block == nil {
		return 0
	}
	return block.ID
}

func cloneBigInt(value *big.Int) *big.Int {
	if value == nil {
		return nil
	}
	return new(big.Int).Set(value)
}

func (fb *functionBuilder) buildResultConstructor(value ast.Expression, resultType TypeID, okResult bool, loc Location) (builtValue, error) {
	typ, exists := fb.owner.module.Types.Lookup(resultType)
	if !exists || typ.Kind != TypeResult {
		return builtValue{}, fb.unsupported("Result constructor without expected Result type", expressionToken(value))
	}
	payloadType := typ.Error
	kind := OpResultErr
	if okResult {
		payloadType, kind = typ.Success, OpResultOk
	}
	op := Operation{Kind: kind, Location: loc}
	if value != nil {
		payload, err := fb.buildExpr(value, payloadType)
		if err != nil {
			return builtValue{}, err
		}
		op.Operands = []ValueID{payload.id}
	} else if !fb.owner.isVoidType(payloadType) {
		return builtValue{}, fb.unsupported("empty non-void Result constructor", lexer.Token{})
	}
	return fb.result(op, resultType), nil
}

func (b *builder) isVoidType(id TypeID) bool {
	t, ok := b.module.Types.Lookup(id)
	return ok && t.Kind == TypeVoid
}

func (fb *functionBuilder) buildTryExpression(expr *ast.TryExpression) (builtValue, error) {
	resolved, ok := fb.owner.analyzer.ResolvedTryOf(expr)
	if !ok {
		return builtValue{}, fb.unsupported("unresolved try expression", expr.Token)
	}
	switch resolved.Kind {
	case sema.ResolvedTryArithmeticPropagation:
		if len(expr.Handlers) != 0 {
			return builtValue{}, fb.unsupported("handled arithmetic try", expr.Token)
		}
		return fb.buildResolvedOperatorWithFailure(expr.Expression, &resolved, nil)
	case sema.ResolvedTryResultPropagation:
		if fb.owner.maxPackage < 10 {
			return builtValue{}, fb.unsupported("Result try propagation", expr.Token)
		}
		return fb.buildResultPropagation(expr, resolved)
	case sema.ResolvedTryHandledResult:
		if fb.owner.maxPackage < 10 {
			return builtValue{}, fb.unsupported("handled Result try", expr.Token)
		}
		return fb.buildHandledResult(expr)
	case sema.ResolvedTryHandledArithmetic:
		if fb.owner.maxPackage < 10 {
			return builtValue{}, fb.unsupported("handled arithmetic try", expr.Token)
		}
		return fb.buildResolvedOperatorWithFailure(expr.Expression, nil, expr)
	case sema.ResolvedTryBoundsPropagation, sema.ResolvedTryHandledBounds:
		return fb.buildBoundsTryExpression(expr, resolved)
	default:
		return builtValue{}, fb.unsupported("handled try expression", expr.Token)
	}
}

// buildBoundsTryExpression lowers one Sema-resolved fixed-array check to typed
// IndexError.OutOfBounds flow without a panic endpoint. Rules:
// rules/mlir/packages/sec-mlir-dialect_package14.md sections 50-54.
func (fb *functionBuilder) buildBoundsTryExpression(expr *ast.TryExpression, resolved sema.ResolvedTry) (builtValue, error) {
	indexExpr, ok := expr.Expression.(*ast.IndexExpression)
	if !ok {
		return builtValue{}, fb.unsupported("bounds try without index expression", expr.Token)
	}
	plan, ok := fb.owner.analyzer.ResolvedArrayIndexPlanOf(indexExpr)
	if !ok || plan.CheckKind != sema.ArrayIndexRuntimeCheck || plan.FailureMode != sema.ArrayIndexFailureFallible || plan.Action != sema.ArrayTransferCopyTrivial {
		return builtValue{}, fb.unsupported("non-runtime fallible fixed-array index", expr.Token)
	}
	arrayType, err := fb.owner.internType(plan.ArrayType)
	if err != nil {
		return builtValue{}, err
	}
	indexType, err := fb.owner.internType(plan.IndexType)
	if err != nil {
		return builtValue{}, err
	}
	elementType, err := fb.owner.internType(plan.ElementType)
	if err != nil {
		return builtValue{}, err
	}
	array, err := fb.buildExpr(indexExpr.Left, arrayType)
	if err != nil {
		return builtValue{}, err
	}
	index, err := fb.buildExpr(indexExpr.Index, indexType)
	if err != nil {
		return builtValue{}, err
	}
	boolType, err := fb.owner.internType(sema.Type{Name: "bool", Kind: sema.BoolType})
	if err != nil {
		return builtValue{}, err
	}
	predicate := fb.result(Operation{Kind: OpArrayIndexInBounds, Operands: []ValueID{array.id, index.id}, ArrayIndexSigned: plan.IndexSigned, Location: location(indexExpr.Token)}, boolType)
	successBlock, errorBlock := fb.newBlock(), fb.newBlock()
	fb.emit(Operation{Kind: OpCondBranch, Operands: []ValueID{predicate.id}, Successors: []BranchTarget{{Block: successBlock.ID}, {Block: errorBlock.ID}}, Location: location(expr.Token)})
	fb.current = successBlock
	successValue := fb.result(Operation{Kind: OpArrayExtract, Operands: []ValueID{array.id, index.id}, ArrayCheckKind: ArrayIndexRuntimeCheck, ArrayProofKind: ArrayIndexProofGuarded, ArrayGuard: predicate.id, ArrayActions: []ArrayTransferAction{ArrayActionCopyTrivial}, Location: location(indexExpr.Token)}, elementType)

	errorType, err := fb.owner.internType(resolved.ErrorType)
	if err != nil {
		return builtValue{}, err
	}
	definition, ok := fb.owner.enumDefinition(errorType)
	if !ok {
		return builtValue{}, fb.unsupported("IndexError enum definition", expr.Token)
	}
	caseID, ok := enumCaseIDByName(definition, "OutOfBounds")
	if !ok {
		return builtValue{}, fb.unsupported("IndexError.OutOfBounds", expr.Token)
	}
	fb.current = errorBlock
	errorValue := fb.result(Operation{Kind: OpEnumConstant, EnumCase: caseID, Location: location(expr.Token)}, errorType)
	if resolved.Kind == sema.ResolvedTryHandledBounds {
		plan, ok := fb.owner.analyzer.ResolvedTryPlanOf(expr)
		if !ok || !plan.Exhaustive {
			return builtValue{}, fb.unsupported("unresolved bounds handlers", expr.Token)
		}
		return fb.buildLocalTryHandlers(expr, plan, successBlock, successValue, errorBlock, errorValue)
	}
	enclosingResult, err := fb.owner.internType(resolved.EnclosingResultType)
	if err != nil {
		return builtValue{}, err
	}
	propagated := fb.result(Operation{Kind: OpResultErr, Operands: []ValueID{errorValue.id}, Location: location(expr.Token)}, enclosingResult)
	fb.emit(Operation{Kind: OpReturn, Operands: []ValueID{propagated.id}, Location: location(expr.Token)})
	fb.current = successBlock
	return successValue, nil
}

func (fb *functionBuilder) buildHandledResult(expr *ast.TryExpression) (builtValue, error) {
	plan, ok := fb.owner.analyzer.ResolvedTryPlanOf(expr)
	if !ok || !plan.Exhaustive {
		return builtValue{}, fb.unsupported("unresolved or non-exhaustive Result handler plan", expr.Token)
	}
	resultValue, err := fb.buildExpr(expr.Expression, 0)
	if err != nil {
		return builtValue{}, err
	}
	resultType, ok := fb.owner.module.Types.Lookup(resultValue.typ)
	if !ok || resultType.Kind != TypeResult {
		return builtValue{}, fb.unsupported("handled try operand without Semantic IR Result type", expr.Token)
	}
	boolType, err := fb.owner.internType(sema.Type{Name: "bool", Kind: sema.BoolType})
	if err != nil {
		return builtValue{}, err
	}
	isErr := fb.result(Operation{Kind: OpResultIsErr, Operands: []ValueID{resultValue.id}, Location: location(expr.Token)}, boolType)
	errorBlock, successBlock := fb.newBlock(), fb.newBlock()
	fb.emit(Operation{Kind: OpCondBranch, Operands: []ValueID{isErr.id}, Successors: []BranchTarget{{Block: errorBlock.ID}, {Block: successBlock.ID}}, Location: location(expr.Token)})

	fb.current = errorBlock
	errorValue := fb.result(Operation{Kind: OpResultUnwrapErr, Operands: []ValueID{resultValue.id}, Location: location(expr.Token)}, resultType.Error)
	fb.current = successBlock
	successValue := builtValue{typ: resultType.Success}
	if !fb.owner.isVoidType(resultType.Success) {
		successValue = fb.result(Operation{Kind: OpResultUnwrapOk, Operands: []ValueID{resultValue.id}, Location: location(expr.Token)}, resultType.Success)
	}
	return fb.buildLocalTryHandlers(expr, plan, successBlock, successValue, errorBlock, errorValue)
}

func (fb *functionBuilder) buildLocalTryHandlers(expr *ast.TryExpression, plan sema.ResolvedTryPlan, successBlock *Block, successValue builtValue, errorBlock *Block, errorValue builtValue) (builtValue, error) {
	successType, err := fb.owner.internType(plan.SuccessType)
	if err != nil {
		return builtValue{}, err
	}
	merge := fb.newBlock()
	var merged builtValue
	if !fb.owner.isVoidType(successType) {
		parameter := fb.newValue(successType, OwnershipImmediate, location(expr.Token))
		merge.Parameters = []Value{parameter}
		merged = builtValue{id: parameter.ID, typ: successType}
	} else {
		merged = builtValue{typ: successType}
	}

	var okHandler *sema.ResolvedTryHandler
	errHandlers := make([]sema.ResolvedTryHandler, 0, len(plan.Handlers))
	for index := range plan.Handlers {
		handler := &plan.Handlers[index]
		switch handler.PatternKind {
		case sema.TryHandlerOkBinding, sema.TryHandlerOkDiscard:
			okHandler = handler
		default:
			errHandlers = append(errHandlers, *handler)
		}
	}

	fb.current = successBlock
	if okHandler == nil {
		fb.branchToTryMerge(merge, successValue, Operation{TryHandlerKind: TryHandlerOK, TryHandlerIndex: -1, Location: location(expr.Token)})
	} else if err := fb.buildTryHandler(expr, *okHandler, successValue, merge, plan.Exhaustive); err != nil {
		return builtValue{}, err
	}

	fb.current = errorBlock
	for index, handler := range errHandlers {
		last := index == len(errHandlers)-1
		if handler.PatternKind == sema.TryHandlerErrCatchAll || (last && plan.Exhaustive) {
			if err := fb.buildTryHandler(expr, handler, errorValue, merge, plan.Exhaustive); err != nil {
				return builtValue{}, err
			}
			break
		}
		boolType, err := fb.owner.internType(sema.Type{Name: "bool", Kind: sema.BoolType})
		if err != nil {
			return builtValue{}, err
		}
		var condition builtValue
		if fb.owner.maxPackage >= 11 {
			definition, ok := fb.owner.enumDefinition(errorValue.typ)
			if !ok {
				return builtValue{}, fb.unsupported("try handler error type without enum definition", expr.Handlers[handler.SourceIndex].Token)
			}
			caseID, ok := enumCaseIDByName(definition, handler.Variant)
			if !ok {
				return builtValue{}, fb.unsupported("unknown resolved enum handler case", expr.Handlers[handler.SourceIndex].Token)
			}
			constant := fb.result(Operation{Kind: OpEnumConstant, EnumCase: caseID, TryHandlerKind: TryHandlerErrVariant, TryHandlerIndex: handler.SourceIndex, Variant: handler.Variant, Location: location(expr.Handlers[handler.SourceIndex].Token)}, errorValue.typ)
			condition = fb.result(Operation{Kind: OpEnumCompare, Operands: []ValueID{errorValue.id, constant.id}, IntegerCompare: IntegerCompareEQ, TryHandlerKind: TryHandlerErrVariant, TryHandlerIndex: handler.SourceIndex, Variant: handler.Variant, Location: location(expr.Handlers[handler.SourceIndex].Token)}, boolType)
		} else {
			condition = fb.result(Operation{Kind: OpCoreErrorIsVariant, Operands: []ValueID{errorValue.id}, Variant: handler.Variant, TryHandlerKind: TryHandlerErrVariant, TryHandlerIndex: handler.SourceIndex, Location: location(expr.Handlers[handler.SourceIndex].Token)}, boolType)
		}
		handlerBlock, nextTest := fb.newBlock(), fb.newBlock()
		fb.emit(Operation{Kind: OpCondBranch, Operands: []ValueID{condition.id}, Successors: []BranchTarget{{Block: handlerBlock.ID}, {Block: nextTest.ID}}, TryHandlerKind: TryHandlerErrVariant, TryHandlerIndex: handler.SourceIndex, Variant: handler.Variant, Location: location(expr.Handlers[handler.SourceIndex].Token)})
		fb.current = handlerBlock
		if err := fb.buildTryHandler(expr, handler, errorValue, merge, plan.Exhaustive); err != nil {
			return builtValue{}, err
		}
		fb.current = nextTest
	}
	if fb.current != nil && fb.current != merge && len(fb.current.Operations) == 0 {
		return builtValue{}, fb.unsupported("non-exhaustive local try handler dispatch", expr.Token)
	}
	fb.current = merge
	return merged, nil
}

func (fb *functionBuilder) buildTryHandler(expr *ast.TryExpression, plan sema.ResolvedTryHandler, input builtValue, merge *Block, exhaustive bool) error {
	if plan.SourceIndex < 0 || plan.SourceIndex >= len(expr.Handlers) {
		return fb.unsupported("invalid resolved try handler source index", expr.Token)
	}
	handler := expr.Handlers[plan.SourceIndex]
	kind := semanticTryHandlerKind(plan.PatternKind)
	previous, hadBinding := binding{}, false
	var bindingID sema.BindingID
	if identifier := tryHandlerPatternIdentifier(handler); identifier != nil && identifier.Value != "_" {
		fact, ok := fb.owner.analyzer.ResolvedBindingOf(identifier)
		if !ok {
			return fmt.Errorf("try handler binding %s has no resolved identity", identifier.Value)
		}
		bindingID = fact.ID
		previous, hadBinding = fb.bindings[bindingID]
		fb.bindings[bindingID] = binding{value: input.id, typ: input.typ}
		defer func() {
			if hadBinding {
				fb.bindings[bindingID] = previous
			} else {
				delete(fb.bindings, bindingID)
			}
		}()
	}

	metadata := Operation{TryHandlerKind: kind, TryHandlerIndex: plan.SourceIndex, TryHandlerExhaustive: exhaustive, Variant: plan.Variant, Location: location(handler.Token)}
	if handler.ReturnBody != nil {
		return fb.buildTryHandlerReturn(handler.ReturnBody, metadata)
	}
	if handler.BlockBody != nil {
		if err := fb.buildStatements(handler.BlockBody.Statements); err != nil {
			return err
		}
		if fb.current != nil {
			fb.branchToTryMerge(merge, builtValue{typ: input.typ}, metadata)
		}
		return nil
	}
	if handler.Body == nil {
		return fb.unsupported("empty resolved try handler", handler.Token)
	}
	value, err := fb.buildExpr(handler.Body, mergeParameterType(merge, input.typ))
	if err != nil {
		return err
	}
	fb.branchToTryMerge(merge, value, metadata)
	return nil
}

func (fb *functionBuilder) buildTryHandlerReturn(statement *ast.ReturnStatement, metadata Operation) error {
	block := fb.current
	if err := fb.buildReturn(statement); err != nil {
		return err
	}
	terminator := &block.Operations[len(block.Operations)-1]
	terminator.TryHandlerKind = metadata.TryHandlerKind
	terminator.TryHandlerIndex = metadata.TryHandlerIndex
	terminator.Variant = metadata.Variant
	return nil
}

func (fb *functionBuilder) branchToTryMerge(merge *Block, value builtValue, metadata Operation) {
	metadata.Kind = OpBranch
	metadata.Successors = []BranchTarget{{Block: merge.ID}}
	if len(merge.Parameters) != 0 {
		metadata.Successors[0].Arguments = []ValueID{value.id}
	}
	fb.emit(metadata)
	fb.current = nil
}

func mergeParameterType(merge *Block, fallback TypeID) TypeID {
	if len(merge.Parameters) != 0 {
		return merge.Parameters[0].Type
	}
	return fallback
}

func semanticTryHandlerKind(kind sema.ResolvedTryHandlerPatternKind) TryHandlerKind {
	switch kind {
	case sema.TryHandlerOkBinding, sema.TryHandlerOkDiscard:
		return TryHandlerOK
	case sema.TryHandlerErrCatchAll:
		return TryHandlerErrCatchAll
	default:
		return TryHandlerErrVariant
	}
}

func tryHandlerPatternIdentifier(handler *ast.TryHandler) *ast.Identifier {
	if handler == nil {
		return nil
	}
	switch pattern := handler.Pattern.(type) {
	case *ast.OkExpression:
		identifier, _ := pattern.Value.(*ast.Identifier)
		return identifier
	case *ast.ErrExpression:
		identifier, _ := pattern.Value.(*ast.Identifier)
		return identifier
	default:
		return nil
	}
}

func (fb *functionBuilder) buildResultPropagation(expr *ast.TryExpression, resolved sema.ResolvedTry) (builtValue, error) {
	resultValue, err := fb.buildExpr(expr.Expression, 0)
	if err != nil {
		return builtValue{}, err
	}
	resultType, ok := fb.owner.module.Types.Lookup(resultValue.typ)
	if !ok || resultType.Kind != TypeResult {
		return builtValue{}, fb.unsupported("try operand without Semantic IR Result type", expr.Token)
	}
	boolType, err := fb.owner.internType(sema.Type{Name: "bool", Kind: sema.BoolType})
	if err != nil {
		return builtValue{}, err
	}
	isErr := fb.result(Operation{Kind: OpResultIsErr, Operands: []ValueID{resultValue.id}, Location: location(expr.Token)}, boolType)
	errorBlock, successBlock := fb.newBlock(), fb.newBlock()
	fb.emit(Operation{Kind: OpCondBranch, Operands: []ValueID{isErr.id}, Successors: []BranchTarget{{Block: errorBlock.ID}, {Block: successBlock.ID}}, Location: location(expr.Token)})

	fb.current = errorBlock
	errorValue := fb.result(Operation{Kind: OpResultUnwrapErr, Operands: []ValueID{resultValue.id}, Location: location(expr.Token)}, resultType.Error)
	enclosingResult, err := fb.owner.internType(resolved.EnclosingResultType)
	if err != nil {
		return builtValue{}, err
	}
	propagated := fb.result(Operation{Kind: OpResultErr, Operands: []ValueID{errorValue.id}, Location: location(expr.Token)}, enclosingResult)
	fb.emit(Operation{Kind: OpReturn, Operands: []ValueID{propagated.id}, Location: location(expr.Token)})

	fb.current = successBlock
	if fb.owner.isVoidType(resultType.Success) {
		return builtValue{typ: resultType.Success}, nil
	}
	return fb.result(Operation{Kind: OpResultUnwrapOk, Operands: []ValueID{resultValue.id}, Location: location(expr.Token)}, resultType.Success), nil
}

func (fb *functionBuilder) buildResolvedOperator(expr ast.Expression) (builtValue, error) {
	return fb.buildResolvedOperatorWithFailure(expr, nil, nil)
}

func (fb *functionBuilder) buildResolvedOperatorWithFailure(expr ast.Expression, arithmeticTry *sema.ResolvedTry, localTry *ast.TryExpression) (builtValue, error) {
	resolved, ok := fb.owner.analyzer.ResolvedOperatorOf(expr)
	if !ok {
		return builtValue{}, fb.unsupported("unresolved or unsupported operator", expressionToken(expr))
	}
	leftType, err := fb.owner.internType(resolved.LeftType)
	if err != nil {
		return builtValue{}, err
	}
	resultType, err := fb.owner.internType(resolved.ResultType)
	if err != nil {
		return builtValue{}, err
	}
	var leftExpr, rightExpr ast.Expression
	switch expression := expr.(type) {
	case *ast.PrefixExpression:
		leftExpr = expression.Right
	case *ast.InfixExpression:
		leftExpr, rightExpr = expression.Left, expression.Right
	default:
		return builtValue{}, fb.unsupported("operator expression", expressionToken(expr))
	}
	if _, ok := leftExpr.(*ast.TryExpression); ok {
		return builtValue{}, fb.unsupported("try arithmetic", expressionToken(leftExpr))
	}
	if _, ok := rightExpr.(*ast.TryExpression); ok {
		return builtValue{}, fb.unsupported("try arithmetic", expressionToken(rightExpr))
	}
	left, err := fb.buildExpr(leftExpr, leftType)
	if err != nil {
		return builtValue{}, err
	}
	operands := []ValueID{left.id}
	if rightExpr != nil {
		rightType, err := fb.owner.internType(*resolved.RightType)
		if err != nil {
			return builtValue{}, err
		}
		right, err := fb.buildExpr(rightExpr, rightType)
		if err != nil {
			return builtValue{}, err
		}
		operands = append(operands, right.id)
	}
	loc := locationFromExpression(expr)
	op := Operation{Operands: operands, Location: loc, Operator: operatorSpelling(expr)}
	switch resolved.Kind {
	case sema.ResolvedIntegerUnaryPlus:
		op.Kind = OpIntUnaryPlus
	case sema.ResolvedIntegerNegateChecked:
		op.Kind = OpIntNegChecked
	case sema.ResolvedIntegerBitNot:
		op.Kind = OpIntBitNot
	case sema.ResolvedIntegerAddChecked:
		op.Kind, op.IntegerBinary = OpIntBinaryChecked, IntegerCheckedAdd
	case sema.ResolvedIntegerSubtractChecked:
		op.Kind, op.IntegerBinary = OpIntBinaryChecked, IntegerCheckedSubtract
	case sema.ResolvedIntegerMultiplyChecked:
		op.Kind, op.IntegerBinary = OpIntBinaryChecked, IntegerCheckedMultiply
	case sema.ResolvedIntegerDivideChecked:
		op.Kind, op.IntegerBinary = OpIntBinaryChecked, IntegerCheckedDivide
	case sema.ResolvedIntegerRemainderChecked:
		op.Kind, op.IntegerBinary = OpIntBinaryChecked, IntegerCheckedRemainder
	case sema.ResolvedIntegerBitAnd:
		op.Kind, op.IntegerBitwise = OpIntBitwise, IntegerBitwiseAnd
	case sema.ResolvedIntegerBitOr:
		op.Kind, op.IntegerBitwise = OpIntBitwise, IntegerBitwiseOr
	case sema.ResolvedIntegerBitXor:
		op.Kind, op.IntegerBitwise = OpIntBitwise, IntegerBitwiseXor
	case sema.ResolvedIntegerShiftLeftUnsignedChecked:
		op.Kind, op.IntegerShift = OpIntShiftChecked, IntegerShiftLeftUnsigned
	case sema.ResolvedIntegerShiftLeftSignedChecked:
		op.Kind, op.IntegerShift = OpIntShiftChecked, IntegerShiftLeftSigned
	case sema.ResolvedIntegerShiftRightUnsignedChecked:
		op.Kind, op.IntegerShift = OpIntShiftChecked, IntegerShiftRightUnsigned
	case sema.ResolvedIntegerShiftRightSignedChecked:
		op.Kind, op.IntegerShift = OpIntShiftChecked, IntegerShiftRightSigned
	case sema.ResolvedIntegerCompareEQ:
		op.Kind, op.IntegerCompare = OpIntCompare, IntegerCompareEQ
	case sema.ResolvedIntegerCompareNE:
		op.Kind, op.IntegerCompare = OpIntCompare, IntegerCompareNE
	case sema.ResolvedIntegerCompareLT:
		op.Kind, op.IntegerCompare = OpIntCompare, IntegerCompareLT
	case sema.ResolvedIntegerCompareLE:
		op.Kind, op.IntegerCompare = OpIntCompare, IntegerCompareLE
	case sema.ResolvedIntegerCompareGT:
		op.Kind, op.IntegerCompare = OpIntCompare, IntegerCompareGT
	case sema.ResolvedIntegerCompareGE:
		op.Kind, op.IntegerCompare = OpIntCompare, IntegerCompareGE
	case sema.ResolvedEnumCompareEQ:
		op.Kind, op.IntegerCompare = OpEnumCompare, IntegerCompareEQ
	case sema.ResolvedEnumCompareNE:
		op.Kind, op.IntegerCompare = OpEnumCompare, IntegerCompareNE
	default:
		return builtValue{}, fb.unsupported("operator "+string(resolved.Kind), expressionToken(expr))
	}
	if resolved.RuntimeCheck {
		return fb.emitCheckedOperator(op, resultType, arithmeticTry, localTry)
	}
	return fb.result(op, resultType), nil
}

func (fb *functionBuilder) emitCheckedOperator(op Operation, resultType TypeID, arithmeticTry *sema.ResolvedTry, localTry *ast.TryExpression) (builtValue, error) {
	boolType, err := fb.owner.internType(sema.Type{Name: "bool", Kind: sema.BoolType})
	if err != nil {
		return builtValue{}, err
	}
	result := fb.newValue(resultType, OwnershipImmediate, op.Location)
	failed := fb.newValue(boolType, OwnershipImmediate, op.Location)
	reasonType := fb.owner.module.Types.Intern(Type{Kind: TypeArithmeticFailureReason, Name: "ArithmeticFailureReason"})
	reason := fb.newValue(reasonType, OwnershipImmediate, op.Location)
	op.Results = []Value{result, failed, reason}
	fb.emit(op)
	failure := fb.newBlock()
	success := fb.newBlock()
	failureReason := fb.newValue(reasonType, OwnershipImmediate, op.Location)
	failure.Parameters = []Value{failureReason}
	fb.emit(Operation{Kind: OpCondBranch, Operands: []ValueID{failed.ID}, Successors: []BranchTarget{{Block: failure.ID, Arguments: []ValueID{reason.ID}}, {Block: success.ID}}, Location: op.Location})
	fb.current = failure
	var localError builtValue
	if localTry != nil {
		plan, ok := fb.owner.analyzer.ResolvedTryPlanOf(localTry)
		if !ok || !plan.Exhaustive {
			return builtValue{}, fb.unsupported("unresolved or non-exhaustive arithmetic handler plan", localTry.Token)
		}
		errorType, err := fb.owner.internType(plan.ErrorType)
		if err != nil {
			return builtValue{}, err
		}
		localError = fb.result(Operation{Kind: OpArithmeticErrorFromReason, Operands: []ValueID{failureReason.ID}, Location: op.Location}, errorType)
	} else if arithmeticTry == nil {
		fb.emit(Operation{Kind: OpArithmeticFailure, Operands: []ValueID{failureReason.ID}, FailureCategory: failureCategory(op), Operator: op.Operator, Location: op.Location})
	} else {
		errorType, err := fb.owner.internType(arithmeticTry.ErrorType)
		if err != nil {
			return builtValue{}, err
		}
		resultContainer, err := fb.owner.internType(arithmeticTry.EnclosingResultType)
		if err != nil {
			return builtValue{}, err
		}
		errorValue := fb.result(Operation{Kind: OpArithmeticErrorFromReason, Operands: []ValueID{failureReason.ID}, Location: op.Location}, errorType)
		errResult := fb.result(Operation{Kind: OpResultErr, Operands: []ValueID{errorValue.id}, Location: op.Location}, resultContainer)
		fb.emit(Operation{Kind: OpReturn, Operands: []ValueID{errResult.id}, Location: op.Location})
	}
	fb.current = success
	successValue := builtValue{id: result.ID, typ: result.Type}
	if localTry != nil {
		plan, _ := fb.owner.analyzer.ResolvedTryPlanOf(localTry)
		return fb.buildLocalTryHandlers(localTry, plan, success, successValue, failure, localError)
	}
	return successValue, nil
}

func failureCategory(op Operation) ArithmeticFailureCategory {
	if op.Kind == OpIntShiftChecked {
		return ArithmeticFailureShift
	}
	if op.Kind == OpIntBinaryChecked {
		switch op.IntegerBinary {
		case IntegerCheckedDivide:
			return ArithmeticFailureDivision
		case IntegerCheckedRemainder:
			return ArithmeticFailureRemainder
		}
	}
	return ArithmeticFailureOverflow
}

func operatorSpelling(expr ast.Expression) string {
	switch expression := expr.(type) {
	case *ast.PrefixExpression:
		return expression.Operator
	case *ast.InfixExpression:
		return expression.Operator
	default:
		return ""
	}
}

func (fb *functionBuilder) buildCall(call *ast.CallExpression) (builtValue, error) {
	resolved, ok := fb.owner.analyzer.ResolvedCallTarget(call)
	if !ok {
		return builtValue{}, fb.unsupported("unresolved function call", call.Token)
	}
	if resolved.Kind == sema.ResolvedStaticMethodCall {
		return builtValue{}, fb.unsupported("method call", call.Token)
	}
	args := make([]ValueID, 0, len(call.Arguments))
	actions := make([]ArgumentAction, 0, len(call.Arguments))
	for index, arg := range call.Arguments {
		if _, isReference := arg.(*ast.RefExpression); isReference {
			return builtValue{}, fb.unsupported("reference argument call", expressionToken(arg))
		}
		expected := TypeID(0)
		if index < len(resolved.Function.Parameters) {
			expected, _ = fb.owner.internType(resolved.Function.Parameters[index].Type)
		}
		v, err := fb.buildExpr(arg, expected)
		if err != nil {
			return builtValue{}, err
		}
		if !fb.storageAllowed(v.typ) {
			return builtValue{}, fb.unsupported("non-trivial call argument", expressionToken(arg))
		}
		args = append(args, v.id)
		actions = append(actions, ArgumentCopyTrivial)
	}
	ret, err := fb.owner.internType(resolved.Function.ReturnType)
	if err != nil {
		return builtValue{}, err
	}
	kind := OpDirectCall
	if resolved.Kind == sema.ResolvedForeignCall {
		kind = OpForeignCall
	}
	op := Operation{Kind: kind, Callee: functionID(resolved.Function, fb.owner.module.Types), Operands: args, ArgumentActions: actions, Location: location(call.Token)}
	if resolved.Function.ReturnType.Kind == sema.VoidType {
		fb.emit(op)
		return builtValue{typ: ret}, nil
	}
	return fb.result(op, ret), nil
}

func (fb *functionBuilder) result(op Operation, typ TypeID) builtValue {
	v := fb.newValue(typ, OwnershipImmediate, op.Location)
	op.Results = []Value{v}
	fb.emit(op)
	return builtValue{id: v.ID, typ: typ}
}
func (fb *functionBuilder) emit(op Operation) {
	fb.current.Operations = append(fb.current.Operations, op)
}
func (fb *functionBuilder) newValue(typ TypeID, own OwnershipClass, loc Location) Value {
	v := Value{ID: fb.nextValue, Type: typ, Ownership: own, Location: loc}
	fb.nextValue++
	return v
}
func (fb *functionBuilder) newBlock() *Block {
	b := &Block{ID: fb.nextBlock}
	fb.nextBlock++
	fb.fn.Blocks = append(fb.fn.Blocks, b)
	return b
}
func (fb *functionBuilder) unsupported(feature string, token lexer.Token) error {
	return &UnsupportedFeatureError{Feature: feature, Package: fb.owner.maxPackage, Location: location(token)}
}
func (fb *functionBuilder) storageAllowed(id TypeID) bool {
	t, ok := fb.owner.module.Types.Lookup(id)
	if !ok {
		return false
	}
	if t.Kind == TypeNamed {
		return fb.storageAllowed(t.Base)
	}
	if t.Kind == TypeArray {
		// rules/mlir/packages/sec-mlir-dialect_package14.md sections 44-45:
		// only recursively copy-trivial, trivially destructible fixed-array
		// values enter P5 high-level storage. No physical layout is selected.
		return fb.owner.maxPackage >= 14 && fb.storageAllowed(t.Element)
	}
	if t.Kind == TypeUnion {
		definition, ok := fb.owner.unionDefinition(id)
		return ok && definition.CopyClassification == string(sema.CopyTrivial) && definition.TriviallyDestructible
	}
	if t.Kind == TypeStruct {
		definition, ok := fb.owner.structDefinition(id)
		return ok && definition.CopyClassification == string(sema.CopyTrivial) && definition.TriviallyDestructible
	}
	return t.Kind != TypeString && t.Kind != TypeVoid && t.Kind != TypeNever
}

// ownershipForParameter classifies parameter ownership at the package boundary.
// Package 14 may temporarily admit an owned non-trivial fixed-array parameter
// so its exact operation boundary can be diagnosed; buildFunction still rejects
// the function if the deferred parameter cleanup obligation remains.
//
// Rules:
//   - rules/mlir/packages/sec-mlir-dialect_package13.md — sections 83-86
//   - rules/mlir/packages/sec-mlir-dialect_package14.md — sections 88-89, 103
func ownershipForParameter(p sema.FunctionParameter, maxPackage uint8) (OwnershipClass, error) {
	if p.MutableRef {
		return OwnershipMutableReference, nil
	}
	if p.Ref {
		return OwnershipSharedReference, nil
	}
	switch p.Type.Kind {
	case sema.RawPtrType:
		return OwnershipRawPointer, nil
	case sema.BoolType, sema.IntType, sema.UintType, sema.FloatType, sema.DecimalType, sema.CharType, sema.RuneType, sema.EnumType:
		return OwnershipImmediate, nil
	case sema.ArrayType:
		if maxPackage >= 14 && sema.CopyClassificationOf(p.Type) == sema.CopyTrivial && sema.TriviallyDestructible(p.Type) {
			return OwnershipImmediate, nil
		}
		if maxPackage >= 14 {
			return OwnershipOwned, nil
		}
	case sema.UnionType, sema.ResultType, sema.StructType:
		if sema.CopyClassificationOf(p.Type) == sema.CopyTrivial && sema.TriviallyDestructible(p.Type) {
			return OwnershipImmediate, nil
		}
	}
	return "", &UnsupportedFeatureError{Feature: "non-scalar parameter type " + p.Type.Name, Package: maxPackage}
}
func functionID(f sema.Function, types *TypeTable) FunctionID {
	parts := make([]string, len(f.Parameters))
	for i, p := range f.Parameters {
		parts[i] = canonicalSemaType(p.Type)
	}
	return FunctionID(f.Module + "::" + f.Name + "(" + strings.Join(parts, ",") + ")")
}
func canonicalSemaType(t sema.Type) string {
	if t.Module != "" && (t.Named || t.Declared) {
		return t.Module + "::" + t.Name
	}
	return t.Name
}
func parseDecimal(lexeme string) (DecimalConstant, error) {
	digits, _ := ast.SplitNumericLiteralSuffix(lexeme)
	digits = ast.NormalizeNumericLiteralLexeme(digits)
	negative := strings.HasPrefix(digits, "-")
	digits = strings.TrimPrefix(digits, "-")
	parts := strings.Split(digits, ".")
	if len(parts) > 2 {
		return DecimalConstant{}, fmt.Errorf("invalid decimal %q", lexeme)
	}
	scale := uint32(0)
	joined := parts[0]
	if len(parts) == 2 {
		scale = uint32(len(parts[1]))
		joined += parts[1]
	}
	if joined == "" {
		joined = "0"
	}
	n, ok := new(big.Int).SetString(joined, 10)
	if !ok {
		return DecimalConstant{}, fmt.Errorf("invalid decimal %q", lexeme)
	}
	if negative {
		n.Neg(n)
	}
	return DecimalConstant{Coefficient: n, Scale: scale, Lexeme: lexeme}, nil
}
func requestedModule(program *ast.Program, files []string) string {
	wanted := map[string]bool{}
	for _, f := range files {
		wanted[f] = true
	}
	result := ""
	for _, s := range program.Statements {
		if m, ok := s.(*ast.ModuleStatement); ok && (len(wanted) == 0 || wanted[m.Token.File]) {
			result = m.Path
		}
	}
	return result
}
func uniqueSorted(values []string) []string {
	seen := map[string]bool{}
	out := []string{}
	for _, v := range values {
		if v != "" && !seen[v] {
			seen[v] = true
			out = append(out, v)
		}
	}
	sort.Strings(out)
	return out
}
func location(t lexer.Token) Location                  { return Location{File: t.File, Line: t.Line, Column: t.Column} }
func locationFromExpression(e ast.Expression) Location { return location(expressionToken(e)) }
func expressionToken(e ast.Expression) lexer.Token {
	switch x := e.(type) {
	case *ast.Identifier:
		return x.Token
	case *ast.IntegerLiteral:
		return x.Token
	case *ast.FloatLiteral:
		return x.Token
	case *ast.BooleanLiteral:
		return x.Token
	case *ast.StringLiteral:
		return x.Token
	case *ast.CallExpression:
		return x.Token
	case *ast.ConversionExpression:
		return x.Token
	case *ast.MemberExpression:
		return x.Token
	case *ast.StructLiteral:
		return x.Token
	case *ast.RefExpression:
		return x.Token
	case *ast.PrefixExpression:
		return x.Token
	case *ast.InfixExpression:
		return x.Token
	}
	return lexer.Token{}
}
func statementToken(s ast.Statement) lexer.Token {
	switch x := s.(type) {
	case *ast.LetStatement:
		return x.Token
	case *ast.AssignmentStatement:
		return x.Token
	case *ast.ReturnStatement:
		return x.Token
	case *ast.AssertStatement:
		return x.Token
	case *ast.IfStatement:
		return x.Token
	case *ast.ExpressionStatement:
		return x.Token
	case *ast.UnsafeStatement:
		return x.Token
	}
	return lexer.Token{}
}
func statementLocation(s ast.Statement) Location { return location(statementToken(s)) }
