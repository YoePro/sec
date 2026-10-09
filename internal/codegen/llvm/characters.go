package llvm

import (
	"fmt"
	"strconv"
	"unicode/utf8"

	"sec/internal/ast"
	astwalk "sec/internal/ast/walk"
	"sec/internal/ir/semantic"
	"sec/internal/sema"
)

// characterCarrier is an immutable snapshot of Sema's selected scalar carrier.
// It does not infer the source type from the decoded character's value.
type characterCarrier struct {
	typ string
}

// resolvedCharacterCarriers consumes completed Sema facts for the exact output
// AST. Missing or unsupported facts fail closed, including stale AST snapshots.
// Rules: rules/types/types.md — char, rune, Character literal, Context shaping;
// rules/corrections/applied/md043-char-rune-literal-correction-20261008.md — §§2–3;
// rules/compiler/compiler_pipeline.md — lowering prerequisites.
func resolvedCharacterCarriers(program *ast.Program, analyzer *sema.Analyzer) (map[*ast.CharLiteral]characterCarrier, error) {
	carriers := map[*ast.CharLiteral]characterCarrier{}
	err := astwalk.Inspect(program, func(node any) error {
		literal, ok := node.(*ast.CharLiteral)
		if !ok {
			return nil
		}
		typ, resolved := analyzer.ResolvedTypeOf(literal)
		if !resolved {
			return characterLoweringFailure(literal, "missing Sema-resolved character literal type")
		}
		switch typ.Kind {
		case sema.CharType:
			carriers[literal] = characterCarrier{typ: "i8"}
		case sema.RuneType:
			carriers[literal] = characterCarrier{typ: "i32"}
		default:
			return characterLoweringFailure(literal, "unsupported resolved character literal type "+typ.Name)
		}
		return nil
	})
	return carriers, err
}

// emitCharacterLiteral emits the decoded scalar at Sema's selected char/rune
// width. Raw generator APIs require analyzed facts rather than guessing from a
// value or the surrounding backend representation. Domain checks defend against
// malformed input ASTs and cannot manufacture a different source type.
// Rules: rules/foundations/lexical_structure.md — §13 Character literals;
// rules/types/types.md — char, rune, Context shaping; MD-043 — §§2–3.
func (g *Generator) emitCharacterLiteral(literal *ast.CharLiteral) (value, error) {
	carrier, ok := g.characterTypes[literal]
	if !ok {
		return value{}, characterLoweringFailure(literal, "character literal requires Sema-resolved char/rune type")
	}
	scalar, size := utf8.DecodeRuneInString(literal.Value)
	if size == 0 || size != len(literal.Value) || !utf8.ValidString(literal.Value) || !utf8.ValidRune(scalar) {
		return value{}, fmt.Errorf("emit-llvm requires one decoded Unicode scalar for %q", literal.Token.Lexeme)
	}
	if carrier.typ == "i8" && scalar > 255 {
		return value{}, fmt.Errorf("Sema-resolved char literal exceeds its 0..255 domain: %q", literal.Token.Lexeme)
	}
	return value{typ: carrier.typ, ref: strconv.FormatInt(int64(scalar), 10), unsigned: true}, nil
}

// characterLoweringFailure preserves source provenance for unsupported/stale
// character facts without publishing partial LLVM output.
// Rules: rules/tooling/diagnostics.md — §9(2,5) Source locations;
// rules/compiler/compiler_pipeline.md — §110(1) implementation boundaries.
func characterLoweringFailure(literal *ast.CharLiteral, feature string) error {
	return &semantic.UnsupportedFeatureError{Feature: feature, Location: semantic.Location{File: literal.Token.File, Line: literal.Token.Line, Column: literal.Token.Column}}
}
