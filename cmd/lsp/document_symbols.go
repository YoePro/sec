package main

import (
	"sec/internal/ast"
	"sec/internal/lexer"
	"strings"
)

// documentSymbolsForSource projects parser-owned declaration categories and scopes.
// Rules: rules/tooling/lsp.md — "Document symbols"; rules/declarations/static.md — §§6,9.
func documentSymbolsForSource(uri string, text string) []documentSymbol {
	program := parseProgramForLSP(uri, text)
	if program == nil {
		return []documentSymbol{}
	}
	symbols := []documentSymbol{}
	for _, stmt := range program.Statements {
		if symbol, ok := documentSymbolForStatement(text, stmt); ok {
			symbols = append(symbols, symbol)
		}
	}
	return symbols
}

// documentSymbolForStatement renders one declaration with its parser-owned kind and children.
// Rules: rules/tooling/lsp.md — "Document symbols"; rules/declarations/static.md — §§6,9.
func documentSymbolForStatement(text string, stmt ast.Statement) (documentSymbol, bool) {
	switch stmt := stmt.(type) {
	case *ast.ModuleStatement:
		if stmt == nil {
			return documentSymbol{}, false
		}
		return namedDocumentSymbol(text, stmt.Path, "module", 2, stmt.Token, stmt.Token), true
	case *ast.TypeDeclStatement:
		if stmt == nil || stmt.Name == nil {
			return documentSymbol{}, false
		}
		kind := 5
		detail := "type"
		if stmt.StructType != nil {
			kind = 23
			detail = "struct"
		} else if stmt.Union {
			kind = 23
			detail = "union"
		} else if len(stmt.Variants) > 0 {
			kind = 10
			detail = "enum"
		}
		symbol := namedDocumentSymbol(text, stmt.Name.Value, detail, kind, stmt.Token, stmt.Name.Token)
		if stmt.StructType != nil {
			for _, field := range stmt.StructType.Fields {
				if field != nil && field.Name != nil {
					symbol.Children = append(symbol.Children, namedDocumentSymbol(text, field.Name.Value, typeReferenceName(field.Type), 8, field.Token, field.Name.Token))
				}
			}
		}
		for _, variant := range stmt.Variants {
			if variant != nil {
				symbol.Children = append(symbol.Children, namedDocumentSymbol(text, variant.Value, "enum member", 22, variant.Token, variant.Token))
			}
		}
		for _, variant := range stmt.UnionVariants {
			if variant != nil && variant.Name != nil {
				symbol.Children = append(symbol.Children, namedDocumentSymbol(text, variant.Name.Value, "union member", 22, variant.Token, variant.Name.Token))
			}
		}
		return symbol, true
	case *ast.UnitDeclStatement:
		if stmt == nil || stmt.Name == nil {
			return documentSymbol{}, false
		}
		return namedDocumentSymbol(text, stmt.Name.Value, "unit", 14, stmt.Token, stmt.Name.Token), true
	case *ast.EnumDeclaration:
		return enumDocumentSymbol(text, stmt)
	case *ast.InterfaceDeclaration:
		if stmt == nil || stmt.Name == nil {
			return documentSymbol{}, false
		}
		symbol := namedDocumentSymbol(text, stmt.Name.Value, "interface", 11, stmt.Token, stmt.Name.Token)
		for _, method := range stmt.Methods {
			if method != nil && method.Name != nil {
				symbol.Children = append(symbol.Children, functionDocumentSymbol(text, method, 6))
			}
		}
		for _, property := range stmt.Properties {
			if property != nil && property.Name != nil {
				symbol.Children = append(symbol.Children, namedDocumentSymbol(text, property.Name.Value, propertyDocumentDetail(typeReferenceName(property.Type), property.Static), 7, property.Token, property.Name.Token))
			}
		}
		for _, event := range stmt.Events {
			if event != nil && event.Name != nil {
				symbol.Children = append(symbol.Children, namedDocumentSymbol(text, event.Name.Value, typeReferenceName(event.Payload), 24, event.Token, event.Name.Token))
			}
		}
		return symbol, true
	case *ast.FunctionDeclaration:
		if stmt == nil || stmt.Name == nil {
			return documentSymbol{}, false
		}
		return functionDocumentSymbol(text, stmt, 12), true
	case *ast.StructStatement:
		if stmt == nil || stmt.Name == nil {
			return documentSymbol{}, false
		}
		symbol := namedDocumentSymbol(text, stmt.Name.Value, "struct", 23, stmt.Token, stmt.Name.Token)
		for _, field := range stmt.Fields {
			if field.Name != nil {
				symbol.Children = append(symbol.Children, namedDocumentSymbol(text, field.Name.Value, typeReferenceName(field.Type), 8, field.Token, field.Name.Token))
			}
		}
		return symbol, true
	case *ast.LetStatement:
		if stmt == nil || stmt.Name == nil {
			return documentSymbol{}, false
		}
		detail := typeReferenceName(stmt.Type)
		if stmt.Static {
			detail = "static let " + detail
			if stmt.Mutable {
				detail = "static let mut " + typeReferenceName(stmt.Type)
			}
		}
		return namedDocumentSymbol(text, stmt.Name.Value, detail, 13, stmt.Token, stmt.Name.Token), true
	case *ast.LetGroupStatement:
		if stmt == nil || len(stmt.Lets) == 0 {
			return documentSymbol{}, false
		}
		group := namedDocumentSymbol(text, "let", "variables", 13, stmt.Token, stmt.Token)
		for _, let := range stmt.Lets {
			if let != nil && let.Name != nil {
				group.Children = append(group.Children, namedDocumentSymbol(text, let.Name.Value, typeReferenceName(let.Type), 13, let.Token, let.Name.Token))
			}
		}
		return group, true
	case *ast.ImplStatement:
		if stmt == nil || stmt.Target == nil {
			return documentSymbol{}, false
		}
		name := "impl " + stmt.Target.Name
		if stmt.Extends {
			name = "impl extends " + stmt.Target.Name
		} else if len(stmt.Implements) > 0 {
			interfaces := make([]string, 0, len(stmt.Implements))
			for _, ref := range stmt.Implements {
				interfaces = append(interfaces, typeReferenceName(ref))
			}
			name += " implements " + strings.Join(interfaces, ", ")
		}
		symbol := namedDocumentSymbol(text, name, "impl", 3, stmt.Token, stmt.Target.Token)
		for _, member := range stmt.Members {
			if child, ok := documentSymbolForImplMember(text, member); ok {
				symbol.Children = append(symbol.Children, child)
			}
		}
		return symbol, true
	default:
		return documentSymbol{}, false
	}
}

// documentSymbolForImplMember retains impl member categories in the nested outline.
// Rules: rules/tooling/lsp.md — "Document symbols"; rules/declarations/static.md — §§6,9.
func documentSymbolForImplMember(text string, member ast.ImplMember) (documentSymbol, bool) {
	switch member := member.(type) {
	case *ast.FunctionDeclaration:
		if member == nil || member.Name == nil {
			return documentSymbol{}, false
		}
		return functionDocumentSymbol(text, member, 6), true
	case *ast.InitDeclaration:
		if member == nil {
			return documentSymbol{}, false
		}
		detail := "constructs"
		if member.ErrorType != nil {
			detail += "; error " + typeReferenceName(member.ErrorType)
		}
		return namedDocumentSymbol(text, "init", detail, 9, member.Token, member.Token), true
	case *ast.FreeDeclaration:
		if member == nil {
			return documentSymbol{}, false
		}
		return namedDocumentSymbol(text, "free", "custom destruction", 9, member.Token, member.Token), true
	case *ast.PropertyDeclaration:
		if member == nil || member.Name == nil {
			return documentSymbol{}, false
		}
		return propertyDocumentSymbol(text, member), true
	case *ast.EventDeclaration:
		if member == nil || member.Name == nil {
			return documentSymbol{}, false
		}
		return namedDocumentSymbol(text, member.Name.Value, "event", 24, member.Token, member.Name.Token), true
	case *ast.TypeDeclStatement:
		return documentSymbolForStatement(text, member)
	case *ast.UnitDeclStatement:
		return documentSymbolForStatement(text, member)
	case *ast.EnumDeclaration:
		return documentSymbolForStatement(text, member)
	case *ast.LetStatement:
		return documentSymbolForStatement(text, member)
	default:
		return documentSymbol{}, false
	}
}

// functionDocumentSymbol retains the callable result and receiver category.
// Rules: rules/tooling/lsp.md — "Document symbols"; rules/declarations/static.md — §§6,9.
func functionDocumentSymbol(text string, fn *ast.FunctionDeclaration, kind int) documentSymbol {
	detail := "fn"
	if fn.Static {
		detail = "static fn"
	}
	if fn.ReturnType != nil {
		detail += " " + typeReferenceName(fn.ReturnType)
	}
	return namedDocumentSymbol(text, fn.Name.Value, detail, kind, fn.Token, fn.Name.Token)
}

// namedDocumentSymbol keeps the selection range inside the declaration range.
// Rules: rules/tooling/lsp.md — "Document symbols"; rules/declarations/static.md — §§6,9.
func namedDocumentSymbol(text string, name string, detail string, kind int, token lexer.Token, selectionToken lexer.Token) documentSymbol {
	selectionRange := tokenRange(text, selectionToken)
	return documentSymbol{
		Name:           name,
		Detail:         detail,
		Kind:           kind,
		Range:          rangeContaining(tokenRange(text, token), selectionRange),
		SelectionRange: selectionRange,
	}
}

// propertyDocumentSymbol retains the accessor declaration's receiver category.
// Rules: rules/declarations/properties.md — §§8,10; rules/tooling/lsp.md — "Document symbols".
func propertyDocumentSymbol(text string, property *ast.PropertyDeclaration) documentSymbol {
	detail := propertyDocumentDetail(typeReferenceName(property.Type), property.Static)
	return namedDocumentSymbol(text, property.Name.Value, detail, 7, property.Token, property.Name.Token)
}

// propertyDocumentDetail preserves the static/instance category in the outline.
// Rules: rules/declarations/properties.md — §§8,10; rules/tooling/lsp.md — "Document symbols".
func propertyDocumentDetail(detail string, static bool) string {
	if static {
		return "static property " + detail
	}
	return detail
}
