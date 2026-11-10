package sema

import (
	"sec/internal/ast"
	"sec/internal/lexer"
)

// analyzePropertyBody checks each declared accessor independently.
// Rules: rules/declarations/properties.md — §§4–6,8,10,14;
// rules/errors/errorhandling.md — §24; rules/declarations/static.md — §§9,12.
func (a *Analyzer) analyzePropertyBody(target Type, property *ast.PropertyDeclaration, propertyType Type) {
	if property.Getter != nil {
		a.analyzeGetterBody(target, property, propertyType)
	}
	if property.Setter != nil {
		a.analyzeSetterBody(target, property, propertyType)
	}
}

// analyzeGetterBody checks the getter return contract and inferred effects.
// Rules: rules/declarations/properties.md — §§4–6,8,10,14;
// rules/errors/errorhandling.md — §24; rules/declarations/static.md — §§9,12.
func (a *Analyzer) analyzeGetterBody(target Type, property *ast.PropertyDeclaration, propertyType Type) {
	restore := a.enterPropertyAccessor(target, property.Name.Value, "get", property.GetterAttributes, property.Getter)
	a.analyzePropertyAccessorBody(target, property.Name.Value+".get", property.Getter, propertyType, a.currentFunctionMetadata.ReceiverMutable, property.Static, nil, Type{})
	restore()
	if !a.blockDefinitelyReturns(property.Getter) {
		a.addErrorAtToken(property.Name.Token, "getter %s must return %s", property.Name.Value, typeDisplayName(propertyType))
	}
}

// analyzeSetterBody checks the setter parameter, error contract and effects.
// Rules: rules/declarations/properties.md — §§4–6,8,10,14;
// rules/errors/errorhandling.md — §24; rules/declarations/static.md — §§9,12.
func (a *Analyzer) analyzeSetterBody(target Type, property *ast.PropertyDeclaration, propertyType Type) {
	// rules/compiler/parser_recovery.md, "Missing setter parameter" and
	// "Interaction with Sema": retain the body for tooling but do not invent a
	// parameter or emit dependent semantic diagnostics beneath an invalid setter.
	if property.Setter.Invalid {
		return
	}
	if !property.Setter.Fallible && blockReturnsErr(property.Setter.Body) {
		a.addErrorAtToken(property.Setter.Token, "non-fallible setter %s cannot return Err", property.Name.Value)
	}
	returnType := Type{Name: "void", Kind: VoidType}
	if property.Setter.Fallible {
		// rules/errors/errorhandling.md — §24: the declared contract is the
		// only source of the setter error type; it is never inferred.
		registered, ok := lookupProperty(target, property.Name.Value)
		if !ok || registered.Error == nil {
			return
		}
		returnType = Type{Name: "Result", Kind: ResultType, TypeArgs: []Type{{Name: "void", Kind: VoidType}, *registered.Error}}
		previous := a.fallibleSetterBody
		a.fallibleSetterBody = true
		defer func() { a.fallibleSetterBody = previous }()
	}
	accessor := "set"
	if property.Setter.Fallible {
		accessor = "try set"
	}
	restore := a.enterPropertyAccessor(target, property.Name.Value, accessor, property.Setter.Attributes, property.Setter.Body)
	a.analyzePropertyAccessorBody(target, property.Name.Value+".set", property.Setter.Body, returnType, true, property.Static, property.Setter.Parameter, propertyType)
	restore()
}

// analyzePropertyAccessorBody analyzes one accessor in an isolated receiver and local scope.
// Rules: rules/declarations/properties.md — §§4–6,8,10,14;
// rules/errors/errorhandling.md — §24; rules/declarations/static.md — §§9,12.
func (a *Analyzer) analyzePropertyAccessorBody(target Type, name string, body *ast.BlockStatement, returnType Type, mutableSelf, static bool, setterParameter *ast.Identifier, setterType Type) {
	if body == nil || returnType.Kind == InvalidType {
		return
	}

	previousSymbols := a.symbols
	previousConstInts := a.constInts
	previousAssigned := a.assigned
	previousMoved := a.moved
	previousMoveReasons := a.availabilityStates
	previousBorrows := a.borrows
	previousLocalRefContainers := a.localRefContainers
	previousArenaGenerations := a.arenaGenerations
	previousFunctionName := a.currentFunctionName
	previousFunctionReturn := a.currentFunctionReturn
	previousInFunctionBody := a.inFunctionBody
	previousScopeDepth := a.scopeDepth
	a.symbols = previousSymbols.child()
	a.scopeModuleVariables()
	a.constInts = copyConstInts(previousConstInts)
	a.assigned = copyAssigned(previousAssigned)
	a.moved = map[string]lexer.Token{}
	a.availabilityStates = placeAvailabilityStates{}
	a.borrows = a.staticHolderBorrows()
	a.localRefContainers = map[string]localReferenceOrigin{}
	a.arenaGenerations = map[string]int{}
	a.currentFunctionName = name
	a.currentFunctionReturn = returnType
	a.inFunctionBody = true
	a.scopeDepth = 0
	defer func() {
		a.symbols = previousSymbols
		a.constInts = previousConstInts
		a.assigned = previousAssigned
		a.moved = previousMoved
		a.availabilityStates = previousMoveReasons
		a.borrows = previousBorrows
		a.localRefContainers = previousLocalRefContainers
		a.arenaGenerations = previousArenaGenerations
		a.currentFunctionName = previousFunctionName
		a.currentFunctionReturn = previousFunctionReturn
		a.inFunctionBody = previousInFunctionBody
		a.scopeDepth = previousScopeDepth
	}()

	// rules/declarations/static.md, sections 9 and 12. Static accessors have
	// no implicit receiver and cannot see instance fields or properties.
	if !static {
		a.defineInstanceSymbols(target, mutableSelf, body.Token)
		a.recordReceiverFacts(a.currentFunctionMetadata)
	}
	if setterParameter != nil {
		// rules/foundations/names_scopes_visibility.md §8: the setter value
		// parameter must not silently replace a visible declaration.
		a.validateParameterTypeShadowing(setterParameter)
		a.symbols.set(setterParameter.Value, Symbol{Name: setterParameter.Value, Type: setterType, Mutable: false, Token: setterParameter.Token, Storage: StorageOriginAutomatic, Local: true, ScopeDepth: 0})
		a.assigned[setterParameter.Value] = true
		delete(a.constInts, setterParameter.Value)
	}

	a.analyzeBlockStatements(body)
}
