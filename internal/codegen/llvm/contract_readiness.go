package llvm

import (
	"reflect"
	"sec/internal/codegen/readiness"
	"sec/internal/codegen/targetplan"

	"sec/internal/ast"
	astwalk "sec/internal/ast/walk"
	"sec/internal/ir/semantic"
	"sec/internal/lexer"
	"sec/internal/sema"
)

// GenerateAnalyzed checks canonical contract facts, including inherited and
// imported types absent from this AST, before selecting legacy LLVM lowering.
// Target-sized Sema facts must agree with the canonical selected scalar plan.
// Character literals retain their exact Sema-resolved char/rune representation.
// Complete string-length conjunctions support native bodyless checked conversions;
// other contract families are rejected before their invariants can be erased.
// Rules: rules/types/contracts.md — Mutation and Conversion failure layers;
// rules/compiler/compiler_pipeline.md — lowering prerequisites;
// rules/types/types.md — "int and uint", "Character literal", "Context shaping";
// correction5.md — canonical scalar facts;
// rules/corrections/applied/md043-char-rune-literal-correction-20261008.md — §§2–3.
func GenerateAnalyzed(program *ast.Program, analyzer *sema.Analyzer, triple string) (string, error) {
	if analyzer == nil || program == nil {
		return "", unsupportedContract("missing analyzed type-contract facts", lexer.Token{})
	}
	if err := readiness.CheckWideProgram(program, analyzer, "LLVM"); err != nil {
		return "", err
	}
	// Loader-owned core declarations are semantic support, not output roots.
	// Any core/imported contract used by source is still checked via Sema facts.
	output := *program
	output.Statements = nil
	for _, statement := range program.Statements {
		token := llvmSourceToken(statement)
		if program.SourceProvenance[token.File] != ast.SourceCore {
			output.Statements = append(output.Statements, statement)
		}
	}
	types := analyzer.Types()
	if err := astwalk.Inspect(&output, func(node any) error {
		var typ sema.Type
		var token lexer.Token
		if expression, ok := node.(ast.Expression); ok {
			typ, _ = analyzer.ResolvedTypeOf(expression)
			token = llvmSourceToken(node)
		} else if reference, ok := node.(*ast.TypeReference); ok {
			typ, token = types[reference.Name], reference.Token
		}
		if typeNeedsUnsupportedContractValidation(&typ, map[*sema.Type]bool{}, true) {
			return unsupportedContract("type contract validation for "+typ.Name, token)
		}
		return nil
	}); err != nil {
		return "", err
	}
	if err := readiness.RejectUnitQuantities(&output, analyzer, "legacy LLVM"); err != nil {
		return "", err
	}
	plan, err := targetplan.Plan(triple)
	if err != nil {
		return "", unsupportedContract(err.Error(), lexer.Token{})
	}
	for _, name := range []string{"int", "uint"} {
		typ := types[name]
		width := 0
		if typ.MaxInteger != nil {
			width = typ.MaxInteger.BitLen()
			if name == "int" {
				width++
			}
		}
		if width != int(plan.PointerWidthBits) {
			return "", unsupportedContract("analyzed "+name+" width does not match selected target scalar plan", lexer.Token{})
		}
	}
	characters, err := resolvedCharacterCarriers(&output, analyzer)
	if err != nil {
		return "", err
	}
	g := NewGenerator()
	g.targetTriple = triple
	facts, err := resolvedStringFacts(&output, analyzer)
	if err != nil {
		return "", err
	}
	g.stringFacts = facts
	g.characterTypes = characters
	return g.Generate(&output)
}

// typeNeedsContractValidation retains nested aggregate, sequence, union and
// callable constraints; erasing a container cannot erase its element invariant.
// Rules: rules/types/contracts.md — Core rule and Composition;
// rules/types/default_values.md — Defaults and contracts.
func typeNeedsContractValidation(typ *sema.Type, visited map[*sema.Type]bool) bool {
	return typeNeedsUnsupportedContractValidation(typ, visited, false)
}

// typeNeedsUnsupportedContractValidation permits only complete supported string
// length conjunctions when exact analyzed facts are available.
// Rules: rules/types/contracts.md — Composition, String and collection contracts.
func typeNeedsUnsupportedContractValidation(typ *sema.Type, visited map[*sema.Type]bool, allowString bool) bool {
	if typ == nil || visited[typ] {
		return false
	}
	visited[typ] = true
	if len(typ.Contracts) > 0 {
		if _, supported := sema.StringLengthContractPlan(*typ); !allowString || !supported {
			return true
		}
	}
	if typeNeedsUnsupportedContractValidation(typ.Element, visited, allowString) || typeNeedsUnsupportedContractValidation(typ.FunctionReturnType, visited, allowString) {
		return true
	}
	for i := range typ.Fields {
		if typeNeedsUnsupportedContractValidation(&typ.Fields[i].Type, visited, allowString) {
			return true
		}
	}
	for i := range typ.TypeArgs {
		if typeNeedsUnsupportedContractValidation(&typ.TypeArgs[i], visited, allowString) {
			return true
		}
	}
	for i := range typ.FunctionParameterTypes {
		if typeNeedsUnsupportedContractValidation(&typ.FunctionParameterTypes[i], visited, allowString) {
			return true
		}
	}
	for i := range typ.UnionVariants {
		variant := &typ.UnionVariants[i]
		if typeNeedsUnsupportedContractValidation(variant.Payload, visited, allowString) {
			return true
		}
		for j := range variant.PayloadFields {
			if typeNeedsUnsupportedContractValidation(&variant.PayloadFields[j].Type, visited, allowString) {
				return true
			}
		}
	}
	return false
}

// llvmSourceToken keeps readiness errors at their source node, without deriving
// semantics from text or trusting module-name spellings for core provenance.
// Rules: rules/tooling/diagnostics.md — source locations.
func llvmSourceToken(node any) lexer.Token {
	v := reflect.ValueOf(node)
	if v.Kind() != reflect.Pointer || v.IsNil() {
		return lexer.Token{}
	}
	field := v.Elem().FieldByName("Token")
	if !field.IsValid() {
		return lexer.Token{}
	}
	token, _ := field.Interface().(lexer.Token)
	return token
}

// rejectASTContracts closes raw generator entry points even without Sema facts.
// Rules: rules/types/contracts.md — Core rule and Mutation.
func rejectASTContracts(program *ast.Program) error {
	return astwalk.Inspect(program, func(node any) error {
		if _, ok := node.(ast.Contract); ok {
			// All contract AST nodes carry their defining source token.
			token := llvmSourceToken(node)
			return unsupportedContract("type contract validation", token)
		}
		return nil
	})
}

// unsupportedContract reports a capability limitation with original provenance.
// Rules: rules/types/contracts.md — Conversion failure layers;
// rules/tooling/diagnostics.md — unsupported lowering versus source errors.
func unsupportedContract(feature string, token lexer.Token) error {
	return &semantic.UnsupportedFeatureError{Feature: feature + "; legacy LLVM lowering is unsupported", Location: semantic.Location{File: token.File, Line: token.Line, Column: token.Column}}
}
