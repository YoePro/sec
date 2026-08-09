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
		options.MaxPackage = 7
	}
	identity := options.RequestedModule
	if identity == "" {
		identity = requestedModule(program, options.SourceFiles)
	}
	module := &Module{Version: Version, Identity: identity, Types: NewTypeTable(), SourceFiles: uniqueSorted(options.SourceFiles)}
	b := &builder{module: module, analyzer: analyzer, maxPackage: options.MaxPackage}
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
	module     *Module
	analyzer   *sema.Analyzer
	maxPackage uint8
}
type functionBuilder struct {
	owner       *builder
	fn          *Function
	current     *Block
	nextValue   ValueID
	nextBlock   BlockID
	nextStorage StorageID
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
	fb := &functionBuilder{owner: b, fn: fn, bindings: map[sema.BindingID]binding{}, nextStorage: 1}
	for i, parameter := range resolved.Parameters {
		typeID, err := b.internType(parameter.Type)
		if err != nil {
			return err
		}
		ownership, err := ownershipForParameter(parameter)
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
	return nil
}

func (b *builder) internType(t sema.Type) (TypeID, error) {
	kind, signed, width, target, ok := builtinType(t)
	if !ok {
		return 0, &UnsupportedFeatureError{Feature: "type " + t.Name}
	}
	base := Type{Kind: kind, Name: t.Name, Signed: signed, BitWidth: width, TargetSize: target}
	if t.Named || t.Declared {
		if len(t.Contracts) > 0 || t.Unit != "" || !t.Dimension.IsZero() {
			return 0, &UnsupportedFeatureError{Feature: "named type contracts or units"}
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
	if stmt.SynthesizedDefault {
		return fb.unsupported("mutable local declaration without initializer", stmt.Token)
	}
	if stmt.Value == nil {
		return fb.unsupported("mutable local declaration without initializer", stmt.Token)
	}
	value, err := fb.buildExpr(stmt.Value, 0)
	if err != nil {
		return err
	}
	fact, ok := fb.owner.analyzer.ResolvedBindingOf(stmt.Name)
	if !ok {
		return fmt.Errorf("missing resolved binding for %s", stmt.Name.Value)
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

func (fb *functionBuilder) buildExpr(expr ast.Expression, expected TypeID) (builtValue, error) {
	resolved, ok := fb.owner.analyzer.ResolvedTypeOf(expr)
	if !ok {
		return builtValue{}, fmt.Errorf("missing resolved type for %T", expr)
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
			return builtValue{}, fmt.Errorf("missing resolved binding for %s", e.Value)
		}
		bind, ok := fb.bindings[fact.ID]
		if !ok {
			return builtValue{}, fmt.Errorf("binding %d has no IR value", fact.ID)
		}
		if bind.storage != 0 {
			return fb.result(Operation{Kind: OpStorageLoad, Storage: bind.storage, Location: loc}, bind.typ), nil
		}
		return builtValue{id: bind.value, typ: bind.typ}, nil
	case *ast.CallExpression:
		if fb.owner.maxPackage < 3 {
			return builtValue{}, fb.unsupported("function call", e.Token)
		}
		return fb.buildCall(e)
	case *ast.PrefixExpression, *ast.InfixExpression:
		if fb.owner.maxPackage < 7 {
			return builtValue{}, fb.unsupported("integer operator", expressionToken(expr))
		}
		return fb.buildResolvedOperator(expr)
	default:
		return builtValue{}, fb.unsupported(fmt.Sprintf("expression %T", expr), expressionToken(expr))
	}
}

func (fb *functionBuilder) buildResolvedOperator(expr ast.Expression) (builtValue, error) {
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
	default:
		return builtValue{}, fb.unsupported("operator "+string(resolved.Kind), expressionToken(expr))
	}
	if resolved.RuntimeCheck {
		return fb.emitCheckedOperator(op, resultType)
	}
	return fb.result(op, resultType), nil
}

func (fb *functionBuilder) emitCheckedOperator(op Operation, resultType TypeID) (builtValue, error) {
	boolType, err := fb.owner.internType(sema.Type{Name: "bool", Kind: sema.BoolType})
	if err != nil {
		return builtValue{}, err
	}
	result := fb.newValue(resultType, OwnershipImmediate, op.Location)
	failed := fb.newValue(boolType, OwnershipImmediate, op.Location)
	op.Results = []Value{result, failed}
	fb.emit(op)
	failure := fb.newBlock()
	success := fb.newBlock()
	fb.emit(Operation{Kind: OpCondBranch, Operands: []ValueID{failed.ID}, Successors: []BranchTarget{{Block: failure.ID}, {Block: success.ID}}, Location: op.Location})
	fb.current = failure
	fb.emit(Operation{Kind: OpArithmeticFailure, FailureCategory: failureCategory(op), Operator: op.Operator, Location: op.Location})
	fb.current = success
	return builtValue{id: result.ID, typ: result.Type}, nil
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
	return t.Kind != TypeString && t.Kind != TypeVoid && t.Kind != TypeNever
}

func ownershipForParameter(p sema.FunctionParameter) (OwnershipClass, error) {
	if p.MutableRef {
		return OwnershipMutableReference, nil
	}
	if p.Ref {
		return OwnershipSharedReference, nil
	}
	switch p.Type.Kind {
	case sema.RawPtrType:
		return OwnershipRawPointer, nil
	case sema.BoolType, sema.IntType, sema.UintType, sema.FloatType, sema.DecimalType, sema.CharType, sema.RuneType:
		return OwnershipImmediate, nil
	}
	return "", &UnsupportedFeatureError{Feature: "non-scalar parameter type " + p.Type.Name}
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
