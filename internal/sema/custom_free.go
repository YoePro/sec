package sema

import (
	"fmt"

	"sec/internal/ast"
	"sec/internal/diagnostics"
	"sec/internal/lexer"
)

func freeFunctionName(target string) string { return "@free:" + target }

// registerCustomFreeDeclarations records every `free` lifecycle member before
// type bodies resolve, so destruction classification and partial-move
// legality see the custom destructor wherever the type is used. A type has at
// most one `free` across its primary and extending impl blocks.
//
// Rules:
//   - rules/declarations/impl.md — §19 "`free`"
//   - rules/memory/destruction.md — §3.3 "Non-trivially destructible", §15.2 "Lifecycle status"
func (a *Analyzer) registerCustomFreeDeclarations(program *ast.Program) {
	a.withProgramModules(program, func(stmt ast.Statement) {
		impl, ok := stmt.(*ast.ImplStatement)
		if !ok || impl == nil || impl.Target == nil {
			return
		}
		for _, member := range impl.Members {
			free, ok := member.(*ast.FreeDeclaration)
			if !ok || free == nil {
				continue
			}
			target, exists := a.types[impl.Target.Name]
			if !exists {
				// Ordinary impl-target resolution reports the unknown type.
				continue
			}
			if target.Kind == InterfaceType {
				a.addErrorAtTokenWithMetadata(free.Token, diagnostics.InvalidCustomFreeDeclaration,
					"Declare free on the concrete type that owns the resource.",
					"interface %s cannot declare free; free belongs to a concrete owning type", impl.Target.Name)
				continue
			}
			if previous, duplicate := a.customFreeDeclarations[impl.Target.Name]; duplicate {
				a.addErrorAtTokenWithMetadataAndPrevious(free.Token, previous, diagnostics.InvalidCustomFreeDeclaration,
					"Merge the cleanup into the single existing free.",
					"type %s already declares free; a type has at most one free across its primary and extending impl blocks", impl.Target.Name)
				continue
			}
			a.customFreeDeclarations[impl.Target.Name] = free.Token
		}
	})
}

// applyCustomFreeDeclarations marks the registered owning types after their
// declarations are fully resolved. Types reached through resolveType are
// marked as they resolve; this pass covers the canonical type table itself.
func (a *Analyzer) applyCustomFreeDeclarations() {
	for name := range a.customFreeDeclarations {
		if typ, ok := a.types[name]; ok {
			typ.CustomFree = true
			a.types[name] = typ
		}
	}
}

// withCustomFree marks a resolved nominal type (or concrete instance of a
// generic nominal type) whose declaration owns a custom `free`.
func (a *Analyzer) withCustomFree(typ Type) Type {
	if typ.Name == "" || typ.CustomFree {
		return typ
	}
	if _, ok := a.customFreeDeclarations[typ.Name]; ok {
		typ.CustomFree = true
	}
	return typ
}

// registerFreeDeclaration gives the free body a non-callable lifecycle
// identity. It is keyed outside the method namespace so `value.free()` never
// resolves, and it has exclusive destruction authority over self.
//
// Rules:
//   - rules/memory/destruction.md — §15.2 "Lifecycle status", §15.3 "`self` during `free`", §15.7 "Fallibility"
func (a *Analyzer) registerFreeDeclaration(targetName string, free *ast.FreeDeclaration) {
	if free == nil || free.Body == nil {
		return
	}
	if token, ok := a.customFreeDeclarations[targetName]; !ok || !sameSourceToken(token, free.Token) {
		// Rejected duplicate or invalid target: the body is not a lifecycle member.
		return
	}
	key := freeFunctionName(targetName)
	before := len(a.functions[key])
	a.registerFunctionDeclarationBody(freeFunctionDeclaration(free), key)
	functions := a.functions[key]
	if len(functions) != before+1 {
		return
	}
	functions[len(functions)-1].CustomFree = true
	functions[len(functions)-1].ReceiverMutable = true
	a.functions[key] = functions
}

func freeFunctionDeclaration(free *ast.FreeDeclaration) *ast.FunctionDeclaration {
	return &ast.FunctionDeclaration{
		Token:      free.Token,
		Name:       &ast.Identifier{Token: free.Token, Value: "free"},
		ReturnType: &ast.TypeReference{Token: free.Token, Name: "void"},
		Body:       free.Body,
	}
}

func (a *Analyzer) analyzeFreeBody(targetName string, free *ast.FreeDeclaration) {
	if free == nil || free.Body == nil {
		return
	}
	a.analyzeFunctionBodyInScope(freeFunctionDeclaration(free), freeFunctionName(targetName))
}

// rejectDeferInsideFree enforces that a free body registers no deferred
// cleanup of its own.
//
// Rules:
//   - rules/memory/destruction.md — §15.6 "`defer` inside `free`"
func (a *Analyzer) rejectDeferInsideFree(token lexer.Token) bool {
	if !a.currentFunctionMetadata.CustomFree {
		return false
	}
	a.addErrorAtTokenWithMetadata(token, diagnostics.DeferInsideFree,
		"Perform the cleanup directly in the free body; locals created inside free are still destroyed automatically.",
		"defer is not allowed inside free")
	return true
}

// rejectFreeWholeSelfConsumption keeps the complete value under the
// compiler's destruction authority while free runs.
//
// Rules:
//   - rules/memory/destruction.md — §15.3 "`self` during `free`"
func (a *Analyzer) rejectFreeWholeSelfConsumption(place Place, token lexer.Token) bool {
	if !a.currentFunctionMetadata.CustomFree || place.Root != "self" || len(place.Projections) != 0 {
		return false
	}
	a.addErrorAtTokenWithMetadata(token, diagnostics.FreeConsumesSelf,
		"Release the resource held by self inside free; destruction ends self after the free body.",
		"free cannot consume, transfer, or resurrect complete self")
	return true
}

// rejectCustomFreePartialMove forbids transferring ownership out of a field
// of a value whose type defines custom free, including self inside free.
//
// Rules:
//   - rules/memory/ownership.md — §19 "Custom `free` forbids partial moves in Sec 0.1"
//   - rules/memory/copy_move.md — §14(4)
//   - rules/memory/destruction.md — §10.3 "Custom `free`", §15.4(3), §33.2(2)
func (a *Analyzer) rejectCustomFreePartialMove(place Place, token lexer.Token) bool {
	if place.CustomFreeOwner == "" {
		return false
	}
	declaration := a.customFreeDeclarations[place.CustomFreeOwner]
	endLine, endColumn := token.EndPosition()
	a.appendError(Error{
		ID:   diagnostics.CustomFreePartialMove,
		File: token.File, Line: token.Line, Column: token.Column,
		EndLine: endLine, EndColumn: endColumn,
		PreviousFile: declaration.File, PreviousLine: declaration.Line, PreviousColumn: declaration.Column,
		RelatedLabel: "custom free requires complete self",
		Help:         "The custom free body requires complete self: every owned field must remain available when cleanup runs. Move the complete value instead, or redesign ownership so the field can be transferred without dismantling a custom-free value.",
		Message: fmt.Sprintf("cannot move %s out of %s because type %s defines custom free; partial moves are not permitted from custom-free types",
			place.String(), place.CustomFreeOwnerPlace, place.CustomFreeOwner),
	})
	return true
}

// customFreeMoveOnlyCause explains a MoveOnly classification that comes from a
// custom free (MD-025.1): ": type Handle defines free, so it is MoveOnly",
// naming the type itself or the owned element, field or payload that carries
// it; "" when free is not the cause.
func customFreeMoveOnlyCause(typ Type) string {
	if name, ok := customFreeTypeIn(typ, map[string]bool{}); ok {
		if name == typeDisplayName(typ) {
			return ": type " + name + " defines free, so it is MoveOnly"
		}
		return ": it owns " + name + ", which defines free and is therefore MoveOnly"
	}
	return ""
}

func customFreeTypeIn(typ Type, visiting map[string]bool) (string, bool) {
	if typ.CustomFree {
		return typeDisplayName(typ), true
	}
	key := typeDestructionKey(typ)
	if key != "" {
		if visiting[key] {
			return "", false
		}
		visiting[key] = true
		defer delete(visiting, key)
	}
	switch typ.Kind {
	case ReferenceType:
		return "", false
	case StructType:
		for _, field := range typ.Fields {
			if name, ok := customFreeTypeIn(field.Type, visiting); ok {
				return name, true
			}
		}
	case UnionType:
		for _, variant := range typ.UnionVariants {
			if variant.Payload != nil {
				if name, ok := customFreeTypeIn(*variant.Payload, visiting); ok {
					return name, true
				}
			}
			for _, field := range variant.PayloadFields {
				if name, ok := customFreeTypeIn(field.Type, visiting); ok {
					return name, true
				}
			}
		}
	}
	if typ.Element != nil {
		return customFreeTypeIn(*typ.Element, visiting)
	}
	for _, argument := range typ.TypeArgs {
		if name, ok := customFreeTypeIn(argument, visiting); ok {
			return name, true
		}
	}
	return "", false
}

// rejectCustomFreeFieldDiscard applies the partial-move restriction to
// discard (MD-025.2): an owned field or nested owned sub-Place of a value
// whose type defines custom free cannot be discarded on its own, whatever the
// field's destruction class, because free relies on a complete value. Inside
// that type's own free the discard would additionally release the field
// before the automatic field destruction that follows free releases it again
// (destruction.md § 15.4(4)). Whole-value discard stays governed by
// discard.md.
func (a *Analyzer) rejectCustomFreeFieldDiscard(place Place, token lexer.Token) bool {
	if place.CustomFreeOwner == "" || len(place.Projections) == 0 {
		return false
	}
	declaration := a.customFreeDeclarations[place.CustomFreeOwner]
	help := "Discard the complete value instead, which runs free and then destroys every field once; a custom-free value must stay complete until its destruction."
	message := fmt.Sprintf("cannot discard owned field %s of %s: type %s defines free and requires a complete owned value during destruction",
		place.String(), place.CustomFreeOwnerPlace, place.CustomFreeOwner)
	if a.currentFunctionMetadata.CustomFree && place.Root == "self" && !TriviallyDestructible(place.Type) {
		help = "Fields of self are destroyed automatically after free completes; releasing one here would release it twice. Remove the discard, or keep the release in the field's own type."
		message = fmt.Sprintf("cannot discard %s inside free: automatic field destruction after free would release it again", place.String())
	}
	endLine, endColumn := token.EndPosition()
	a.appendError(Error{
		ID:   diagnostics.CustomFreePartialMove,
		File: token.File, Line: token.Line, Column: token.Column,
		EndLine: endLine, EndColumn: endColumn,
		PreviousFile: declaration.File, PreviousLine: declaration.Line, PreviousColumn: declaration.Column,
		RelatedLabel: "custom free requires complete self",
		Help:         help,
		Message:      message,
	})
	return true
}
