package semantic

import (
	"fmt"
	"math/big"
	"strings"

	"sec/internal/ast"
	"sec/internal/lexer"
	"sec/internal/sema"
)

// typePackSpecialization is one concrete expansion of a type-pack template,
// demanded by a selected call. Its identity is the ordered type sequence, so
// order and arity select distinct specializations.
type typePackSpecialization struct {
	id       FunctionID
	function sema.Function
	facts    *TypePackFacts
	binding  sema.BindingID
	token    lexer.Token
}

// typePackState collects the module's templates and demanded specializations.
type typePackState struct {
	templates map[string]*ast.FunctionDeclaration
	demanded  map[FunctionID]*typePackSpecialization
	pending   []*typePackSpecialization
}

func typePackTemplateKey(function sema.Function) string {
	return fmt.Sprintf("%s:%d:%d:%s", function.Token.File, function.Token.Line, function.Token.Column, function.Name)
}

func (b *builder) registerTypePackTemplate(decl *ast.FunctionDeclaration, resolved sema.Function) {
	if b.typePacks.templates == nil {
		b.typePacks.templates = map[string]*ast.FunctionDeclaration{}
	}
	b.typePacks.templates[typePackTemplateKey(resolved)] = decl
}

// demandTypePackSpecialization resolves one selected call to its concrete
// specialization, recording it for construction once all templates are known.
// A body may depend on a scalar generic prefix, which needs the monomorphization
// that ordinary generic functions lack as well, so that form stays a boundary.
//
// Rules:
//   - rules/compiler/semantic_ir.md — § 27A
//   - rules/declarations/functions.md — § 38A(2)–(4), (9)
//   - rules/compiler/monomorphization.md — § 9.1
func (b *builder) demandTypePackSpecialization(call *ast.CallExpression, target sema.Function) (*typePackSpecialization, error) {
	unsupported := func(what string) error {
		return &UnsupportedFeatureError{Feature: what + " for " + target.Name, Package: b.maxPackage, Location: location(call.Token)}
	}
	fact, ok := b.analyzer.ResolvedTypePackCallOf(call)
	if !ok {
		return nil, unsupported("heterogeneous type-pack callable expansion without a resolved sequence")
	}
	if len(target.GenericSubstitution) != 1 {
		return nil, unsupported("type-pack callable with a scalar generic prefix")
	}
	if target.ImplTarget != "" {
		return nil, unsupported("type-pack method")
	}
	facts := &TypePackFacts{Parameter: fact.Name, FixedCount: fact.FixedCount}
	sequence := make([]string, len(fact.Types))
	for index, typ := range fact.Types {
		typeID, err := b.internType(typ)
		if err != nil {
			return nil, locateUnsupportedType(err, call.Token)
		}
		facts.Types = append(facts.Types, typeID)
		sequence[index] = canonicalSemaType(typ)
	}
	parameters := append([]sema.FunctionParameter(nil), fact.Parameters...)
	signature := make([]string, len(parameters))
	for index := range parameters {
		parameter := &parameters[index]
		if index >= fact.FixedCount {
			if parameter.Type.Kind != sema.ReferenceType || parameter.Type.ReferenceMutable {
				return nil, unsupported("non-shared type-pack position")
			}
			facts.Positions = append(facts.Positions, TypePackPosition{Mode: TypePackShared, Location: location(parameter.Token)})
			parameter.Name = fmt.Sprintf("%s[%d]", parameter.Name, index-fact.FixedCount)
			// The expanded type still spells the template's `ref Types`.
			parameter.Type.Name = "ref " + sequence[index-fact.FixedCount]
		}
		signature[index] = canonicalSemaType(parameter.Type)
	}
	id := FunctionID(fmt.Sprintf("%s::%s[...%s<%s>](%s)", target.Module, target.Name, fact.Name, strings.Join(sequence, ","), strings.Join(signature, ",")))
	if existing, seen := b.typePacks.demanded[id]; seen {
		return existing, nil
	}
	function := target
	function.Parameters = parameters
	function.GenericTypePack = ""
	spec := &typePackSpecialization{id: id, function: function, facts: facts, token: call.Token}
	if b.typePacks.demanded == nil {
		b.typePacks.demanded = map[FunctionID]*typePackSpecialization{}
	}
	b.typePacks.demanded[id] = spec
	b.typePacks.pending = append(b.typePacks.pending, spec)
	return spec, nil
}

// buildTypePackSpecializations builds demanded specializations in first-demand
// order. Building a body may demand more of them.
func (b *builder) buildTypePackSpecializations() error {
	for len(b.typePacks.pending) > 0 {
		spec := b.typePacks.pending[0]
		b.typePacks.pending = b.typePacks.pending[1:]
		decl := b.typePacks.templates[typePackTemplateKey(spec.function)]
		if decl == nil || decl.Body == nil || len(decl.Parameters) == 0 {
			return &UnsupportedFeatureError{Feature: "heterogeneous type-pack callable expansion for " + spec.function.Name, Package: b.maxPackage, Location: location(spec.token)}
		}
		if name := decl.Parameters[len(decl.Parameters)-1].Name; name != nil {
			if fact, ok := b.analyzer.ResolvedBindingOf(name); ok {
				spec.binding = fact.ID
			}
		}
		if err := b.buildConcreteFunction(decl, spec.function, spec); err != nil {
			return err
		}
	}
	return nil
}

// buildTypePackLength lowers `values.Len` of the pack being specialized to the
// exact length of this specialization (functions.md § 38A(5)(1–2)); the shared
// template AST holds no per-specialization value.
func (fb *functionBuilder) buildTypePackLength(member *ast.MemberExpression) (builtValue, bool, error) {
	if fb.typePack == nil || member.Property == nil || member.Property.Value != "Len" {
		return builtValue{}, false, nil
	}
	receiver, ok := member.Object.(*ast.Identifier)
	if !ok {
		return builtValue{}, false, nil
	}
	fact, ok := fb.owner.analyzer.ResolvedBindingOf(receiver)
	if !ok || fact.ID != fb.typePack.binding {
		return builtValue{}, false, nil
	}
	typeID, err := fb.owner.internType(sema.Type{Name: "uint", Kind: sema.UintType})
	if err != nil {
		return builtValue{}, true, err
	}
	length := new(big.Int).SetUint64(uint64(len(fb.typePack.facts.Types)))
	return fb.result(Operation{Kind: OpConstInt, Integer: length, Location: location(member.Token)}, typeID), true, nil
}

func formatTypePackFacts(facts *TypePackFacts) string {
	types := make([]string, len(facts.Types))
	for index, typ := range facts.Types {
		types[index] = fmt.Sprintf("!%d", typ)
	}
	return fmt.Sprintf(" type-pack(%s=[%s] len=%d fixed=%d)", facts.Parameter, strings.Join(types, ", "), len(facts.Types), facts.FixedCount)
}

// verifyTypePackFacts checks the § 27A facts against the concrete signature:
// each position receives exactly its element type in its recorded mode, and
// the sequence, positions and parameters agree in length.
func verifyTypePackFacts(module *Module, fn *Function) error {
	facts := fn.TypePack
	if facts == nil {
		return nil
	}
	if facts.Parameter == "" || facts.FixedCount < 0 || len(facts.Positions) != len(facts.Types) || facts.FixedCount+len(facts.Types) != len(fn.Parameters) {
		return fmt.Errorf("function %s has inconsistent type-pack facts", fn.ID)
	}
	for index, element := range facts.Types {
		if _, ok := module.Types.Lookup(element); !ok {
			return fmt.Errorf("function %s type-pack position %d has an invalid type", fn.ID, index)
		}
		value := fn.Parameters[facts.FixedCount+index].Value
		received, ok := module.Types.Lookup(value.Type)
		if facts.Positions[index].Mode != TypePackShared || !ok || received.Kind != TypeReference || received.Element != element || value.Ownership != OwnershipSharedReference {
			return fmt.Errorf("function %s type-pack position %d does not receive a shared reference to its element type", fn.ID, index)
		}
	}
	return nil
}
