package llvm

import (
	"fmt"
	"strings"

	"sec/internal/ast"
	astwalk "sec/internal/ast/walk"
	"sec/internal/codegen/readiness"
	"sec/internal/codegen/targetplan"
	"sec/internal/ir/semantic"
	"sec/internal/layout"
	"sec/internal/sema"
)

type Generator struct {
	out                   strings.Builder
	lambdaDefs            strings.Builder
	activeOut             *strings.Builder
	globals               strings.Builder
	label                 int
	temp                  int
	stringID              int
	generatedIdentities   *sema.GeneratedIdentityIndex
	needsPuts             bool
	needsDecimal          bool
	locals                map[string]local
	functions             map[string]*ast.FunctionDeclaration
	typeAliases           map[string]*ast.TypeReference
	enums                 map[string]enumInfo
	loops                 []loopContext
	returnType            string
	targetTriple          string
	scalarPlan            layout.ResolvedScalarPlan
	blockOpen             bool
	typeFailure           error
	resolvingAliases      map[string]bool
	characterTypes        map[*ast.CharLiteral]characterCarrier
	stringFacts           *stringFacts
	stringTry             bool
	needsRuneLength       bool
	needsStringResult     bool
	needsStringDescriptor bool
}

type loopContext struct {
	breakLabel    string
	continueLabel string
}

type local struct {
	typ      string
	ptr      string
	ref      string
	lenRef   string
	fnType   *ast.TypeReference
	direct   bool
	unsigned bool
}

type enumInfo struct {
	typ    string
	values map[string]string
}

func NewGenerator() *Generator {
	return &Generator{}
}

func Generate(program *ast.Program) (string, error) {
	return NewGenerator().Generate(program)
}

func GenerateWithTriple(program *ast.Program, triple string) (string, error) {
	g := NewGenerator()
	g.targetTriple = triple
	return g.Generate(program)
}

// Generate emits legacy LLVM only after resolving the canonical scalar plan and
// checking contract and floating-point prerequisites. Character emission consumes
// analyzed type facts. Each invocation discards buffers and character facts from
// earlier attempts.
// Rules: rules/types/contracts.md — Core rule and Mutation;
// rules/types/types.md — "int and uint", "Binary floating-point types",
// "Character literal"; MD-014 §6; MD-043 §§2–3;
// rules/compiler/compiler_pipeline.md — lowering prerequisites.
func (g *Generator) Generate(program *ast.Program) (string, error) {
	defer func() { g.characterTypes = nil; g.stringFacts = nil }()
	if err := readiness.RejectInstant(program); err != nil {
		return "", err
	}
	if err := readiness.CheckWideProgram(program, nil, "LLVM"); err != nil {
		return "", err
	}
	configuredTriple := g.targetTriple
	characters := g.characterTypes
	facts := g.stringFacts
	*g = Generator{targetTriple: configuredTriple, characterTypes: characters, stringFacts: facts}
	g.generatedIdentities = sema.NewGeneratedIdentityIndex(program)
	if err := g.stringContractASTGate(program); err != nil {
		return "", err
	}
	if err := readiness.RejectUnitQuantities(program, nil, "legacy LLVM"); err != nil {
		return "", err
	}
	if err := readiness.RejectPlatformFloat(program, "LLVM"); err != nil {
		return "", err
	}
	plan, err := targetplan.Plan(g.targetTriple)
	if err != nil {
		return "", &semantic.UnsupportedFeatureError{Feature: err.Error()}
	}
	if err := readiness.ValidateCompilerAuthority(program, plan, "LLVM"); err != nil {
		return "", err
	}
	g.scalarPlan = plan
	g.targetTriple = plan.LLVMTriple
	if err := validateEntrypoint(program); err != nil {
		return "", err
	}
	g.functions = map[string]*ast.FunctionDeclaration{}
	g.typeAliases = map[string]*ast.TypeReference{}
	g.enums = map[string]enumInfo{}
	for _, stmt := range program.Statements {
		switch stmt := stmt.(type) {
		case *ast.FunctionDeclaration:
			if stmt.Name != nil {
				g.functions[stmt.Name.Value] = stmt
			}
		case *ast.TypeDeclStatement:
			g.registerTypeDeclaration(stmt, "")
		case *ast.EnumDeclaration:
			g.registerEnum(stmt, "")
		case *ast.ImplStatement:
			target := ""
			if stmt.Target != nil {
				target = stmt.Target.Name
			}
			for _, member := range stmt.Members {
				if typeDecl, ok := member.(*ast.TypeDeclStatement); ok {
					g.registerTypeDeclaration(typeDecl, target)
				}
				if enumDecl, ok := member.(*ast.EnumDeclaration); ok {
					g.registerEnum(enumDecl, target)
				}
			}
		}
	}
	// A raw AST API cannot assume unknown constructor/type-argument names are
	// unconstrained. Require a representation for every referenced source type.
	// Rules: rules/types/contracts.md — Core rule and Composition.
	if err := astwalk.Inspect(program, func(node any) error {
		if ref, ok := node.(*ast.TypeReference); ok {
			if g.stringFacts != nil && g.stringFacts.errorRefs[ref] {
				return nil
			}
			g.llvmType(ref)
			return g.typeFailure
		}
		return nil
	}); err != nil {
		return "", err
	}

	for _, stmt := range program.Statements {
		fn, ok := stmt.(*ast.FunctionDeclaration)
		if !ok || fn.Name == nil {
			continue
		}
		if err := g.emitFunction(fn); err != nil {
			return "", err
		}
		if g.typeFailure != nil {
			return "", g.typeFailure
		}
	}

	var result strings.Builder
	result.WriteString("; generated by sec\n")
	result.WriteString(fmt.Sprintf("target triple = %q\n\n", g.scalarPlan.LLVMTriple))
	if g.needsDecimal {
		result.WriteString(llvmDecimalType + " = type { i64, i8 }\n\n")
	}
	if g.needsStringDescriptor {
		result.WriteString("%sec.string = type { ptr, i64 }\n\n")
	}
	if g.needsStringResult {
		result.WriteString(stringResultType + " = type { i1, ptr, i64, " + g.nativeIntegerType() + ", " + g.nativeIntegerType() + " }\n\n")
	}
	if g.needsRuneLength {
		for _, declaration := range g.functions {
			if declaration.LinkName == ".sec.generated.string_rune_length" {
				return "", fmt.Errorf("generated rune-length helper conflicts with source linkage")
			}
		}
		result.WriteString(runeLengthLLVM)
	}
	result.WriteString(g.globals.String())
	if g.globals.Len() > 0 {
		result.WriteString("\n")
	}
	if g.needsPuts {
		result.WriteString("declare i32 @puts(ptr)\n\n")
	}
	result.WriteString(g.lambdaDefs.String())
	result.WriteString(g.out.String())
	return result.String(), nil
}

func validateEntrypoint(program *ast.Program) error {
	hasMainModule := false
	for _, stmt := range program.Statements {
		module, ok := stmt.(*ast.ModuleStatement)
		if ok && module.Path == "main" {
			hasMainModule = true
			break
		}
	}
	if !hasMainModule {
		return fmt.Errorf("emit-llvm requires module main")
	}

	mainFn := findMainFunction(program)
	if mainFn == nil {
		return fmt.Errorf("emit-llvm requires fn main() int or fn main() void")
	}
	if len(mainFn.Parameters) != 0 {
		return fmt.Errorf("emit-llvm requires fn main() with no parameters")
	}
	if mainFn.ReturnType == nil || (mainFn.ReturnType.Name != "int" && mainFn.ReturnType.Name != "void") {
		return fmt.Errorf("emit-llvm requires fn main() int or fn main() void")
	}

	return nil
}

func findMainFunction(program *ast.Program) *ast.FunctionDeclaration {
	for _, stmt := range program.Statements {
		fn, ok := stmt.(*ast.FunctionDeclaration)
		if ok && fn.Name != nil && fn.Name.Value == "main" {
			return fn
		}
	}
	return nil
}

func (g *Generator) write(format string, args ...any) {
	if g.activeOut != nil {
		fmt.Fprintf(g.activeOut, format, args...)
		return
	}
	fmt.Fprintf(&g.out, format, args...)
}

func (g *Generator) nextLabel(prefix string) string {
	name := fmt.Sprintf("%s.%d", prefix, g.label)
	g.label++
	return name
}

func (g *Generator) nextTemp() string {
	name := fmt.Sprintf("%%t%d", g.temp)
	g.temp++
	return name
}
