package semantic

import (
	"fmt"
	"math/big"
	"sort"
	"strings"

	"sec/internal/ast"
	"sec/internal/lexer"
	"sec/internal/sema"
)

// specialization is one demand-driven concrete instantiation of a generic
// template: ordinary generic parameters, a heterogeneous type pack, or both.
// Its identity carries the concrete arguments in order, so a type-pack
// sequence's order and arity select distinct specializations.
type specialization struct {
	id       FunctionID
	function sema.Function
	// substitution maps every template parameter to its concrete type and is
	// applied while the typed template body is built.
	substitution map[string]sema.Type
	generic      *SpecializationFacts
	facts        *TypePackFacts
	binding      sema.BindingID
	token        lexer.Token
	// hardware marks the `T.M[hardware]` variant of an impl T member, whose
	// self is a hardware-instance authority (hardware_operations.go).
	hardware bool
}

// specializationState collects the module's templates and demanded
// specializations.
type specializationState struct {
	templates map[string]*ast.FunctionDeclaration
	demanded  map[FunctionID]*specialization
	pending   []*specialization
}

// maxSpecializations bounds demand so that polymorphic recursion such as
// F[T] calling F[Box[T]] terminates with a resource-limit failure
// (monomorphization.md §§ 18.1, 18.7).
const maxSpecializations = 4096

func genericTemplateKey(function sema.Function) string {
	return fmt.Sprintf("%s:%d:%d:%s", function.Token.File, function.Token.Line, function.Token.Column, function.Name)
}

func (b *builder) registerGenericTemplate(decl *ast.FunctionDeclaration, resolved sema.Function) {
	if b.specializations.templates == nil {
		b.specializations.templates = map[string]*ast.FunctionDeclaration{}
	}
	b.specializations.templates[genericTemplateKey(resolved)] = decl
}

// isGenericInstance reports a call target that Sema instantiated from a
// generic template.
func isGenericInstance(function sema.Function) bool {
	return len(function.GenericSubstitution) != 0
}

// substituteTemplateType applies a substitution to a resolved type, first
// turning a generic declaration's own template type (the receiver `Box` of a
// method of `impl Box[T]`, which names no arguments) into the instance the
// substitution selects.
func (b *builder) substituteTemplateType(typ sema.Type, substitution map[string]sema.Type) sema.Type {
	parameters := typ.GenericParameters
	if len(parameters) == 0 && len(typ.TypeArgs) == 0 && (typ.Kind == sema.StructType || typ.Kind == sema.UnionType) {
		parameters = b.analyzer.GenericTypeParameters(typ.Name)
	}
	if len(parameters) != 0 && len(typ.TypeArgs) == 0 {
		arguments := make([]sema.Type, len(parameters))
		for index, name := range parameters {
			argument, ok := substitution[name]
			if !ok {
				return typ
			}
			arguments[index] = argument
		}
		typ.TypeArgs = arguments
		typ.GenericParameters = nil
	}
	return sema.SubstituteGenericType(typ, substitution)
}

// concreteSemaType reports a type with no unresolved generic parameter left,
// the generic-closure condition of monomorphization.md § 13.3.
func concreteSemaType(typ sema.Type) bool {
	if typ.Kind == sema.GenericType {
		return false
	}
	nested := append([]sema.Type(nil), typ.TypeArgs...)
	nested = append(nested, typ.FunctionParameterTypes...)
	for _, pointer := range []*sema.Type{typ.Element, typ.NamedBase, typ.FunctionReturnType} {
		if pointer != nil {
			nested = append(nested, *pointer)
		}
	}
	for _, field := range typ.Fields {
		nested = append(nested, field.Type)
	}
	for _, variant := range typ.UnionVariants {
		if variant.Payload != nil {
			nested = append(nested, *variant.Payload)
		}
		for _, field := range variant.PayloadFields {
			nested = append(nested, field.Type)
		}
	}
	for _, inner := range nested {
		if !concreteSemaType(inner) {
			return false
		}
	}
	return true
}

// demandSpecialization resolves one selected call of a generic template to its
// concrete specialization and records it for construction once all templates
// are known. Inside a specialization, forwarded arguments such as Id[T] are
// first made concrete through the enclosing substitution.
//
// Rules:
//   - rules/compiler/monomorphization.md — §§ 4, 6.1–6.2, 8.1–8.3, 9.1, 13.3
//   - rules/compiler/semantic_ir.md — § 27A
//   - rules/declarations/functions.md — § 38A(2)–(4), (9)
func (b *builder) demandSpecialization(call *ast.CallExpression, target sema.Function) (*specialization, error) {
	unsupported := func(what string) error {
		return &UnsupportedFeatureError{Feature: what + " for " + target.Name, Package: b.maxPackage, Location: location(call.Token)}
	}
	outer := b.substitution
	concrete := func(typ sema.Type) sema.Type {
		if len(outer) == 0 {
			return typ
		}
		return sema.SubstituteGenericType(typ, outer)
	}
	substitution := make(map[string]sema.Type, len(target.GenericSubstitution))
	names := make([]string, 0, len(target.GenericSubstitution))
	for name, typ := range target.GenericSubstitution {
		substitution[name] = concrete(typ)
		if name != target.GenericTypePack {
			names = append(names, name)
		}
	}
	sort.Strings(names)
	generic := &SpecializationFacts{Template: target.Name}
	arguments := make([]string, 0, len(names)+1)
	for _, name := range names {
		typ := substitution[name]
		if !concreteSemaType(typ) {
			return nil, unsupported("generic specialization with an unresolved argument " + name)
		}
		// The template's ownership, cleanup and transfer plans were computed
		// for an unknown parameter. Only a trivially copyable and destructible
		// argument is guaranteed to need none of them (monomorphization.md
		// § 11.2), so other arguments stay a boundary until those plans are
		// derived per specialization.
		if !sema.TriviallyCopyable(typ) || !sema.TriviallyDestructible(typ) {
			return nil, unsupported("generic specialization with the ownership-sensitive argument " + name + "=" + canonicalSemaType(typ))
		}
		typeID, err := b.internType(typ)
		if err != nil {
			return nil, locateUnsupportedType(err, call.Token)
		}
		generic.Arguments = append(generic.Arguments, SpecializationArgument{Name: name, Type: typeID})
		arguments = append(arguments, name+"="+canonicalSemaType(typ))
	}
	if len(generic.Arguments) == 0 {
		generic = nil
	}
	parameters := make([]sema.FunctionParameter, 0, len(target.Parameters))
	var facts *TypePackFacts
	if target.GenericTypePack != "" {
		fact, ok := b.analyzer.ResolvedTypePackCallOf(call)
		if !ok {
			return nil, unsupported("heterogeneous type-pack callable expansion without a resolved sequence")
		}
		facts = &TypePackFacts{Parameter: fact.Name, FixedCount: fact.FixedCount}
		sequence := make([]string, len(fact.Types))
		for index, element := range fact.Types {
			element = concrete(element)
			if !concreteSemaType(element) {
				return nil, unsupported("type-pack specialization with an unresolved element")
			}
			typeID, err := b.internType(element)
			if err != nil {
				return nil, locateUnsupportedType(err, call.Token)
			}
			facts.Types = append(facts.Types, typeID)
			sequence[index] = canonicalSemaType(element)
		}
		arguments = append(arguments, "..."+fact.Name+"<"+strings.Join(sequence, ",")+">")
		for index, parameter := range fact.Parameters {
			parameter.Type = concrete(parameter.Type)
			if index >= fact.FixedCount {
				mode, prefix := TypePackShared, "ref "
				if parameter.Type.Kind != sema.ReferenceType {
					// Concrete no-op destruction is established before omitting
					// cleanup (rules/compiler/monomorphization.md §11.2,11.4).
					if !sema.TriviallyCopyable(parameter.Type) || !sema.TriviallyDestructible(parameter.Type) {
						return nil, unsupported("ownership-sensitive owned type-pack position")
					}
					mode, prefix = TypePackOwned, ""
				} else if parameter.Type.ReferenceMutable {
					mode, prefix = TypePackMutable, "ref mut "
				}
				facts.Positions = append(facts.Positions, TypePackPosition{Mode: mode, Location: location(parameter.Token)})
				parameter.Name = fmt.Sprintf("%s[%d]", parameter.Name, index-fact.FixedCount)
				// Preserve concrete type spelling and shared/exclusive authority.
				if mode != TypePackOwned {
					parameter.Type.Name = prefix + sequence[index-fact.FixedCount]
				}
			}
			parameters = append(parameters, parameter)
		}
	} else {
		for _, parameter := range target.Parameters {
			parameter.Type = concrete(parameter.Type)
			parameters = append(parameters, parameter)
		}
	}
	signature := make([]string, len(parameters))
	for index, parameter := range parameters {
		if !concreteSemaType(parameter.Type) {
			return nil, unsupported("generic specialization with an unresolved parameter type")
		}
		signature[index] = canonicalSemaType(parameter.Type)
	}
	id := FunctionID(fmt.Sprintf("%s::%s[%s](%s)", target.Module, target.Name, strings.Join(arguments, ","), strings.Join(signature, ",")))
	if existing, seen := b.specializations.demanded[id]; seen {
		return existing, nil
	}
	if len(b.specializations.demanded) >= maxSpecializations {
		// A resource limit, not proof of non-convergence (monomorphization.md
		// §§ 18.1, 18.7).
		return nil, unsupported(fmt.Sprintf("generic specialization resource limit of %d specializations", maxSpecializations))
	}
	function := target
	function.Parameters = parameters
	function.ReturnType = concrete(target.ReturnType)
	function.GenericTypePack = ""
	function.GenericSubstitution = nil
	if !concreteSemaType(function.ReturnType) {
		return nil, unsupported("generic specialization with an unresolved return type")
	}
	spec := &specialization{id: id, function: function, substitution: substitution, generic: generic, facts: facts, token: call.Token}
	if b.specializations.demanded == nil {
		b.specializations.demanded = map[FunctionID]*specialization{}
	}
	b.specializations.demanded[id] = spec
	b.specializations.pending = append(b.specializations.pending, spec)
	return spec, nil
}

// buildSpecializations builds demanded specializations in first-demand order.
// Building a body may demand more of them. Pack methods use ordinary explicit
// receiver parameters where needed, followed by their concrete pack positions.
// Rules: rules/compiler/semantic_ir.md §27A(2),(4);
// rules/declarations/functions.md §38A(9); rules/declarations/impl.md §§8,11,13.
func (b *builder) buildSpecializations() error {
	for len(b.specializations.pending) > 0 {
		spec := b.specializations.pending[0]
		b.specializations.pending = b.specializations.pending[1:]
		decl := b.specializations.templates[genericTemplateKey(spec.function)]
		if decl == nil || decl.Body == nil {
			return &UnsupportedFeatureError{Feature: "generic specialization of a template outside this module for " + spec.function.Name, Package: b.maxPackage, Location: location(spec.token)}
		}
		// Pack methods reuse the concrete pack and ordinary receiver producers
		// (rules/compiler/semantic_ir.md §§27A(4),60(3),61(4)).
		if spec.function.ImplTarget != "" && len(decl.GenericParameters) != 0 && spec.facts == nil {
			return &UnsupportedFeatureError{Feature: "generic method with generic parameters of its own for " + spec.function.Name, Package: b.maxPackage, Location: location(spec.token)}
		}
		if spec.facts != nil && len(decl.Parameters) != 0 {
			if name := decl.Parameters[len(decl.Parameters)-1].Name; name != nil {
				if fact, ok := b.analyzer.ResolvedBindingOf(name); ok {
					spec.binding = fact.ID
				}
			}
		}
		b.substitution = spec.substitution
		err := b.buildConcreteFunction(decl, spec.function, spec)
		b.substitution = nil
		if err != nil {
			return err
		}
	}
	return nil
}

// concreteTriviallyCopyable classifies a Sema plan type after the active
// specialization's substitution; a template plan computed for an unknown
// parameter is conservative and would otherwise reject a trivial instance.
func (b *builder) concreteTriviallyCopyable(typ sema.Type) bool {
	if len(b.substitution) != 0 {
		typ = b.substituteTemplateType(typ, b.substitution)
	}
	return concreteSemaType(typ) && sema.TriviallyCopyable(typ)
}

// concreteTriviallyDestructible is concreteTriviallyCopyable's destruction
// counterpart.
func (b *builder) concreteTriviallyDestructible(typ sema.Type) bool {
	if len(b.substitution) != 0 {
		typ = b.substituteTemplateType(typ, b.substitution)
	}
	return concreteSemaType(typ) && sema.TriviallyDestructible(typ)
}

func formatSpecializationFacts(facts *SpecializationFacts) string {
	arguments := make([]string, len(facts.Arguments))
	for index, argument := range facts.Arguments {
		arguments[index] = fmt.Sprintf("%s=!%d", argument.Name, argument.Type)
	}
	return fmt.Sprintf(" specialization(%s: %s)", facts.Template, strings.Join(arguments, ", "))
}

// verifySpecializationFacts checks that a specialization names its template and
// each concrete argument once with a valid type.
func verifySpecializationFacts(module *Module, fn *Function) error {
	facts := fn.Specialization
	if facts == nil {
		return nil
	}
	if facts.Template == "" || len(facts.Arguments) == 0 {
		return fmt.Errorf("function %s has incomplete specialization facts", fn.ID)
	}
	seen := map[string]bool{}
	for _, argument := range facts.Arguments {
		if _, ok := module.Types.Lookup(argument.Type); argument.Name == "" || seen[argument.Name] || !ok {
			return fmt.Errorf("function %s has an invalid specialization argument %q", fn.ID, argument.Name)
		}
		seen[argument.Name] = true
	}
	return nil
}

// buildTypePackLength lowers `values.Len` of the pack being specialized to the
// exact length of this specialization (functions.md § 38A(5)(1–2)); the shared
// template AST holds no per-specialization value.
func (fb *functionBuilder) buildTypePackLength(member *ast.MemberExpression) (builtValue, bool, error) {
	if fb.specialization == nil || fb.specialization.facts == nil || member.Property == nil || member.Property.Value != "Len" {
		return builtValue{}, false, nil
	}
	receiver, ok := member.Object.(*ast.Identifier)
	if !ok {
		return builtValue{}, false, nil
	}
	fact, ok := fb.owner.analyzer.ResolvedBindingOf(receiver)
	if !ok || fact.ID != fb.specialization.binding {
		return builtValue{}, false, nil
	}
	typeID, err := fb.owner.internType(sema.Type{Name: "uint", Kind: sema.UintType})
	if err != nil {
		return builtValue{}, true, err
	}
	length := new(big.Int).SetUint64(uint64(len(fb.specialization.facts.Types)))
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
// the sequence, positions and parameters agree in length and authority.
// Rules: rules/compiler/semantic_ir.md §27A(1–2);
// rules/declarations/functions.md §38A(3)(2–5),(9).
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
		mode := facts.Positions[index].Mode
		if mode == TypePackOwned {
			if !ok || value.Type != element || (value.Ownership != OwnershipImmediate && value.Ownership != OwnershipRawPointer) || !package14TrivialValue(module, element) || !trivialDestruction(module, element) || fn.Parameters[facts.FixedCount+index].Reference != nil {
				return fmt.Errorf("function %s type-pack position %d does not receive a trivial owned value", fn.ID, index)
			}
			continue
		}
		ownership := OwnershipSharedReference
		if mode == TypePackMutable {
			ownership = OwnershipMutableReference
		}
		if (mode != TypePackShared && mode != TypePackMutable) || !ok || received.Kind != TypeReference || received.Element != element || received.Mutable != (mode == TypePackMutable) || value.Ownership != ownership {
			return fmt.Errorf("function %s type-pack position %d does not receive a %s reference to its element type", fn.ID, index, mode)
		}
	}
	return nil
}
