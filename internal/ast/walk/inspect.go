// Package walk traverses source AST graphs without entering semantic metadata.
package walk

import (
	"reflect"

	"sec/internal/ast"
	"sec/internal/lexer"
)

// Inspect visits source AST pointers in field/source order, once each,
// allowing readiness checks to cover nested declarations and unused bodies.
// Rules: rules/compiler/compiler_pipeline.md — lowering prerequisites.
func Inspect(root any, inspect func(any) error) error {
	return traverse(root, inspect, nil)
}

// Span returns the exact parser-token envelope before synthesized values are added.
// Rules: rules/tooling/lsp.md — "Snapshots", "Safe fixes".
func Span(root any) (int, int, bool) {
	start, end := -1, -1
	_ = traverse(root, func(any) error { return nil }, func(token lexer.Token) {
		if token.Line > 0 && token.ByteEnd > token.ByteStart {
			if start < 0 || token.ByteStart < start {
				start = token.ByteStart
			}
			if token.ByteEnd > end {
				end = token.ByteEnd
			}
		}
	})
	return start, end, start >= 0 && end > start
}

// traverse keeps pointer identity and source field order across DAGs and cycles.
// Rules: rules/compiler/compiler_pipeline.md — lowering prerequisites;
// rules/tooling/lsp.md — source snapshots.
func traverse(root any, inspect func(any) error, tokenVisit func(lexer.Token)) error {
	visited := map[uintptr]bool{}
	var walk func(reflect.Value) error
	walk = func(v reflect.Value) error {
		if !v.IsValid() {
			return nil
		}
		if v.Kind() == reflect.Interface {
			if v.IsNil() {
				return nil
			}
			return walk(v.Elem())
		}
		if v.Kind() == reflect.Pointer {
			if v.IsNil() || visited[v.Pointer()] {
				return nil
			}
			visited[v.Pointer()] = true
			if err := inspect(v.Interface()); err != nil {
				return err
			}
			return walk(v.Elem())
		}
		if tokenVisit != nil && v.CanInterface() {
			if token, ok := v.Interface().(lexer.Token); ok {
				tokenVisit(token)
				return nil
			}
		}
		switch v.Kind() {
		case reflect.Struct:
			if v.Type().PkgPath() != reflect.TypeOf(ast.Program{}).PkgPath() {
				return nil
			}
			for i := 0; i < v.NumField(); i++ {
				if err := walk(v.Field(i)); err != nil {
					return err
				}
			}
		case reflect.Slice:
			for i := 0; i < v.Len(); i++ {
				if err := walk(v.Index(i)); err != nil {
					return err
				}
			}
		}
		return nil
	}
	return walk(reflect.ValueOf(root))
}
