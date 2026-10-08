package readiness

import (
	"math/big"
	"reflect"
	"strings"

	"sec/internal/ast"
	astwalk "sec/internal/ast/walk"
	"sec/internal/ir/semantic"
	"sec/internal/lexer"
	"sec/internal/sema"
)

// CheckWideOperation rejects operations whose legacy implementation lacks
// checked wide arithmetic or exact decimal128 validation. Storage, calls,
// integer comparisons and bitwise operations retain their existing support.
// Rules: rules/types/types.md — Integer types, Exact decimal types;
// rules/foundations/operators.md — Checked integer arithmetic, Runtime overflow;
// rules/compiler/compiler_pipeline.md — lowering prerequisites.
func CheckWideOperation(carrier, operation string, token lexer.Token) error {
	operation = strings.TrimSuffix(operation, "=")
	if operation == "" {
		return nil
	}
	wideInteger := carrier == "i128" || carrier == "i256"
	decimal := carrier == "!llvm.struct<(i128, i32)>"
	checked := false
	switch operation {
	case "+", "-", "*", "/", "%", "<<", ">>", "add", "sub", "mul", "sdiv", "udiv", "srem", "urem", "shl", "ashr", "lshr":
		checked = true
	}
	if (wideInteger && checked) || decimal {
		return wideFailure("checked wide numeric operation "+operation+" on "+carrier, token)
	}
	return nil
}

// CheckWideProgram checks source and canonical facts before legacy output.
// LLVM has no decimal128 carrier; MLIR supports exact storage but not decimal
// operations. Canonical operand facts also cover imported aliases and bodies
// which the raw AST emitter might otherwise ignore.
// Rules: rules/types/types.md — Named types, Integer types, Exact decimal types;
// rules/foundations/operators.md — Checked integer arithmetic;
// rules/compiler/compiler_pipeline.md — lowering prerequisites.
func CheckWideProgram(program *ast.Program, analyzer *sema.Analyzer, backend string) error {
	types := map[string]sema.Type{}
	if analyzer != nil {
		types = analyzer.Types()
	}
	declared := map[string]bool{}
	_ = astwalk.Inspect(program, func(node any) error {
		switch declaration := node.(type) {
		case *ast.TypeDeclStatement:
			if declaration.Name != nil {
				declared[declaration.Name.Value] = true
			}
		case *ast.EnumDeclaration:
			if declaration.Name != nil {
				declared[declaration.Name.Value] = true
			}
		}
		return nil
	})
	return astwalk.Inspect(program, func(node any) error {
		token := wideToken(node)
		if program.SourceProvenance[token.File] == ast.SourceCore {
			return nil
		}
		var typ sema.Type
		if analyzer != nil {
			if expr, ok := node.(ast.Expression); ok {
				typ, _ = analyzer.ResolvedTypeOf(expr)
			}
			if ref, ok := node.(*ast.TypeReference); ok {
				typ = types[ref.Name]
			}
		}
		if ref, ok := node.(*ast.TypeReference); ok && analyzer != nil && wideIntegerCarrier(typ, types) != "" {
			switch ref.Name {
			case "int128", "uint128", "int256", "uint256":
			default:
				if !declared[ref.Name] {
					return wideFailure("imported wide type representation for "+ref.Name, token)
				}
			}
		}
		if backend == "LLVM" {
			if ref, ok := node.(*ast.TypeReference); ok && ref.Name == "decimal128" {
				return wideFailure("decimal128 representation in legacy LLVM", token)
			}
			if ident, ok := node.(*ast.Identifier); ok && ident.Value == "decimal128" {
				return wideFailure("decimal128 representation in legacy LLVM", token)
			}
			if containsDecimal128(&typ, types, map[*sema.Type]bool{}, map[string]bool{}) {
				return wideFailure("decimal128 representation in legacy LLVM", token)
			}
		}
		if analyzer == nil {
			return nil
		}
		var operand ast.Expression
		operation := ""
		switch node := node.(type) {
		case *ast.InfixExpression:
			operand, operation = node.Left, node.Operator
		case *ast.PrefixExpression:
			if _, literal := node.Right.(*ast.IntegerLiteral); literal {
				return nil
			}
			if node.Operator != "+" {
				operand, operation = node.Right, node.Operator
			}
		case *ast.AssignmentStatement:
			if node.Operator != "=" {
				operand, operation = node.Target, node.Operator
			}
		}
		if operand != nil {
			typ, _ := analyzer.ResolvedTypeOf(operand)
			carrier := wideIntegerCarrier(typ, types)
			if binary, ok := node.(*ast.InfixExpression); ok {
				right, _ := analyzer.ResolvedTypeOf(binary.Right)
				if carrier == "" {
					carrier = wideIntegerCarrier(right, types)
				}
				if containsDecimal128(&right, types, map[*sema.Type]bool{}, map[string]bool{}) {
					carrier = "!llvm.struct<(i128, i32)>"
				}
			}
			if containsDecimal128(&typ, types, map[*sema.Type]bool{}, map[string]bool{}) {
				carrier = "!llvm.struct<(i128, i32)>"
			}
			return CheckWideOperation(carrier, operation, token)
		}
		return nil
	})
}

// containsDecimal128 follows named carrier identity and recursively stored types
// without guessing a source type from its LLVM layout.
// Rules: rules/types/types.md — Named types, Exact decimal types.
func containsDecimal128(typ *sema.Type, types map[string]sema.Type, seen map[*sema.Type]bool, names map[string]bool) bool {
	if typ == nil || seen[typ] {
		return false
	}
	seen[typ] = true
	if typ.Name == "decimal128" || typ.Underlying == "decimal128" {
		return true
	}
	if typ.Underlying != "" && !names[typ.Underlying] {
		names[typ.Underlying] = true
		underlying := types[typ.Underlying]
		if containsDecimal128(&underlying, types, seen, names) {
			return true
		}
	}
	if containsDecimal128(typ.Element, types, seen, names) || containsDecimal128(typ.FunctionReturnType, types, seen, names) {
		return true
	}
	for i := range typ.Fields {
		if containsDecimal128(&typ.Fields[i].Type, types, seen, names) {
			return true
		}
	}
	for i := range typ.TypeArgs {
		if containsDecimal128(&typ.TypeArgs[i], types, seen, names) {
			return true
		}
	}
	for i := range typ.FunctionParameterTypes {
		if containsDecimal128(&typ.FunctionParameterTypes[i], types, seen, names) {
			return true
		}
	}
	for i := range typ.UnionVariants {
		v := &typ.UnionVariants[i]
		if containsDecimal128(v.Payload, types, seen, names) {
			return true
		}
		for j := range v.PayloadFields {
			if containsDecimal128(&v.PayloadFields[j].Type, types, seen, names) {
				return true
			}
		}
	}
	return false
}

// wideToken preserves the defining source location for unsupported lowering.
// Rules: rules/tooling/diagnostics.md — source locations.
func wideToken(node any) lexer.Token {
	v := reflect.ValueOf(node)
	if v.Kind() != reflect.Pointer || v.IsNil() || v.Elem().Kind() != reflect.Struct {
		return lexer.Token{}
	}
	f := v.Elem().FieldByName("Token")
	if !f.IsValid() {
		return lexer.Token{}
	}
	token, _ := f.Interface().(lexer.Token)
	return token
}

// wideFailure reports an implementation limitation without changing type legality.
// Rules: rules/compiler/compiler_pipeline.md — lowering prerequisites;
// rules/tooling/diagnostics.md — unsupported lowering versus source errors.
func wideFailure(feature string, token lexer.Token) error {
	return &semantic.UnsupportedFeatureError{Feature: feature + " is unsupported by legacy lowering", Location: semantic.Location{File: token.File, Line: token.Line, Column: token.Column}}
}

// CheckWideConstant proves a folded constant fits the exact wide carrier before
// publishing it; LLVM constants must never normalize an overflowing value.
// Rules: rules/types/types.md — integer representability;
// rules/foundations/operators.md — Compile-time overflow and Runtime overflow.
func CheckWideConstant(carrier string, unsigned bool, literal string, token lexer.Token) error {
	bits := 0
	if carrier == "i128" {
		bits = 128
	}
	if carrier == "i256" {
		bits = 256
	}
	if bits == 0 {
		return nil
	}
	value, ok := new(big.Int).SetString(literal, 10)
	if !ok {
		return wideFailure("unresolved wide constant", token)
	}
	min, max := integerDomain(bits, unsigned)
	if value.Cmp(min) < 0 || value.Cmp(max) > 0 {
		return wideFailure("overflowing wide constant requires checked arithmetic", token)
	}
	return nil
}

// wideIntegerCarrier follows canonical nominal carriers, using exact intrinsic
// domains when older scalar facts do not carry an explicit BitWidth field.
// Rules: rules/types/types.md — Integer types and Named types.
func wideIntegerCarrier(typ sema.Type, types map[string]sema.Type) string {
	seen := map[string]bool{}
	for {
		switch typ.Name {
		case "int128", "uint128":
			return "i128"
		case "int256", "uint256":
			return "i256"
		}
		if typ.BitWidth == 128 {
			return "i128"
		}
		if typ.BitWidth == 256 {
			return "i256"
		}
		if typ.Underlying == "" || seen[typ.Underlying] {
			break
		}
		seen[typ.Underlying] = true
		underlying, ok := types[typ.Underlying]
		if !ok {
			break
		}
		typ = underlying
	}
	return ""
}
