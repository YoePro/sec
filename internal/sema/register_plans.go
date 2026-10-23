package sema

import (
	"math/big"

	"sec/internal/ast"
)

// RegisterRootKind classifies the value a register field access starts from
// (MD-077 § 2(5), § 9(5)). Detached values and hardware instances are never
// interchangeable, even when their bit layouts coincide.
type RegisterRootKind string

const (
	// RegisterRootDetached is an ordinary register[N] value.
	RegisterRootDetached RegisterRootKind = "detached"
	// RegisterRootHardware is a field of a HardwareInstance[T] reached inside
	// the owning impl T.
	RegisterRootHardware RegisterRootKind = "hardware-instance"
	// RegisterRootImplReceiver is a field of `self` inside impl T, whose
	// receiver may be a hardware instance (MD-077 § 4(11)).
	RegisterRootImplReceiver RegisterRootKind = "impl-receiver"
)

// ResolvedRegisterFieldPlan is Sema's immutable resolution of one physical
// register field step (MD-077 § 2(5) RegisterPhysicalField, § 11(2)).
// Semantic IR builds register operations from it and never re-resolves the
// member spelling.
type ResolvedRegisterFieldPlan struct {
	Register   Type
	FieldIndex int
	Field      RegisterField
	Root       RegisterRootKind
	// Member is the enclosing instance member of a register impl, if any.
	Member string
	// Request is the logical hardware request of a leaf step on a hardware
	// or impl-receiver root (hardware-register-access.md § 13.1); nil for a
	// detached value and for an intermediate step of a nested path.
	Request *ResolvedRegisterHardwareRequest
}

// RegisterWriteProofBasis is how Sema proved that a field write fits the
// field's value domain without truncation (MD-077 § 6(5), MD-046).
type RegisterWriteProofBasis string

const (
	// RegisterWriteTyped: the value already has the field's bool, enum or
	// nested register type.
	RegisterWriteTyped RegisterWriteProofBasis = "typed"
	// RegisterWriteConstant: a constant within 0..2^N-1 (for a one-bit field
	// the integer 0 or 1).
	RegisterWriteConstant RegisterWriteProofBasis = "constant"
	// RegisterWriteInterval: a runtime value whose proven interval lies in
	// 0..2^N-1.
	RegisterWriteInterval RegisterWriteProofBasis = "interval"
)

// ResolvedRegisterWriteProof is the validated value side of a field write.
// Low and High are the proven interval (equal for a constant); both are nil
// for a typed write.
type ResolvedRegisterWriteProof struct {
	Basis RegisterWriteProofBasis
	Low   *big.Int
	High  *big.Int
}

// ResolvedRegisterFieldOf returns the register field step resolved for a
// member expression (read or assignment target).
func (a *Analyzer) ResolvedRegisterFieldOf(expr *ast.MemberExpression) (ResolvedRegisterFieldPlan, bool) {
	if a == nil || expr == nil {
		return ResolvedRegisterFieldPlan{}, false
	}
	plan, ok := a.resolvedRegisterFields[expr]
	return plan, ok
}

// ResolvedRegisterWriteOf returns the proof Sema established for a register
// field assignment.
func (a *Analyzer) ResolvedRegisterWriteOf(stmt *ast.AssignmentStatement) (ResolvedRegisterWriteProof, bool) {
	if a == nil || stmt == nil {
		return ResolvedRegisterWriteProof{}, false
	}
	proof, ok := a.resolvedRegisterWrites[stmt]
	if ok && proof.Low != nil {
		proof.Low, proof.High = new(big.Int).Set(proof.Low), new(big.Int).Set(proof.High)
	}
	return proof, ok
}

// recordRegisterField resolves member as a register field step of
// objectType, classifying its root.
func (a *Analyzer) recordRegisterField(member *ast.MemberExpression, objectType Type) {
	register := dereferenceType(objectType)
	if register.Kind != RegisterType || member.Property == nil {
		return
	}
	for index, field := range register.RegisterFields {
		if field.Name == member.Property.Value && field.Name != "_" {
			if a.resolvedRegisterFields == nil {
				a.resolvedRegisterFields = map[*ast.MemberExpression]ResolvedRegisterFieldPlan{}
			}
			plan := ResolvedRegisterFieldPlan{Register: register, FieldIndex: index, Field: field, Root: a.registerRootKind(member.Object), Member: a.currentRegisterMemberKey()}
			a.resolvedRegisterFields[member] = plan
			return
		}
	}
}

// registerRootKind follows nested register field steps to the root value.
func (a *Analyzer) registerRootKind(object ast.Expression) RegisterRootKind {
	if member, ok := object.(*ast.MemberExpression); ok {
		if plan, resolved := a.resolvedRegisterFields[member]; resolved {
			return plan.Root
		}
	}
	if isHardwareInstance(a.expressionTypes[object]) {
		return RegisterRootHardware
	}
	if identifier, ok := object.(*ast.Identifier); ok {
		if symbol, exists := a.symbols[identifier.Value]; exists && isHardwareInstance(symbol.Type) {
			return RegisterRootHardware
		}
		if identifier.Value == "self" {
			if target, exists := a.types[a.currentImplTarget]; exists && target.Kind == RegisterType {
				return RegisterRootImplReceiver
			}
		}
	}
	return RegisterRootDetached
}

func (a *Analyzer) recordRegisterWrite(stmt *ast.AssignmentStatement, proof ResolvedRegisterWriteProof) {
	if a.resolvedRegisterWrites == nil {
		a.resolvedRegisterWrites = map[*ast.AssignmentStatement]ResolvedRegisterWriteProof{}
	}
	a.resolvedRegisterWrites[stmt] = proof
}
