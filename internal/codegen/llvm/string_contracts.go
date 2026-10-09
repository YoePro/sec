package llvm

import (
	"math/big"
	"strings"

	"sec/internal/ast"
	astwalk "sec/internal/ast/walk"
	"sec/internal/lexer"
	"sec/internal/sema"
	"sec/internal/sema/stringcontract"
)

const stringResultType = "%sec.string_conversion_result"

type stringConversion struct {
	plan    stringcontract.Plan
	runtime bool
}
type stringFacts struct {
	contracts   map[ast.Contract]bool
	conversions map[*ast.CallExpression]stringConversion
	kinds       map[string]string
	errorRefs   map[*ast.TypeReference]bool
}

// resolvedStringFacts snapshots analyzed string conjunctions, public ContractKind
// values and conversion checks. Raw/stale ASTs cannot manufacture these facts.
// Rules: rules/types/contracts.md — Composition, Conversion failure layers,
// Public conversion and contract errors; MD-012 §§3–5.
func resolvedStringFacts(program *ast.Program, a *sema.Analyzer) (*stringFacts, error) {
	f := &stringFacts{errorRefs: map[*ast.TypeReference]bool{}, contracts: map[ast.Contract]bool{}, conversions: map[*ast.CallExpression]stringConversion{}, kinds: map[string]string{}}
	types := a.Types()
	if kind, ok := types["ContractKind"]; ok {
		for _, name := range kind.EnumValues {
			if value, exists := kind.EnumConsts[name]; exists && value.Value != nil && program.SourceProvenance[value.Token.File] == ast.SourceCore {
				f.kinds[name] = value.Value.String()
			}
		}
	}
	err := astwalk.Inspect(program, func(node any) error {
		switch n := node.(type) {
		case *ast.TypeReference:
			if n.Name == "Result" && len(n.TypeArgs) == 2 && n.TypeArgs[1].Name == "ConversionError" {
				f.errorRefs[n.TypeArgs[1]] = true
			}
		case *ast.TypeDeclStatement:
			typ, ok := types[n.Name.Value]
			if !ok {
				return nil
			}
			if _, supported := sema.StringLengthContractPlan(typ); supported {
				if n.Contract != nil && !a.ResolvedContractNode(n.Contract) {
					return unsupportedContract("missing exact analyzed string contract declaration", n.Token)
				}
				if list, ok := n.Contract.(*ast.ContractList); ok {
					f.contracts[list] = true
					for _, c := range list.Contracts {
						f.contracts[c] = true
					}
				} else if n.Contract != nil {
					f.contracts[n.Contract] = true
				}
			}
		case *ast.CallExpression:
			if target, found := types[callExpressionName(n)]; found {
				if _, supported := sema.StringLengthContractPlan(target); supported {
					if _, resolved := a.ResolvedTypeOf(n); !resolved {
						return unsupportedContract("missing exact analyzed string conversion", n.Token)
					}
				}
			}
			plan, runtime, supported := a.ResolvedStringContractConversionOf(n)
			if supported {
				f.conversions[n] = stringConversion{plan: plan, runtime: runtime}
			}
		}
		return nil
	})
	return f, err
}

// emitStringContractConversion evaluates its operand once and constructs no
// constrained value until all source-ordered requirements succeed. A bodyless
// try propagates the exact ConversionError.Contract payload on the failure edge.
// Rules: rules/types/contracts.md — String and collection contracts, Conversion
// failure layers; rules/errors/errorhandling.md — bodyless try propagation.
func (g *Generator) emitStringContractConversion(call *ast.CallExpression, fact stringConversion) (value, error) {
	operand, err := g.emitExpression(call.Arguments[0])
	if err != nil {
		return value{}, err
	}
	if operand.typ != "string" {
		return value{}, unsupportedContract("string contract source representation", call.Token)
	}
	if !fact.runtime {
		if literal, ok := call.Arguments[0].(*ast.StringLiteral); ok {
			if _, valid := fact.plan.Validate(literal.Value); !valid {
				return value{}, unsupportedContract("stale compile-time string contract proof", call.Token)
			}
		}
		return operand, nil
	}
	if !g.stringTry || g.returnType != stringResultType {
		return value{}, unsupportedContract("string validation requires bodyless try with Result[string-like, ConversionError]", call.Token)
	}
	var runeLength string
	for _, requirement := range fact.plan.Requirements() {
		byteUnit := strings.Contains(requirement.Kind, "ByteLen")
		count := operand.lenRef
		if !byteUnit {
			if runeLength == "" {
				runeLength = g.emitStringRuneLength(operand)
			}
			count = runeLength
		}
		name := strings.Replace(requirement.Kind, "ByteLen", "Len", 1)
		bound := requirement.Bound
		predicate := "eq"
		switch name {
		case "minLen":
			predicate = "uge"
		case "maxLen":
			predicate = "ule"
		case "notEmpty":
			predicate = "ugt"
			bound = "0"
		}
		valid := g.nextTemp()
		parsed, ok := new(big.Int).SetString(bound, 10)
		if !ok {
			return value{}, unsupportedContract("unresolved string length bound", call.Token)
		}
		if parsed.BitLen() > 64 {
			constant := "false"
			if predicate == "ule" {
				constant = "true"
			}
			g.write("  %s = or i1 %s, false\n", valid, constant)
		} else {
			g.write("  %s = icmp %s i64 %s, %s\n", valid, predicate, count, bound)
		}
		success, failure := g.nextLabel("string.valid"), g.nextLabel("string.invalid")
		g.write("  br i1 %s, label %%%s, label %%%s\n", valid, success, failure)
		g.write("\n%s:\n", failure)
		kindName := strings.ToUpper(requirement.Kind[:1]) + requirement.Kind[1:]
		kind, known := g.stringFacts.kinds[kindName]
		if !known {
			return value{}, unsupportedContract("missing canonical ContractKind."+kindName, call.Token)
		}
		width := g.nativeIntegerType()
		g.write("  ret %s { i1 true, ptr null, i64 0, %s %s, %s %d }\n", stringResultType, width, kind, width, requirement.DeclarationIndex)
		g.write("\n%s:\n", success)
	}
	return operand, nil
}

// emitStringRuneLength scans an immutable valid UTF-8 string without allocation;
// continuation bytes never introduce a scalar. Embedded NUL is ordinary data.
// Rules: rules/compiler/compiler_known_members.md — Len, RuneLen, ByteLen on strings;
// rules/types/contracts.md — String and collection contracts (revision 2.1).
func (g *Generator) emitStringRuneLength(operand value) string {
	g.needsRuneLength = true
	result := g.nextTemp()
	g.write("  %s = call i64 @.sec.generated.string_rune_length(ptr %s, i64 %s)\n", result, operand.ref, operand.lenRef)
	return result
}

const runeLengthLLVM = `
; sec-generated string rune-length helper; rules/types/contracts.md — string length contracts
define private i64 @.sec.generated.string_rune_length(ptr %data, i64 %length) {
entry:
  br label %scan
scan:
  %index = phi i64 [ 0, %entry ], [ %next, %byte ]
  %count = phi i64 [ 0, %entry ], [ %updated, %byte ]
  %done = icmp eq i64 %index, %length
  br i1 %done, label %finish, label %byte
byte:
  %address = getelementptr i8, ptr %data, i64 %index
  %value = load i8, ptr %address
  %prefix = and i8 %value, -64
  %leading = icmp ne i8 %prefix, -128
  %increment = zext i1 %leading to i64
  %updated = add i64 %count, %increment
  %next = add i64 %index, 1
  br label %scan
finish:
  ret i64 %count
}
`

// emitStringResultOk packs the validated string without changing ownership or
// fabricating an error value. The compact payload describes only the supported
// ConversionError.Contract branch; other constructors remain explicit boundaries.
// Rules: rules/types/contracts.md — Public conversion and contract errors;
// rules/errors/errorhandling.md — Result success construction.
func (g *Generator) emitStringResultOk(expr *ast.OkExpression) (value, error) {
	operand, err := g.emitExpression(expr.Value)
	if err != nil {
		return value{}, err
	}
	if operand.typ != "string" {
		return value{}, unsupportedContract("string Result success representation", expr.Token)
	}
	first, second := g.nextTemp(), g.nextTemp()
	g.write("  %s = insertvalue %s zeroinitializer, ptr %s, 1\n", first, stringResultType, operand.ref)
	g.write("  %s = insertvalue %s %s, i64 %s, 2\n", second, stringResultType, first, operand.lenRef)
	return value{typ: stringResultType, ref: second}, nil
}

// emitStringTry propagates the complete supported string conversion result.
// Handled/mixed checks require complete handler dispatch and remain unsupported.
// Rules: rules/errors/errorhandling.md — compiler-internal failure sets.
func (g *Generator) emitStringTry(expr *ast.TryExpression) (value, error) {
	if len(expr.Handlers) > 0 {
		return value{}, unsupportedContract("handled string contract validation", expr.Token)
	}
	previous := g.stringTry
	g.stringTry = true
	defer func() { g.stringTry = previous }()
	operand, err := g.emitExpression(expr.Expression)
	if err != nil || operand.typ != stringResultType {
		return operand, err
	}
	if g.returnType != stringResultType {
		return value{}, unsupportedContract("string Result propagation channel", expr.Token)
	}
	failed := g.nextTemp()
	g.write("  %s = extractvalue %s %s, 0\n", failed, stringResultType, operand.ref)
	success, failure := g.nextLabel("string.result.ok"), g.nextLabel("string.result.err")
	g.write("  br i1 %s, label %%%s, label %%%s\n", failed, failure, success)
	g.write("\n%s:\n  ret %s %s\n\n%s:\n", failure, stringResultType, operand.ref, success)
	pointer, length := g.nextTemp(), g.nextTemp()
	g.write("  %s = extractvalue %s %s, 1\n", pointer, stringResultType, operand.ref)
	g.write("  %s = extractvalue %s %s, 2\n", length, stringResultType, operand.ref)
	return value{typ: "string", ref: pointer, lenRef: length}, nil
}

// stringContractASTGate consumes only exact analyzed declarations; all other
// contract ASTs retain the fail-closed raw boundary.
// Rules: rules/types/contracts.md — Core rule and Composition.
func (g *Generator) stringContractASTGate(program *ast.Program) error {
	return astwalk.Inspect(program, func(node any) error {
		if c, ok := node.(ast.Contract); ok && (g.stringFacts == nil || !g.stringFacts.contracts[c]) {
			return unsupportedContract("type contract validation", llvmSourceToken(node))
		}
		return nil
	})
}

// stringTypeReference preserves source string identity through named aliases;
// pointer layout alone cannot classify a string.
// Rules: rules/types/types.md — Named types; contracts.md — Core rule.
func (g *Generator) stringTypeReference(ref *ast.TypeReference) bool {
	seen := map[string]bool{}
	for ref != nil && !seen[ref.Name] {
		if ref.Name == "string" {
			return true
		}
		seen[ref.Name] = true
		ref = g.typeAliases[ref.Name]
	}
	return false
}

// stringReturnType transports the complete descriptor for string-returning calls
// in the analyzed length-validation path, preserving the once-evaluated operand.
// Rules: rules/types/types.md — string; contracts.md — Conversion failure layers.
func (g *Generator) stringReturnType(ref *ast.TypeReference) string {
	if g.stringFacts != nil && len(g.stringFacts.conversions) > 0 && g.stringTypeReference(ref) {
		g.needsStringDescriptor = true
		return "%sec.string"
	}
	return g.llvmType(ref)
}

// packStringDescriptor retains both pointer and encoded byte length; no
// terminator search or hidden allocation may replace the source representation.
// Rules: rules/types/types.md — string; compiler_known_members.md — ByteLen.
func (g *Generator) packStringDescriptor(text value) (value, error) {
	if text.typ != "string" {
		return value{}, unsupportedContract("string descriptor return representation", lexer.Token{})
	}
	first, second := g.nextTemp(), g.nextTemp()
	g.write("  %s = insertvalue %%sec.string zeroinitializer, ptr %s, 0\n", first, text.ref)
	g.write("  %s = insertvalue %%sec.string %s, i64 %s, 1\n", second, first, text.lenRef)
	return value{typ: "%sec.string", ref: second}, nil
}
