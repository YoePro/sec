package semantic

import (
	"errors"
	"fmt"
	"strings"

	"sec/internal/lexer"
	"sec/internal/sema"
)

// semanticIdentity retains declaration provenance and complete concrete source
// arguments. It never infers source identity from fields or physical layout.
// Rules: rules/types/types.md — Type identity, Generic and parameterized types;
// rules/compiler/semantic_ir.md — nominal and generic type identity.
func (b *builder) semanticIdentity(t sema.Type) (string, string) {
	module := t.Module
	if module == "" {
		if t.Intrinsic {
			module = "core"
		} else {
			module = b.module.Identity
		}
	}
	identity := module + "::" + t.Name
	if len(t.TypeArgs) != 0 && !t.FixedNamedArguments {
		parts := make([]string, len(t.TypeArgs))
		for index, argument := range t.TypeArgs {
			parts[index] = canonicalSemaType(argument)
		}
		identity += "<" + strings.Join(parts, ",") + ">"
	}
	return module, identity
}

// canonicalSemaType recursively retains nominal argument identities, including
// nested specializations and constant shape parameters. Fixed carrier arguments
// on a nongeneric named declaration do not create source generic parameters.
// Rules: rules/types/types.md — Type identity, Named types, Generic and parameterized types.
func canonicalSemaType(t sema.Type) string {
	name := t.Name
	if t.Module != "" && (t.Named || t.Declared) {
		name = t.Module + "::" + name
	}
	if t.FixedNamedArguments {
		return name
	}
	if len(t.TypeArgs) > 0 || len(t.ConstArgs) > 0 {
		args := make([]string, 0, len(t.TypeArgs)+len(t.ConstArgs))
		for _, arg := range t.TypeArgs {
			args = append(args, canonicalSemaType(arg))
		}
		for _, arg := range t.ConstArgs {
			args = append(args, fmt.Sprint(arg))
		}
		return name + "<" + strings.Join(args, ",") + ">"
	}
	return name
}

// locateUnsupportedType retains the signature's source location when a type
// family has no Semantic IR representation, without changing source validity.
// Rules: rules/compiler/semantic_ir.md — §90 Source locations;
// rules/compiler/compiler_pipeline.md — unsupported lowering diagnostics.
func locateUnsupportedType(err error, token lexer.Token) error {
	var unsupported *UnsupportedFeatureError
	if errors.As(err, &unsupported) && unsupported.Location.Line == 0 {
		copy := *unsupported
		copy.Location = location(token)
		return &copy
	}
	return err
}
