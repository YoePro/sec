package sema

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"sec/internal/ast"
	"sec/internal/diagnostics"
	"sec/internal/lexer"
)

// SourceSelection is the part of a concrete CompilationPlan that selects
// active source (attributes.md §§ 13–28): the plan's target identity by
// selector name (os, arch, cpu, device, board; a selector the plan does not
// define never matches) and the typed compile-time parameters `config.<name>`.
// Normalize, when set, canonicalizes a selector value before comparison.
type SourceSelection struct {
	Target     map[string]string
	Parameters map[string]CompileTimeParameter
	Normalize  func(selector, value string) string
}

// CompileTimeParameter is one typed project parameter (projects.md § 24):
// Kind is "bool", "integer", "float" or "string"; Text is the canonical value.
type CompileTimeParameter struct {
	Kind string
	Bool bool
	Text string
}

// SetSourceSelection configures the plan facts that select active source.
// Without it every declaration is active, as before source selection.
func (a *Analyzer) SetSourceSelection(selection SourceSelection) {
	a.recordConfiguration(func(b *Analyzer) { b.SetSourceSelection(selection) })
	a.sourceSelection = &selection
}

// ActiveProgram is the program this analysis analyzed after source
// selection: excluded declarations are absent, so Semantic IR and lowering
// built from it never see them. Tooling keeps the complete parsed program.
func (a *Analyzer) ActiveProgram() *ast.Program {
	if a == nil {
		return nil
	}
	return a.activeProgram
}

// ExcludedDeclarations are the top-level statements source selection
// removed from this analysis, parsed but inactive (attributes.md § 13(2)).
func (a *Analyzer) ExcludedDeclarations() []ast.Statement {
	if a == nil {
		return nil
	}
	return append([]ast.Statement(nil), a.excludedStatements...)
}

// selectActiveSource applies file-level and statement-level `@target` and
// statement-level `@when` (attributes.md § 13 steps 2–5) and returns the
// active program with the selection diagnostics.
// Rules: rules/foundations/attributes.md — §§ 13, 15–17, 21–28; rules/projects/projects.md — § 24.
func (a *Analyzer) selectActiveSource(program *ast.Program) (*ast.Program, []Error) {
	a.excludedStatements = nil
	a.fileTargets = map[string][]ast.TargetSelector{}
	if program == nil || a.sourceSelection == nil {
		return program, nil
	}
	selection := *a.sourceSelection
	errors := []Error{}
	fileActive := map[string]bool{}
	for _, statement := range program.Statements {
		directive, ok := statement.(*ast.TargetDirective)
		// The #target compatibility form keeps its existing module-assembly
		// and validation paths; the canonical form selects here.
		if !ok || !directive.Canonical {
			continue
		}
		file := directive.Token.File
		if _, decided := fileActive[file]; !decided {
			fileActive[file] = selection.targetMatches(directive.Selectors)
			a.fileTargets[file] = directive.Selectors
		}
	}
	active := make([]ast.Statement, 0, len(program.Statements))
	variants := []selectionVariant{}
	for _, statement := range program.Statements {
		token := selectionStatementToken(statement)
		if fileOK, decided := fileActive[token.File]; decided && !fileOK {
			if _, directive := statement.(*ast.TargetDirective); !directive {
				a.excludedStatements = append(a.excludedStatements, statement)
			}
			continue
		}
		attributes := ast.StatementAttributes(statement)
		selected := true
		var target []ast.TargetSelector
		hasSelection := false
		for _, attribute := range attributes {
			if attribute == nil || attribute.Name == nil {
				continue
			}
			switch attribute.Name.Value {
			case "target":
				hasSelection = true
				target = attributeSelectors(attribute)
				selected = selected && selection.targetMatches(target)
			case "when":
				hasSelection = true
				if len(attribute.Arguments) != 1 || attribute.Arguments[0] == nil {
					continue
				}
				value, problem := selection.whenValue(attribute.Arguments[0].Value)
				if problem != nil {
					errors = append(errors, *problem)
					selected = false
					continue
				}
				selected = selected && value
			}
		}
		if hasSelection {
			if name, kind, ok := selectionIdentity(statement); ok {
				variants = append(variants, selectionVariant{statement: statement, name: name, kind: kind, target: target, active: selected})
			}
		}
		if !selected {
			a.excludedStatements = append(a.excludedStatements, statement)
			continue
		}
		active = append(active, statement)
	}
	shapeErrors, duplicates := checkSelectionVariants(variants, selection)
	errors = append(errors, shapeErrors...)
	if len(duplicates) > 0 {
		filtered := active[:0:0]
		for _, statement := range active {
			if !duplicates[statement] {
				filtered = append(filtered, statement)
			}
		}
		active = filtered
	}
	selected := *program
	selected.Statements = active
	return &selected, errors
}

// targetMatches applies logical AND over the supplied selectors; a selector
// the plan does not define does not match (attributes.md § 17(4)–(6)).
func (selection SourceSelection) targetMatches(selectors []ast.TargetSelector) bool {
	for _, selector := range selectors {
		planValue, defined := selection.Target[selector.Name]
		if !defined {
			return false
		}
		value := selector.Value
		if selection.Normalize != nil {
			value = selection.Normalize(selector.Name, value)
			planValue = selection.Normalize(selector.Name, planValue)
		}
		if value != planValue {
			return false
		}
	}
	return true
}

func attributeSelectors(attribute *ast.Attribute) []ast.TargetSelector {
	selectors := []ast.TargetSelector{}
	for _, argument := range attribute.Arguments {
		if argument == nil || argument.Name == nil {
			continue
		}
		if literal, ok := argument.Value.(*ast.StringLiteral); ok {
			selectors = append(selectors, ast.TargetSelector{Token: argument.Token, Name: argument.Name.Value, Value: literal.Value})
		}
	}
	return selectors
}

// selectionValue is a typed value of the @when condition language.
type selectionValue struct {
	kind string
	text string
}

// whenValue evaluates a restricted @when condition (attributes.md § 28):
// boolean `config.<name>` parameters, true, false, !, &&, ||, == and !=.
// It runs while the compilation is planned and never calls user code.
func (selection SourceSelection) whenValue(expr ast.Expression) (bool, *Error) {
	value, problem := selection.whenOperand(expr)
	if problem != nil {
		return false, problem
	}
	if value.kind != "bool" {
		e := selectionError(expressionToken(expr), diagnostics.UnknownConfigParameter,
			"An @when condition is boolean: use a bool parameter or compare a parameter with ==/!=.",
			"@when condition %s is %s, not bool", expr.String(), value.kind)
		return false, &e
	}
	return value.text == "true", nil
}

func (selection SourceSelection) whenOperand(expr ast.Expression) (selectionValue, *Error) {
	switch expr := expr.(type) {
	case *ast.BooleanLiteral:
		return selectionValue{kind: "bool", text: fmt.Sprint(expr.Value)}, nil
	case *ast.IntegerLiteral:
		return selectionValue{kind: "integer", text: expr.Token.Lexeme}, nil
	case *ast.StringLiteral:
		return selectionValue{kind: "string", text: expr.Value}, nil
	case *ast.MemberExpression:
		object, ok := expr.Object.(*ast.Identifier)
		if ok && object.Value == "config" && expr.Property != nil {
			parameter, defined := selection.Parameters[expr.Property.Value]
			if !defined {
				known := make([]string, 0, len(selection.Parameters))
				for name := range selection.Parameters {
					known = append(known, name)
				}
				sort.Strings(known)
				help := "Declare it under [parameters] in the project manifest."
				if len(known) > 0 {
					help += " Declared parameters: " + strings.Join(known, ", ") + "."
				}
				e := selectionError(expr.Property.Token, diagnostics.UnknownConfigParameter, help,
					"unknown compile-time parameter config.%s", expr.Property.Value)
				return selectionValue{}, &e
			}
			if parameter.Kind == "bool" {
				return selectionValue{kind: "bool", text: fmt.Sprint(parameter.Bool)}, nil
			}
			return selectionValue{kind: parameter.Kind, text: parameter.Text}, nil
		}
	case *ast.PrefixExpression:
		if expr.Operator == "!" {
			value, problem := selection.whenValue(expr.Right)
			if problem != nil {
				return selectionValue{}, problem
			}
			return selectionValue{kind: "bool", text: fmt.Sprint(!value)}, nil
		}
	case *ast.InfixExpression:
		switch expr.Operator {
		case "&&", "||":
			left, problem := selection.whenValue(expr.Left)
			if problem != nil {
				return selectionValue{}, problem
			}
			right, problem := selection.whenValue(expr.Right)
			if problem != nil {
				return selectionValue{}, problem
			}
			result := left && right
			if expr.Operator == "||" {
				result = left || right
			}
			return selectionValue{kind: "bool", text: fmt.Sprint(result)}, nil
		case "==", "!=":
			left, problem := selection.whenOperand(expr.Left)
			if problem != nil {
				return selectionValue{}, problem
			}
			right, problem := selection.whenOperand(expr.Right)
			if problem != nil {
				return selectionValue{}, problem
			}
			if left.kind != right.kind {
				e := selectionError(expr.Token, diagnostics.UnknownConfigParameter, "Compare a parameter with a literal of its own type.",
					"@when compares %s with %s", left.kind, right.kind)
				return selectionValue{}, &e
			}
			equal := left.text == right.text
			if expr.Operator == "!=" {
				equal = !equal
			}
			return selectionValue{kind: "bool", text: fmt.Sprint(equal)}, nil
		}
	}
	// The plan-time evaluator never calls user code: selecting the active
	// source graph cannot depend on that graph's semantics (§ 5(3)–(4)).
	if offending, what := planTimeViolation(expr); offending != nil {
		if _, call := offending.(*ast.CallExpression); call || what != "this expression" {
			e := selectionError(expressionToken(offending), diagnostics.AttributeArgumentNotCompileTime,
				"Attribute arguments are evaluated while the compilation is planned, before any code runs. Use a literal, a configuration parameter, or a named compile-time constant.",
				"argument of @when must be known while the compilation is planned; %s needs run-time execution", what)
			return selectionValue{}, &e
		}
	}
	e := selectionError(expressionToken(expr), diagnostics.AttributeInvalidArgument,
		"Target identity belongs in @target; @when combines config.<name> parameters only.",
		"@when conditions use config.<name>, true, false, !, &&, ||, ==, and != only")
	return selectionValue{}, &e
}

// selectionVariant is one top-level declaration carrying selection
// attributes, with its logical identity.
type selectionVariant struct {
	statement ast.Statement
	name      string
	kind      string
	target    []ast.TargetSelector
	active    bool
}

// selectionIdentity names the logical symbol a selectable declaration
// defines: functions by name and impl target, types, variables and
// interfaces by name.
func selectionIdentity(statement ast.Statement) (string, string, bool) {
	switch statement := statement.(type) {
	case *ast.FunctionDeclaration:
		if statement.Name == nil {
			return "", "", false
		}
		return statement.Name.Value, "function", true
	case *ast.LetStatement:
		if statement.Name == nil {
			return "", "", false
		}
		return statement.Name.Value, "variable", true
	case *ast.TypeDeclStatement:
		if statement.Name == nil {
			return "", "", false
		}
		return statement.Name.Value, "type", true
	case *ast.EnumDeclaration:
		if statement.Name == nil {
			return "", "", false
		}
		return statement.Name.Value, "type", true
	case *ast.InterfaceDeclaration:
		if statement.Name == nil {
			return "", "", false
		}
		return statement.Name.Value, "type", true
	}
	return "", "", false
}

// exclusiveTargets reports whether two @target selections can never both
// match one plan: some selector both name has different values.
func exclusiveTargets(left, right []ast.TargetSelector) bool {
	for _, l := range left {
		for _, r := range right {
			if l.Name == r.Name && l.Value != r.Value {
				return true
			}
		}
	}
	return false
}

// checkSelectionVariants validates declaration variants (attributes.md
// §§ 21–23): mutually exclusive variants of one symbol must have the same
// public shape, and two variants active in this plan with the same shape
// are an overlap rejected rather than resolved by specificity. It returns
// the later overlapping declarations, which are removed so ordinary
// duplicate checks do not report them again.
func checkSelectionVariants(variants []selectionVariant, selection SourceSelection) ([]Error, map[ast.Statement]bool) {
	errors := []Error{}
	duplicates := map[ast.Statement]bool{}
	for i := 0; i < len(variants); i++ {
		for j := i + 1; j < len(variants); j++ {
			left, right := variants[i], variants[j]
			if left.name != right.name || left.kind != right.kind {
				continue
			}
			leftShape, rightShape := declarationShape(left.statement), declarationShape(right.statement)
			if exclusiveTargets(left.target, right.target) {
				if leftShape != rightShape {
					e := selectionError(selectionStatementToken(right.statement), diagnostics.IncompatibleVariantShape,
						"Target variants of one symbol keep one public shape; only bodies, addresses and private details may differ.",
						"target variant of `%s` has public shape %s, but the variant at %d:%d has %s", right.name, rightShape,
						selectionStatementToken(left.statement).Line, selectionStatementToken(left.statement).Column, leftShape)
					attachPrevious(&e, selectionStatementToken(left.statement))
					errors = append(errors, e)
				}
				continue
			}
			if left.active && right.active && (left.kind != "function" || leftShape == rightShape) && !duplicates[right.statement] {
				plan := []string{}
				for _, name := range []string{"os", "arch", "cpu", "device", "board"} {
					if value, ok := selection.Target[name]; ok {
						plan = append(plan, name+" = "+value)
					}
				}
				e := selectionError(selectionStatementToken(right.statement), diagnostics.OverlappingTargetVariants,
					"Make the variants' selectors mutually exclusive; the compiler never picks the more specific one.",
					"multiple active target variants define `%s` for this compilation plan (%s)", right.name, strings.Join(plan, ", "))
				attachPrevious(&e, selectionStatementToken(left.statement))
				errors = append(errors, e)
				duplicates[right.statement] = true
			}
		}
	}
	return errors, duplicates
}

// declarationShape renders the public shape of a declaration variant
// (attributes.md § 23(2)–(4)).
func declarationShape(statement ast.Statement) string {
	switch statement := statement.(type) {
	case *ast.FunctionDeclaration:
		parameters := []string{}
		for _, parameter := range statement.Parameters {
			if parameter != nil && parameter.Type != nil {
				parameters = append(parameters, typeReferenceDisplayName(parameter.Type))
			}
		}
		result := "void"
		if statement.ReturnType != nil {
			result = typeReferenceDisplayName(statement.ReturnType)
		}
		return fmt.Sprintf("fn[%d](%s) %s", len(statement.GenericParameters), strings.Join(parameters, ", "), result)
	case *ast.LetStatement:
		declared := "inferred"
		if statement.Type != nil {
			declared = typeReferenceDisplayName(statement.Type)
		}
		mutable := ""
		if statement.Mutable {
			mutable = "mut "
		}
		return "let " + mutable + declared
	}
	return fmt.Sprintf("%T", statement)
}

func attachPrevious(e *Error, previous lexer.Token) {
	e.PreviousFile, e.PreviousLine, e.PreviousColumn = previous.File, previous.Line, previous.Column
	e.RelatedLabel = "other variant"
}

func selectionError(token lexer.Token, id, help, format string, args ...any) Error {
	endLine, endColumn := token.EndPosition()
	return Error{ID: id, Severity: diagnostics.SeverityError, Help: help, Message: fmt.Sprintf(format, args...),
		File: token.File, Line: token.Line, Column: token.Column, EndLine: endLine, EndColumn: endColumn}
}

// selectionStatementToken locates a top-level statement.
func selectionStatementToken(statement ast.Statement) lexer.Token {
	switch statement := statement.(type) {
	case *ast.FunctionDeclaration:
		if statement.Name != nil {
			return statement.Name.Token
		}
	case *ast.LetStatement:
		if statement.Name != nil {
			return statement.Name.Token
		}
	case *ast.TypeDeclStatement:
		if statement.Name != nil {
			return statement.Name.Token
		}
	}
	return statementToken(statement)
}

// explainUnavailableVariants turns an unresolved use of a symbol that exists
// only in excluded variants into a target-aware diagnostic naming the plan
// and the variants that would provide it (attributes.md § 24).
// Rules: rules/foundations/attributes.md — § 24 "No active variant".
func (a *Analyzer) explainUnavailableVariants() {
	if a.sourceSelection == nil || len(a.excludedStatements) == 0 {
		return
	}
	variants := map[string][]string{}
	for _, statement := range a.excludedStatements {
		name, _, ok := selectionIdentity(statement)
		if !ok {
			continue
		}
		variants[name] = append(variants[name], a.variantCondition(statement))
	}
	if len(variants) == 0 {
		return
	}
	plan := []string{}
	for _, name := range []string{"os", "arch", "cpu", "device", "board"} {
		if value, ok := a.sourceSelection.Target[name]; ok {
			plan = append(plan, name+" = "+value)
		}
	}
	for index := range a.errors {
		message := a.errors[index].Message
		name := ""
		for _, prefix := range []string{"undefined variable ", "unknown function or type ", "unknown type "} {
			if strings.HasPrefix(message, prefix) {
				name = strings.TrimPrefix(message, prefix)
			}
		}
		available, excluded := variants[name]
		if name == "" || !excluded {
			continue
		}
		sort.Strings(available)
		a.errors[index].ID = diagnostics.NoActiveVariant
		a.errors[index].Message = fmt.Sprintf("`%s` is unavailable for this compilation plan (%s); available variants: %s", name, strings.Join(plan, ", "), strings.Join(available, "; "))
		a.errors[index].Help = "Select a plan that one of the variants matches, or add a variant for this plan."
	}
}

// variantCondition renders the selection that excluded a declaration: its
// file-level target, its @target selectors and its @when condition.
func (a *Analyzer) variantCondition(statement ast.Statement) string {
	parts := []string{}
	file := selectionStatementToken(statement).File
	for _, candidate := range a.fileTargets[file] {
		parts = append(parts, candidate.Name+" = "+candidate.Value)
	}
	for _, attribute := range ast.StatementAttributes(statement) {
		if attribute == nil || attribute.Name == nil {
			continue
		}
		switch attribute.Name.Value {
		case "target":
			for _, selector := range attributeSelectors(attribute) {
				parts = append(parts, selector.Name+" = "+strconv.Quote(selector.Value))
			}
		case "when":
			if len(attribute.Arguments) == 1 && attribute.Arguments[0] != nil {
				parts = append(parts, "when "+attribute.Arguments[0].Value.String())
			}
		}
	}
	if len(parts) == 0 {
		return "unconditional"
	}
	return strings.Join(parts, ", ")
}
