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
// The backend cannot preserve runtime validation, so it rejects contracted
// types rather than erasing their invariants into primitive LLVM types.
// Rules: rules/types/contracts.md — Mutation and Conversion failure layers;
// rules/compiler/compiler_pipeline.md — lowering prerequisites;
// rules/types/types.md — "int and uint"; correction5.md — canonical scalar facts.
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
		if typeNeedsContractValidation(&typ, map[*sema.Type]bool{}) {
			return unsupportedContract("type contract validation for "+typ.Name, token)
		}
		return nil
	}); err != nil {
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
	return GenerateWithTriple(&output, triple)
}

// typeNeedsContractValidation retains nested aggregate, sequence, union and
// callable constraints; erasing a container cannot erase its element invariant.
// Rules: rules/types/contracts.md — Core rule and Composition;
// rules/types/default_values.md — Defaults and contracts.
func typeNeedsContractValidation(typ *sema.Type, visited map[*sema.Type]bool) bool {
	if typ == nil || visited[typ] {
		return false
	}
	visited[typ] = true
	if len(typ.Contracts) > 0 {
		return true
	}
	if typeNeedsContractValidation(typ.Element, visited) || typeNeedsContractValidation(typ.FunctionReturnType, visited) {
		return true
	}
	for i := range typ.Fields {
		if typeNeedsContractValidation(&typ.Fields[i].Type, visited) {
			return true
		}
	}
	for i := range typ.TypeArgs {
		if typeNeedsContractValidation(&typ.TypeArgs[i], visited) {
			return true
		}
	}
	for i := range typ.FunctionParameterTypes {
		if typeNeedsContractValidation(&typ.FunctionParameterTypes[i], visited) {
			return true
		}
	}
	for i := range typ.UnionVariants {
		variant := &typ.UnionVariants[i]
		if typeNeedsContractValidation(variant.Payload, visited) {
			return true
		}
		for j := range variant.PayloadFields {
			if typeNeedsContractValidation(&variant.PayloadFields[j].Type, visited) {
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
