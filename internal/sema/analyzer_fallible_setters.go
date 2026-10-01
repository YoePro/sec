package sema

import (
	"sort"

	"sec/internal/ast"
	"sec/internal/lexer"
)

// resolveSetterErrorContract resolves the explicit error type of a fallible
// setter. The contract must be written on the setter line and must be error or
// a concrete error type; it is never inferred from the setter body.
//
// Rules:
//   - rules/errors/errorhandling.md — §24 "Fallible property setters"
//   - rules/errors/errorhandling.md — §2.1 "Error assignability"
func (a *Analyzer) resolveSetterErrorContract(reference *ast.TypeReference, setterToken lexer.Token, propertyName string) *Type {
	if reference == nil {
		a.addErrorAtToken(setterToken, "fallible setter %s must declare its error type after the value parameter: try set value ErrorType { ... }", propertyName)
		return nil
	}
	errorType, ok := a.resolveType(reference)
	if !ok || errorType.Kind == InvalidType {
		return nil
	}
	if errorType.Kind != ErrorRootType && !errorType.ErrorAssignable {
		a.addErrorAtToken(reference.Token, "setter error type %s is not an error type; use error or declare %s with the error marker", typeDisplayName(errorType), typeDisplayName(errorType))
		return nil
	}
	return &errorType
}

// recordInterfaceSetterContract remembers the declared error contract of a
// fallible setter requirement and reports a requirement that omits it.
// Resolution is deferred because concrete error types are analyzed after
// interface declarations.
//
// Rules:
//   - rules/errors/errorhandling.md — §24.1 "Interfaces"
func (a *Analyzer) recordInterfaceSetterContract(interfaceName string, property *ast.InterfaceProperty) {
	if !property.RequiresSet || !property.SetterFallible || property.Name == nil {
		return
	}
	if property.SetterErrorType == nil {
		a.addErrorAtToken(property.SetToken, "fallible setter %s must declare its error type after the value parameter: try set value ErrorType", property.Name.Value)
		return
	}
	if a.interfaceSetterErrorRefs == nil {
		a.interfaceSetterErrorRefs = map[string]*ast.TypeReference{}
	}
	a.interfaceSetterErrorRefs[interfaceName+"."+property.Name.Value] = property.SetterErrorType
}

// resolveInterfaceSetterContracts resolves every recorded interface setter
// error contract once, after all error types have been analyzed.
//
// Rules:
//   - rules/errors/errorhandling.md — §24.1 "Interfaces"
func (a *Analyzer) resolveInterfaceSetterContracts() {
	a.interfaceSetterErrors = map[string]*Type{}
	keys := make([]string, 0, len(a.interfaceSetterErrorRefs))
	for key := range a.interfaceSetterErrorRefs {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		reference := a.interfaceSetterErrorRefs[key]
		if contract := a.resolveSetterErrorContract(reference, reference.Token, key); contract != nil {
			a.interfaceSetterErrors[key] = contract
		}
	}
}

// setterErrorContractSatisfied reports whether an implementation's declared
// setter error is assignable to the error contract the interface requires.
//
// Rules:
//   - rules/errors/errorhandling.md — §24.1 "Interfaces", §2.1 "Error assignability"
func setterErrorContractSatisfied(property Property, contract *Type) bool {
	if property.Error == nil || contract == nil {
		return true
	}
	return canInitialize(*contract, *property.Error, nil)
}

// analyzeFallibleSetterReturn applies the try set success model: a bare return
// is early success, return Err(error) is failure and must match the declared
// error contract, and any other returned value, including Ok(), is invalid.
//
// Rules:
//   - rules/errors/errorhandling.md — §24 "Fallible property setters"
func (a *Analyzer) analyzeFallibleSetterReturn(functionName string, returnType Type, stmt *ast.ReturnStatement) {
	switch value := stmt.Value.(type) {
	case nil:
		return
	case *ast.ErrExpression:
		a.analyzeResultReturnStatement(functionName, returnType, stmt)
	case *ast.OkExpression:
		a.addErrorAtToken(value.Token, "return Ok() is invalid in a try set body; normal completion or a bare return is success")
	default:
		a.addErrorAtToken(expressionToken(stmt.Value), "a try set body returns no value; use return for early success or return Err(error) for failure")
	}
}
