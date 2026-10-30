package parser

import (
	"strings"

	"sec/internal/ast"
	compilerdiagnostics "sec/internal/diagnostics"
	"sec/internal/lexer"
)

// attributeTargets is the set of declaration kinds an attribute may modify.
type attributeTargets uint16

const (
	attributeOnFunction attributeTargets = 1 << iota
	attributeOnMethod
	attributeOnExtern
	attributeOnNominalType
	attributeOnLet
	attributeOnAnyTopLevel
	// The callable-body targets of MD-030: init, free, one property accessor
	// (get, set or try set) and a let that directly initializes a lambda.
	attributeOnInit
	attributeOnFree
	attributeOnAccessor
	attributeOnLambdaLet
)

// noPanicTargetText names every MD-030 placement of @noPanic.
const noPanicTargetText = "a function, method, extern declaration, init, free, property get/set/try set accessor, or a let that directly initializes a lambda"

// attributeArgumentShape is the argument form an attribute defines.
type attributeArgumentShape uint8

const (
	attributeNoArguments attributeArgumentShape = iota
	attributeOnePositional
	attributeNamedArguments
	attributeOneStringLiteral
)

// attributeSpec is the registry entry of one compiler-known attribute: its
// allowed targets and argument form. Sema decides whether the attribute's
// meaning is implemented.
type attributeSpec struct {
	targets    attributeTargets
	targetText string
	shape      attributeArgumentShape
	named      []string
	required   []string
	summary    string
}

// compilerKnownAttributes is the closed Sec 0.1 attribute set.
//
// Rules:
//   - rules/foundations/attributes.md — "Initial compiler-known attribute set", "Positional and named arguments",
//     "Allowed targets for verified effects", "@address", "@noCopy allowed targets", "@link_name"
var compilerKnownAttributes = map[string]attributeSpec{
	contextualTarget: {targets: attributeOnAnyTopLevel, targetText: "a top-level declaration", shape: attributeNamedArguments, named: []string{"os", "arch", "cpu", "device", "board"}, summary: "Selects the declaration for the compilation plans whose target matches every selector."},
	"when":           {targets: attributeOnAnyTopLevel, targetText: "a top-level declaration", shape: attributeOnePositional, summary: "Selects the declaration when a compile-time configuration condition holds."},
	"address":        {targets: attributeOnLet, targetText: "a let declaration", shape: attributeOnePositional, summary: "Binds a module-scope register declaration to an absolute address."},
	"interrupt":      {targets: attributeOnFunction | attributeOnMethod, targetText: "a function or method", shape: attributeNamedArguments, named: []string{"vector"}, required: []string{"vector"}, summary: "Binds the function as the handler of one interrupt vector; implies @isr."},
	"isr":            {targets: attributeOnFunction | attributeOnMethod, targetText: "a function or method", shape: attributeNoArguments, summary: "Declares and verifies an interrupt service routine without binding a vector; implies @noPanic, @noAlloc, and @noBlock."},
	"interruptSafe":  {targets: attributeOnFunction | attributeOnMethod, targetText: "a function or method", shape: attributeNoArguments, summary: "Declares that the function may be called from ISR code; implies @noPanic, @noAlloc, and @noBlock."},
	"noCopy":         {targets: attributeOnNominalType, targetText: "a nominal type declaration", shape: attributeNoArguments, summary: "Makes the nominal type non-copyable: values move or are borrowed, never implicitly copied."},
	"noAlloc":        {targets: attributeOnFunction | attributeOnMethod | attributeOnExtern, targetText: "a function or method", shape: attributeNoArguments, summary: "Compiler-verified guarantee that no reachable path allocates; a trusted foreign contract on an extern declaration."},
	"noPanic":        {targets: attributeOnFunction | attributeOnMethod | attributeOnExtern | attributeOnInit | attributeOnFree | attributeOnAccessor | attributeOnLambdaLet, targetText: noPanicTargetText, shape: attributeNoArguments, summary: "Compiler-verified guarantee that no reachable path panics; a trusted foreign contract on an extern declaration."},
	"noBlock":        {targets: attributeOnFunction | attributeOnMethod | attributeOnExtern, targetText: "a function or method", shape: attributeNoArguments, summary: "Compiler-verified guarantee that no reachable path waits or parks the current execution; a trusted foreign contract on an extern declaration."},
	"link_name":      {targets: attributeOnExtern, targetText: "an extern declaration", shape: attributeOneStringLiteral, summary: "Sets the foreign link symbol of an extern declaration without changing its Sec name, ABI, or effects."},
}

// compilerKnownAttributeName distinguishes the closed Sec 0.1 attribute set
// from truly unknown names.
//
// Rules:
//   - rules/foundations/attributes.md — "Initial compiler-known attribute set", "Closed attribute set"
func compilerKnownAttributeName(name string) bool {
	_, ok := compilerKnownAttributes[name]
	return ok
}

// parseAttributedStatement parses one attachment set — every consecutive
// attribute, with comments allowed between them — and attaches it to the
// following declaration. Unknown attributes are reported and dropped from the
// set; each known attribute is validated for its argument shape, duplicates,
// duplicate argument names, and allowed target, and the specialized fields
// (@address, @link_name) are derived from the generic attributes. Order inside
// the set is not significant.
//
// Rules:
//   - rules/foundations/attributes.md — "General syntax", "Positional and named arguments", "Attribute attachment",
//     "Comments and whitespace", "Allowed attachment level", "Duplicate attributes", "Duplicate arguments",
//     "Attribute order", "Parser representation"
func (p *Parser) parseAttributedStatement() ast.Statement {
	local := p.recoveryContext != RecoveryContextTopLevel && p.recoveryContext != RecoveryContextMember
	attributes, attached := p.parseAttributeSet()
	if !attached {
		return nil
	}
	member := p.recoveryContext == RecoveryContextMember
	p.nextToken()
	start := p.curToken
	statement := p.parseStatement()
	if isNilStatement(statement) {
		return statement
	}
	// MD-030: @noPanic before a let that directly initializes a lambda
	// annotates the lambda's body, at top level and in local scope alike.
	if let, ok := statement.(*ast.LetStatement); ok {
		if lambda, direct := let.Value.(*ast.LambdaExpression); direct && lambda != nil {
			bodyAttributes, rest := splitLambdaBodyAttributes(attributes)
			lambda.Attributes = append(lambda.Attributes, p.acceptAttributes(bodyAttributes, attributeOnLambdaLet, start)...)
			attributes = rest
		}
	}
	if local {
		for _, attribute := range attributes {
			if compilerKnownAttributes[attribute.Name.Value].targets&attributeOnLambdaLet != 0 {
				p.addDiagnostic(compilerdiagnostics.AttributeNotAllowedOnTarget, attribute.Token, nil, nil,
					"@%s may annotate a local let only when it directly initializes a lambda (fn or capture(...) fn) at %d:%d", attribute.Name.Value, attribute.Token.Line, attribute.Token.Column)
				continue
			}
			p.addDiagnostic(compilerdiagnostics.AttributeNotAllowedOnTarget, attribute.Token, nil, nil,
				"@%s may only annotate a top-level declaration at %d:%d", attribute.Name.Value, attribute.Token.Line, attribute.Token.Column)
		}
		return statement
	}
	p.attachAttributes(statement, attributes, member, start)
	return statement
}

// parseAttributeSet parses one attachment set — every consecutive attribute,
// with comments allowed between them — with the current token on the first
// `@`, and leaves the current token on the end of the last attribute. It
// reports duplicates and conflicts and returns false (after reporting) when
// the set is not followed by anything it could attach to.
//
// Rules:
//   - rules/foundations/attributes.md — "General syntax", "Comments and whitespace", "Duplicate attributes"
func (p *Parser) parseAttributeSet() ([]*ast.Attribute, bool) {
	attributes := []*ast.Attribute{}
	first := map[string]*ast.Attribute{}
	for {
		attribute, known := p.parseAttribute()
		if attribute != nil && known {
			name := attribute.Name.Value
			if previous, duplicate := first[name]; duplicate {
				if conflicting, rule := attributesConflict(previous, attribute); conflicting {
					p.addDiagnostic(compilerdiagnostics.AttributeConflict, attribute.Token, nil, nil,
						"conflicting @%s attributes at %d:%d and %d:%d; %s",
						name, attribute.Token.Line, attribute.Token.Column, previous.Token.Line, previous.Token.Column, rule)
				} else {
					p.addDiagnostic(compilerdiagnostics.AttributeDuplicate, attribute.Token, nil, nil,
						"duplicate attribute @%s at %d:%d; first declared at %d:%d",
						name, attribute.Token.Line, attribute.Token.Column, previous.Token.Line, previous.Token.Column)
				}
			} else {
				first[name] = attribute
				attributes = append(attributes, attribute)
			}
		}
		p.skipPeekComments()
		if p.peekToken.Type != lexer.AT || p.tokenAfterPeek() != lexer.IDENT {
			break
		}
		p.nextToken()
	}
	if p.peekToken.Type == lexer.EOF {
		if len(attributes) > 0 {
			attribute := attributes[0]
			p.addDiagnostic(compilerdiagnostics.UnattachedAttribute, attribute.Name.Token, nil, nil,
				"attribute @%s is not attached to a statement at %d:%d", attribute.Name.Value, attribute.Name.Token.Line, attribute.Name.Token.Column)
		}
		return nil, false
	}
	return attributes, true
}

// splitLambdaBodyAttributes separates the attributes that may annotate a
// directly initialized lambda body from the rest of a let's attachment set.
func splitLambdaBodyAttributes(attributes []*ast.Attribute) (body, rest []*ast.Attribute) {
	for _, attribute := range attributes {
		if compilerKnownAttributes[attribute.Name.Value].targets&attributeOnLambdaLet != 0 {
			body = append(body, attribute)
		} else {
			rest = append(rest, attribute)
		}
	}
	return body, rest
}

// acceptAttributes keeps the attributes allowed on one callable-body target
// and reports every other one with its allowed placements (MD-030 § 8(1)).
func (p *Parser) acceptAttributes(attributes []*ast.Attribute, kind attributeTargets, declaration lexer.Token) []*ast.Attribute {
	accepted := []*ast.Attribute{}
	for _, attribute := range attributes {
		spec := compilerKnownAttributes[attribute.Name.Value]
		if spec.targets&kind == 0 {
			p.addDiagnostic(compilerdiagnostics.AttributeNotAllowedOnTarget, attribute.Token, nil, nil,
				"@%s may only annotate %s at %d:%d", attribute.Name.Value, spec.targetText, declaration.Line, declaration.Column)
			continue
		}
		accepted = append(accepted, attribute)
	}
	return accepted
}

// parseAttributedImplMember parses an attachment set inside an impl and the
// member it annotates: a method, `init` or `free` (MD-030). A set before
// `property` is rejected: each accessor carries its own attributes.
//
// Rules:
//   - rules/foundations/attributes.md — "Callable-body attachment (MD-030)"
//   - rules/declarations/impl.md — `init`, `free`
func (p *Parser) parseAttributedImplMember() ast.ImplMember {
	attributes, attached := p.parseAttributeSet()
	if !attached {
		return nil
	}
	switch {
	case p.peekToken.Type == lexer.IDENT && p.peekToken.Lexeme == contextualInit && p.tokenAfterPeek() == lexer.LPAREN:
		p.nextToken()
		start := p.curToken
		initializer := p.parseInitDeclaration()
		if initializer == nil {
			return nil
		}
		initializer.Attributes = p.acceptAttributes(attributes, attributeOnInit, start)
		return initializer
	case p.peekToken.Type == lexer.FREE && p.tokenAfterPeek() == lexer.LBRACE:
		p.nextToken()
		free := &ast.FreeDeclaration{Token: p.curToken}
		free.Body = p.parseFunctionBlockStatement()
		if free.Body == nil {
			return nil
		}
		free.Attributes = p.acceptAttributes(attributes, attributeOnFree, free.Token)
		return free
	case p.peekToken.Type == lexer.PROPERTY:
		for _, attribute := range attributes {
			p.addDiagnostic(compilerdiagnostics.AttributeNotAllowedOnTarget, attribute.Token, nil, nil,
				"@%s cannot annotate a whole property; write it immediately before get, set or try set at %d:%d", attribute.Name.Value, attribute.Token.Line, attribute.Token.Column)
		}
		p.nextToken()
		property := p.parsePropertyDeclaration()
		if property == nil {
			return nil
		}
		return property
	}
	if p.peekToken.Type == lexer.MUT && p.tokenAfterPeek() == lexer.FN {
		p.nextToken()
		p.reportMutFnInImpl()
	}
	p.nextToken()
	start := p.curToken
	statement := p.parseStatement()
	fn, ok := statement.(*ast.FunctionDeclaration)
	if !ok || fn == nil {
		return nil
	}
	p.attachAttributes(fn, attributes, true, start)
	return fn
}

// parseAttribute parses `@name` and its optional argument list with the
// current token on `@`. It reports unknown names and returns whether the name
// is compiler-known; arguments are validated against the attribute's form.
func (p *Parser) parseAttribute() (*ast.Attribute, bool) {
	attributeToken := p.curToken
	if !p.expectPeek(lexer.IDENT) {
		return nil, false
	}
	nameToken := p.curToken
	attribute := &ast.Attribute{Token: attributeToken, Name: &ast.Identifier{Token: nameToken, Value: nameToken.Lexeme}}
	spec, known := compilerKnownAttributes[nameToken.Lexeme]
	if !known {
		p.addDiagnostic(compilerdiagnostics.UnknownAttribute, attributeToken, nil, &nameToken,
			"unknown attribute @%s at %d:%d", nameToken.Lexeme, attributeToken.Line, attributeToken.Column)
		if p.peekToken.Type == lexer.LPAREN {
			p.consumeAttributeArguments()
		}
		return nil, false
	}
	if p.peekToken.Type == lexer.LPAREN {
		open := p.peekToken
		if spec.shape == attributeNoArguments {
			p.addDiagnostic(compilerdiagnostics.AttributeInvalidArgument, open, nil, nil,
				"@%s does not take arguments at %d:%d", nameToken.Lexeme, open.Line, open.Column)
			p.consumeAttributeArguments()
			return attribute, true
		}
		p.nextToken()
		arguments, ok := p.parseAttributeArguments()
		if !ok {
			return attribute, true
		}
		attribute.Arguments = arguments
	}
	p.validateAttributeArguments(attribute, spec)
	return attribute, true
}

// parseAttributeArguments parses `( argument, ... )` with the current token on
// `(`. An argument is `name: expression` or an expression; a trailing comma is
// allowed.
func (p *Parser) parseAttributeArguments() ([]*ast.AttributeArgument, bool) {
	arguments := []*ast.AttributeArgument{}
	for p.peekToken.Type != lexer.RPAREN {
		argument := &ast.AttributeArgument{Token: p.peekToken}
		if p.peekToken.Type == lexer.IDENT && p.tokenAfterPeek() == lexer.COLON {
			p.nextToken()
			argument.Name = &ast.Identifier{Token: p.curToken, Value: p.curToken.Lexeme}
			p.nextToken()
		}
		p.nextToken()
		argument.Value = p.parseExpression(LOWEST)
		if argument.Value == nil {
			p.recoverAttributeArguments()
			return nil, false
		}
		arguments = append(arguments, argument)
		if p.peekToken.Type == lexer.COMMA {
			p.nextToken()
			continue
		}
		if p.peekToken.Type != lexer.RPAREN {
			unexpected := p.peekToken
			p.addDiagnostic(compilerdiagnostics.AttributeInvalidArgument, unexpected, []lexer.TokenType{lexer.COMMA, lexer.RPAREN}, &unexpected,
				"expected ',' or ')' in attribute arguments at %d:%d", unexpected.Line, unexpected.Column)
			p.recoverAttributeArguments()
			return nil, false
		}
	}
	p.nextToken()
	return arguments, true
}

// validateAttributeArguments enforces the attribute's argument form and
// reports a repeated argument name with both locations.
//
// Rules:
//   - rules/foundations/attributes.md — "Positional and named arguments", "Duplicate arguments", "@link_name"
func (p *Parser) validateAttributeArguments(attribute *ast.Attribute, spec attributeSpec) {
	name := attribute.Name.Value
	invalid := func(token lexer.Token, format string, args ...any) {
		p.addDiagnostic(compilerdiagnostics.AttributeInvalidArgument, token, nil, nil, format+" at %d:%d", append(args, token.Line, token.Column)...)
	}
	switch spec.shape {
	case attributeOnePositional, attributeOneStringLiteral:
		if len(attribute.Arguments) != 1 || attribute.Arguments[0].Name != nil {
			invalid(attribute.Name.Token, "@%s requires exactly one positional argument", name)
			return
		}
		if spec.shape == attributeOneStringLiteral {
			literal, ok := attribute.Arguments[0].Value.(*ast.StringLiteral)
			if !ok {
				invalid(attribute.Arguments[0].Token, "@%s requires a string literal", name)
			} else if literal.Value == "" {
				invalid(attribute.Arguments[0].Token, "@%s requires a non-empty symbol name", name)
			}
		}
	case attributeNamedArguments:
		seen := map[string]*ast.AttributeArgument{}
		for _, argument := range attribute.Arguments {
			if argument.Name == nil {
				invalid(argument.Token, "@%s takes only named arguments (%s)", name, joinNames(spec.named))
				continue
			}
			if previous, duplicate := seen[argument.Name.Value]; duplicate {
				p.addDiagnostic(compilerdiagnostics.AttributeDuplicateArgument, argument.Token, nil, nil,
					"duplicate argument %s in @%s at %d:%d; first given at %d:%d",
					argument.Name.Value, name, argument.Token.Line, argument.Token.Column, previous.Token.Line, previous.Token.Column)
				continue
			}
			seen[argument.Name.Value] = argument
			if !containsName(spec.named, argument.Name.Value) {
				invalid(argument.Token, "@%s has no argument named %s; expected %s", name, argument.Name.Value, joinNames(spec.named))
			}
		}
		for _, required := range spec.required {
			if seen[required] == nil {
				invalid(attribute.Name.Token, "@%s requires the named argument %s", name, required)
			}
		}
	}
}

// attachAttributes checks every attribute of the set against the attached
// declaration, records the set on it, and derives the specialized fields.
//
// Rules:
//   - rules/foundations/attributes.md — "Attribute attachment", "Allowed targets for verified effects", "Parser representation"
func (p *Parser) attachAttributes(statement ast.Statement, attributes []*ast.Attribute, member bool, declaration lexer.Token) {
	if len(attributes) == 0 {
		return
	}
	kind := attributeTargetOf(statement, member)
	accepted := []*ast.Attribute{}
	for _, attribute := range attributes {
		spec := compilerKnownAttributes[attribute.Name.Value]
		if spec.targets&kind == 0 && spec.targets&attributeOnAnyTopLevel == 0 {
			p.addDiagnostic(compilerdiagnostics.AttributeNotAllowedOnTarget, attribute.Token, nil, nil,
				"@%s may only annotate %s at %d:%d", attribute.Name.Value, spec.targetText, declaration.Line, declaration.Column)
			continue
		}
		accepted = append(accepted, attribute)
	}
	if target := findAttribute(accepted, contextualTarget); target != nil {
		p.checkTargetAgainstFile(target)
	}
	switch statement := statement.(type) {
	case *ast.FunctionDeclaration:
		statement.Attributes = append(statement.Attributes, accepted...)
		if linkName := findAttribute(accepted, "link_name"); linkName != nil && len(linkName.Arguments) == 1 {
			if literal, ok := linkName.Arguments[0].Value.(*ast.StringLiteral); ok {
				statement.LinkName = literal.Value
			}
		}
	case *ast.TypeDeclStatement:
		statement.Attributes = append(statement.Attributes, accepted...)
	case *ast.EnumDeclaration:
		statement.Attributes = append(statement.Attributes, accepted...)
	case *ast.LetStatement:
		statement.Attributes = append(statement.Attributes, accepted...)
		if address := findAttribute(accepted, "address"); address != nil && len(address.Arguments) == 1 {
			statement.Address = address.Arguments[0].Value
			statement.AddressToken = address.Token
		}
	case *ast.LetGroupStatement:
		if address := findAttribute(accepted, "address"); address != nil {
			p.addDiagnostic(compilerdiagnostics.AttributeNotAllowedOnTarget, statement.Token, nil, nil,
				"@address cannot annotate grouped let declarations at %d:%d", statement.Token.Line, statement.Token.Column)
		}
	}
}

// attributeTargetOf classifies the declaration an attribute set attaches to.
func attributeTargetOf(statement ast.Statement, member bool) attributeTargets {
	switch statement := statement.(type) {
	case *ast.FunctionDeclaration:
		switch {
		case statement.Extern:
			return attributeOnExtern
		case member:
			return attributeOnMethod
		default:
			return attributeOnFunction
		}
	case *ast.TypeDeclStatement, *ast.EnumDeclaration:
		return attributeOnNominalType
	case *ast.LetStatement, *ast.LetGroupStatement:
		return attributeOnLet
	}
	return 0
}

func findAttribute(attributes []*ast.Attribute, name string) *ast.Attribute {
	for _, attribute := range attributes {
		if attribute.Name.Value == name {
			return attribute
		}
	}
	return nil
}

func containsName(names []string, name string) bool {
	for _, candidate := range names {
		if candidate == name {
			return true
		}
	}
	return false
}

func joinNames(names []string) string {
	text := ""
	for index, name := range names {
		if index > 0 {
			text += ", "
		}
		text += name
	}
	return text
}

func isNilStatement(statement ast.Statement) bool {
	if statement == nil {
		return true
	}
	switch statement := statement.(type) {
	case *ast.FunctionDeclaration:
		return statement == nil
	case *ast.TypeDeclStatement:
		return statement == nil
	case *ast.EnumDeclaration:
		return statement == nil
	case *ast.LetStatement:
		return statement == nil
	}
	return false
}

// attributesConflict reports whether a repeated single-valued attribute
// states a different value: two absolute addresses or two interrupt vectors
// are a conflict, while an identical repetition is a duplicate.
//
// Rules:
//   - rules/foundations/attributes.md — "Duplicate attributes", "Conflicting attributes"
func attributesConflict(previous, current *ast.Attribute) (bool, string) {
	value := func(attribute *ast.Attribute) string {
		parts := []string{}
		for _, argument := range attribute.Arguments {
			if argument.Value != nil {
				parts = append(parts, argument.Value.String())
			}
		}
		return strings.Join(parts, ",")
	}
	if value(previous) == value(current) {
		return false, ""
	}
	switch current.Name.Value {
	case "address":
		return true, "a declaration has exactly one absolute address"
	case "interrupt":
		return true, "a handler binds exactly one interrupt vector"
	}
	return false, ""
}

// checkTargetAgainstFile rejects a statement-level @target selector that
// contradicts the file's #target directive: such a declaration could never
// be active.
//
// Rules:
//   - rules/foundations/attributes.md — "Conflicting attributes" (contradictory target selectors), "#target compatibility form"
func (p *Parser) checkTargetAgainstFile(target *ast.Attribute) {
	if p.fileTarget == nil {
		return
	}
	for _, argument := range target.Arguments {
		literal, ok := argument.Value.(*ast.StringLiteral)
		if argument.Name == nil || !ok {
			continue
		}
		file := ""
		switch argument.Name.Value {
		case "os":
			file = p.fileTarget.OS
		case "arch":
			file = p.fileTarget.Arch
		default:
			continue
		}
		if literal.Value != file {
			p.addDiagnostic(compilerdiagnostics.AttributeConflict, argument.Token, nil, nil,
				"@target(%s: %q) at %d:%d contradicts the file's #target(%s: %q) at %d:%d; the declaration could never be active",
				argument.Name.Value, literal.Value, argument.Token.Line, argument.Token.Column,
				argument.Name.Value, file, p.fileTarget.Token.Line, p.fileTarget.Token.Column)
		}
	}
}

// AttributeTarget names a declaration kind an attribute can modify.
type AttributeTarget string

const (
	AttributeTargetFunction AttributeTarget = "function"
	AttributeTargetMethod   AttributeTarget = "method"
	AttributeTargetExtern   AttributeTarget = "extern"
	AttributeTargetType     AttributeTarget = "type"
	AttributeTargetLet      AttributeTarget = "let"
)

// AttributeInfo is the tooling view of one compiler-known attribute,
// derived from the registry the parser validates against.
type AttributeInfo struct {
	Name           string
	Summary        string
	TargetText     string
	Arguments      string
	NamedArguments []string
	spec           attributeSpec
}

// AllowedOn reports whether the attribute may modify the given declaration
// kind.
func (info AttributeInfo) AllowedOn(target AttributeTarget) bool {
	if info.spec.targets&attributeOnAnyTopLevel != 0 {
		return true
	}
	switch target {
	case AttributeTargetFunction:
		return info.spec.targets&attributeOnFunction != 0
	case AttributeTargetMethod:
		return info.spec.targets&attributeOnMethod != 0
	case AttributeTargetExtern:
		return info.spec.targets&attributeOnExtern != 0
	case AttributeTargetType:
		return info.spec.targets&attributeOnNominalType != 0
	case AttributeTargetLet:
		return info.spec.targets&attributeOnLet != 0
	}
	return false
}

// attributeOrder is the rulebook order of the initial attribute set.
var attributeOrder = []string{contextualTarget, "when", "address", "interrupt", "isr", "interruptSafe", "noCopy", "noAlloc", "noPanic", "noBlock", "link_name"}

// CompilerKnownAttributes lists the closed attribute set in rulebook order.
//
// Rules:
//   - rules/foundations/attributes.md — "Initial compiler-known attribute set", "LSP completion"
func CompilerKnownAttributes() []AttributeInfo {
	result := make([]AttributeInfo, 0, len(attributeOrder))
	for _, name := range attributeOrder {
		info, _ := CompilerKnownAttribute(name)
		result = append(result, info)
	}
	return result
}

// CompilerKnownAttribute returns the tooling view of one attribute.
func CompilerKnownAttribute(name string) (AttributeInfo, bool) {
	spec, ok := compilerKnownAttributes[name]
	if !ok {
		return AttributeInfo{}, false
	}
	info := AttributeInfo{Name: name, Summary: spec.summary, TargetText: spec.targetText, NamedArguments: append([]string(nil), spec.named...), spec: spec}
	switch spec.shape {
	case attributeNoArguments:
		info.Arguments = "none"
	case attributeOnePositional:
		info.Arguments = "one positional compile-time value"
	case attributeOneStringLiteral:
		info.Arguments = "one non-empty string literal"
	case attributeNamedArguments:
		info.Arguments = "named: " + strings.Join(spec.named, ", ")
		if len(spec.required) > 0 {
			info.Arguments += " (required: " + strings.Join(spec.required, ", ") + ")"
		}
	}
	if spec.targets&attributeOnAnyTopLevel != 0 {
		info.TargetText = "any top-level declaration"
	}
	return info, true
}
