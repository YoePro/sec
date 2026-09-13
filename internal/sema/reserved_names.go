package sema

import (
	"reflect"

	"sec/internal/ast"
	"sec/internal/diagnostics"
	"sec/internal/lexer"
)

// validateReservedDeclarationNames applies one lexer-owned reservation policy
// to every declaration-bearing AST node. Parser-reserved hard keywords normally
// cannot form these nodes, but use the same policy through
// lexer.IsReservedDeclarationName.
//
// Rules:
//   - rules/foundations/lexical_structure.md — §§7–9
//   - rules/foundations/names_scopes_visibility.md — §9 "Reserved language names"
func (a *Analyzer) validateReservedDeclarationNames(program *ast.Program) {
	if program == nil {
		return
	}
	seen := map[sourceTokenKey]bool{}
	checkToken := func(name string, token lexer.Token, category string) {
		if name == "" || !lexer.IsReservedDeclarationName(name) {
			return
		}
		key := sourceTokenLocation(token)
		if seen[key] {
			return
		}
		seen[key] = true
		a.addErrorAtTokenWithID(token, diagnostics.ReservedDeclarationName, "%s name %q is reserved by the language", category, name)
	}
	checkIdentifier := func(identifier *ast.Identifier, category string) {
		if identifier != nil {
			checkToken(identifier.Value, identifier.Token, category)
		}
	}

	var visit func(reflect.Value)
	visit = func(value reflect.Value) {
		if !value.IsValid() {
			return
		}
		if value.Kind() == reflect.Interface {
			if !value.IsNil() {
				visit(value.Elem())
			}
			return
		}
		if value.Kind() == reflect.Pointer {
			if value.IsNil() {
				return
			}
			if value.CanInterface() {
				switch node := value.Interface().(type) {
				case *ast.ModuleStatement:
					if !a.isTrustedCoreSourceToken(node.NameToken) {
						checkToken(node.Path, node.NameToken, "module")
					}
				case *ast.ImportStatement:
					checkToken(node.Alias, node.AliasToken, "import alias")
				case *ast.TypeDeclStatement:
					for _, variant := range node.Variants {
						checkIdentifier(variant, "variant")
					}
				case *ast.GenericParameter:
					checkIdentifier(node.Name, "generic parameter")
				case *ast.EnumValue:
					checkIdentifier(node.Name, "enum member")
				case *ast.InterfaceProperty:
					checkIdentifier(node.Name, "property")
					checkIdentifier(node.SetterParameter, "setter parameter")
				case *ast.InterfaceEvent:
					checkIdentifier(node.Name, "event")
				case *ast.StructField:
					checkIdentifier(node.Name, "field")
				case *ast.RegisterField:
					if node.Name != nil && node.Name.Value != "_" {
						checkIdentifier(node.Name, "register field")
					}
				case *ast.FunctionDeclaration:
					checkIdentifier(node.Name, "function")
				case *ast.Parameter:
					checkIdentifier(node.Name, "parameter")
				case *ast.LetStatement:
					checkIdentifier(node.Name, "variable")
				case *ast.SelectBranch:
					checkIdentifier(node.Binding, "select binding")
				case *ast.MatchPatternBinding:
					if node.Name != nil && node.Name.Value != "_" {
						checkIdentifier(node.Name, "match binding")
					}
				case *ast.PropertyDeclaration:
					checkIdentifier(node.Name, "property")
				case *ast.PropertySetter:
					checkIdentifier(node.Parameter, "setter parameter")
				case *ast.EventDeclaration:
					checkIdentifier(node.Name, "event")
				}
			}
			if value.Type().Elem().PkgPath() == "sec/internal/ast" {
				visit(value.Elem())
			}
			return
		}
		if value.CanInterface() {
			switch node := value.Interface().(type) {
			case ast.ForBinding:
				if !node.Discard {
					checkToken(node.Name, node.Token, "loop binding")
				}
			case ast.AsmOutput:
				checkToken(node.Name, node.Token, "assembly output binding")
			}
		}
		switch value.Kind() {
		case reflect.Struct:
			if value.Type().PkgPath() != "sec/internal/ast" {
				return
			}
			for index := 0; index < value.NumField(); index++ {
				visit(value.Field(index))
			}
		case reflect.Slice, reflect.Array:
			for index := 0; index < value.Len(); index++ {
				visit(value.Index(index))
			}
		}
	}
	visit(reflect.ValueOf(program))
}
