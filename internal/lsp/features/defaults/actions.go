package defaults

import (
	"strings"

	"sec/internal/ast"
	"sec/internal/lexer"
	"sec/internal/sema"
)

// Action is an opt-in UTF-8 replacement, independent of LSP transport.
type Action struct {
	Title      string
	Site       Site
	Start, End int
	Text       string
}

// Actions expands only canonical defaults established by the selected analyzer.
// Explicit type defaults are offered only where the frontend can materialize
// their expression. The LSP adapter reanalyzes each proposed source edit.
// Rules: rules/types/default_values.md — "LSP", "Defaults and contracts",
// "Explicit type defaults", "Struct spread and defaults".
func Actions(source string, analyzer *sema.Analyzer, sites []Site) []Action {
	var actions []Action
	for _, site := range sites {
		switch node := site.Node.(type) {
		case *ast.LetStatement:
			if !node.SynthesizedDefault {
				continue
			}
			binding, ok := analyzer.ResolvedBindingOf(node.Name)
			if !ok {
				continue
			}
			if value, ok := sourceDefault(binding.Type); ok {
				actions = append(actions, Action{Title: "Insert explicit default for " + node.Name.Value, Site: site, Start: site.End, End: site.End, Text: " := " + value})
			}
		case *ast.TypeDeclStatement:
			typ, ok := analyzer.Types()[node.Name.Value]
			if !ok || typ.InvalidExplicitDefault {
				continue
			}
			// The maintained frontend evaluates explicit scalar type defaults;
			// aggregate default expressions await its separate CTE executor.
			switch typ.Kind {
			case sema.IntType, sema.UintType, sema.FloatType, sema.DecimalType, sema.BoolType, sema.StringType, sema.CharType, sema.RuneType:
				if value, ok := sourceDefault(typ); ok {
					actions = append(actions, Action{Title: "Declare explicit default for " + node.Name.Value, Site: site, Start: site.End, End: site.End, Text: " default " + value})
				}
			}
		case *ast.StructLiteral:
			plan, ok := analyzer.ResolvedStructLiteralPlanOf(node)
			if !ok || !plan.FullyInitialized || site.End <= node.Open.ByteEnd {
				continue
			}
			var fields []string
			valid := true
			for _, field := range plan.FinalFields {
				if field.SourceKind != sema.StructFieldSourceDefault {
					continue
				}
				value, ok := sourceDefault(field.FieldType)
				if !ok {
					valid = false
					break
				}
				fields = append(fields, field.FieldName+": "+value)
			}
			if !valid || len(fields) == 0 {
				continue
			}
			// Append after all source entries so spread and explicit-field
			// evaluation order stays intact; only omitted final fields change.
			at := node.Open.ByteEnd
			prefix := " "
			if site.SourceEntries > 0 {
				at = site.End - 1
				last := lastSourceToken(source, node.Open.ByteEnd, at)
				prefix = ", "
				if last == lexer.COMMA {
					prefix = " "
				}
			}
			actions = append(actions, Action{Title: "Expand defaulted fields", Site: site, Start: at, End: at, Text: prefix + strings.Join(fields, ", ") + ", "})
		}
	}
	return actions
}
