package ast

import (
	"reflect"

	"sec/internal/lexer"
)

var lexerTokenType = reflect.TypeOf(lexer.Token{})

// Visit calls visit for root and every AST node pointer reachable from it
// through exported fields, slices, maps and interfaces, in pre-order. It
// skips a node's children when visit returns false. The walk is reflective,
// so it reaches every node kind without a hand-maintained switch; each
// pointer is visited once.
func Visit(root any, visit func(node any) bool) {
	visited := map[uintptr]bool{}
	var walk func(value reflect.Value)
	walk = func(value reflect.Value) {
		switch value.Kind() {
		case reflect.Interface:
			if !value.IsNil() {
				walk(value.Elem())
			}
		case reflect.Pointer:
			if value.IsNil() || visited[value.Pointer()] {
				return
			}
			visited[value.Pointer()] = true
			if value.Elem().Kind() == reflect.Struct && value.CanInterface() && !visit(value.Interface()) {
				return
			}
			walk(value.Elem())
		case reflect.Struct:
			if value.Type() == lexerTokenType {
				return
			}
			for index := 0; index < value.NumField(); index++ {
				if value.Type().Field(index).IsExported() {
					walk(value.Field(index))
				}
			}
		case reflect.Slice, reflect.Array:
			for index := 0; index < value.Len(); index++ {
				walk(value.Index(index))
			}
		case reflect.Map:
			for _, key := range value.MapKeys() {
				walk(value.MapIndex(key))
			}
		}
	}
	walk(reflect.ValueOf(root))
}
