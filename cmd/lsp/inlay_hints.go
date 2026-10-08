package main

import (
	"encoding/json"
	"sort"
	"strconv"
	"strings"

	"sec/internal/ast"
	"sec/internal/lexer"
	"sec/internal/sema"
)

type inlayHintParams struct {
	TextDocument textDocumentIdentifier `json:"textDocument"`
	Range        lspRange               `json:"range"`
}

// inlayHint is the LSP InlayHint; Kind 1 is Type and 2 is Parameter.
type inlayHint struct {
	Position     position `json:"position"`
	Label        string   `json:"label"`
	Kind         int      `json:"kind"`
	PaddingLeft  bool     `json:"paddingLeft,omitempty"`
	PaddingRight bool     `json:"paddingRight,omitempty"`
}

const (
	inlayHintKindType      = 1
	inlayHintKindParameter = 2
)

// inlayHintSettings are the individually configurable hint categories
// (`sec.inlayHints.types`, `sec.inlayHints.parameters`,
// `sec.inlayHints.ownership`).
type inlayHintSettings struct {
	Types      bool `json:"types"`
	Parameters bool `json:"parameters"`
	Ownership  bool `json:"ownership"`
}

func defaultInlayHintSettings() inlayHintSettings {
	return inlayHintSettings{Types: true, Parameters: true, Ownership: true}
}

// inlayHintSettingsFrom reads `{"inlayHints": {...}}` from initialization
// options or from the `sec` section of a configuration change, keeping the
// current value of every category the payload does not mention.
func inlayHintSettingsFrom(current inlayHintSettings, raw json.RawMessage) inlayHintSettings {
	if len(raw) == 0 {
		return current
	}
	var payload struct {
		InlayHints *inlayHintSettingsPayload `json:"inlayHints"`
		Sec        *struct {
			InlayHints *inlayHintSettingsPayload `json:"inlayHints"`
		} `json:"sec"`
	}
	if json.Unmarshal(raw, &payload) != nil {
		return current
	}
	hints := payload.InlayHints
	if hints == nil && payload.Sec != nil {
		hints = payload.Sec.InlayHints
	}
	if hints == nil {
		return current
	}
	if hints.Types != nil {
		current.Types = *hints.Types
	}
	if hints.Parameters != nil {
		current.Parameters = *hints.Parameters
	}
	if hints.Ownership != nil {
		current.Ownership = *hints.Ownership
	}
	return current
}

type inlayHintSettingsPayload struct {
	Types      *bool `json:"types"`
	Parameters *bool `json:"parameters"`
	Ownership  *bool `json:"ownership"`
}

// inlayHintsForSource answers textDocument/inlayHint from Sema facts:
//
//   - inferred type: `: T` after the name of a `let` without a declared type,
//     except when the initializer already spells the type (a struct literal,
//     `new`, or a conversion call named like the type);
//   - parameter name: `name:` before each argument of a resolved call with
//     two or more arguments, except where the argument is a name that
//     already equals the parameter name.
//
// Hints never change source and are left out when Sema has no fact, so they
// do not overwhelm the source.
//
// Rules:
//   - rules/tooling/lsp.md — "Inlay hints", A.16 "Implement signature help and inlay hints"
func inlayHintsForSource(uri string, text string, requested lspRange, settings inlayHintSettings, overlays ...sourceOverlay) (hints []inlayHint) {
	hints = []inlayHint{}
	defer func() {
		if recover() != nil {
			hints = []inlayHint{}
		}
	}()
	if !settings.Types && !settings.Parameters && !settings.Ownership {
		return hints
	}
	program := parseProgramForLSP(uri, text)
	if program == nil {
		return hints
	}
	path := pathFromURI(uri)
	prepareProgramForLSP(program, path, firstSourceOverlay(overlays))
	analyzer := newLSPAnalyzerWithOverlay(uri, program, firstSourceOverlay(overlays))
	analyzer.Analyze(program)

	inRange := func(at position) bool {
		if at.Line < requested.Start.Line || at.Line > requested.End.Line {
			return false
		}
		if at.Line == requested.Start.Line && at.Character < requested.Start.Character {
			return false
		}
		return at.Line != requested.End.Line || at.Character <= requested.End.Character
	}
	inThisFile := func(token lexer.Token) bool {
		return token.File == "" || path == "" || normalizedSourcePath(token.File) == normalizedSourcePath(path)
	}
	seen := map[string]bool{}
	add := func(hint inlayHint) {
		key := hint.Label + "@" + positionKey(hint.Position)
		if !inRange(hint.Position) || seen[key] {
			return
		}
		seen[key] = true
		hints = append(hints, hint)
	}

	walkLSPNodes(program, func(node any) {
		switch node := node.(type) {
		case *ast.LetStatement:
			if node == nil || node.Name == nil || node.Value == nil || !inThisFile(node.Name.Token) {
				return
			}
			if settings.Ownership && node.Ownership == "" {
				if hint, ok := semanticCopyHint(analyzer, text, node.Value); ok {
					add(hint)
				}
			}
			if !settings.Types || node.Type != nil || node.Name.Value == "_" {
				return
			}
			binding, ok := analyzer.ResolvedBindingAt(node.Name.Token.File, node.Name.Token.Line, node.Name.Token.Column)
			if !ok || binding.Type.Kind == sema.InvalidType || binding.Type.Kind == "" {
				return
			}
			name := lspTypeName(binding.Type)
			if name == "" || initializerSpellsType(node.Value, binding.Type) {
				return
			}
			add(inlayHint{Position: tokenRange(text, node.Name.Token).End, Label: ": " + name, Kind: inlayHintKindType})
		case *ast.MatchExpression:
			if settings.Ownership && node != nil && inThisFile(node.Token) {
				for _, hint := range matchBindingOwnershipHints(analyzer, text, node) {
					add(hint)
				}
			}
		case *ast.CallExpression:
			if node == nil || !inThisFile(node.Token) {
				return
			}
			resolved, ok := analyzer.ResolvedCallTarget(node)
			if !ok {
				return
			}
			if settings.Ownership {
				for _, hint := range callArgumentOwnershipHints(analyzer, text, node, resolved.Function) {
					add(hint)
				}
			}
			if !settings.Parameters || len(node.Arguments) < 2 {
				return
			}
			parameters := make([]sema.FunctionParameter, 0, len(resolved.Function.Parameters))
			for _, parameter := range resolved.Function.Parameters {
				if parameter.Name != "self" {
					parameters = append(parameters, parameter)
				}
			}
			for index, argument := range node.Arguments {
				if index >= len(parameters) || parameters[index].Variadic {
					break
				}
				if _, spread := argument.(*ast.SpreadExpression); spread {
					break
				}
				name := parameters[index].Name
				if name == "" || argumentNamedLikeParameter(argument, name) {
					continue
				}
				token := leftmostExpressionToken(argument)
				if !validDefinitionTokenForHint(token) {
					continue
				}
				add(inlayHint{Position: tokenRange(text, token).Start, Label: name + ":", Kind: inlayHintKindParameter, PaddingRight: true})
			}
		}
	})
	sort.SliceStable(hints, func(i, j int) bool {
		if hints[i].Position.Line != hints[j].Position.Line {
			return hints[i].Position.Line < hints[j].Position.Line
		}
		return hints[i].Position.Character < hints[j].Position.Character
	})
	return hints
}

func positionKey(at position) string {
	return strconv.Itoa(at.Line) + ":" + strconv.Itoa(at.Character)
}

func validDefinitionTokenForHint(token lexer.Token) bool {
	return token.Line > 0 && token.Column > 0
}

// initializerSpellsType reports an initializer whose source already names
// the inferred type, where a type hint would only repeat it.
func initializerSpellsType(value ast.Expression, typ sema.Type) bool {
	switch value := value.(type) {
	case *ast.StructLiteral, *ast.NewExpression:
		return true
	case *ast.CallExpression:
		if callee, ok := value.Callee.(*ast.Identifier); ok && callee != nil {
			return callee.Value == typ.Name
		}
	}
	return false
}

// argumentNamedLikeParameter reports an argument whose final name already
// equals the parameter name, such as `count` or `self.count` for `count`.
func argumentNamedLikeParameter(argument ast.Expression, parameter string) bool {
	switch argument := argument.(type) {
	case *ast.Identifier:
		return argument != nil && strings.EqualFold(argument.Value, parameter)
	case *ast.MemberExpression:
		return argument != nil && argument.Property != nil && strings.EqualFold(argument.Property.Value, parameter)
	case *ast.RefExpression:
		return argument != nil && argumentNamedLikeParameter(argument.Value, parameter)
	}
	return false
}

// callArgumentOwnershipHints shows the ownership effect of arguments that the
// call site does not spell: `ref` or `ref mut` before a plain place passed to
// a reference parameter (an implicit call-bounded borrow), and `copy` before a
// plain place passed by value whose type copies semantically. Explicit `<-`
// and `ref` arguments, temporaries, existing references, and trivially
// copyable values get no hint.
//
// Rules:
//   - rules/memory/borrowing.md — § 15.2 "Call-site borrow creation", § 33(4)
//   - rules/memory/copy_move.md — copy classification
//   - rules/tooling/lsp.md — "Inlay hints" (ownership)
func callArgumentOwnershipHints(analyzer *sema.Analyzer, text string, call *ast.CallExpression, function sema.Function) []inlayHint {
	hints := []inlayHint{}
	parameters := make([]sema.FunctionParameter, 0, len(function.Parameters))
	for _, parameter := range function.Parameters {
		if parameter.Name != "self" {
			parameters = append(parameters, parameter)
		}
	}
	for index, argument := range call.Arguments {
		if index >= len(parameters) || parameters[index].Variadic {
			break
		}
		if !ownershipPlaceExpression(argument) {
			continue
		}
		parameter := parameters[index]
		argumentType, ok := subjectExpressionType(analyzer, argument)
		if !ok || argumentType.Kind == sema.ReferenceType {
			continue
		}
		label := ""
		switch {
		case parameter.MutableRef || (parameter.Type.Kind == sema.ReferenceType && parameter.Type.ReferenceMutable):
			label = "ref mut"
		case parameter.Ref || parameter.Type.Kind == sema.ReferenceType:
			label = "ref"
		case !parameter.Consuming && sema.CopyClassificationOf(argumentType) == sema.CopySemantic:
			label = "copy"
		default:
			continue
		}
		token := leftmostExpressionToken(argument)
		if !validDefinitionTokenForHint(token) {
			continue
		}
		hints = append(hints, inlayHint{Position: tokenRange(text, token).Start, Label: label, PaddingRight: true})
	}
	return hints
}

// semanticCopyHint shows `copy` before a plain place initializer whose type
// copies semantically, such as `let b := a` for a string.
func semanticCopyHint(analyzer *sema.Analyzer, text string, value ast.Expression) (inlayHint, bool) {
	if !ownershipPlaceExpression(value) {
		return inlayHint{}, false
	}
	typ, ok := subjectExpressionType(analyzer, value)
	if !ok || sema.CopyClassificationOf(typ) != sema.CopySemantic {
		return inlayHint{}, false
	}
	token := leftmostExpressionToken(value)
	if !validDefinitionTokenForHint(token) {
		return inlayHint{}, false
	}
	return inlayHint{Position: tokenRange(text, token).Start, Label: "copy", PaddingRight: true}, true
}

// matchBindingOwnershipHints shows the compiler-resolved action of a by-value
// match payload binding after its name: `moves if selected` when the arm
// would move a move-only payload, and `copy` for a semantic copy. The hint is
// placed on the binding, never on the subject, because pattern success alone
// does not move the subject.
//
// Rules:
//   - rules/memory/copy_move.md — § 15.3(2) whole-payload by-value match binding
//   - rules/tooling/lsp.md — "Inlay hints" ("Pattern success alone must not make the subject appear moved")
func matchBindingOwnershipHints(analyzer *sema.Analyzer, text string, match *ast.MatchExpression) []inlayHint {
	hints := []inlayHint{}
	plan, ok := analyzer.ResolvedMatchPlanOf(match)
	if !ok {
		return hints
	}
	for _, resolved := range plan.Arms {
		if resolved.SourceIndex < 0 || resolved.SourceIndex >= len(match.Arms) {
			continue
		}
		arm := match.Arms[resolved.SourceIndex]
		if arm == nil || arm.Pattern == nil || arm.Pattern.Binding == nil || arm.Pattern.Binding.Name == nil || arm.Pattern.Binding.Mode != ast.MatchBindingValue {
			continue
		}
		label := ""
		switch resolved.BindingAction {
		case sema.MatchBindingMove:
			label = "moves if selected"
		case sema.MatchBindingCopySemantic:
			label = "copy"
		default:
			continue
		}
		hints = append(hints, inlayHint{Position: tokenRange(text, arm.Pattern.Binding.Name.Token).End, Label: label, PaddingLeft: true})
	}
	return hints
}

// ownershipPlaceExpression reports a reusable source place: a name, field, or
// indexed element, as opposed to a temporary or an explicit ownership form.
func ownershipPlaceExpression(expression ast.Expression) bool {
	switch expression := expression.(type) {
	case *ast.Identifier:
		return expression != nil
	case *ast.MemberExpression:
		return expression != nil && expression.Property != nil
	case *ast.IndexExpression:
		return expression != nil
	}
	return false
}
