package main

import (
	"flag"
	"fmt"
	"os"
	"text/tabwriter"

	"sec/internal/ast"
	"sec/internal/lexer"
	"sec/internal/parser"
	"sec/internal/sema"
)

func main() {
	flag.Parse()

	if flag.NArg() != 2 {
		printUsage()
		os.Exit(1)
	}

	command := flag.Arg(0)
	file := flag.Arg(1)

	data, err := os.ReadFile(file)
	if err != nil {
		fmt.Fprintf(os.Stderr, "read error: %v\n", err)
		os.Exit(1)
	}

	switch command {
	case "lex":
		runLex(string(data))

	case "token", "tokens", "lexer":
		runTokens(string(data))

	case "parse":
		runParse(string(data))

	case "ast":
		runAST(string(data))

	case "sema":
		runSema(string(data))

	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\n", command)
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Fprintln(os.Stderr, "usage: sec <lex|token|parse|ast|sema> <file.sec>")
}

func runLex(input string) {
	l := lexer.New(input)

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "LEXEME\tLINE\tCOLUMN")

	exitCode := 0

	for {
		tok := l.NextToken()

		if tok.Type != lexer.EOF {
			fmt.Fprintf(
				w,
				"%q\t%d\t%d\n",
				tok.Lexeme,
				tok.Line,
				tok.Column,
			)
		}

		if tok.Type == lexer.ILLEGAL {
			exitCode = 2
		}

		if tok.Type == lexer.EOF {
			break
		}
	}

	_ = w.Flush()
	os.Exit(exitCode)
}

func runTokens(input string) {
	l := lexer.New(input)

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "TYPE\tLEXEME\tLINE\tCOLUMN")

	exitCode := 0

	for {
		tok := l.NextToken()

		fmt.Fprintf(
			w,
			"%s\t%q\t%d\t%d\n",
			tok.Type,
			tok.Lexeme,
			tok.Line,
			tok.Column,
		)

		if tok.Type == lexer.ILLEGAL {
			exitCode = 2
		}

		if tok.Type == lexer.EOF {
			break
		}
	}

	_ = w.Flush()
	os.Exit(exitCode)
}

func runParse(input string) {
	l := lexer.New(input)
	p := parser.New(l)

	program := p.ParseProgram()

	if len(p.Errors()) > 0 {
		for _, err := range p.Errors() {
			fmt.Fprintf(os.Stderr, "parse error: %s\n", err)
		}
		os.Exit(2)
	}

	printProgram(program)
}

func runAST(input string) {
	l := lexer.New(input)
	p := parser.New(l)

	program := p.ParseProgram()

	if len(p.Errors()) > 0 {
		for _, err := range p.Errors() {
			fmt.Fprintf(os.Stderr, "parse error: %s\n", err)
		}
		os.Exit(2)
	}

	printAST(program)
}

func runSema(input string) {
	l := lexer.New(input)
	p := parser.New(l)

	program := p.ParseProgram()

	if len(p.Errors()) > 0 {
		for _, err := range p.Errors() {
			fmt.Fprintf(os.Stderr, "parse error: %s\n", err)
		}
		os.Exit(2)
	}

	analyzer := sema.NewAnalyzer()
	errors := analyzer.Analyze(program)
	if len(errors) > 0 {
		for _, err := range errors {
			fmt.Fprintf(os.Stderr, "sema error: %s\n", err)
		}
		os.Exit(3)
	}

	fmt.Println("OK")
}

func printProgram(program *ast.Program) {
	for i, stmt := range program.Statements {
		if i > 0 {
			fmt.Println()
		}

		printStatement(stmt)
	}
}

func printAST(program *ast.Program) {
	fmt.Println("Program")

	for i, stmt := range program.Statements {
		printASTStatement(stmt, "", i == len(program.Statements)-1)
	}
}

func printASTStatement(stmt ast.Statement, prefix string, last bool) {
	switch stmt := stmt.(type) {
	case *ast.ModuleStatement:
		printASTBranch(prefix, last, "Module")
		printASTLeaf(childPrefix(prefix, last), true, "Path: "+stmt.Path)

	case *ast.ImportStatement:
		printASTBranch(prefix, last, "Import")
		children := []string{}
		if stmt.Alias != "" {
			children = append(children, "Alias: "+stmt.Alias)
		}
		children = append(children, fmt.Sprintf("Path: %q", stmt.Path))
		printASTLeaves(childPrefix(prefix, last), children)

	case *ast.TypeDeclStatement:
		printASTTypeDecl(stmt, prefix, last)

	case *ast.EnumDeclaration:
		printASTEnum(stmt, prefix, last)

	case *ast.FunctionDeclaration:
		printASTFunction(stmt, prefix, last)

	case *ast.StructStatement:
		printASTBranch(prefix, last, "Struct")
		childrenPrefix := childPrefix(prefix, last)
		printASTLeaf(childrenPrefix, len(stmt.Fields) == 0, "Name: "+stmt.Name.Value)
		for i, field := range stmt.Fields {
			printASTField(childrenPrefix, field, i == len(stmt.Fields)-1)
		}

	case *ast.LetStatement:
		printASTLet(stmt, prefix, last)

	case *ast.LetGroupStatement:
		printASTBranch(prefix, last, "LetGroup")
		childrenPrefix := childPrefix(prefix, last)
		for i, let := range stmt.Lets {
			printASTLet(let, childrenPrefix, i == len(stmt.Lets)-1)
		}

	case *ast.AssignmentStatement:
		printASTAssignment(stmt, prefix, last)

	case *ast.ExpressionStatement:
		printASTExpression(prefix, last, "Expression", stmt.Expression)

	case *ast.ReturnStatement:
		printASTReturn(stmt, prefix, last)

	case *ast.IfStatement:
		printASTIf(stmt, prefix, last)

	case *ast.MatchStatement:
		printASTExpression(prefix, last, "Match", stmt.Match)

	case *ast.ImplStatement:
		printASTImpl(stmt, prefix, last)

	case *ast.CommentStatement:
		printASTBranch(prefix, last, "Comment")
		printASTLeaf(childPrefix(prefix, last), true, fmt.Sprintf("Text: %q", stmt.Text))

	case *ast.InvalidStatement:
		printASTBranch(prefix, last, "Invalid")
		printASTLeaf(childPrefix(prefix, last), true, "Token: "+stmt.TokenLiteral())

	default:
		printASTBranch(prefix, last, fmt.Sprintf("%T", stmt))
		printASTLeaf(childPrefix(prefix, last), true, "Token: "+stmt.TokenLiteral())
	}
}

func printASTIf(stmt *ast.IfStatement, prefix string, last bool) {
	printASTBranch(prefix, last, "If")
	childrenPrefix := childPrefix(prefix, last)
	printASTExpression(childrenPrefix, false, "Condition", stmt.Condition)

	hasAlternative := stmt.Alternative != nil
	printASTBranch(childrenPrefix, !hasAlternative, "Then")
	thenPrefix := childPrefix(childrenPrefix, !hasAlternative)
	for i, bodyStmt := range stmt.Consequence.Statements {
		printASTStatement(bodyStmt, thenPrefix, i == len(stmt.Consequence.Statements)-1)
	}

	if hasAlternative {
		printASTBranch(childrenPrefix, true, "Else")
		elsePrefix := childPrefix(childrenPrefix, true)
		for i, bodyStmt := range stmt.Alternative.Statements {
			printASTStatement(bodyStmt, elsePrefix, i == len(stmt.Alternative.Statements)-1)
		}
	}
}

func printASTImpl(stmt *ast.ImplStatement, prefix string, last bool) {
	printASTBranch(prefix, last, "Impl")

	childrenPrefix := childPrefix(prefix, last)
	printASTLeaf(childrenPrefix, len(stmt.Members) == 0, "Target: "+formatTypeRef(stmt.Target))

	for i, member := range stmt.Members {
		printASTImplMember(childrenPrefix, member, i == len(stmt.Members)-1)
	}
}

func printASTImplMember(prefix string, member ast.ImplMember, last bool) {
	switch member := member.(type) {
	case *ast.TypeDeclStatement:
		printASTTypeDecl(member, prefix, last)
	case *ast.EnumDeclaration:
		printASTEnum(member, prefix, last)
	case *ast.FunctionDeclaration:
		printASTFunction(member, prefix, last)
	case *ast.PropertyDeclaration:
		printASTBranch(prefix, last, "Property")
		children := []string{
			"Name: " + member.Name.Value,
			"Type: " + formatTypeRef(member.Type),
		}
		if member.Getter != nil {
			children = append(children, "Getter: true")
		}
		if member.Setter != nil {
			children = append(children, fmt.Sprintf("Setter: %s fallible=%t", member.Setter.Parameter.Value, member.Setter.Fallible))
		}
		printASTLeaves(childPrefix(prefix, last), children)
	default:
		printASTBranch(prefix, last, fmt.Sprintf("%T", member))
		printASTLeaf(childPrefix(prefix, last), true, "Token: "+member.TokenLiteral())
	}
}

func printASTLet(stmt *ast.LetStatement, prefix string, last bool) {
	printASTBranch(prefix, last, "Let")

	childrenPrefix := childPrefix(prefix, last)
	children := []string{
		fmt.Sprintf("Mutable: %t", stmt.Mutable),
		"Name: " + stmt.Name.Value,
	}

	if stmt.Type != nil {
		children = append(children, "Type: "+formatTypeRef(stmt.Type))
	}

	if stmt.Value == nil {
		printASTLeaves(childrenPrefix, children)
		return
	}

	for _, child := range children {
		printASTLeaf(childrenPrefix, false, child)
	}

	printASTExpression(childrenPrefix, true, "Value", stmt.Value)
}

func printASTAssignment(stmt *ast.AssignmentStatement, prefix string, last bool) {
	printASTBranch(prefix, last, "Assignment")

	childrenPrefix := childPrefix(prefix, last)
	printASTExpression(childrenPrefix, false, "Target", stmt.Target)
	printASTLeaf(childrenPrefix, false, "Operator: "+stmt.Operator)
	printASTExpression(childrenPrefix, true, "Value", stmt.Value)
}

func printASTTypeDecl(stmt *ast.TypeDeclStatement, prefix string, last bool) {
	printASTBranch(prefix, last, "Type")

	children := []string{"Name: " + stmt.Name.Value}

	if stmt.BaseType != nil {
		children = append(children, "Base: "+formatTypeRef(stmt.BaseType))
	}

	if stmt.AssignedType != nil {
		children = append(children, "Assigned: "+formatTypeRef(stmt.AssignedType))
	}

	if len(stmt.Variants) > 0 {
		children = append(children, "Variants: "+formatVariants(stmt.Variants))
	}

	if stmt.Contract != nil {
		children = append(children, formatASTContract(stmt.Contract))
	}

	childrenPrefix := childPrefix(prefix, last)

	if stmt.StructType == nil {
		printASTLeaves(childrenPrefix, children)
		return
	}

	for _, child := range children {
		printASTLeaf(childrenPrefix, false, child)
	}

	printASTBranch(childrenPrefix, true, "Struct")
	structPrefix := childPrefix(childrenPrefix, true)
	for i, field := range stmt.StructType.Fields {
		printASTField(structPrefix, field, i == len(stmt.StructType.Fields)-1)
	}
}

func printASTEnum(stmt *ast.EnumDeclaration, prefix string, last bool) {
	printASTBranch(prefix, last, "Enum")

	children := []string{"Name: " + stmt.Name.Value}
	if stmt.UnderlyingType != nil {
		children = append(children, "Underlying: "+formatTypeRef(stmt.UnderlyingType))
	}
	if len(stmt.Values) > 0 {
		values := ""
		for i, value := range stmt.Values {
			if i > 0 {
				values += ", "
			}
			values += formatEnumValue(value)
		}
		children = append(children, "Values: "+values)
	}

	printASTLeaves(childPrefix(prefix, last), children)
}

func printASTFunction(stmt *ast.FunctionDeclaration, prefix string, last bool) {
	printASTBranch(prefix, last, "Function")
	childrenPrefix := childPrefix(prefix, last)
	printASTLeaf(childrenPrefix, false, "Name: "+stmt.Name.Value)
	printASTLeaf(childrenPrefix, false, "Parameters: "+formatParameters(stmt.Parameters))
	printASTLeaf(childrenPrefix, false, "Return: "+formatTypeRef(stmt.ReturnType))
	printASTBranch(childrenPrefix, true, "Body")
	bodyPrefix := childPrefix(childrenPrefix, true)
	for i, bodyStmt := range stmt.Body.Statements {
		printASTStatement(bodyStmt, bodyPrefix, i == len(stmt.Body.Statements)-1)
	}
}

func printASTReturn(stmt *ast.ReturnStatement, prefix string, last bool) {
	printASTBranch(prefix, last, "Return")
	if stmt.Value == nil {
		printASTLeaf(childPrefix(prefix, last), true, "Value: <nil>")
		return
	}
	printASTExpression(childPrefix(prefix, last), true, "Value", stmt.Value)
}

func printASTField(prefix string, field *ast.StructField, last bool) {
	printASTBranch(prefix, last, "Field")
	children := []string{
		"Name: " + field.Name.Value,
		"Type: " + formatTypeRef(field.Type),
	}
	if field.Contract != nil {
		children = append(children, formatASTContract(field.Contract))
	}
	printASTLeaves(childPrefix(prefix, last), children)
}

func printASTExpression(prefix string, last bool, role string, expr ast.Expression) {
	if expr == nil {
		printASTLeaf(prefix, last, role+": <nil>")
		return
	}

	switch expr := expr.(type) {
	case *ast.PrefixExpression:
		printASTBranch(prefix, last, role+": Prefix("+expr.Operator+")")
		printASTExpression(childPrefix(prefix, last), true, "Right", expr.Right)

	case *ast.InfixExpression:
		printASTBranch(prefix, last, role+": Infix("+expr.Operator+")")
		childrenPrefix := childPrefix(prefix, last)
		printASTExpression(childrenPrefix, false, "Left", expr.Left)
		printASTExpression(childrenPrefix, true, "Right", expr.Right)

	case *ast.TryExpression:
		printASTBranch(prefix, last, role+": Try")
		childrenPrefix := childPrefix(prefix, last)
		hasHandlers := len(expr.Handlers) > 0
		printASTExpression(childrenPrefix, !hasHandlers, "Expression", expr.Expression)
		for i, handler := range expr.Handlers {
			printASTTryHandler(childrenPrefix, handler, i == len(expr.Handlers)-1)
		}

	case *ast.MatchExpression:
		printASTBranch(prefix, last, role+": Match")
		childrenPrefix := childPrefix(prefix, last)
		printASTExpression(childrenPrefix, len(expr.Arms) == 0, "Subject", expr.Subject)
		for i, arm := range expr.Arms {
			printASTMatchArm(childrenPrefix, arm, i == len(expr.Arms)-1)
		}

	default:
		printASTLeaf(prefix, last, role+": "+formatASTExpression(expr))
	}
}

func printASTTryHandler(prefix string, handler *ast.TryHandler, last bool) {
	printASTBranch(prefix, last, "Handler")
	childrenPrefix := childPrefix(prefix, last)
	printASTExpression(childrenPrefix, false, "Pattern", handler.Pattern)

	switch {
	case handler.ReturnBody != nil:
		printASTReturn(handler.ReturnBody, childrenPrefix, true)
	case handler.BlockBody != nil:
		printASTLeaf(childrenPrefix, true, "Body: Block")
	case handler.Body != nil:
		printASTExpression(childrenPrefix, true, "Body", handler.Body)
	default:
		printASTLeaf(childrenPrefix, true, "Body: <nil>")
	}
}

func printASTMatchArm(prefix string, arm *ast.MatchArm, last bool) {
	printASTBranch(prefix, last, "Arm")
	childrenPrefix := childPrefix(prefix, last)
	printASTExpression(childrenPrefix, false, "Pattern", arm.Pattern)

	switch {
	case arm.ReturnBody != nil:
		printASTReturn(arm.ReturnBody, childrenPrefix, true)
	case arm.BlockBody != nil:
		printASTLeaf(childrenPrefix, true, "Body: Block")
	case arm.Body != nil:
		printASTExpression(childrenPrefix, true, "Body", arm.Body)
	default:
		printASTLeaf(childrenPrefix, true, "Body: <nil>")
	}
}

func printASTLeaves(prefix string, leaves []string) {
	for i, leaf := range leaves {
		printASTLeaf(prefix, i == len(leaves)-1, leaf)
	}
}

func printASTBranch(prefix string, last bool, label string) {
	fmt.Printf("%s%s %s\n", prefix, branch(last), label)
}

func printASTLeaf(prefix string, last bool, label string) {
	fmt.Printf("%s%s %s\n", prefix, branch(last), label)
}

func branch(last bool) string {
	if last {
		return "└─"
	}

	return "├─"
}

func childPrefix(prefix string, last bool) string {
	if last {
		return prefix + "   "
	}

	return prefix + "│  "
}

func formatASTContract(contract ast.Contract) string {
	switch contract := contract.(type) {
	case *ast.RangeContract:
		return "Range: " + formatRangeContract(contract)
	default:
		return fmt.Sprintf("Contract: %T", contract)
	}
}

func formatASTExpression(expr ast.Expression) string {
	switch expr := expr.(type) {
	case *ast.Identifier:
		return "Identifier(" + expr.Value + ")"
	case *ast.IntegerLiteral:
		return fmt.Sprintf("Int(%d)", expr.Value)
	case *ast.FloatLiteral:
		return fmt.Sprintf("Float(%g)", expr.Value)
	case *ast.StringLiteral:
		return fmt.Sprintf("String(%q)", expr.Value)
	case *ast.BooleanLiteral:
		return fmt.Sprintf("Bool(%t)", expr.Value)
	case *ast.InterpolatedStringLiteral:
		return fmt.Sprintf("InterpolatedString(%q)", expr.Value)
	case *ast.PrefixExpression:
		return "Prefix(" + expr.Operator + ")"
	case *ast.InfixExpression:
		return "Infix(" + expr.Operator + ")"
	case *ast.MemberExpression:
		return "Member(" + formatASTExpression(expr.Object) + "." + expr.Property.Value + ")"
	case *ast.CallExpression:
		return "Call(" + expr.Function.Value + ")"
	case *ast.OkExpression:
		return "Ok(" + formatASTExpression(expr.Value) + ")"
	case *ast.ErrExpression:
		return "Err(" + formatASTExpression(expr.Value) + ")"
	case *ast.TryExpression:
		return "Try(" + formatASTExpression(expr.Expression) + ")"
	case *ast.MatchExpression:
		return "Match(" + formatASTExpression(expr.Subject) + ")"
	case *ast.RangeExpression:
		return "Range(" + expr.String() + ")"
	case *ast.StructLiteral:
		return "StructLiteral(" + formatTypeRef(expr.Type) + ")"
	default:
		return fmt.Sprintf("%T", expr)
	}
}

func formatVariants(variants []*ast.Identifier) string {
	out := ""

	for i, variant := range variants {
		if i > 0 {
			out += ", "
		}
		out += variant.Value
	}

	return out
}

func printStatement(stmt ast.Statement) {
	switch stmt := stmt.(type) {
	case *ast.ModuleStatement:
		fmt.Printf("Module %s\n", stmt.Path)

	case *ast.ImportStatement:
		if stmt.Alias != "" {
			fmt.Printf("Import %s %q\n", stmt.Alias, stmt.Path)
			return
		}
		fmt.Printf("Import %q\n", stmt.Path)

	case *ast.TypeDeclStatement:
		printTypeDecl(stmt)

	case *ast.EnumDeclaration:
		printEnum(stmt)

	case *ast.FunctionDeclaration:
		printFunction(stmt)

	case *ast.StructStatement:
		fmt.Printf("Struct %s\n", stmt.Name.Value)
		printStructFields(stmt.Fields)

	case *ast.LetStatement:
		printLet(stmt)

	case *ast.LetGroupStatement:
		for _, let := range stmt.Lets {
			printLet(let)
		}

	case *ast.AssignmentStatement:
		printAssignment(stmt)

	case *ast.ExpressionStatement:
		fmt.Printf("Expression %s\n", stmt.Expression.String())

	case *ast.ReturnStatement:
		printReturn(stmt)

	case *ast.ImplStatement:
		printImpl(stmt)

	case *ast.CommentStatement:
		fmt.Printf("Comment %q\n", stmt.Text)

	case *ast.InvalidStatement:
		fmt.Printf("Invalid %q\n", stmt.TokenLiteral())

	default:
		fmt.Printf("%T %q\n", stmt, stmt.TokenLiteral())
	}
}

func printTypeDecl(stmt *ast.TypeDeclStatement) {
	fmt.Printf("Type %s", stmt.Name.Value)

	switch {
	case stmt.BaseType != nil:
		fmt.Printf(" %s", formatTypeRef(stmt.BaseType))
	case stmt.AssignedType != nil:
		fmt.Printf(" = %s", formatTypeRef(stmt.AssignedType))
	case len(stmt.Variants) > 0:
		fmt.Print(" =")
		for _, variant := range stmt.Variants {
			fmt.Printf(" %s", variant.Value)
		}
	case stmt.StructType != nil:
		fmt.Print(" struct")
	}

	if stmt.Contract != nil {
		fmt.Printf(" %s", formatContract(stmt.Contract))
	}

	fmt.Println()

	if stmt.StructType != nil {
		printStructFields(stmt.StructType.Fields)
	}
}

func printEnum(stmt *ast.EnumDeclaration) {
	fmt.Printf("Enum %s", stmt.Name.Value)
	if stmt.UnderlyingType != nil {
		fmt.Printf(" %s", formatTypeRef(stmt.UnderlyingType))
	}
	fmt.Println()
	for _, value := range stmt.Values {
		fmt.Printf("  Value %s\n", formatEnumValue(value))
	}
}

func printFunction(stmt *ast.FunctionDeclaration) {
	fmt.Printf("Function %s(%s) %s\n", stmt.Name.Value, formatParameters(stmt.Parameters), formatTypeRef(stmt.ReturnType))
	for _, bodyStmt := range stmt.Body.Statements {
		fmt.Print("  ")
		printStatement(bodyStmt)
	}
}

func printLet(stmt *ast.LetStatement) {
	fmt.Print("Let ")

	if stmt.Mutable {
		fmt.Print("mut ")
	}

	fmt.Print(stmt.Name.Value)

	if stmt.Type != nil {
		fmt.Printf(": %s", formatTypeRef(stmt.Type))
	}

	if stmt.Value != nil {
		fmt.Printf(" := %s", stmt.Value.String())
	}

	fmt.Println()
}

func printAssignment(stmt *ast.AssignmentStatement) {
	fmt.Printf("Assignment %s %s %s\n", stmt.Target.String(), stmt.Operator, stmt.Value.String())
}

func printReturn(stmt *ast.ReturnStatement) {
	if stmt.Value == nil {
		fmt.Println("Return")
		return
	}
	fmt.Printf("Return %s\n", stmt.Value.String())
}

func printImpl(stmt *ast.ImplStatement) {
	fmt.Printf("Impl %s\n", formatTypeRef(stmt.Target))
	for _, member := range stmt.Members {
		switch member := member.(type) {
		case *ast.TypeDeclStatement:
			fmt.Print("  ")
			printTypeDecl(member)
		case *ast.EnumDeclaration:
			fmt.Printf("  Enum %s", member.Name.Value)
			if member.UnderlyingType != nil {
				fmt.Printf(" %s", formatTypeRef(member.UnderlyingType))
			}
			fmt.Println()
			for _, value := range member.Values {
				fmt.Printf("    Value %s\n", formatEnumValue(value))
			}
		case *ast.PropertyDeclaration:
			fmt.Printf("  Property %s: %s\n", member.Name.Value, formatTypeRef(member.Type))
		case *ast.FunctionDeclaration:
			fmt.Print("  ")
			printFunction(member)
		default:
			fmt.Printf("  %T %q\n", member, member.TokenLiteral())
		}
	}
}

func formatEnumValue(value *ast.EnumValue) string {
	out := value.Name.Value
	if value.Initializer != nil {
		out += " = " + value.Initializer.String()
	}
	return out
}

func formatParameters(parameters []*ast.Parameter) string {
	out := ""
	for i, param := range parameters {
		if i > 0 {
			out += ", "
		}
		out += param.Name.Value + ": " + formatTypeRef(param.Type)
	}
	return out
}

func printStructFields(fields []*ast.StructField) {
	for _, field := range fields {
		fmt.Printf("  Field %s %s\n", field.Name.Value, formatTypeRef(field.Type))
	}
}

func formatTypeRef(ref *ast.TypeReference) string {
	if ref == nil {
		return "<nil>"
	}

	if ref.ElementType != nil {
		return "[]" + formatTypeRef(ref.ElementType)
	}

	out := ref.Name

	if ref.Unit != "" {
		out += "<" + ref.Unit + ">"
	}

	if len(ref.TypeArgs) > 0 {
		out += "["
		for i, arg := range ref.TypeArgs {
			if i > 0 {
				out += ", "
			}
			out += formatTypeRef(arg)
		}
		out += "]"
	}

	return out
}

func formatContract(contract ast.Contract) string {
	switch contract := contract.(type) {
	case *ast.RangeContract:
		return "range " + formatRangeContract(contract)
	default:
		return fmt.Sprintf("%T", contract)
	}
}

func formatRangeContract(contract *ast.RangeContract) string {
	operator := ".."
	if contract.Exclusive {
		operator = "..<"
	}

	return formatRangeBound(contract.Min) + operator + formatRangeBound(contract.Max)
}

func formatRangeBound(expr ast.Expression) string {
	if expr == nil {
		return ""
	}

	if prefix, ok := expr.(*ast.PrefixExpression); ok {
		if prefix.Operator == "-" && prefix.Right != nil {
			return "-" + formatExpression(prefix.Right)
		}
	}

	return formatExpression(expr)
}

func formatExpression(expr ast.Expression) string {
	if expr == nil {
		return "<nil>"
	}

	return expr.String()
}
