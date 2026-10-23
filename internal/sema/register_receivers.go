package sema

import (
	"sort"
	"strings"

	"sec/internal/ast"
	"sec/internal/lexer"
)

// Receiver kinds of a register impl (MD-077 §§ 1, 4). One `impl T` serves both
// kinds of register: on a software register (a detached value in a memory
// Place, such as a protocol header) `self.Field` is an ordinary field like a
// struct field; on a hardware register (HardwareInstance[T]) it is the
// field's declared hardware access. An operation that is valid only on a
// software register — borrowing `self.Field` (physical bits are not memory,
// § 4(1)), a compound update of a specialized field, an observation with a
// collateral destructive effect or an unresolved special write — makes the
// member software-only: it, and every member that uses it on its own
// receiver, cannot be used on a hardware instance.

// registerMemberUse is one call or property access of an instance member of a
// register impl, with the kind of its receiver.
type registerMemberUse struct {
	caller   string
	callee   string
	receiver RegisterRootKind
	token    lexer.Token
}

// currentRegisterMemberKey names the instance member of a register impl being
// analyzed, or "" elsewhere.
func (a *Analyzer) currentRegisterMemberKey() string {
	target, ok := a.types[a.currentImplTarget]
	if !ok || target.Kind != RegisterType || a.currentFunctionName == "" || !a.inFunctionBody {
		return ""
	}
	return registerMemberKey(a.currentImplTarget, a.currentFunctionName)
}

func registerMemberKey(target, member string) string {
	return target + "." + strings.TrimPrefix(member, target+".")
}

// softwareOnlyReason is the first operation that makes a register impl member
// software-only.
type softwareOnlyReason struct {
	token  lexer.Token
	reason string
}

// markSoftwareOnlyMember records that the member being analyzed performs an
// operation valid only on a software register receiver.
func (a *Analyzer) markSoftwareOnlyMember(token lexer.Token, reason string) {
	a.markSoftwareOnlyMemberKey(a.currentRegisterMemberKey(), token, reason)
}

func (a *Analyzer) markSoftwareOnlyMemberKey(key string, token lexer.Token, reason string) {
	if key == "" || a.summaryPass {
		return
	}
	if a.softwareOnlyRegisterMembers == nil {
		a.softwareOnlyRegisterMembers = map[string]softwareOnlyReason{}
	}
	if _, exists := a.softwareOnlyRegisterMembers[key]; !exists {
		a.softwareOnlyRegisterMembers[key] = softwareOnlyReason{token: token, reason: reason}
	}
}

// recordRegisterMemberUse records a use of an instance member of a register
// impl; receiver is the explicit object, or nil for implicit self.
func (a *Analyzer) recordRegisterMemberUse(owner, member string, receiver ast.Expression, token lexer.Token) {
	if a.summaryPass {
		return
	}
	if target, ok := a.types[owner]; !ok || target.Kind != RegisterType {
		return
	}
	kind := RegisterRootImplReceiver
	if receiver != nil {
		kind = a.registerRootKind(receiver)
	}
	a.registerMemberUses = append(a.registerMemberUses, registerMemberUse{
		caller: a.currentRegisterMemberKey(), callee: registerMemberKey(owner, member), receiver: kind, token: token,
	})
}

// recordRegisterMethodCall records a resolved instance method call.
func (a *Analyzer) recordRegisterMethodCall(call *ast.CallExpression, function Function) {
	if function.ImplTarget == "" || function.Static {
		return
	}
	var receiver ast.Expression
	if member, ok := call.Callee.(*ast.MemberExpression); ok {
		receiver = member.Object
	}
	a.recordRegisterMemberUse(function.ImplTarget, function.Name, receiver, call.Token)
}

// recordRegisterPropertyAccess records the getter and/or setter a property
// access uses.
func (a *Analyzer) recordRegisterPropertyAccess(expr ast.Expression, owner Type, property Property, kind ResolvedPropertyAccessKind) {
	if property.Static {
		return
	}
	var receiver ast.Expression
	if member, ok := expr.(*ast.MemberExpression); ok {
		receiver = member.Object
	}
	register := dereferenceType(owner)
	if kind == PropertyRead || kind == PropertyCompoundUpdate {
		a.recordRegisterMemberUse(register.Name, property.Name+".get", receiver, expressionToken(expr))
	}
	if kind != PropertyRead {
		a.recordRegisterMemberUse(register.Name, property.Name+".set", receiver, expressionToken(expr))
	}
}

// checkRegisterReceiverRequirements closes the software-only requirement over
// implicit-self uses and rejects a software-only member used on a hardware
// instance.
func (a *Analyzer) checkRegisterReceiverRequirements() {
	if len(a.softwareOnlyRegisterMembers) == 0 && len(a.hardwareOnlyRegisterMembers) == 0 {
		return
	}
	a.closeRegisterRequirement(a.softwareOnlyRegisterMembers)
	a.closeRegisterRequirement(a.hardwareOnlyRegisterMembers)
	uses := append([]registerMemberUse(nil), a.registerMemberUses...)
	sort.SliceStable(uses, func(i, j int) bool {
		if uses[i].token.File != uses[j].token.File {
			return uses[i].token.File < uses[j].token.File
		}
		if uses[i].token.Line != uses[j].token.Line {
			return uses[i].token.Line < uses[j].token.Line
		}
		return uses[i].token.Column < uses[j].token.Column
	})
	for _, use := range uses {
		member := strings.TrimSuffix(strings.TrimSuffix(use.callee, ".get"), ".set")
		register := member[:strings.Index(member, ".")]
		switch use.receiver {
		case RegisterRootHardware:
			if why, required := a.softwareOnlyRegisterMembers[use.callee]; required {
				a.addErrorAtTokenWithMetadata(use.token, "",
					"Use this member on a software "+register+" value, or give impl "+register+" a member whose field operations are valid on hardware.",
					"%s cannot be used on hardware instance of %s: it %s (at %d:%d)",
					member, register, why.reason, why.token.Line, why.token.Column)
			}
		case RegisterRootDetached:
			if why, required := a.hardwareOnlyRegisterMembers[use.callee]; required {
				a.addErrorAtTokenWithMetadata(use.token, "",
					"Call it on a HardwareInstance["+register+"]; a software "+register+" value is read and written through its fields.",
					"%s requires a hardware instance receiver: it %s (at %d:%d)",
					member, why.reason, why.token.Line, why.token.Column)
			}
		}
	}
}

// closeRegisterRequirement propagates a receiver requirement to members that
// use a required member on their own receiver.
func (a *Analyzer) closeRegisterRequirement(required map[string]softwareOnlyReason) {
	for changed := true; changed && len(required) > 0; {
		changed = false
		for _, use := range a.registerMemberUses {
			if use.receiver != RegisterRootImplReceiver || use.caller == "" {
				continue
			}
			callee, requires := required[use.callee]
			if !requires {
				continue
			}
			if _, marked := required[use.caller]; !marked {
				required[use.caller] = softwareOnlyReason{token: use.token, reason: callee.reason}
				changed = true
			}
		}
	}
}

// markHardwareOnlyMember records that the member being analyzed performs a
// hardware-only operation on its receiver (MD-077 § 4(11)).
func (a *Analyzer) markHardwareOnlyMember(token lexer.Token, reason string) {
	key := a.currentRegisterMemberKey()
	if key == "" || a.summaryPass {
		return
	}
	if a.hardwareOnlyRegisterMembers == nil {
		a.hardwareOnlyRegisterMembers = map[string]softwareOnlyReason{}
	}
	if _, exists := a.hardwareOnlyRegisterMembers[key]; !exists {
		a.hardwareOnlyRegisterMembers[key] = softwareOnlyReason{token: token, reason: reason}
	}
}
