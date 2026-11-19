package sema

import "sec/internal/ast"

// ConstructionSignature is a declaration preview, not evidence that an init
// has been selected or that a construction is valid. Parameter and error types
// come from the compiler's registered lifecycle declarations, including impl
// extensions. Implicit marks the permitted default zero-argument path.
// Rules: rules/declarations/impl.md — §§15.4,15.6;
// rules/tooling/lsp.md — §§4.1(22),13.
type ConstructionSignature struct {
	Target     Type
	Parameters []FunctionParameter
	ErrorType  *Type
	Implicit   bool
}

// recordConstructionSignatures captures declaration metadata before argument
// validation, so recovery and failed overload selection retain useful previews
// without publishing a successful ResolvedConstruction or resolving a call in
// the editor. The implicit path follows the same defaultability and explicit
// zero-arity precedence as lifecycle construction.
// Rules: rules/declarations/impl.md — §§15.4,15.6;
// rules/tooling/lsp.md — §§4.1(22),13, "Unknown facts".
func (a *Analyzer) recordConstructionSignatures(expr *ast.NewExpression, target Type, initializers []Function) {
	signatures := make([]ConstructionSignature, 0, len(initializers)+1)
	explicitZero := false
	for _, initializer := range initializers {
		if !initializer.Initializer {
			continue
		}
		explicitZero = explicitZero || len(initializer.Parameters) == 0
		signatures = append(signatures, cloneConstructionSignature(ConstructionSignature{
			Target: target, Parameters: initializer.Parameters, ErrorType: initializer.ConstructionError,
		}))
	}
	if !explicitZero && IsDefaultable(target) {
		signatures = append(signatures, cloneConstructionSignature(ConstructionSignature{Target: target, Implicit: true}))
	}
	if a.constructionSignatures == nil {
		a.constructionSignatures = map[*ast.NewExpression][]ConstructionSignature{}
	}
	a.constructionSignatures[expr] = signatures
}

// cloneConstructionSignature detaches the exposed types and parameter slice
// from both declaration storage and previous readers of the analysis snapshot.
// Rules: rules/tooling/lsp.md — "Snapshots", "Read-only request behavior".
func cloneConstructionSignature(signature ConstructionSignature) ConstructionSignature {
	signature.Target = semanticSnapshotType(signature.Target)
	signature.Parameters = append([]FunctionParameter(nil), signature.Parameters...)
	for i := range signature.Parameters {
		signature.Parameters[i].Type = semanticSnapshotType(signature.Parameters[i].Type)
	}
	if signature.ErrorType != nil {
		errorType := cloneSlicePlanErrorType(*signature.ErrorType)
		signature.ErrorType = &errorType
	}
	return signature
}

// ConstructionSignaturesOf returns detached compiler-owned previews even when
// incomplete arguments prevent overload selection. It performs no inference,
// caches no specialization and never claims candidate applicability.
// Rules: rules/declarations/impl.md — §§15.4,15.6;
// rules/tooling/lsp.md — §§4.1(22),13, "Snapshots".
func (a *Analyzer) ConstructionSignaturesOf(expr *ast.NewExpression) []ConstructionSignature {
	if a == nil || expr == nil {
		return nil
	}
	signatures := a.constructionSignatures[expr]
	result := make([]ConstructionSignature, len(signatures))
	for i := range signatures {
		result[i] = cloneConstructionSignature(signatures[i])
	}
	return result
}
