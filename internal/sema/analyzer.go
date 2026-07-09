package sema

import (
	"fmt"
	"math/big"
	"strings"

	"sec/internal/ast"
	"sec/internal/lexer"
	"sec/internal/parser"
)

type Analyzer struct {
	types     map[string]Type
	symbols   map[string]Symbol
	constInts map[string]*big.Int
	errors    []Error
}

func NewAnalyzer() *Analyzer {
	return &Analyzer{
		types: builtinTypes(),
	}
}

func (a *Analyzer) Analyze(program *ast.Program) []Error {
	a.errors = nil
	a.symbols = map[string]Symbol{}
	a.constInts = map[string]*big.Int{}
	a.registerTypeDeclarations(program)
	a.analyzeTypeDeclarations(program)

	for _, stmt := range program.Statements {
		if _, ok := stmt.(*ast.TypeDeclStatement); ok {
			continue
		}
		a.analyzeStatement(stmt)
	}

	return a.errors
}

func (a *Analyzer) registerTypeDeclarations(program *ast.Program) {
	for _, stmt := range program.Statements {
		typeDecl, ok := stmt.(*ast.TypeDeclStatement)
		if !ok || typeDecl.Name == nil {
			continue
		}

		a.types[typeDecl.Name.Value] = Type{Name: typeDecl.Name.Value, Kind: InvalidType}
	}
}

func (a *Analyzer) analyzeTypeDeclarations(program *ast.Program) {
	for _, stmt := range program.Statements {
		typeDecl, ok := stmt.(*ast.TypeDeclStatement)
		if !ok {
			continue
		}

		a.analyzeTypeDeclaration(typeDecl)
	}
}

func (a *Analyzer) analyzeStatement(stmt ast.Statement) {
	switch stmt := stmt.(type) {
	case *ast.TypeDeclStatement:
		a.analyzeTypeDeclaration(stmt)
	case *ast.LetStatement:
		a.analyzeLetStatement(stmt)
	case *ast.LetGroupStatement:
		for _, let := range stmt.Lets {
			a.analyzeLetStatement(let)
		}
	case *ast.AssignmentStatement:
		a.analyzeAssignmentStatement(stmt)
	case *ast.ImplStatement:
		a.analyzeImplStatement(stmt)
	case *ast.StructStatement:
		for _, field := range stmt.Fields {
			a.resolveType(field.Type)
		}
	}
}

func (a *Analyzer) analyzeTypeDeclaration(stmt *ast.TypeDeclStatement) {
	if stmt.StructType != nil {
		a.types[stmt.Name.Value] = a.typeFromStructDeclaration(stmt)
		return
	}

	var baseType Type
	var baseTypeOK bool
	if stmt.BaseType != nil {
		baseType, baseTypeOK = a.resolveType(stmt.BaseType)
	}

	if stmt.AssignedType != nil {
		baseType, baseTypeOK = a.resolveType(stmt.AssignedType)
	}

	if contract, ok := stmt.Contract.(*ast.RangeContract); ok && baseTypeOK {
		if contract.Min != nil {
			a.checkIntegerLiteralRange(baseType, contract.Min)
		}
		if contract.Max != nil {
			a.checkIntegerLiteralRange(baseType, contract.Max)
		}
	}

	if baseTypeOK {
		a.types[stmt.Name.Value] = a.typeFromDeclaration(stmt, baseType)
	}
}

func (a *Analyzer) typeFromStructDeclaration(stmt *ast.TypeDeclStatement) Type {
	typ := Type{
		Name:       stmt.Name.Value,
		Kind:       StructType,
		Named:      true,
		Declared:   true,
		Underlying: "struct",
	}

	seen := map[string]lexer.Token{}
	for _, field := range stmt.StructType.Fields {
		if previous, exists := seen[field.Name.Value]; exists {
			_ = previous
			a.addErrorAtToken(field.Name.Token, "duplicate field %q in struct %s", field.Name.Value, stmt.Name.Value)
			continue
		}
		seen[field.Name.Value] = field.Name.Token

		fieldType, ok := a.resolveType(field.Type)
		if !ok {
			continue
		}

		typ.Fields = append(typ.Fields, StructField{
			Name:  field.Name.Value,
			Type:  fieldType,
			Token: field.Name.Token,
			Tags:  semaStructTags(field.Tags),
		})
	}

	return typ
}

func semaStructTags(tags []ast.StructTag) []StructTag {
	out := make([]StructTag, 0, len(tags))
	for _, tag := range tags {
		out = append(out, StructTag{Key: tag.Key, Value: tag.Value})
	}
	return out
}

func (a *Analyzer) analyzeImplStatement(stmt *ast.ImplStatement) {
	target, ok := a.types[stmt.Target.Name]
	if !ok {
		a.addErrorAtToken(stmt.Target.Token, "unknown impl target %s", stmt.Target.Name)
		return
	}

	if !target.Named {
		a.addErrorAtToken(stmt.Target.Token, "impl target %s is not a named type", stmt.Target.Name)
		return
	}

	properties := map[string]lexer.Token{}
	targetChanged := false
	for _, member := range stmt.Members {
		property, ok := member.(*ast.PropertyDeclaration)
		if !ok {
			continue
		}

		if previous, exists := properties[property.Name.Value]; exists {
			_ = previous
			a.addErrorAtToken(property.Name.Token, "duplicate property %q in impl %s", property.Name.Value, stmt.Target.Name)
			continue
		}
		properties[property.Name.Value] = property.Name.Token

		propertyType, ok := a.resolveType(property.Type)
		if !ok {
			continue
		}

		target.Properties = append(target.Properties, Property{
			Name:     property.Name.Value,
			Type:     propertyType,
			Token:    property.Name.Token,
			Fallible: property.Setter != nil && property.Setter.Fallible,
		})
		targetChanged = true

		a.analyzePropertyBody(target, property, propertyType)
	}

	if targetChanged {
		a.types[target.Name] = target
	}
}

func (a *Analyzer) analyzePropertyBody(target Type, property *ast.PropertyDeclaration, propertyType Type) {
	if property.Getter != nil {
		a.analyzeGetterBody(target, property, propertyType)
	}
	if property.Setter != nil {
		a.analyzeSetterBody(target, property, propertyType)
	}
}

func (a *Analyzer) analyzeGetterBody(target Type, property *ast.PropertyDeclaration, propertyType Type) {
	foundReturn := false
	for i, token := range property.Getter.Tokens {
		if token.Type != lexer.RETURN || i+1 >= len(property.Getter.Tokens) {
			continue
		}
		foundReturn = true

		returnToken := property.Getter.Tokens[i+1]
		returnExpr := parseBodyExpression(property.Getter.Tokens[i+1:])
		if returnExpr == nil {
			continue
		}

		returnType, ok := a.inferPropertyBodyExpression(target, nil, propertyType, returnExpr)
		if ok && returnType.Kind != InvalidType && !canInitialize(propertyType, returnType, returnExpr) {
			a.addErrorAtToken(returnToken, "getter %s must return %s, got %s", property.Name.Value, typeDisplayName(propertyType), typeDisplayName(returnType))
		}
		return
	}

	if !foundReturn {
		a.addErrorAtToken(property.Name.Token, "getter %s must return %s", property.Name.Value, typeDisplayName(propertyType))
	}
}

func (a *Analyzer) analyzeSetterBody(target Type, property *ast.PropertyDeclaration, propertyType Type) {
	if !property.Setter.Fallible && blockReturnsErr(property.Setter.Body) {
		a.addErrorAtToken(property.Setter.Token, "non-fallible setter %s cannot return Err", property.Name.Value)
	}

	for i, token := range property.Setter.Body.Tokens {
		if token.Type != lexer.IDENT || i+2 >= len(property.Setter.Body.Tokens) {
			continue
		}
		if property.Setter.Body.Tokens[i+1].Type != lexer.ASSIGN {
			continue
		}

		targetType, ok := a.resolveBodyValueType(target, property.Setter, propertyType, token)
		if !ok {
			continue
		}

		valueType, ok := a.resolveBodyValueType(target, property.Setter, propertyType, property.Setter.Body.Tokens[i+2])
		if ok && !sameConcreteType(targetType, valueType) {
			a.addErrorAtToken(property.Setter.Body.Tokens[i+2], "cannot assign %s to %s in setter %s", typeDisplayName(valueType), typeDisplayName(targetType), property.Name.Value)
		}
	}
}

func blockReturnsErr(block *ast.BlockStatement) bool {
	for i, token := range block.Tokens {
		if token.Type == lexer.RETURN && i+1 < len(block.Tokens) && block.Tokens[i+1].Type == lexer.IDENT && block.Tokens[i+1].Lexeme == "Err" {
			return true
		}
	}
	return false
}

func parseBodyExpression(tokens []lexer.Token) ast.Expression {
	exprTokens := collectBodyExpressionTokens(tokens)
	if len(exprTokens) == 0 {
		return nil
	}

	source := "let value := " + tokensSource(exprTokens)
	l := lexer.New(source)
	p := parser.New(l)
	program := p.ParseProgram()
	if len(p.Errors()) > 0 || len(program.Statements) != 1 {
		return nil
	}

	stmt, ok := program.Statements[0].(*ast.LetStatement)
	if !ok {
		return nil
	}

	return stmt.Value
}

func collectBodyExpressionTokens(tokens []lexer.Token) []lexer.Token {
	if len(tokens) == 0 {
		return nil
	}

	line := tokens[0].Line
	depth := 0
	out := []lexer.Token{}
	for _, token := range tokens {
		if depth == 0 && token.Line != line {
			break
		}

		switch token.Type {
		case lexer.LPAREN, lexer.LBRACKET, lexer.LBRACE:
			depth++
		case lexer.RPAREN, lexer.RBRACKET, lexer.RBRACE:
			if depth == 0 {
				return out
			}
			depth--
		case lexer.SEMICOLON, lexer.RETURN:
			if depth == 0 {
				return out
			}
		}

		out = append(out, token)
	}

	return out
}

func tokensSource(tokens []lexer.Token) string {
	parts := make([]string, 0, len(tokens))
	for _, token := range tokens {
		parts = append(parts, token.Lexeme)
	}
	return strings.Join(parts, " ")
}

func (a *Analyzer) inferPropertyBodyExpression(target Type, setter *ast.PropertySetter, setterType Type, expr ast.Expression) (Type, bool) {
	switch expr := expr.(type) {
	case *ast.Identifier:
		if setter != nil && setter.Parameter != nil && expr.Value == setter.Parameter.Value {
			return setterType, true
		}
		if fieldType, ok := lookupStructField(target, expr.Value); ok {
			return fieldType, true
		}
		if symbol, ok := a.symbols[expr.Value]; ok {
			return symbol.Type, true
		}
		return Type{Kind: InvalidType}, false
	case *ast.ConversionExpression:
		targetType, ok := a.resolveType(expr.Type)
		if !ok {
			return Type{Kind: InvalidType}, false
		}
		valueType, ok := a.inferPropertyBodyExpression(target, setter, setterType, expr.Value)
		if !ok || valueType.Kind == InvalidType {
			return Type{Kind: InvalidType}, ok
		}
		if !canExplicitConvert(targetType, valueType) {
			a.addErrorAtToken(expr.Token, "cannot convert %s to %s", typeDisplayName(valueType), typeDisplayName(targetType))
			return Type{Kind: InvalidType}, false
		}
		return targetType, true
	case *ast.PrefixExpression:
		rightType, ok := a.inferPropertyBodyExpression(target, setter, setterType, expr.Right)
		if !ok {
			return Type{Kind: InvalidType}, false
		}
		switch expr.Operator {
		case "-":
			if rightType.Kind == IntType || rightType.Kind == FloatType || rightType.Kind == DecimalType {
				return rightType, true
			}
		case "!":
			return Type{Name: "bool", Kind: BoolType}, true
		}
		return Type{Kind: InvalidType}, false
	case *ast.InfixExpression:
		return a.inferPropertyBodyInfixExpression(target, setter, setterType, expr)
	case *ast.MemberExpression:
		objectType, ok := a.inferPropertyBodyExpression(target, setter, setterType, expr.Object)
		if !ok || objectType.Kind == InvalidType {
			return Type{Kind: InvalidType}, ok
		}
		if fieldType, ok := lookupStructField(objectType, expr.Property.Value); ok {
			return fieldType, true
		}
		if property, ok := lookupProperty(objectType, expr.Property.Value); ok {
			return property.Type, true
		}
		a.addErrorAtToken(expr.Property.Token, "unknown member %s on %s", expr.Property.Value, typeDisplayName(objectType))
		return Type{Kind: InvalidType}, false
	default:
		typ, _ := a.inferExpression(expr)
		return typ, typ.Kind != InvalidType
	}
}

func (a *Analyzer) inferPropertyBodyInfixExpression(target Type, setter *ast.PropertySetter, setterType Type, expr *ast.InfixExpression) (Type, bool) {
	leftType, leftOK := a.inferPropertyBodyExpression(target, setter, setterType, expr.Left)
	rightType, rightOK := a.inferPropertyBodyExpression(target, setter, setterType, expr.Right)
	if !leftOK || !rightOK || leftType.Kind == InvalidType || rightType.Kind == InvalidType {
		return Type{Kind: InvalidType}, false
	}

	if leftType.Kind == DecimalType || rightType.Kind == DecimalType {
		typ, _ := a.inferDecimalInfixExpression(expr, leftType, rightType)
		return typ, typ.Kind != InvalidType
	}

	if leftType.Kind == rightType.Kind {
		return leftType, true
	}

	if leftType.Kind == UintType && rightType.Kind == IntType {
		return leftType, true
	}

	if leftType.Kind == IntType && rightType.Kind == UintType {
		return rightType, true
	}

	return Type{Kind: InvalidType}, false
}

func (a *Analyzer) resolveBodyValueType(target Type, setter *ast.PropertySetter, setterType Type, token lexer.Token) (Type, bool) {
	if setter != nil && setter.Parameter != nil && token.Type == lexer.IDENT && token.Lexeme == setter.Parameter.Value {
		return setterType, true
	}

	if token.Type == lexer.IDENT {
		for _, field := range target.Fields {
			if field.Name == token.Lexeme {
				return field.Type, true
			}
		}
	}

	return Type{Kind: InvalidType}, false
}

func (a *Analyzer) analyzeLetStatement(stmt *ast.LetStatement) {
	var declaredType Type
	var ok bool

	if stmt.Type != nil {
		declaredType, ok = a.resolveType(stmt.Type)
	} else if stmt.Value != nil {
		declaredType, _ = a.inferExpression(stmt.Value)
		ok = declaredType.Kind != InvalidType
	}

	defined := false
	if ok {
		defined = a.defineSymbol(stmt.Name.Value, declaredType, stmt.Mutable, stmt.Name.Token)
	}

	if !ok || stmt.Value == nil || stmt.Type == nil {
		if defined && stmt.Value != nil {
			a.setConstInt(stmt.Name.Value, stmt.Value)
		}
		return
	}

	exprType, _ := a.inferExpression(stmt.Value)
	if exprType.Kind == InvalidType {
		return
	}

	if a.checkIntegerExpressionRange(declaredType, stmt.Value) {
		return
	}

	if a.checkInitializerType(declaredType, exprType, stmt.Value) && defined {
		a.setConstInt(stmt.Name.Value, stmt.Value)
	}
}

func (a *Analyzer) analyzeAssignmentStatement(stmt *ast.AssignmentStatement) {
	if member, ok := stmt.Target.(*ast.MemberExpression); ok {
		a.analyzeMemberAssignmentStatement(stmt, member)
		return
	}

	target, ok := stmt.Target.(*ast.Identifier)
	if !ok {
		a.addErrorAtToken(expressionToken(stmt.Target), "invalid assignment target")
		return
	}

	symbol, ok := a.symbols[target.Value]
	if !ok {
		a.addErrorAtToken(target.Token, "undefined variable %s", target.Value)
		return
	}

	if !symbol.Mutable {
		a.addErrorAtToken(target.Token, "cannot assign to immutable variable %s", target.Value)
		return
	}

	exprType, _ := a.inferExpression(stmt.Value)
	if exprType.Kind == InvalidType {
		return
	}

	if stmt.Operator != "=" && !canInitialize(symbol.Type, exprType, stmt.Value) {
		a.addErrorAtToken(
			expressionToken(stmt.Value),
			"cannot %s %s to %s",
			assignmentVerb(stmt.Operator),
			typeDisplayName(exprType),
			typeDisplayName(symbol.Type),
		)
		return
	}

	if a.checkIntegerAssignmentRange(symbol, stmt) {
		return
	}

	if a.checkInitializerType(symbol.Type, exprType, stmt.Value) {
		a.updateAssignedConstInt(symbol.Name, stmt)
	}
}

func (a *Analyzer) analyzeMemberAssignmentStatement(stmt *ast.AssignmentStatement, member *ast.MemberExpression) {
	targetType, ok := a.inferMemberExpression(member)
	if !ok {
		return
	}

	if property, ok := a.lookupPropertyOnMember(member); ok && property.Fallible {
		a.addErrorAtToken(member.Property.Token, "assigning fallible property %s requires try", member.Property.Value)
		return
	}

	valueType, _ := a.inferExpression(stmt.Value)
	if valueType.Kind == InvalidType {
		return
	}

	if !canInitialize(targetType, valueType, stmt.Value) {
		a.addErrorAtToken(expressionToken(stmt.Value), "cannot assign %s to %s", typeDisplayName(valueType), typeDisplayName(targetType))
	}
}

func (a *Analyzer) resolveType(ref *ast.TypeReference) (Type, bool) {
	if ref == nil {
		return Type{Kind: InvalidType}, false
	}

	if ref.ElementType != nil {
		_, ok := a.resolveType(ref.ElementType)
		return Type{Name: "slice", Kind: InvalidType}, ok
	}

	for _, arg := range ref.TypeArgs {
		a.resolveType(arg)
	}

	typ, ok := a.types[ref.Name]
	if !ok {
		a.addErrorAtToken(ref.Token, "unknown type %s", ref.Name)
		return Type{Kind: InvalidType}, false
	}

	return typ, true
}

type expressionValue struct {
	Display  string
	Negative bool
}

func (a *Analyzer) inferExpression(expr ast.Expression) (Type, expressionValue) {
	switch expr := expr.(type) {
	case *ast.IntegerLiteral:
		return Type{Name: "int", Kind: IntType}, expressionValue{Display: expr.String()}
	case *ast.FloatLiteral:
		return Type{Name: "decimal", Kind: DecimalType}, expressionValue{Display: expr.String()}
	case *ast.StringLiteral, *ast.InterpolatedStringLiteral:
		return Type{Name: "string", Kind: StringType}, expressionValue{Display: expr.String()}
	case *ast.BooleanLiteral:
		return Type{Name: "bool", Kind: BoolType}, expressionValue{Display: expr.String()}
	case *ast.Identifier:
		symbol, ok := a.symbols[expr.Value]
		if !ok {
			a.addErrorAtToken(expr.Token, "undefined variable %s", expr.Value)
			return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}
		}
		return symbol.Type, expressionValue{Display: expr.String()}
	case *ast.PrefixExpression:
		return a.inferPrefixExpression(expr)
	case *ast.InfixExpression:
		return a.inferInfixExpression(expr)
	case *ast.ConversionExpression:
		return a.inferConversionExpression(expr)
	case *ast.MemberExpression:
		typ, ok := a.inferMemberExpression(expr)
		if !ok {
			return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}
		}
		return typ, expressionValue{Display: expr.String()}
	case *ast.StructLiteral:
		return a.inferStructLiteral(expr)
	default:
		return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}
	}
}

func (a *Analyzer) inferStructLiteral(expr *ast.StructLiteral) (Type, expressionValue) {
	typ, ok := a.resolveType(expr.Type)
	if !ok {
		return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}
	}

	if typ.Kind != StructType {
		a.addErrorAtToken(expr.Token, "%s is not a struct type", typ.Name)
		return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}
	}

	seen := map[string]lexer.Token{}
	for _, field := range expr.Fields {
		if _, exists := seen[field.Name.Value]; exists {
			a.addErrorAtToken(field.Name.Token, "duplicate field %q in struct literal %s", field.Name.Value, typ.Name)
			continue
		}
		seen[field.Name.Value] = field.Name.Token

		fieldType, ok := lookupStructField(typ, field.Name.Value)
		if !ok {
			a.addErrorAtToken(field.Name.Token, "unknown field %q in struct %s", field.Name.Value, typ.Name)
			continue
		}

		valueType, _ := a.inferExpression(field.Value)
		if valueType.Kind != InvalidType && !canInitialize(fieldType, valueType, field.Value) {
			a.addErrorAtToken(expressionToken(field.Value), "cannot initialize field %s with %s", field.Name.Value, typeDisplayName(valueType))
		}
	}

	return typ, expressionValue{Display: expr.String()}
}

func (a *Analyzer) inferMemberExpression(expr *ast.MemberExpression) (Type, bool) {
	objectType, _ := a.inferExpression(expr.Object)
	if objectType.Kind == InvalidType {
		return Type{Kind: InvalidType}, false
	}

	if fieldType, ok := lookupStructField(objectType, expr.Property.Value); ok {
		return fieldType, true
	}

	if property, ok := lookupProperty(objectType, expr.Property.Value); ok {
		return property.Type, true
	}

	a.addErrorAtToken(expr.Property.Token, "unknown member %s on %s", expr.Property.Value, typeDisplayName(objectType))
	return Type{Kind: InvalidType}, false
}

func (a *Analyzer) lookupPropertyOnMember(expr *ast.MemberExpression) (Property, bool) {
	objectType, _ := a.inferExpression(expr.Object)
	if objectType.Kind == InvalidType {
		return Property{}, false
	}
	return lookupProperty(objectType, expr.Property.Value)
}

func lookupStructField(typ Type, name string) (Type, bool) {
	for _, field := range typ.Fields {
		if field.Name == name {
			return field.Type, true
		}
	}
	return Type{}, false
}

func lookupProperty(typ Type, name string) (Property, bool) {
	for _, property := range typ.Properties {
		if property.Name == name {
			return property, true
		}
	}
	return Property{}, false
}

func (a *Analyzer) inferConversionExpression(expr *ast.ConversionExpression) (Type, expressionValue) {
	targetType, ok := a.resolveType(expr.Type)
	if !ok {
		return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}
	}

	valueType, _ := a.inferExpression(expr.Value)
	if valueType.Kind == InvalidType {
		return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}
	}

	if !canExplicitConvert(targetType, valueType) {
		a.addErrorAtToken(expr.Token, "cannot convert %s to %s", typeDisplayName(valueType), typeDisplayName(targetType))
		return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}
	}

	return targetType, expressionValue{Display: expr.String()}
}

func (a *Analyzer) inferInfixExpression(expr *ast.InfixExpression) (Type, expressionValue) {
	leftType, _ := a.inferExpression(expr.Left)
	rightType, _ := a.inferExpression(expr.Right)
	if leftType.Kind == InvalidType || rightType.Kind == InvalidType {
		return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}
	}

	if leftType.Kind == DecimalType || rightType.Kind == DecimalType {
		return a.inferDecimalInfixExpression(expr, leftType, rightType)
	}

	if leftType.Kind == rightType.Kind {
		return leftType, expressionValue{Display: expr.String()}
	}

	if leftType.Kind == UintType && rightType.Kind == IntType {
		return leftType, expressionValue{Display: expr.String()}
	}

	if leftType.Kind == IntType && rightType.Kind == UintType {
		return rightType, expressionValue{Display: expr.String()}
	}

	return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}
}

func (a *Analyzer) inferDecimalInfixExpression(expr *ast.InfixExpression, leftType Type, rightType Type) (Type, expressionValue) {
	if leftType.Kind == DecimalType && (rightType.Kind == IntType || rightType.Kind == UintType) {
		switch expr.Operator {
		case "*", "/":
			return leftType, expressionValue{Display: expr.String()}
		}
	}

	if (leftType.Kind == IntType || leftType.Kind == UintType) && rightType.Kind == DecimalType {
		switch expr.Operator {
		case "*":
			return rightType, expressionValue{Display: expr.String()}
		}
	}

	if leftType.Kind != DecimalType || rightType.Kind != DecimalType {
		return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}
	}

	switch expr.Operator {
	case "+", "-":
		if sameConcreteType(leftType, rightType) {
			return leftType, expressionValue{Display: expr.String()}
		}
		if leftType.Dimension.Equal(rightType.Dimension) {
			return a.typeForDimension(DecimalType, leftType.Dimension), expressionValue{Display: expr.String()}
		}
		a.addErrorAtToken(
			expr.Token,
			"cannot %s %s to %s",
			infixVerb(expr.Operator),
			typeDisplayName(rightType),
			typeDisplayName(leftType),
		)
		return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}
	case "*":
		if leftType.Dimension.IsZero() && !rightType.Dimension.IsZero() {
			return rightType, expressionValue{Display: expr.String()}
		}
		if rightType.Dimension.IsZero() && !leftType.Dimension.IsZero() {
			return leftType, expressionValue{Display: expr.String()}
		}
		if leftType.Dimension.IsZero() && rightType.Dimension.IsZero() {
			return leftType, expressionValue{Display: expr.String()}
		}
		if leftType.Dimension.Equal(rightType.Dimension) && leftType.Dimension.HasCurrencyBase() {
			a.addErrorAtToken(
				expr.Token,
				"cannot multiply %s by %s",
				typeDisplayName(leftType),
				typeDisplayName(rightType),
			)
			return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}
		}
		return a.typeForDimension(DecimalType, leftType.Dimension.Mul(rightType.Dimension)), expressionValue{Display: expr.String()}
	case "/":
		return a.typeForDimension(DecimalType, leftType.Dimension.Div(rightType.Dimension)), expressionValue{Display: expr.String()}
	}

	return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}
}

func (a *Analyzer) inferPrefixExpression(expr *ast.PrefixExpression) (Type, expressionValue) {
	rightType, rightValue := a.inferExpression(expr.Right)

	switch expr.Operator {
	case "-":
		if rightType.Kind == IntType || rightType.Kind == FloatType || rightType.Kind == DecimalType {
			return rightType, expressionValue{
				Display:  "-" + rightValue.Display,
				Negative: true,
			}
		}
	case "!":
		return Type{Name: "bool", Kind: BoolType}, expressionValue{Display: expr.String()}
	}

	return Type{Kind: InvalidType}, expressionValue{Display: expr.String()}
}

func (a *Analyzer) addError(format string, args ...any) {
	a.errors = append(a.errors, Error{Message: fmt.Sprintf(format, args...)})
}

func (a *Analyzer) addErrorAtToken(token lexer.Token, format string, args ...any) {
	a.errors = append(a.errors, Error{
		Message: fmt.Sprintf(format, args...),
		Line:    token.Line,
		Column:  token.Column,
	})
}

func (a *Analyzer) defineSymbol(name string, typ Type, mutable bool, token lexer.Token) bool {
	if previous, exists := a.symbols[name]; exists {
		a.errors = append(a.errors, Error{
			Message:        fmt.Sprintf("variable %q already declared", name),
			Line:           token.Line,
			Column:         token.Column,
			PreviousLine:   previous.Token.Line,
			PreviousColumn: previous.Token.Column,
		})
		return false
	}

	a.symbols[name] = Symbol{Name: name, Type: typ, Mutable: mutable, Token: token}
	return true
}

func (a *Analyzer) checkInitializerType(target Type, value Type, expr ast.Expression) bool {
	if canInitialize(target, value, expr) {
		return true
	}

	a.addErrorAtToken(
		expressionToken(expr),
		"cannot initialize %s with %s",
		typeDisplayName(target),
		typeDisplayName(value),
	)
	return false
}

func canInitialize(target Type, value Type, expr ast.Expression) bool {
	if target.Kind == InvalidType || value.Kind == InvalidType {
		return true
	}

	if isNominal(target) && target.Name != value.Name {
		return isUntypedNumericExpression(expr)
	}

	if target.Kind == value.Kind {
		return true
	}

	if target.Kind == UintType && value.Kind == IntType {
		return true
	}

	if target.Kind == DecimalType && isNumericLiteral(expr) {
		_, ok := decimalLiteralValue(expr)
		return ok
	}

	if target.Kind == FloatType && value.Kind == DecimalType && isNumericLiteral(expr) {
		return true
	}

	return false
}

func canExplicitConvert(target Type, value Type) bool {
	if target.Kind == InvalidType || value.Kind == InvalidType {
		return false
	}

	if target.Kind == value.Kind {
		return true
	}

	if target.Kind == DecimalType && (value.Kind == IntType || value.Kind == FloatType) {
		return true
	}

	return false
}

func hasContracts(typ Type) bool {
	return len(typ.Contracts) > 0
}

func isNominal(typ Type) bool {
	return typ.Named && (hasContracts(typ) || !typ.Dimension.IsZero())
}

func sameConcreteType(left Type, right Type) bool {
	if left.Name != "" || right.Name != "" {
		return left.Name == right.Name
	}
	return left.Kind == right.Kind
}

func assignmentVerb(operator string) string {
	switch operator {
	case "+=":
		return "add"
	case "-=":
		return "subtract"
	case "*=":
		return "multiply"
	case "/=":
		return "divide"
	default:
		return "assign"
	}
}

func infixVerb(operator string) string {
	switch operator {
	case "+":
		return "add"
	case "-":
		return "subtract"
	default:
		return "combine"
	}
}

func isUntypedNumericExpression(expr ast.Expression) bool {
	if isNumericLiteral(expr) {
		return true
	}

	_, ok := constantIntegerValue(expr)
	return ok
}

func typeDisplayName(typ Type) string {
	if typ.Name != "" {
		return typ.Name
	}
	return string(typ.Kind)
}

func expressionToken(expr ast.Expression) lexer.Token {
	switch expr := expr.(type) {
	case *ast.Identifier:
		return expr.Token
	case *ast.IntegerLiteral:
		return expr.Token
	case *ast.FloatLiteral:
		return expr.Token
	case *ast.StringLiteral:
		return expr.Token
	case *ast.BooleanLiteral:
		return expr.Token
	case *ast.InterpolatedStringLiteral:
		return expr.Token
	case *ast.PrefixExpression:
		return expr.Token
	case *ast.InfixExpression:
		return expr.Token
	case *ast.ConversionExpression:
		return expr.Token
	case *ast.MemberExpression:
		return expr.Token
	case *ast.StructLiteral:
		return expr.Token
	default:
		return lexer.Token{}
	}
}
