package sema

import (
	"reflect"
	"sort"
	"strings"

	"sec/internal/ast"
	"sec/internal/lexer"
)

// Static shadow-knowledge analysis (MD-113). Each hardware instance — `self`
// inside a register impl member, or a hardware binding — starts with every
// shadow field Unknown. A read of a shadow field needs a complete known value:
// an observation (`Read()`) or an ordinary write establishes it, a specialized
// write action never does, `Write()` forgets the action fields and
// `Write(value)` everything, and the true branch of `IsShadowKnown(T.F)`
// knows F. Register impl members get summaries (requires, ensures and
// ensures-when-true) that are applied at every call; a requirement on `self`
// becomes the member's own requirement, and an unproven requirement on a
// hardware binding is an error. Nothing here ever reads hardware.
//
// Rules:
//   - rules/platform/hardware-register-access.md — § 9.8 (Amendment MD-113)
//   - rules/corrections/applied/MD-113-shadow-knowledge-across-calls-decision-20261030.md — §§ 2, 5, 7, 8, 11

// ShadowKnowledgeSummary is the published static contract of one register
// impl member ("T.M", "T.P.get" or "T.P.set"): field paths it needs known on
// its receiver, field paths known on every normal return, and field paths
// known when it returns true.
type ShadowKnowledgeSummary struct {
	Requires        []string
	Ensures         []string
	EnsuresWhenTrue []string
	// Invalidates lists the fields the member may make unknown on its
	// receiver (a commit forgets action fields, Write(value) everything).
	Invalidates []string
}

// ShadowContractAt returns the register impl member, and its summary, whose
// declaration name or call name is at a source position, for tooling.
func (a *Analyzer) ShadowContractAt(file string, line, column int) (string, ShadowKnowledgeSummary, bool) {
	if a == nil {
		return "", ShadowKnowledgeSummary{}, false
	}
	member, ok := a.shadowContractTokens[sourceTokenKey{File: file, Line: line, Column: column}]
	if !ok {
		return "", ShadowKnowledgeSummary{}, false
	}
	summary, ok := a.ShadowKnowledgeSummaryOf(member)
	return member, summary, ok
}

// ShadowKnowledgeSummaryOf returns the summary of a register impl member.
func (a *Analyzer) ShadowKnowledgeSummaryOf(member string) (ShadowKnowledgeSummary, bool) {
	if a == nil {
		return ShadowKnowledgeSummary{}, false
	}
	summary, ok := a.shadowSummaries[member]
	if !ok {
		return ShadowKnowledgeSummary{}, false
	}
	return ShadowKnowledgeSummary{Requires: sortedSet(summary.requires), Ensures: sortedSet(summary.ensures), EnsuresWhenTrue: sortedSet(summary.whenTrue), Invalidates: sortedSet(summary.invalidates)}, true
}

type shadowSummary struct {
	requires    map[string]bool
	ensures     map[string]bool
	whenTrue    map[string]bool
	invalidates map[string]bool
}

func newShadowSummary() *shadowSummary {
	return &shadowSummary{requires: map[string]bool{}, ensures: map[string]bool{}, whenTrue: map[string]bool{}, invalidates: map[string]bool{}}
}

func (s *shadowSummary) equal(other *shadowSummary) bool {
	return reflect.DeepEqual(s.requires, other.requires) && reflect.DeepEqual(s.ensures, other.ensures) && reflect.DeepEqual(s.whenTrue, other.whenTrue) && reflect.DeepEqual(s.invalidates, other.invalidates)
}

// shadowKnowledge maps an instance ("self" or a binding name) to its known
// shadow field paths.
type shadowKnowledge map[string]map[string]bool

func (k shadowKnowledge) clone() shadowKnowledge {
	out := shadowKnowledge{}
	for instance, fields := range k {
		copied := map[string]bool{}
		for field := range fields {
			copied[field] = true
		}
		out[instance] = copied
	}
	return out
}

func (k shadowKnowledge) knows(instance, field string) bool { return k[instance][field] }

func (k shadowKnowledge) add(instance string, fields ...string) {
	if k[instance] == nil {
		k[instance] = map[string]bool{}
	}
	for _, field := range fields {
		k[instance][field] = true
	}
}

// intersect keeps only what every path knows.
func intersectKnowledge(paths ...shadowKnowledge) shadowKnowledge {
	if len(paths) == 0 {
		return shadowKnowledge{}
	}
	out := shadowKnowledge{}
	for instance, fields := range paths[0] {
		for field := range fields {
			everywhere := true
			for _, other := range paths[1:] {
				everywhere = everywhere && other.knows(instance, field)
			}
			if everywhere {
				out.add(instance, field)
			}
		}
	}
	return out
}

// shadowWalk is the state of analyzing one body.
type shadowWalk struct {
	a        *Analyzer
	member   string // the register impl member being summarized, or ""
	summary  *shadowSummary
	report   bool
	returns  []shadowKnowledge
	whenTrue []map[string]bool
}

type shadowBody struct {
	member string
	body   *ast.BlockStatement
}

// analyzeShadowKnowledge computes member summaries to a fixed point and then
// checks every body.
func (a *Analyzer) analyzeShadowKnowledge(program *ast.Program) {
	if program == nil || !a.hasShadowRegister() {
		return
	}
	var members, others []shadowBody
	a.shadowContractTokens = map[sourceTokenKey]string{}
	for _, statement := range program.Statements {
		switch statement := statement.(type) {
		case *ast.FunctionDeclaration:
			if statement != nil && statement.Body != nil {
				others = append(others, shadowBody{body: statement.Body})
			}
		case *ast.TestDeclaration:
			if statement != nil && statement.Body != nil {
				others = append(others, shadowBody{body: statement.Body})
			}
		case *ast.ImplStatement:
			if statement == nil || statement.Target == nil {
				continue
			}
			target, isRegister := a.types[statement.Target.Name]
			for _, member := range statement.Members {
				switch member := member.(type) {
				case *ast.FunctionDeclaration:
					if member == nil || member.Name == nil || member.Body == nil {
						continue
					}
					if isRegister && target.Kind == RegisterType && !member.Static {
						a.shadowContractTokens[sourceTokenLocation(member.Name.Token)] = statement.Target.Name + "." + member.Name.Value
						members = append(members, shadowBody{member: statement.Target.Name + "." + member.Name.Value, body: member.Body})
					} else {
						others = append(others, shadowBody{body: member.Body})
					}
				case *ast.PropertyDeclaration:
					if member == nil || member.Name == nil || !isRegister || target.Kind != RegisterType || member.Static {
						continue
					}
					if member.Getter != nil {
						members = append(members, shadowBody{member: statement.Target.Name + "." + member.Name.Value + ".get", body: member.Getter})
					}
					if member.Setter != nil && member.Setter.Body != nil {
						members = append(members, shadowBody{member: statement.Target.Name + "." + member.Name.Value + ".set", body: member.Setter.Body})
					}
				}
			}
		}
	}
	a.shadowSummaries = map[string]*shadowSummary{}
	for _, member := range members {
		a.shadowSummaries[member.member] = newShadowSummary()
	}
	for round := 0; round <= len(members)+1; round++ {
		changed := false
		for _, member := range members {
			next := a.summarizeShadowBody(member, false)
			if !next.equal(a.shadowSummaries[member.member]) {
				a.shadowSummaries[member.member] = next
				changed = true
			}
		}
		if !changed {
			break
		}
	}
	for _, member := range members {
		a.summarizeShadowBody(member, true)
	}
	for _, other := range others {
		a.summarizeShadowBody(other, true)
	}
}

func (a *Analyzer) hasShadowRegister() bool {
	for _, typ := range a.types {
		if typ.Kind == RegisterType && registerHasShadowField(typ) {
			return true
		}
	}
	return false
}

func registerHasShadowField(register Type) bool {
	for _, field := range register.RegisterFields {
		if field.Shadow == RegisterShadowed || field.Type.Kind == RegisterType && registerHasShadowField(field.Type) {
			return true
		}
	}
	return false
}

func (a *Analyzer) summarizeShadowBody(body shadowBody, report bool) *shadowSummary {
	walk := &shadowWalk{a: a, member: body.member, report: report, summary: newShadowSummary()}
	end, falls := walk.block(body.body, shadowKnowledge{})
	if falls {
		walk.returns = append(walk.returns, end)
		walk.whenTrue = append(walk.whenTrue, nil)
	}
	if len(walk.returns) > 0 {
		for field := range intersectKnowledge(walk.returns...)["self"] {
			walk.summary.ensures[field] = true
		}
		conditional := []shadowKnowledge{}
		for index, known := range walk.returns {
			if walk.whenTrue[index] == nil {
				conditional = nil
				break
			}
			extended := known.clone()
			for field := range walk.whenTrue[index] {
				extended.add("self", field)
			}
			conditional = append(conditional, extended)
		}
		if conditional != nil {
			for field := range intersectKnowledge(conditional...)["self"] {
				if !walk.summary.ensures[field] {
					walk.summary.whenTrue[field] = true
				}
			}
		}
	}
	return walk.summary
}

// block walks statements in order and reports whether control reaches the end.
func (w *shadowWalk) block(block *ast.BlockStatement, known shadowKnowledge) (shadowKnowledge, bool) {
	if block == nil {
		return known, true
	}
	for _, statement := range block.Statements {
		var falls bool
		known, falls = w.statement(statement, known)
		if !falls {
			return known, false
		}
	}
	return known, true
}

func (w *shadowWalk) statement(statement ast.Statement, known shadowKnowledge) (shadowKnowledge, bool) {
	switch statement := statement.(type) {
	case *ast.ReturnStatement:
		if statement == nil {
			return known, false
		}
		var whenTrue map[string]bool
		if statement.Value != nil {
			known = w.expression(statement.Value, known)
			whenTrue = w.conditionKnowledge(statement.Value)
			if literal, ok := statement.Value.(*ast.BooleanLiteral); ok && !literal.Value {
				whenTrue = map[string]bool{"\x00false": true}
			}
		}
		w.returns = append(w.returns, known.clone())
		w.whenTrue = append(w.whenTrue, whenTrue)
		return known, false
	case *ast.ExpressionStatement:
		if statement != nil {
			known = w.expression(statement.Expression, known)
		}
	case *ast.LetStatement:
		if statement != nil && statement.Value != nil {
			known = w.expression(statement.Value, known)
		}
	case *ast.AssignmentStatement:
		if statement == nil {
			break
		}
		known = w.expression(statement.Value, known)
		if statement.Operator != "=" {
			known = w.expression(statement.Target, known)
		}
		w.assignTarget(statement.Target, known)
	case *ast.IfStatement:
		if statement == nil {
			break
		}
		known = w.expression(statement.Condition, known)
		then := known.clone()
		for field, instance := range w.conditionFields(statement.Condition) {
			then.add(instance, field)
		}
		thenEnd, thenFalls := w.block(statement.Consequence, then)
		elseEnd, elseFalls := w.block(statement.Alternative, known.clone())
		switch {
		case thenFalls && elseFalls:
			return intersectKnowledge(thenEnd, elseEnd), true
		case thenFalls:
			return thenEnd, true
		case elseFalls:
			return elseEnd, true
		}
		return known, false
	case *ast.WhileStatement:
		if statement == nil {
			break
		}
		known = w.expression(statement.Condition, known)
		first, _ := w.block(statement.Body, known.clone())
		again := intersectKnowledge(known, first)
		second, _ := w.block(statement.Body, again.clone())
		return intersectKnowledge(known, second), true
	case *ast.ForStatement:
		if statement == nil {
			break
		}
		known = w.expression(statement.Iterable, known)
		first, _ := w.block(statement.Body, known.clone())
		again := intersectKnowledge(known, first)
		second, _ := w.block(statement.Body, again.clone())
		return intersectKnowledge(known, second), true
	case *ast.MatchStatement:
		if statement != nil && statement.Match != nil {
			return w.match(statement.Match, known)
		}
	case *ast.SwitchStatement:
		if statement == nil {
			break
		}
		known = w.expression(statement.Subject, known)
		var ends []shadowKnowledge
		cases := append([]*ast.SwitchCase(nil), statement.Cases...)
		if statement.Default != nil {
			cases = append(cases, statement.Default)
		}
		for _, item := range cases {
			if end, falls := w.block(item.Body, known.clone()); falls {
				ends = append(ends, end)
			}
		}
		if statement.Default == nil {
			ends = append(ends, known)
		}
		if len(ends) == 0 {
			return known, false
		}
		return intersectKnowledge(ends...), true
	default:
		w.nested(statement, known)
	}
	return known, true
}

func (w *shadowWalk) match(match *ast.MatchExpression, known shadowKnowledge) (shadowKnowledge, bool) {
	known = w.expression(match.Subject, known)
	var ends []shadowKnowledge
	for _, arm := range match.Arms {
		armKnown := known.clone()
		if arm.Guard != nil {
			armKnown = w.expression(arm.Guard, armKnown)
		}
		switch {
		case arm.ReturnBody != nil:
			w.statement(arm.ReturnBody, armKnown)
		case arm.BlockBody != nil:
			if end, falls := w.block(arm.BlockBody, armKnown); falls {
				ends = append(ends, end)
			}
		case arm.Body != nil:
			ends = append(ends, w.expression(arm.Body, armKnown))
		default:
			ends = append(ends, armKnown)
		}
	}
	if len(ends) == 0 {
		return known, false
	}
	return intersectKnowledge(ends...), true
}

// nested walks any other node conservatively: requirements are checked, but
// knowledge it might establish is not kept.
func (w *shadowWalk) nested(node any, known shadowKnowledge) {
	walkASTValue(reflect.ValueOf(node), func(child any) {
		if expression, ok := child.(ast.Expression); ok && child != node {
			w.expression(expression, known.clone())
		}
	})
}

// expression applies the events of an expression in evaluation order.
func (w *shadowWalk) expression(expression ast.Expression, known shadowKnowledge) shadowKnowledge {
	if expression == nil || reflect.ValueOf(expression).IsNil() {
		return known
	}
	switch node := expression.(type) {
	case *ast.CallExpression:
		if member, ok := node.Callee.(*ast.MemberExpression); ok {
			known = w.expression(member.Object, known)
		}
		for _, argument := range node.Arguments {
			if operation, isOperation := w.a.resolvedRegisterOperations[node]; isOperation && operation.Kind == RegisterOperationShadowKnown {
				break
			}
			known = w.expression(argument, known)
		}
		return w.call(node, known)
	case *ast.MemberExpression:
		if _, field := w.a.resolvedRegisterFields[node]; field {
			return w.fieldRead(node, known)
		}
		if access, ok := w.a.resolvedPropertyAccesses[node]; ok && (access.Kind == PropertyRead || access.Kind == PropertyCompoundUpdate) {
			known = w.expression(node.Object, known)
			return w.memberUse(registerOwnerName(access.OwnerType)+"."+access.Name+".get", node.Object, expressionToken(node), known)
		}
		return w.expression(node.Object, known)
	case *ast.Identifier:
		if use, ok := w.a.implicitReceiverUses[node]; ok && use.RegisterField != nil {
			return w.fieldRead(node, known)
		}
		return known
	case *ast.MatchExpression:
		end, _ := w.match(node, known)
		return end
	case *ast.LambdaExpression:
		return known
	}
	walkASTValue(reflect.ValueOf(expression), func(child any) {
		switch child := child.(type) {
		case ast.Expression:
			if child != expression {
				known = w.expression(child, known)
			}
		case *ast.BlockStatement:
			w.block(child, known.clone())
		}
	})
	return known
}

// fieldPath returns the register field path and receiver of a field step.
func (w *shadowWalk) fieldPath(expression ast.Expression) (ResolvedRegisterFieldPlan, []string, ast.Expression, bool) {
	var names []string
	var leaf ResolvedRegisterFieldPlan
	current := expression
	for {
		switch node := current.(type) {
		case *ast.MemberExpression:
			plan, ok := w.a.resolvedRegisterFields[node]
			if !ok {
				return leaf, nil, nil, false
			}
			if names == nil {
				leaf = plan
			}
			names = append([]string{node.Property.Value}, names...)
			if _, nested := w.a.resolvedRegisterFields[asMember(node.Object)]; nested {
				current = node.Object
				continue
			}
			return leaf, names, node.Object, true
		case *ast.Identifier:
			use, ok := w.a.implicitReceiverUses[node]
			if !ok || use.RegisterField == nil {
				return leaf, nil, nil, false
			}
			if names == nil {
				leaf = *use.RegisterField
			}
			return leaf, append([]string{node.Value}, names...), nil, true
		default:
			return leaf, nil, nil, false
		}
	}
}

func asMember(expression ast.Expression) *ast.MemberExpression {
	member, _ := expression.(*ast.MemberExpression)
	return member
}

// instanceOf names the hardware instance a receiver denotes: "self" for the
// impl receiver (explicit or implicit) or a hardware binding's name.
func (w *shadowWalk) instanceOf(receiver ast.Expression) (string, bool) {
	switch node := receiver.(type) {
	case nil:
		return "self", true
	case *ast.Identifier:
		if node.Value == "self" {
			return "self", true
		}
		if symbol, ok := w.a.symbols[node.Value]; ok && isHardwareInstance(symbol.Type) {
			return node.Value, true
		}
	}
	return "", false
}

func (w *shadowWalk) fieldRead(expression ast.Expression, known shadowKnowledge) shadowKnowledge {
	plan, names, receiver, ok := w.fieldPath(expression)
	if !ok {
		return known
	}
	if receiver != nil {
		known = w.expression(receiver, known)
	}
	if plan.Request == nil || plan.Field.Shadow != RegisterShadowed || plan.Request.Kind != RegisterRequestObserve && plan.Request.Kind != RegisterRequestDestructiveObserve {
		return known
	}
	instance, hardware := w.instanceOf(receiver)
	if !hardware {
		return known
	}
	w.require(instance, strings.Join(names, "."), "", expressionToken(expression), known)
	return known
}

func (w *shadowWalk) assignTarget(target ast.Expression, known shadowKnowledge) {
	plan, names, receiver, ok := w.fieldPath(target)
	if !ok || plan.Request == nil || plan.Field.Shadow != RegisterShadowed {
		if access, isProperty := w.a.resolvedPropertyAccesses[target]; isProperty && access.Kind != PropertyRead {
			if member, isMember := target.(*ast.MemberExpression); isMember {
				w.memberUse(registerOwnerName(access.OwnerType)+"."+access.Name+".set", member.Object, expressionToken(target), known)
			}
		}
		return
	}
	instance, hardware := w.instanceOf(receiver)
	if hardware && plan.Request.Kind == RegisterRequestWriteValue {
		known.add(instance, strings.Join(names, "."))
	}
}

// require checks that a field is known on an instance. On `self` inside a
// summarized member an unknown field becomes the member's requirement;
// anywhere else it is an error when reporting.
func (w *shadowWalk) require(instance, field, callee string, token lexer.Token, known shadowKnowledge) {
	if known.knows(instance, field) {
		return
	}
	if instance == "self" && w.member != "" {
		w.summary.requires[field] = true
		return
	}
	if !w.report {
		return
	}
	reason := "it is read from software shadow state"
	if callee != "" {
		reason = callee + " requires it (RequiresKnown)"
	}
	w.a.addErrorAtTokenWithMetadata(token, "",
		"Establish it first with a member that observes the register (Read()) or writes "+field+", or guard the use with a member that returns IsShadowKnown(...); shadow knowledge is never refreshed by a hidden hardware read.",
		"shadow field %s of %s may be unknown here: %s", field, instance, reason)
}

func (w *shadowWalk) call(call *ast.CallExpression, known shadowKnowledge) shadowKnowledge {
	if operation, ok := w.a.resolvedRegisterOperations[call]; ok {
		var receiver ast.Expression
		if member, isMember := call.Callee.(*ast.MemberExpression); isMember {
			receiver = member.Object
		}
		instance, hardware := w.instanceOf(receiver)
		if !hardware {
			return known
		}
		switch operation.Kind {
		case RegisterOperationRead:
			known.add(instance, registerShadowPaths(operation.Register, "", true)...)
		case RegisterOperationCommit:
			w.forget(instance, registerActionPaths(operation.Register, ""), known)
		case RegisterOperationWriteWhole:
			w.forget(instance, registerShadowPaths(operation.Register, "", false), known)
		}
		return known
	}
	resolved, ok := w.a.resolvedCalls[call]
	if !ok || resolved.Function.ImplTarget == "" || resolved.Function.Static {
		return known
	}
	var receiver ast.Expression
	if member, isMember := call.Callee.(*ast.MemberExpression); isMember {
		receiver = member.Object
	}
	key := registerMemberKey(resolved.Function.ImplTarget, resolved.Function.Name)
	if member, isMember := call.Callee.(*ast.MemberExpression); isMember && member.Property != nil {
		w.a.shadowContractTokens[sourceTokenLocation(member.Property.Token)] = key
	}
	return w.memberUse(key, receiver, call.Token, known)
}

// memberUse applies a register member's summary at a use on receiver.
func (w *shadowWalk) memberUse(member string, receiver ast.Expression, token lexer.Token, known shadowKnowledge) shadowKnowledge {
	summary, ok := w.a.shadowSummaries[member]
	if !ok {
		return known
	}
	instance, hardware := w.instanceOf(receiver)
	if !hardware {
		return known
	}
	display := strings.TrimSuffix(strings.TrimSuffix(member, ".get"), ".set")
	for _, field := range sortedSet(summary.requires) {
		w.require(instance, field, display, token, known)
	}
	w.forget(instance, sortedSet(summary.invalidates), known)
	known.add(instance, sortedSet(summary.ensures)...)
	return known
}

// forget makes fields unknown on an instance; inside a summarized member a
// loss on `self` becomes part of the member's invalidations.
func (w *shadowWalk) forget(instance string, fields []string, known shadowKnowledge) {
	for _, field := range fields {
		delete(known[instance], field)
		if instance == "self" && w.member != "" {
			w.summary.invalidates[field] = true
		}
	}
}

// conditionFields returns the fields a true condition makes known, keyed by
// field with the instance as value.
func (w *shadowWalk) conditionFields(condition ast.Expression) map[string]string {
	out := map[string]string{}
	call, ok := condition.(*ast.CallExpression)
	if !ok {
		return out
	}
	var receiver ast.Expression
	if member, isMember := call.Callee.(*ast.MemberExpression); isMember {
		receiver = member.Object
	}
	instance, hardware := w.instanceOf(receiver)
	if !hardware {
		return out
	}
	if operation, isOperation := w.a.resolvedRegisterOperations[call]; isOperation && operation.Kind == RegisterOperationShadowKnown {
		names := make([]string, 0, len(operation.FieldPath))
		for _, field := range operation.FieldPath {
			names = append(names, field.Name)
		}
		out[strings.Join(names, ".")] = instance
		return out
	}
	if resolved, isCall := w.a.resolvedCalls[call]; isCall && resolved.Function.ImplTarget != "" {
		if summary, has := w.a.shadowSummaries[registerMemberKey(resolved.Function.ImplTarget, resolved.Function.Name)]; has {
			for field := range summary.whenTrue {
				out[field] = instance
			}
		}
	}
	return out
}

// conditionKnowledge is the self knowledge a returned condition proves when
// it is true.
func (w *shadowWalk) conditionKnowledge(value ast.Expression) map[string]bool {
	fields := map[string]bool{}
	for field, instance := range w.conditionFields(value) {
		if instance == "self" {
			fields[field] = true
		}
	}
	if len(fields) == 0 {
		return nil
	}
	return fields
}

// registerShadowPaths lists the shadow field paths of a register, optionally
// only those an observation establishes (not write-only).
func registerShadowPaths(register Type, prefix string, observable bool) []string {
	var paths []string
	for _, field := range register.RegisterFields {
		if field.Name == "_" {
			continue
		}
		path := prefix + field.Name
		if field.Type.Kind == RegisterType {
			paths = append(paths, registerShadowPaths(field.Type, path+".", observable)...)
		}
		if field.Shadow == RegisterShadowed && (!observable || field.Access != RegisterWriteOnly) {
			paths = append(paths, path)
		}
	}
	return paths
}

// registerActionPaths lists the shadow fields with specialized write actions,
// whose knowledge a commit does not preserve.
func registerActionPaths(register Type, prefix string) []string {
	var paths []string
	for _, field := range register.RegisterFields {
		path := prefix + field.Name
		if field.Type.Kind == RegisterType {
			paths = append(paths, registerActionPaths(field.Type, path+".")...)
		}
		if _, _, special := registerWriteTrigger(field.Access); special {
			paths = append(paths, path)
		}
	}
	return paths
}

func registerOwnerName(owner Type) string {
	return dereferenceType(owner).Name
}

func sortedSet(set map[string]bool) []string {
	out := make([]string, 0, len(set))
	for item := range set {
		if !strings.HasPrefix(item, "\x00") {
			out = append(out, item)
		}
	}
	sort.Strings(out)
	return out
}
