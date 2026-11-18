package parser

import (
	"testing"

	"sec/internal/diagnostics"
	"sec/internal/lexer"
)

// Specialized errors and repairs identify their syntax owner, even when the
// caller records the repair after that owner's parse has returned.
// Rules: rules/compiler/parser_recovery.md §§11,16–17,21,26–66.
func TestSpecializedRecoveryContexts(t *testing.T) {
	for _, tt := range []struct {
		context RecoveryContext
		source  string
	}{

		{RecoveryContextTypeDeclaration, "type"},
		{RecoveryContextAssignment, "fn F() void { x = }"},
		{RecoveryContextReturn, "fn F() void { return <- }"},
		{RecoveryContextDefer, "fn F() void { defer return }"},
		{RecoveryContextDiscard, "fn F() void { discard }"},
		{RecoveryContextUnsafe, "fn F() void { unsafe }"},
		{RecoveryContextMemberAccess, "fn F() void { x. }"},
		{RecoveryContextRange, "fn F() void { f(1..) }"},
		{RecoveryContextExpression, "fn F() void { let x := - }"},
		{RecoveryContextInterpolation, `fn F() void { let x := $"{?}" }`},
		{RecoveryContextCall, `fn F() void { let x := $"{f(1 2)}" }`},
		{RecoveryContextModule, "module"},
		{RecoveryContextImport, "import"},
		{RecoveryContextDirective, "#target()"},
		{RecoveryContextGenericParameters, "fn F[,]() void {}"},
		{RecoveryContextGenericArguments, "type X Box[,]"},
		{RecoveryContextParameters, "fn F(,) void {}"},
		{RecoveryContextType, "type X ref"},
		{RecoveryContextFunction, "fn"},
		{RecoveryContextStruct, "type X struct ("},
		{RecoveryContextEnum, "enum X { = }"},
		{RecoveryContextUnion, "type X union { = }"},
		{RecoveryContextInterface, "interface X { ? }"},
		{RecoveryContextImpl, "impl {"},
		{RecoveryContextProperty, "impl X { property P: int { get ? } }"},
		{RecoveryContextAttribute, "@target(,) fn F() void {}"},
		{RecoveryContextContract, "type X int range"},
		{RecoveryContextAsm, "fn F() void { asm }"},
		{RecoveryContextLambda, "fn F() void { let x := fn ? }"},
		{RecoveryContextMatch, "fn F() int { return match { } }"},
		{RecoveryContextMatchArm, "fn F(v: int) int { return match v { => 1 } }"},
		{RecoveryContextTry, "fn F() void { try }"},
		{RecoveryContextTryHandler, "fn F() void { try f() { ? => 1 } }"},
		{RecoveryContextSwitch, "fn F() void { switch x { ? } }"},
		{RecoveryContextSelect, "fn F() void { select { ? } }"},
		{RecoveryContextArrayLiteral, "fn F() void { let x := [1 2] }"},
		{RecoveryContextStructLiteral, "fn F() void { let x := X{ = } }"},
		{RecoveryContextIndexSlice, "fn F() void { let x := a[] }"},
		{RecoveryContextCall, "fn F() void { f(1 2) }"},
		{RecoveryContextGroupedExpression, "fn F() void { let x := (1 }"},
		{RecoveryContextEvent, "impl X { event }"},
		{RecoveryContextUnit, "unit"},
		{RecoveryContextRegister, "type X register uint { ? }"},
		{RecoveryContextHardware, "hardware region X { base: }"},
		{RecoveryContextTest, "test"},
		{RecoveryContextSpawn, "fn F() void { let x := spawn }"},
		{RecoveryContextAwait, "fn F() void { let x := await }"},
		{RecoveryContextThreadConfiguration, "fn F() void { let x := spawn thread <-config }"},
		{RecoveryContextNew, "fn F() void { let x := new }"},
		{RecoveryContextBinding, "fn F() void { let }"},
		{RecoveryContextIf, "fn F() void { if }"},
		{RecoveryContextWhile, "fn F() void { while }"},
		{RecoveryContextFor, "fn F() void { for }"},
	} {
		t.Run(string(tt.context), func(t *testing.T) {
			p := New(lexer.NewWithFile(tt.source, "context.sec"))
			result := p.Parse()
			if !result.HasErrors || result.Fatal {
				t.Fatalf("wrong flags: %+v", result)
			}
			found := false
			for _, diagnostic := range result.Diagnostics {
				if diagnostic.Context == tt.context {
					found = true
				}
			}
			if !found {
				t.Fatalf("missing %s context in %+v", tt.context, result.Diagnostics)
			}
			for _, event := range result.Recovery {
				if event.DiagnosticID == "" {
					continue
				}
				attached := false
				for _, diagnostic := range result.Diagnostics {
					if diagnostic.ID == event.DiagnosticID && diagnostic.Episode == event.Episode && diagnostic.Context == event.Context {
						attached = true
					}
				}
				if !attached {
					t.Fatalf("repair detached from syntax owner: %+v", event)
				}
			}
			if p.diagnosticContext != "" || p.recoveryContext != RecoveryContextTopLevel {
				t.Fatalf("scope leaked: %s / %s", p.diagnosticContext, p.recoveryContext)
			}
		})
	}
}

// Scope restoration must survive rollback and preserve the coarse scope used
// for member/statement dispatch. A caller's skip retains the child's context.
// Rules: rules/compiler/parser_recovery.md §§11,16–17,21.
func TestRecoveryContextRestorationAndCallerRepair(t *testing.T) {
	p := New(lexer.New("?"))
	p.recoveryContext = RecoveryContextMember
	outer := p.withRecoveryContext(RecoveryContextParameters)
	inner := p.withRecoveryContext(RecoveryContextType)
	token := p.curToken
	p.addDiagnostic(diagnostics.ParserInvalidTypeReference, token, nil, nil, "speculative")
	p.rollbackErrors(0)
	if p.currentRecoveryContext() != RecoveryContextType {
		t.Fatal("rollback lost active syntax owner")
	}
	p.addDiagnostic(diagnostics.ParserInvalidTypeReference, token, nil, nil, "committed")
	inner()
	if p.currentRecoveryContext() != RecoveryContextParameters {
		t.Fatal("lost caller context")
	}
	repair := p.recordSkippedRecovery(token, token, 1, RecoveryProbable)
	if repair.Context != RecoveryContextType || repair.DiagnosticID != diagnostics.ParserInvalidTypeReference || repair.Episode != p.Diagnostics()[0].Episode {
		t.Fatalf("detached repair: %+v", repair)
	}
	p.addDiagnostic(diagnostics.ParserMissingToken, token, []lexer.TokenType{lexer.COMMA}, nil, "parameter separator")
	if len(p.Diagnostics()) != 2 || p.Diagnostics()[1].Context != RecoveryContextParameters {
		t.Fatalf("context-sensitive dedup lost sibling: %+v", p.Diagnostics())
	}
	outer()
	if p.currentRecoveryContext() != RecoveryContextMember || p.recoveryContext != RecoveryContextMember {
		t.Fatal("syntax scope changed dispatch scope")
	}
}
