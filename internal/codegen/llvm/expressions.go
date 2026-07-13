package llvm

import (
	"fmt"
	"strconv"
	"strings"

	"sec/internal/ast"
)

type value struct {
	typ    string
	ref    string
	lenRef string
	fnType *ast.TypeReference
}

func (g *Generator) emitExpression(expr ast.Expression) (value, error) {
	switch expr := expr.(type) {
	case *ast.Identifier:
		return g.emitIdentifier(expr)
	case *ast.IntegerLiteral:
		if expr.Suffix() == "d" {
			return g.emitDecimalLiteral(expr, false)
		}
		parsed, ok := ast.ParseIntegerLiteralLexeme(expr.Token.Lexeme)
		if !ok {
			return value{}, fmt.Errorf("emit-llvm could not parse integer %q", expr.Token.Lexeme)
		}
		typ := "i32"
		if expr.Suffix() == "u" {
			typ = "i64"
		}
		return value{typ: typ, ref: parsed.String()}, nil
	case *ast.FloatLiteral:
		if expr.Suffix() == "f" {
			return value{}, fmt.Errorf("emit-llvm does not support float literals yet")
		}
		return g.emitDecimalLiteral(expr, false)
	case *ast.BooleanLiteral:
		if expr.Value {
			return value{typ: "i1", ref: "true"}, nil
		}
		return value{typ: "i1", ref: "false"}, nil
	case *ast.StringLiteral:
		return g.emitStringLiteral(expr)
	case *ast.InterpolatedStringLiteral:
		return g.emitInterpolatedStringLiteral(expr)
	case *ast.MemberExpression:
		return g.emitMemberExpression(expr)
	case *ast.StructLiteral:
		return value{typ: "i8", ref: "0"}, nil
	case *ast.MatchExpression:
		return g.emitMatchExpression(expr)
	case *ast.ConversionExpression:
		return g.emitConversionExpression(expr)
	case *ast.TryExpression:
		return g.emitTryExpression(expr)
	case *ast.SpawnExpression:
		return value{typ: "i8", ref: "0"}, nil
	case *ast.AwaitExpression:
		if expr.Value != nil {
			if _, err := g.emitExpression(expr.Value); err != nil {
				return value{}, err
			}
		}
		return value{typ: "void"}, nil
	case *ast.OkExpression:
		if expr.Value == nil {
			return value{typ: g.returnType, ref: llvmZeroValue(g.returnType)}, nil
		}
		return g.emitExpression(expr.Value)
	case *ast.ErrExpression:
		return value{typ: g.returnType, ref: llvmZeroValue(g.returnType)}, nil
	case *ast.PrefixExpression:
		return g.emitPrefixExpression(expr)
	case *ast.InfixExpression:
		return g.emitInfixExpression(expr)
	case *ast.CallExpression:
		return g.emitCallExpression(expr)
	case *ast.LambdaExpression:
		return g.emitLambdaExpression(expr)
	default:
		return value{}, fmt.Errorf("emit-llvm does not support expression %T yet", expr)
	}
}

func (g *Generator) emitTryExpression(expr *ast.TryExpression) (value, error) {
	return g.emitExpression(expr.Expression)
}

func (g *Generator) emitConversionExpression(expr *ast.ConversionExpression) (value, error) {
	if expr.Type == nil || expr.Value == nil {
		return value{}, fmt.Errorf("emit-llvm requires complete conversion expression")
	}
	if g.llvmType(expr.Type) == llvmDecimalType {
		if decimal, ok := decimalLiteralValue(expr.Value, true); ok {
			return g.emitDecimalValue(decimal), nil
		}
	}
	val, err := g.emitExpression(expr.Value)
	if err != nil {
		return value{}, err
	}
	return g.coerceValue(val, g.llvmType(expr.Type))
}

type decimalLiteral struct {
	number int64
	scale  uint8
}

func (g *Generator) emitDecimalLiteral(expr ast.Expression, allowPlainInteger bool) (value, error) {
	decimal, ok := decimalLiteralValue(expr, allowPlainInteger)
	if !ok {
		return value{}, fmt.Errorf("emit-llvm could not parse decimal literal %q", expr.TokenLiteral())
	}
	return g.emitDecimalValue(decimal), nil
}

func (g *Generator) emitDecimalValue(decimal decimalLiteral) value {
	g.needsDecimal = true
	tmp := g.nextTemp()
	g.write("  %s = insertvalue %s undef, i64 %d, 0\n", tmp, llvmDecimalType, decimal.number)
	result := g.nextTemp()
	g.write("  %s = insertvalue %s %s, i8 %d, 1\n", result, llvmDecimalType, tmp, decimal.scale)
	return value{typ: llvmDecimalType, ref: result}
}

func decimalLiteralValue(expr ast.Expression, allowPlainInteger bool) (decimalLiteral, bool) {
	switch expr := expr.(type) {
	case *ast.IntegerLiteral:
		if !allowPlainInteger && expr.Suffix() != "d" {
			return decimalLiteral{}, false
		}
		return parseDecimalLiteralLexeme(expr.Token.Lexeme)
	case *ast.FloatLiteral:
		if expr.Suffix() == "f" {
			return decimalLiteral{}, false
		}
		return parseDecimalLiteralLexeme(expr.Token.Lexeme)
	case *ast.PrefixExpression:
		if expr.Operator != "-" {
			return decimalLiteral{}, false
		}
		decimal, ok := decimalLiteralValue(expr.Right, allowPlainInteger)
		if !ok {
			return decimalLiteral{}, false
		}
		decimal.number = -decimal.number
		return decimal, true
	default:
		return decimalLiteral{}, false
	}
}

func parseDecimalLiteralLexeme(lexeme string) (decimalLiteral, bool) {
	digits, suffix := ast.SplitNumericLiteralSuffix(lexeme)
	if suffix == "f" || suffix == "i" || suffix == "u" || digits == "" {
		return decimalLiteral{}, false
	}
	if strings.HasPrefix(digits, "0x") || strings.HasPrefix(digits, "0X") ||
		strings.HasPrefix(digits, "0b") || strings.HasPrefix(digits, "0B") ||
		strings.HasPrefix(digits, "0o") || strings.HasPrefix(digits, "0O") {
		return decimalLiteral{}, false
	}

	negative := false
	if strings.HasPrefix(digits, "-") {
		negative = true
		digits = strings.TrimPrefix(digits, "-")
	}

	parts := strings.Split(digits, ".")
	if len(parts) > 2 {
		return decimalLiteral{}, false
	}
	scale := 0
	wholeDigits := parts[0]
	if len(parts) == 2 {
		wholeDigits += parts[1]
		scale = len(parts[1])
	}
	if wholeDigits == "" || scale > 255 {
		return decimalLiteral{}, false
	}
	number, err := strconv.ParseInt(wholeDigits, 10, 64)
	if err != nil {
		return decimalLiteral{}, false
	}
	if negative {
		number = -number
	}
	return decimalLiteral{number: number, scale: uint8(scale)}, true
}

func (g *Generator) emitIdentifier(expr *ast.Identifier) (value, error) {
	slot, ok := g.locals[expr.Value]
	if !ok {
		if fn, exists := g.functions[expr.Value]; exists {
			return value{typ: "ptr", ref: "@" + expr.Value, fnType: functionDeclarationType(fn)}, nil
		}
		return value{}, fmt.Errorf("emit-llvm unknown identifier %s", expr.Value)
	}
	if slot.direct {
		return value{typ: slot.typ, ref: slot.ref, lenRef: slot.lenRef, fnType: slot.fnType}, nil
	}
	if slot.typ == "string" {
		return value{typ: "string", ref: slot.ref, lenRef: slot.lenRef}, nil
	}
	temp := g.nextTemp()
	g.write("  %s = load %s, ptr %s\n", temp, slot.typ, slot.ptr)
	return value{typ: slot.typ, ref: temp, fnType: slot.fnType}, nil
}

func (g *Generator) emitMemberExpression(expr *ast.MemberExpression) (value, error) {
	if typeName, ok := expressionPath(expr.Object); ok {
		if enum, exists := g.enums[typeName]; exists {
			enumValue, ok := enum.values[expr.Property.Value]
			if !ok {
				return value{}, fmt.Errorf("unknown enum value %s.%s", typeName, expr.Property.Value)
			}
			return value{typ: enum.typ, ref: enumValue}, nil
		}
	}

	object, err := g.emitExpression(expr.Object)
	if err != nil {
		return value{}, err
	}
	if object.typ != "string" {
		return value{}, fmt.Errorf("emit-llvm only supports members on string for now")
	}
	switch expr.Property.Value {
	case "ptr":
		return value{typ: "ptr", ref: object.ref}, nil
	case "len":
		return value{typ: "i64", ref: object.lenRef}, nil
	default:
		return value{}, fmt.Errorf("unknown string member %s", expr.Property.Value)
	}
}

func (g *Generator) emitPrefixExpression(expr *ast.PrefixExpression) (value, error) {
	right, err := g.emitExpression(expr.Right)
	if err != nil {
		return value{}, err
	}
	switch expr.Operator {
	case "-":
		if right.typ != "i32" {
			return value{}, fmt.Errorf("emit-llvm unary - currently expects int")
		}
		temp := g.nextTemp()
		g.write("  %s = sub i32 0, %s\n", temp, right.ref)
		return value{typ: "i32", ref: temp}, nil
	case "!":
		if right.typ != "i1" {
			return value{}, fmt.Errorf("emit-llvm unary ! currently expects bool")
		}
		temp := g.nextTemp()
		g.write("  %s = xor i1 %s, true\n", temp, right.ref)
		return value{typ: "i1", ref: temp}, nil
	default:
		return value{}, fmt.Errorf("emit-llvm does not support prefix operator %q yet", expr.Operator)
	}
}

func (g *Generator) emitInfixExpression(expr *ast.InfixExpression) (value, error) {
	switch expr.Operator {
	case "&&":
		return g.emitShortCircuitAnd(expr)
	case "||":
		return g.emitShortCircuitOr(expr)
	}

	left, err := g.emitExpression(expr.Left)
	if err != nil {
		return value{}, err
	}
	if expr.Operator == "in" {
		return g.emitMembershipExpression(left, expr.Right)
	}
	right, err := g.emitExpression(expr.Right)
	if err != nil {
		return value{}, err
	}
	if left.typ == llvmDecimalType || right.typ == llvmDecimalType {
		return g.emitDecimalInfixPlaceholder(expr.Operator, left, right)
	}

	switch expr.Operator {
	case "+":
		return g.emitIntegerBinary("add", left, right)
	case "-":
		return g.emitIntegerBinary("sub", left, right)
	case "*":
		return g.emitIntegerBinary("mul", left, right)
	case "/":
		return g.emitIntegerBinary("sdiv", left, right)
	case "==":
		return g.emitCompare("eq", left, right), nil
	case "!=":
		return g.emitCompare("ne", left, right), nil
	case "<":
		return g.emitCompare("slt", left, right), nil
	case "<=":
		return g.emitCompare("sle", left, right), nil
	case ">":
		return g.emitCompare("sgt", left, right), nil
	case ">=":
		return g.emitCompare("sge", left, right), nil
	default:
		return value{}, fmt.Errorf("emit-llvm does not support operator %q yet", expr.Operator)
	}
}

func (g *Generator) emitMembershipExpression(left value, right ast.Expression) (value, error) {
	rangeExpr, ok := right.(*ast.RangeExpression)
	if !ok {
		return value{}, fmt.Errorf("emit-llvm membership requires range expression")
	}
	if rangeExpr.Start == nil || rangeExpr.End == nil {
		return value{}, fmt.Errorf("emit-llvm membership currently requires finite range")
	}
	start, err := g.emitExpression(rangeExpr.Start)
	if err != nil {
		return value{}, err
	}
	end, err := g.emitExpression(rangeExpr.End)
	if err != nil {
		return value{}, err
	}
	start, err = g.coerceValue(start, left.typ)
	if err != nil {
		return value{}, err
	}
	end, err = g.coerceValue(end, left.typ)
	if err != nil {
		return value{}, err
	}

	lower := g.emitCompare("sge", left, start)
	upperPredicate := "sle"
	if rangeExpr.Exclusive {
		upperPredicate = "slt"
	}
	upper := g.emitCompare(upperPredicate, left, end)
	result := g.nextTemp()
	g.write("  %s = and i1 %s, %s\n", result, lower.ref, upper.ref)
	return value{typ: "i1", ref: result}, nil
}

func (g *Generator) emitDecimalInfixPlaceholder(operator string, left value, right value) (value, error) {
	if left.typ != llvmDecimalType || right.typ != llvmDecimalType {
		return value{}, fmt.Errorf("emit-llvm decimal operator requires decimal operands")
	}
	switch operator {
	case "+", "-", "*", "/":
		// TODO: Lower decimal arithmetic through the decimal runtime.
		return left, nil
	case "==", "!=", "<", "<=", ">", ">=":
		// TODO: Lower decimal comparison through the decimal runtime.
		return value{typ: "i1", ref: "false"}, nil
	default:
		return value{}, fmt.Errorf("emit-llvm does not support decimal operator %q yet", operator)
	}
}

func (g *Generator) emitShortCircuitAnd(expr *ast.InfixExpression) (value, error) {
	rightLabel := g.nextLabel("and.rhs")
	falseLabel := g.nextLabel("and.false")
	endLabel := g.nextLabel("and.end")

	left, err := g.emitExpression(expr.Left)
	if err != nil {
		return value{}, err
	}
	if left.typ != "i1" {
		return value{}, fmt.Errorf("emit-llvm && expects bool operands")
	}
	g.write("  br i1 %s, label %%%s, label %%%s\n\n", left.ref, rightLabel, falseLabel)
	g.blockOpen = false

	g.write("%s:\n", rightLabel)
	g.blockOpen = true
	right, err := g.emitExpression(expr.Right)
	if err != nil {
		return value{}, err
	}
	if right.typ != "i1" {
		return value{}, fmt.Errorf("emit-llvm && expects bool operands")
	}
	g.write("  br label %%%s\n\n", endLabel)
	g.blockOpen = false

	g.write("%s:\n", falseLabel)
	g.blockOpen = true
	g.write("  br label %%%s\n\n", endLabel)
	g.blockOpen = false

	result := g.nextTemp()
	g.write("%s:\n", endLabel)
	g.blockOpen = true
	g.write("  %s = phi i1 [%s, %%%s], [false, %%%s]\n", result, right.ref, rightLabel, falseLabel)
	return value{typ: "i1", ref: result}, nil
}

func (g *Generator) emitShortCircuitOr(expr *ast.InfixExpression) (value, error) {
	trueLabel := g.nextLabel("or.true")
	rightLabel := g.nextLabel("or.rhs")
	endLabel := g.nextLabel("or.end")

	left, err := g.emitExpression(expr.Left)
	if err != nil {
		return value{}, err
	}
	if left.typ != "i1" {
		return value{}, fmt.Errorf("emit-llvm || expects bool operands")
	}
	g.write("  br i1 %s, label %%%s, label %%%s\n\n", left.ref, trueLabel, rightLabel)
	g.blockOpen = false

	g.write("%s:\n", trueLabel)
	g.blockOpen = true
	g.write("  br label %%%s\n\n", endLabel)
	g.blockOpen = false

	g.write("%s:\n", rightLabel)
	g.blockOpen = true
	right, err := g.emitExpression(expr.Right)
	if err != nil {
		return value{}, err
	}
	if right.typ != "i1" {
		return value{}, fmt.Errorf("emit-llvm || expects bool operands")
	}
	g.write("  br label %%%s\n\n", endLabel)
	g.blockOpen = false

	result := g.nextTemp()
	g.write("%s:\n", endLabel)
	g.blockOpen = true
	g.write("  %s = phi i1 [true, %%%s], [%s, %%%s]\n", result, trueLabel, right.ref, rightLabel)
	return value{typ: "i1", ref: result}, nil
}

func (g *Generator) emitIntegerBinary(op string, left value, right value) (value, error) {
	if left.typ != right.typ {
		return value{}, fmt.Errorf("emit-llvm binary operator requires matching operand types")
	}
	if left.typ != "i32" && left.typ != "i64" {
		return value{}, fmt.Errorf("emit-llvm binary operator currently expects integers")
	}
	temp := g.nextTemp()
	g.write("  %s = %s %s %s, %s\n", temp, op, left.typ, left.ref, right.ref)
	return value{typ: left.typ, ref: temp}, nil
}

func (g *Generator) emitCompare(predicate string, left value, right value) value {
	temp := g.nextTemp()
	g.write("  %s = icmp %s %s %s, %s\n", temp, predicate, left.typ, left.ref, right.ref)
	return value{typ: "i1", ref: temp}
}

func (g *Generator) emitBoolAnd(left value, right value) (value, error) {
	if left.typ != "i1" || right.typ != "i1" {
		return value{}, fmt.Errorf("emit-llvm boolean and expects bool operands")
	}
	temp := g.nextTemp()
	g.write("  %s = and i1 %s, %s\n", temp, left.ref, right.ref)
	return value{typ: "i1", ref: temp}, nil
}

func (g *Generator) emitBoolOr(left value, right value) (value, error) {
	if left.typ != "i1" || right.typ != "i1" {
		return value{}, fmt.Errorf("emit-llvm boolean or expects bool operands")
	}
	temp := g.nextTemp()
	g.write("  %s = or i1 %s, %s\n", temp, left.ref, right.ref)
	return value{typ: "i1", ref: temp}, nil
}

func (g *Generator) emitStringLiteral(expr *ast.StringLiteral) (value, error) {
	name := fmt.Sprintf("@.str.%d", g.stringID)
	g.stringID++

	bytes := append([]byte(expr.Value), 0)
	g.globals.WriteString(fmt.Sprintf("%s = private unnamed_addr constant [%d x i8] c\"%s\"\n", name, len(bytes), llvmCString(bytes)))

	temp := g.nextTemp()
	g.write("  %s = getelementptr inbounds [%d x i8], ptr %s, i64 0, i64 0\n", temp, len(bytes), name)
	return value{typ: "string", ref: temp, lenRef: fmt.Sprintf("%d", len(expr.Value))}, nil
}

func (g *Generator) emitInterpolatedStringLiteral(expr *ast.InterpolatedStringLiteral) (value, error) {
	return g.emitStringLiteral(&ast.StringLiteral{Token: expr.Token, Value: expr.Value})
}

func llvmCString(bytes []byte) string {
	var out strings.Builder
	for _, b := range bytes {
		switch b {
		case '\\':
			out.WriteString("\\5C")
		case '"':
			out.WriteString("\\22")
		case '\n':
			out.WriteString("\\0A")
		case '\t':
			out.WriteString("\\09")
		case 0:
			out.WriteString("\\00")
		default:
			if b >= 32 && b <= 126 {
				out.WriteByte(b)
				continue
			}
			out.WriteString("\\")
			out.WriteString(fmt.Sprintf("%02X", b))
		}
	}
	return out.String()
}

func callExpressionName(expr *ast.CallExpression) string {
	if expr.Callee != nil {
		if name, ok := expressionPath(expr.Callee); ok {
			return name
		}
	}
	if expr.Function != nil {
		return expr.Function.Value
	}
	return ""
}

func expressionPath(expr ast.Expression) (string, bool) {
	switch expr := expr.(type) {
	case *ast.Identifier:
		return expr.Value, true
	case *ast.MemberExpression:
		left, ok := expressionPath(expr.Object)
		if !ok {
			return "", false
		}
		return left + "." + expr.Property.Value, true
	default:
		return "", false
	}
}
