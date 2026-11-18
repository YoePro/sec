package parser

// Specialized contexts describe syntax, independently of the enclosing declaration
// or statement scope used by parser dispatch. Rules: rules/compiler/parser_recovery.md §§11,17,21,26–66.
const (
	RecoveryContextTypeDeclaration     RecoveryContext = "type-declaration"
	RecoveryContextAssignment          RecoveryContext = "assignment"
	RecoveryContextReturn              RecoveryContext = "return"
	RecoveryContextDefer               RecoveryContext = "defer"
	RecoveryContextDiscard             RecoveryContext = "discard"
	RecoveryContextUnsafe              RecoveryContext = "unsafe"
	RecoveryContextMemberAccess        RecoveryContext = "member-access"
	RecoveryContextRange               RecoveryContext = "range"
	RecoveryContextExpression          RecoveryContext = "expression"
	RecoveryContextModule              RecoveryContext = "module"
	RecoveryContextImport              RecoveryContext = "import"
	RecoveryContextDirective           RecoveryContext = "directive"
	RecoveryContextGenericParameters   RecoveryContext = "generic-parameters"
	RecoveryContextGenericArguments    RecoveryContext = "generic-arguments"
	RecoveryContextParameters          RecoveryContext = "parameters"
	RecoveryContextType                RecoveryContext = "type-reference"
	RecoveryContextFunction            RecoveryContext = "function-signature"
	RecoveryContextStruct              RecoveryContext = "struct"
	RecoveryContextEnum                RecoveryContext = "enum"
	RecoveryContextUnion               RecoveryContext = "union"
	RecoveryContextInterface           RecoveryContext = "interface"
	RecoveryContextImpl                RecoveryContext = "impl"
	RecoveryContextProperty            RecoveryContext = "property"
	RecoveryContextAttribute           RecoveryContext = "attribute"
	RecoveryContextContract            RecoveryContext = "contract"
	RecoveryContextAsm                 RecoveryContext = "asm"
	RecoveryContextLambda              RecoveryContext = "lambda"
	RecoveryContextMatch               RecoveryContext = "match"
	RecoveryContextTry                 RecoveryContext = "try"
	RecoveryContextSwitch              RecoveryContext = "switch"
	RecoveryContextSelect              RecoveryContext = "select"
	RecoveryContextArrayLiteral        RecoveryContext = "array-literal"
	RecoveryContextStructLiteral       RecoveryContext = "struct-literal"
	RecoveryContextIndexSlice          RecoveryContext = "index-or-slice"
	RecoveryContextCall                RecoveryContext = "call-arguments"
	RecoveryContextGroupedExpression   RecoveryContext = "grouped-expression"
	RecoveryContextInterpolation       RecoveryContext = "interpolation"
	RecoveryContextEvent               RecoveryContext = "event"
	RecoveryContextUnit                RecoveryContext = "unit"
	RecoveryContextRegister            RecoveryContext = "register"
	RecoveryContextHardware            RecoveryContext = "hardware"
	RecoveryContextTest                RecoveryContext = "test"
	RecoveryContextSpawn               RecoveryContext = "spawn"
	RecoveryContextAwait               RecoveryContext = "await"
	RecoveryContextThreadConfiguration RecoveryContext = "thread-configuration"
	RecoveryContextNew                 RecoveryContext = "new"
	RecoveryContextBinding             RecoveryContext = "binding"
	RecoveryContextIf                  RecoveryContext = "if"
	RecoveryContextWhile               RecoveryContext = "while"
	RecoveryContextFor                 RecoveryContext = "for"
)

// withRecoveryContext restores the caller on every return, including speculative
// failure. It does not change dispatch scope, synchronization sets or episode
// boundaries. Rules: rules/compiler/parser_recovery.md §§11,16–17,21.
func (p *Parser) withRecoveryContext(context RecoveryContext) func() {
	previous := p.diagnosticContext
	p.diagnosticContext = context
	return func() { p.diagnosticContext = previous }
}

// currentRecoveryContext returns the innermost syntax owner, falling back to the
// enclosing top-level/block/member scope. Rules: rules/compiler/parser_recovery.md §§11,17,21.
func (p *Parser) currentRecoveryContext() RecoveryContext {
	if p.diagnosticContext != "" {
		return p.diagnosticContext
	}
	return p.recoveryContext
}
