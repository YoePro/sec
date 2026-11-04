package cst

import (
	"reflect"
	"sort"
	"strings"

	"sec/internal/ast"
	"sec/internal/lexer"
)

// VerticalBoundary is a whitespace-only sibling boundary. Structured requests
// separation around aggregate groups or before a documented major declaration;
// Required preserves function/comment separation in every style;
// Together groups consecutive defer blocks. Compact avoids adding optional
// separation while preserving an existing declaration gap (§7(4)).
// Rules: rules/tooling/formatter.md — §7(3–12),§12,§29(4).
type VerticalBoundary struct {
	Whitespace int
	Required   bool
	Structured bool
	Compact    bool
	Together   bool
}

// VerticalBoundaries projects sibling relationships and parser-owned comment
// attachments onto real whitespace. Detached/inline comments protect their
// boundaries, so changing style cannot manufacture a documentation attachment.
// Rules: rules/tooling/formatter.md — §5(2–6),§7(3–12),§12,§25.
func (d Document) VerticalBoundaries(program *ast.Program) []VerticalBoundary {
	var result []VerticalBoundary
	if program == nil {
		return result
	}
	indexes := map[int]int{}
	for index, element := range d.Elements {
		indexes[element.Span.Start] = index
	}
	leading := map[ast.Node]int{}
	documented := map[ast.Node]bool{}
	placements := map[int]ast.CommentPlacement{}
	for _, attachment := range program.Comments {
		for _, token := range attachment.Comments {
			placements[token.ByteStart] = attachment.Placement
		}
		if attachment.Target != nil && len(attachment.Comments) > 0 && (attachment.Placement == ast.CommentLeading || attachment.Placement == ast.CommentDocumentation) {
			start := attachment.Comments[0].ByteStart
			if previous, ok := leading[attachment.Target]; !ok || start < previous {
				leading[attachment.Target] = start
			}
		}
	}
	for _, attachment := range program.Documentation {
		documented[attachment.Declaration] = true
		if len(attachment.Comments) > 0 {
			start := attachment.Comments[0].ByteStart
			if previous, ok := leading[attachment.Declaration]; !ok || start < previous {
				leading[attachment.Declaration] = start
			}
		}
	}
	// Required function/comment separation precedes optional sibling boundaries,
	// so compact style cannot erase it. No following declaration is needed.
	// Rules: rules/tooling/formatter.md — §7(7a),§12,§29(4).
	visitProgramNodes(program, func(node any) {
		var body *ast.BlockStatement
		switch node := node.(type) {
		case *ast.FunctionDeclaration:
			body = node.Body
		case *ast.LambdaExpression:
			body = node.Body
		}
		if body == nil || body.EndToken.Type != lexer.RBRACE {
			return
		}
		close, ok := indexes[body.EndToken.ByteStart]
		if !ok {
			return
		}
		gap := close + 1
		// A trailing comment is part of the function's closing line.
		trailing := gap
		if trailing < len(d.Elements) && d.Elements[trailing].Kind == Whitespace && !strings.ContainsAny(d.Elements[trailing].Text, "\r\n") {
			trailing++
		}
		if trailing < len(d.Elements) && d.Elements[trailing].Kind == Comment && placements[d.Elements[trailing].Span.Start] == ast.CommentTrailing {
			gap = trailing + 1
		}
		if gap+1 < len(d.Elements) && d.Elements[gap].Kind == Whitespace &&
			strings.ContainsAny(d.Elements[gap].Text, "\r\n") && d.Elements[gap+1].Kind == Comment {
			result = append(result, VerticalBoundary{Whitespace: gap, Required: true})
		}
	})
	group := func(statements []ast.Node) {
		for index := 1; index < len(statements); index++ {
			previous, current := statements[index-1], statements[index]
			if verticalHeader(previous) || verticalHeader(current) {
				continue // Module/import separation belongs to its dedicated rules.
			}
			start := verticalNodeStart(reflect.ValueOf(current))
			if first, ok := indexes[start]; ok {
				for first >= 2 && d.Elements[first-1].Kind == Whitespace && d.Elements[first-2].Token.Line == d.Elements[first].Token.Line {
					switch d.Elements[first-2].Token.Type {
					case lexer.STATIC, lexer.MUT, lexer.CONSUME_ARROW:
						first -= 2
						start = d.Elements[first].Span.Start
					default:
						first = 0
					}
				}
			}
			if attached, ok := leading[current]; ok && attached < start {
				start = attached
			}
			first, ok := indexes[start]
			if !ok || first < 2 || d.Elements[first-1].Kind != Whitespace {
				continue
			}
			before := d.Elements[first-2]
			if before.Kind == Comment && placements[before.Span.Start] != ast.CommentTrailing {
				continue
			}
			if before.Kind != Token && before.Kind != Comment {
				continue
			}
			_, previousDefer := previous.(*ast.DeferStatement)
			_, currentDefer := current.(*ast.DeferStatement)
			result = append(result, VerticalBoundary{Whitespace: first - 1,
				Structured: verticalAggregate(previous) || verticalAggregate(current) || (documented[current] && verticalMajor(current)),
				Compact:    verticalMajor(previous) || verticalMajor(current),
				Together:   previousDefer && currentDefer && start == verticalNodeStart(reflect.ValueOf(current)) && before.Kind == Token,
			})
		}
	}
	nodes := func(statements []ast.Statement) []ast.Node {
		result := make([]ast.Node, len(statements))
		for i, statement := range statements {
			result[i] = statement
		}
		return result
	}
	group(nodes(program.Statements))
	visitProgramNodes(program, func(node any) {
		switch node := node.(type) {
		case *ast.BlockStatement:
			group(nodes(node.Statements))
		case *ast.ImplStatement:
			statements := []ast.Node{}
			for _, member := range node.Members {
				statements = append(statements, member)
			}
			group(statements)
		case *ast.InterfaceDeclaration:
			members := []ast.Node{}
			for _, method := range node.Methods {
				members = append(members, method)
			}
			for _, property := range node.Properties {
				members = append(members, property)
			}
			for _, event := range node.Events {
				members = append(members, event)
			}
			sort.Slice(members, func(i, j int) bool {
				return verticalNodeStart(reflect.ValueOf(members[i])) < verticalNodeStart(reflect.ValueOf(members[j]))
			})
			group(members)
		}
	})
	return result
}

// verticalNodeStart finds the earliest retained syntax token, including
// attributes. It never infers declaration boundaries from source spelling.
// Rules: rules/tooling/formatter.md — §5,§16(14–17).
func verticalNodeStart(value reflect.Value) int {
	if !value.IsValid() {
		return -1
	}
	if value.Type() == reflect.TypeOf(lexer.Token{}) {
		token := value.Interface().(lexer.Token)
		if token.ByteEnd > token.ByteStart {
			return token.ByteStart
		}
		return -1
	}
	start := -1
	visit := func(child reflect.Value) {
		if next := verticalNodeStart(child); next >= 0 && (start < 0 || next < start) {
			start = next
		}
	}
	switch value.Kind() {
	case reflect.Interface, reflect.Pointer:
		if !value.IsNil() {
			visit(value.Elem())
		}
	case reflect.Struct:
		for i := 0; i < value.NumField(); i++ {
			if value.Type().Field(i).IsExported() {
				visit(value.Field(i))
			}
		}
	case reflect.Slice, reflect.Array:
		for i := 0; i < value.Len(); i++ {
			visit(value.Index(i))
		}
	}
	return start
}

// verticalHeader keeps dedicated module/import separation outside this policy.
// Rules: rules/tooling/formatter.md — §13,§14.
func verticalHeader(node ast.Node) bool {
	switch node.(type) {
	case *ast.ModuleStatement, *ast.ImportStatement:
		return true
	}
	return false
}

// verticalAggregate identifies grammar-owned aggregate and group declarations.
// Rules: rules/tooling/formatter.md — §7(5–6),§15.
func verticalAggregate(node ast.Node) bool {
	switch node := node.(type) {
	case *ast.TypeDeclStatement:
		return node.StructType != nil || node.RegisterType != nil || node.Union || len(node.Variants) > 0
	case *ast.EnumDeclaration, *ast.InterfaceDeclaration, *ast.ImplStatement:
		return true
	}
	return false
}

// verticalMajor selects documented declarations requiring structured separation.
// Rules: rules/tooling/formatter.md — §7(7–8).
func verticalMajor(node ast.Node) bool {
	switch node.(type) {
	case *ast.FunctionDeclaration, *ast.InterfaceProperty, *ast.InterfaceEvent, *ast.PropertyDeclaration, *ast.TestDeclaration, *ast.InitDeclaration, *ast.FreeDeclaration, *ast.EventDeclaration:
		return true
	}
	return verticalAggregate(node)
}
