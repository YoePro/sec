package sema

import (
	"strings"

	"sec/internal/ast"
	"sec/internal/diagnostics"
)

// implementedAttributes names the compiler-known attributes whose meaning Sema
// applies. The parser accepts the whole closed attribute set; an attribute
// outside this list would otherwise be silently ignored, which for a
// verified guarantee or a selection attribute would change what the program
// means, so it is rejected.
var implementedAttributes = map[string]bool{
	"address": true, "noCopy": true, "noAlloc": true, "noPanic": true, "noBlock": true, "link_name": true,
}

// validateAttributeSemantics rejects attached attributes whose semantics are
// not implemented yet (@target, @when, @interrupt, @isr, @interruptSafe).
//
// Rules:
//   - rules/foundations/attributes.md — "Verified guarantee attributes": the compiler must verify the declared guarantee
//   - rules/foundations/attributes.md — "Selection attributes", "Target-binding attributes"
func (a *Analyzer) validateAttributeSemantics(program *ast.Program) {
	if program == nil {
		return
	}
	check := func(attributes []*ast.Attribute) {
		for _, attribute := range attributes {
			if attribute == nil || attribute.Name == nil {
				continue
			}
			a.validateAttributeArgumentValues(attribute)
			if !implementedAttributes[attribute.Name.Value] {
				a.addErrorAtToken(attribute.Token, "attribute @%s is not implemented yet", attribute.Name.Value)
			}
		}
	}
	for _, statement := range program.Statements {
		switch statement := statement.(type) {
		case *ast.FunctionDeclaration:
			if statement != nil {
				check(statement.Attributes)
			}
		case *ast.TypeDeclStatement:
			if statement != nil {
				check(statement.Attributes)
			}
		case *ast.EnumDeclaration:
			if statement != nil {
				check(statement.Attributes)
			}
		case *ast.LetStatement:
			if statement != nil {
				check(statement.Attributes)
			}
		case *ast.ImplStatement:
			if statement == nil {
				continue
			}
			for _, member := range statement.Members {
				if function, ok := member.(*ast.FunctionDeclaration); ok && function != nil {
					check(function.Attributes)
				}
			}
		}
	}
}

// Guarantee names one verified guarantee that a declaration can carry.
type Guarantee string

const (
	GuaranteeNoPanic       Guarantee = "noPanic"
	GuaranteeNoAlloc       Guarantee = "noAlloc"
	GuaranteeNoBlock       Guarantee = "noBlock"
	GuaranteeISR           Guarantee = "isr"
	GuaranteeInterruptSafe Guarantee = "interruptSafe"
)

// attributeImplications is the initial implication graph. @isr additionally
// requires an interrupt-safe call graph, which is a verification obligation
// of the ISR analysis rather than a guarantee attribute.
//
// Rules:
//   - rules/foundations/attributes.md — "Attribute implications", "ISR verification", "Interrupt-safe guarantee"
var attributeImplications = map[string][]Guarantee{
	"interrupt":     {GuaranteeISR},
	"isr":           {GuaranteeNoPanic, GuaranteeNoAlloc, GuaranteeNoBlock},
	"interruptSafe": {GuaranteeNoPanic, GuaranteeNoAlloc, GuaranteeNoBlock},
}

// EffectiveGuarantee is one guarantee in force on a declaration. Source is
// the explicitly written attribute that introduces it: the guarantee's own
// attribute when written, otherwise the attribute the implication starts
// from. ImpliedBy is the implication chain from Source, empty when explicit.
// Implied guarantees are recorded here, never as synthesized AST attributes.
type EffectiveGuarantee struct {
	Guarantee Guarantee
	Source    *ast.Attribute
	ImpliedBy []string
}

// Explicit reports whether the guarantee was written on the declaration.
func (g EffectiveGuarantee) Explicit() bool { return len(g.ImpliedBy) == 0 }

func (g EffectiveGuarantee) impliedSuffix() string {
	if g.Explicit() {
		return ""
	}
	return " (implied by @" + strings.Join(g.ImpliedBy, " via @") + ")"
}

// EffectiveGuarantees closes the explicitly written attributes over the
// implication graph. Explicit guarantees come first in source order; each
// implied guarantee appears once, with the shortest chain from the first
// attribute that implies it, and an explicit guarantee is never replaced by
// an implied one.
//
// Rules:
//   - rules/foundations/attributes.md — "Attribute implications", "Redundant attributes"
func EffectiveGuarantees(attributes []*ast.Attribute) []EffectiveGuarantee {
	result := []EffectiveGuarantee{}
	seen := map[Guarantee]bool{}
	type pending struct {
		name   string
		source *ast.Attribute
		chain  []string
	}
	queue := []pending{}
	for _, attribute := range attributes {
		if attribute == nil || attribute.Name == nil {
			continue
		}
		name := attribute.Name.Value
		if guarantee := Guarantee(name); isGuaranteeName(guarantee) && !seen[guarantee] {
			seen[guarantee] = true
			result = append(result, EffectiveGuarantee{Guarantee: guarantee, Source: attribute})
		}
		queue = append(queue, pending{name: name, source: attribute})
	}
	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]
		for _, implied := range attributeImplications[current.name] {
			chain := append(append([]string(nil), current.chain...), current.name)
			if !seen[implied] {
				seen[implied] = true
				result = append(result, EffectiveGuarantee{Guarantee: implied, Source: current.source, ImpliedBy: chain})
			}
			queue = append(queue, pending{name: string(implied), source: current.source, chain: chain})
		}
	}
	return result
}

// EffectiveGuaranteeOf returns the effective guarantee g of a declaration,
// preferring the explicitly written attribute.
func EffectiveGuaranteeOf(attributes []*ast.Attribute, g Guarantee) (EffectiveGuarantee, bool) {
	for _, guarantee := range EffectiveGuarantees(attributes) {
		if guarantee.Guarantee == g {
			return guarantee, true
		}
	}
	return EffectiveGuarantee{}, false
}

func isGuaranteeName(g Guarantee) bool {
	switch g {
	case GuaranteeNoPanic, GuaranteeNoAlloc, GuaranteeNoBlock, GuaranteeISR, GuaranteeInterruptSafe:
		return true
	}
	return false
}

// validateAttributeArgumentValues requires every attribute argument to be
// known while the compilation plan is built and then checks the value form
// its attribute defines: string-literal target selectors, an integer or
// named constant interrupt vector, and the restricted @when condition
// language. @address values keep their dedicated register-binding check and
// @link_name its string-literal check in the parser.
//
// Rules:
//   - rules/foundations/attributes.md — "Compile-time arguments", "Attribute validation order", "Selector values",
//     "Interrupt vector argument", "@when condition language"
func (a *Analyzer) validateAttributeArgumentValues(attribute *ast.Attribute) {
	name := attribute.Name.Value
	if name == "address" {
		return
	}
	for _, argument := range attribute.Arguments {
		if argument == nil || argument.Value == nil {
			continue
		}
		if offending, what := planTimeViolation(argument.Value); offending != nil {
			token := expressionToken(offending)
			a.addErrorAtTokenWithMetadata(token, diagnostics.AttributeArgumentNotCompileTime,
				"Attribute arguments are evaluated while the compilation is planned, before any code runs. Use a literal, a configuration parameter, or a named compile-time constant.",
				"argument of @%s must be known while the compilation is planned; %s needs run-time execution", name, what)
			continue
		}
		invalid := func(help string, format string, args ...any) {
			a.addErrorAtTokenWithMetadata(argument.Token, diagnostics.AttributeInvalidArgument, help, format, args...)
		}
		switch name {
		case "target":
			if _, ok := argument.Value.(*ast.StringLiteral); !ok {
				invalid("Write the selector value as a string literal, for example os: \"linux\".", "@target selector values must be string literals")
			}
		case "interrupt":
			if !integerOrNamedConstant(argument.Value) {
				invalid("Name the vector, for example vector: Interrupt.Timer0, or give its number.", "@interrupt vector must be an integer vector literal or a named interrupt vector")
			}
		case "when":
			if !whenCondition(argument.Value) {
				invalid("Target identity belongs in @target; @when combines boolean config.<name> parameters only.", "@when conditions use config.<name>, true, false, !, &&, ||, ==, and != only")
			}
		}
	}
}

// planTimeViolation returns the first subexpression that would need run-time
// execution, allocation, or calls, with a description; literals, names,
// member chains, and operators over them are plan-time forms.
func planTimeViolation(expr ast.Expression) (ast.Expression, string) {
	switch expr := expr.(type) {
	case *ast.IntegerLiteral, *ast.FloatLiteral, *ast.StringLiteral, *ast.BooleanLiteral, *ast.CharLiteral, *ast.Identifier:
		return nil, ""
	case *ast.MemberExpression:
		return planTimeViolation(expr.Object)
	case *ast.PrefixExpression:
		return planTimeViolation(expr.Right)
	case *ast.InfixExpression:
		if offending, what := planTimeViolation(expr.Left); offending != nil {
			return offending, what
		}
		return planTimeViolation(expr.Right)
	case *ast.CallExpression:
		return expr, "a function call"
	case *ast.AwaitExpression:
		return expr, "await"
	case *ast.TryExpression:
		return expr, "try"
	case *ast.LambdaExpression:
		return expr, "a lambda"
	case *ast.InterpolatedStringLiteral:
		return expr, "string interpolation"
	case *ast.IndexExpression:
		return expr, "indexing"
	}
	return expr, "this expression"
}

func integerOrNamedConstant(expr ast.Expression) bool {
	switch expr := expr.(type) {
	case *ast.IntegerLiteral, *ast.Identifier:
		return true
	case *ast.MemberExpression:
		return expr.Property != nil && integerOrNamedConstant(expr.Object)
	}
	return false
}

func whenCondition(expr ast.Expression) bool {
	switch expr := expr.(type) {
	case *ast.BooleanLiteral:
		return true
	case *ast.MemberExpression:
		root, ok := expr.Object.(*ast.Identifier)
		return ok && root.Value == "config" && expr.Property != nil
	case *ast.PrefixExpression:
		return expr.Operator == "!" && whenCondition(expr.Right)
	case *ast.InfixExpression:
		switch expr.Operator {
		case "&&", "||", "==", "!=":
			return whenCondition(expr.Left) && whenCondition(expr.Right)
		}
	}
	return false
}

// AttributeImplemented reports whether Sema applies the meaning of a
// compiler-known attribute; tooling uses it to describe an attribute's status.
func AttributeImplemented(name string) bool { return implementedAttributes[name] }
