package sema

import (
	"sec/internal/ast"
)

// CompileTimeContextClass is the compile-time evaluator class of a required
// source position (compile_time_evaluation.md § 4).
type CompileTimeContextClass string

const (
	// PlanTimeRequiredContext positions are resolved while the compilation
	// is planned, before the active source graph exists, by the restricted
	// plan-time evaluator (§ 5).
	PlanTimeRequiredContext CompileTimeContextClass = "PlanTimeRequiredContext"
	// SemanticCompileTimeRequiredContext positions are established by
	// semantic CTE once typed semantics exist (§ 6).
	SemanticCompileTimeRequiredContext CompileTimeContextClass = "SemanticCompileTimeRequiredContext"
)

// CompileTimeContext classifies one required position: its class and the
// owning construct (§ 8(2)).
type CompileTimeContext struct {
	Class     CompileTimeContextClass
	Construct string
}

// CompileTimeContextOf returns the classification of a compile-time-required
// source position of the analyzed program.
// Rules: rules/compiler/compile_time_evaluation.md — §§ 4, 5, 8, 9.
func (a *Analyzer) CompileTimeContextOf(expr ast.Expression) (CompileTimeContext, bool) {
	if a == nil || expr == nil {
		return CompileTimeContext{}, false
	}
	context, ok := a.compileTimeContexts[expr]
	return context, ok
}

// classifyCompileTimeContexts records the class of every compile-time-required
// position of the complete parsed program, including excluded source, from
// the owning language rules (§ 8): attribute arguments, @target selectors and
// @when conditions are plan-time (attributes.md §§ 13, 28); type-contract
// arguments, explicit type defaults, binding and field contracts and static
// initializers are semantic CTE positions (§§ 8(5), 9; static.md § 15).
func (a *Analyzer) classifyCompileTimeContexts(program *ast.Program) {
	a.compileTimeContexts = map[ast.Expression]CompileTimeContext{}
	if program == nil {
		return
	}
	record := func(expr ast.Expression, class CompileTimeContextClass, construct string) {
		if expr != nil {
			a.compileTimeContexts[expr] = CompileTimeContext{Class: class, Construct: construct}
		}
	}
	attributes := func(list []*ast.Attribute) {
		for _, attribute := range list {
			if attribute == nil || attribute.Name == nil {
				continue
			}
			for _, argument := range attribute.Arguments {
				if argument == nil {
					continue
				}
				construct := "@" + attribute.Name.Value + " argument"
				switch {
				case attribute.Name.Value == "target" && argument.Name != nil:
					construct = "@target " + argument.Name.Value + " selector"
				case attribute.Name.Value == "when":
					construct = "@when condition"
				case argument.Name != nil:
					construct = "@" + attribute.Name.Value + " " + argument.Name.Value + " argument"
				}
				record(argument.Value, PlanTimeRequiredContext, construct)
			}
		}
	}
	var contract func(ast.Contract)
	contract = func(node ast.Contract) {
		switch node := node.(type) {
		case *ast.ContractList:
			for _, inner := range node.Contracts {
				contract(inner)
			}
		case *ast.RangeContract:
			record(node.Min, SemanticCompileTimeRequiredContext, "range lower bound")
			record(node.Max, SemanticCompileTimeRequiredContext, "range upper bound")
		case *ast.MembershipContract:
			for _, value := range node.Values {
				record(value, SemanticCompileTimeRequiredContext, "in contract member")
			}
		case *ast.MarkerContract:
			construct := node.Name + " value"
			if node.Name == "multipleOf" {
				construct = "multipleOf divisor"
			}
			record(node.Value, SemanticCompileTimeRequiredContext, construct)
		case *ast.RegexContract:
			record(node.Pattern, SemanticCompileTimeRequiredContext, "regex pattern")
		}
	}
	var statements func([]ast.Statement, bool)
	block := func(body *ast.BlockStatement) {
		if body != nil {
			statements(body.Statements, false)
		}
	}
	statements = func(list []ast.Statement, topLevel bool) {
		for _, statement := range list {
			if topLevel {
				attributes(ast.StatementAttributes(statement))
			}
			switch statement := statement.(type) {
			case *ast.TargetDirective:
				if statement != nil && statement.Attribute != nil {
					attributes([]*ast.Attribute{statement.Attribute})
				}
			case *ast.TypeDeclStatement:
				if statement == nil {
					continue
				}
				contract(statement.Contract)
				record(statement.Default, SemanticCompileTimeRequiredContext, "explicit type default")
			case *ast.StructStatement:
				if statement == nil {
					continue
				}
				for _, field := range statement.Fields {
					if field != nil {
						contract(field.Contract)
					}
				}
			case *ast.LetStatement:
				if statement == nil {
					continue
				}
				contract(statement.Contract)
				if topLevel || statement.Static {
					record(statement.Value, SemanticCompileTimeRequiredContext, "static initializer")
				}
			case *ast.FunctionDeclaration:
				if statement != nil {
					block(statement.Body)
				}
			case *ast.ImplStatement:
				if statement == nil {
					continue
				}
				for _, member := range statement.Members {
					if function, ok := member.(*ast.FunctionDeclaration); ok && function != nil {
						attributes(function.Attributes)
						block(function.Body)
					}
				}
			case *ast.IfStatement:
				if statement != nil {
					block(statement.Consequence)
					block(statement.Alternative)
				}
			case *ast.WhileStatement:
				if statement != nil {
					block(statement.Body)
				}
			case *ast.ForStatement:
				if statement != nil {
					block(statement.Body)
				}
			case *ast.UnsafeStatement:
				if statement != nil {
					block(statement.Body)
				}
			case *ast.SwitchStatement:
				if statement != nil {
					for _, switchCase := range statement.Cases {
						if switchCase != nil {
							block(switchCase.Body)
						}
					}
				}
			case *ast.MatchStatement:
				if statement != nil && statement.Match != nil {
					for _, arm := range statement.Match.Arms {
						if arm != nil {
							block(arm.BlockBody)
						}
					}
				}
			}
		}
	}
	statements(program.Statements, true)
}
